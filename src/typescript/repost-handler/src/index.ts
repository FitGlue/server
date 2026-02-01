/**
 * repost-handler
 *
 * Handles re-post mechanisms for synchronized activities:
 * - POST /api/repost/missed-destination - Send to new destination
 * - POST /api/repost/retry-destination  - Retry existing destination
 * - POST /api/repost/full-pipeline      - Full pipeline re-execution
 */

import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, getEffectiveTier, HttpError, FrameworkHandler } from '@fitglue/shared';
import { TOPICS } from '@fitglue/shared/dist/config';
import { parseDestination, getDestinationName } from '@fitglue/shared/dist/types/events-helper';
import { PubSub } from '@google-cloud/pubsub';
import { Storage } from '@google-cloud/storage';
import { v4 as uuidv4 } from 'uuid';

const pubsub = new PubSub();
const storage = new Storage();

/**
 * Create a CloudEvent envelope matching the Go enricher format.
 * The router expects: { specversion, id, source, type, data, datacontenttype }
 */
function createCloudEvent(data: Record<string, unknown>, type = 'com.fitglue.activity.enriched'): Record<string, unknown> {
  return {
    specversion: '1.0',
    id: uuidv4(),
    source: '/repost-handler',
    type,
    datacontenttype: 'application/json',
    data,
  };
}

// Response types
interface RepostResponse {
  success: boolean;
  message: string;
  newPipelineExecutionId?: string;
  destination?: string;
  promptUpdatePipeline?: boolean;
}

interface RepostRequest {
  activityId: string;
  destination?: string;
}

/**
 * Generate a fresh pipeline execution ID for re-posts.
 * Format: {timestamp}-{activityId}
 */
function generateRepostExecutionId(activityId: string): string {
  return `repost-${Date.now()}-${activityId}`;
}

/**
 * Parse and retrieve the original router inputsJson data.
 * Returns the ORIGINAL data object (snake_case or camelCase) without modification.
 * The caller is responsible for ensuring the correct format when publishing.
 */
function parseEnrichedActivityEvent(inputsJson: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(inputsJson);

    // Helper to check for activity identifier in either case
    const hasActivityId = (obj: Record<string, unknown>): boolean =>
      !!(obj.activityId || obj.activity_id);
    const hasUserId = (obj: Record<string, unknown>): boolean =>
      !!(obj.userId || obj.user_id);

    // The inputsJson might be the raw event or wrapped
    if (hasActivityId(parsed) && hasUserId(parsed)) {
      return parsed;
    }

    // Try unwrapping from data field (CloudEvent format)
    if (parsed.data && hasActivityId(parsed.data)) {
      return parsed.data;
    }

    return null;
  } catch {
    return null;
  }
}

export const handler: FrameworkHandler = async (req, ctx) => {
  // CORS handled by gateway

  if (req.method !== 'POST') {
    throw new HttpError(405, 'Method Not Allowed');
  }

  // Auth check
  if (!ctx.userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  // Store userId after auth check for type narrowing
  const userId = ctx.userId;

  // Check tier (Pro/Athlete only)
  try {
    const user = await ctx.stores.users.get(userId);
    if (!user) {
      throw new HttpError(401, 'User not found');
    }

    const effectiveTier = getEffectiveTier(user);
    const hasPro = effectiveTier === 'athlete' || user.isAdmin ||
      (user.trialEndsAt && user.trialEndsAt > new Date());

    if (!hasPro) {
      throw new HttpError(403, 'Athlete tier required for re-post features');
    }
  } catch (err) {
    if (err instanceof HttpError) throw err;
    ctx.logger.error('Failed to check user tier', { error: err });
    throw new HttpError(500, 'Internal Server Error');
  }

  // Route to appropriate handler
  const path = req.path;

  if (path.endsWith('/missed-destination')) {
    return await handleMissedDestination(req, ctx, userId);
  } else if (path.endsWith('/retry-destination')) {
    return await handleRetryDestination(req, ctx, userId);
  } else if (path.endsWith('/full-pipeline')) {
    return await handleFullPipeline(req, ctx, userId);
  } else {
    throw new HttpError(404, 'Not Found');
  }
};

/**
 * POST /api/repost/missed-destination
   * Send activity to a new destination that wasn't in the original pipeline.
   */
async function handleMissedDestination(req: { body: RepostRequest }, ctx: FrameworkContext, userId: string): Promise<RepostResponse> {
  const { activityId, destination } = req.body;

  if (!activityId || !destination) {
    throw new HttpError(400, 'activityId and destination are required');
  }

  // Validate destination enum
  const destEnum = parseDestination(destination);
  if (destEnum === undefined) {
    throw new HttpError(400, `Invalid destination: ${destination}`);
  }

  // Get synchronized activity
  const activity = await ctx.stores.activities.getSynchronized(userId, activityId);
  if (!activity) {
    throw new HttpError(404, 'Activity not found');
  }

  // Check destination isn't already synced
  const destKey = getDestinationName(destEnum);
  if (activity.destinations && activity.destinations[destKey]) {
    throw new HttpError(400, `Activity already synced to ${destKey}`);
  }

  // Try to get enriched event from PipelineRun first (new architecture)
  // Fall back to executions for backwards compatibility with existing data
  let enrichedEvent: Record<string, unknown> | null = null;

  const pipelineRun = await ctx.stores.pipelineRuns.get(userId, activity.pipelineExecutionId);
  if (pipelineRun?.enrichedEvent) {
    // Converter already parses JSON to object
    enrichedEvent = pipelineRun.enrichedEvent as unknown as Record<string, unknown>;
    ctx.logger.info('Retrieved enriched event from PipelineRun', { pipelineExecutionId: activity.pipelineExecutionId });
  }

  // Fallback to executions collection (backwards compatibility)
  if (!enrichedEvent) {
    const routerExec = await ctx.stores.executions.getRouterExecution(activity.pipelineExecutionId);
    if (routerExec?.data.inputsJson) {
      enrichedEvent = parseEnrichedActivityEvent(routerExec.data.inputsJson);
      ctx.logger.info('Retrieved enriched event from executions (fallback)', { pipelineExecutionId: activity.pipelineExecutionId });
    }
  }

  if (!enrichedEvent) {
    throw new HttpError(500, 'Unable to retrieve original activity data from execution logs');
  }

  // Generate new execution ID
  const newPipelineExecutionId = generateRepostExecutionId(activityId);

  // CRITICAL: Explicitly remove fields to prevent duplicate field errors in Go (camelCase vs snake_case)
  const {
    destinations: _originalDests,
    Destinations: _originalDestsAlt,
    pipeline_execution_id: _peid,
    pipelineExecutionId: _peidAlt,
    activity_id: _aid,
    activityId: _aidAlt,
    user_id: _uid,
    userId: _uidAlt,
    ...eventWithoutConflictingFields
  } = enrichedEvent as Record<string, unknown>;

  // Update the event with ONLY the new destination (snake_case for proto JSON)
  const repostData: Record<string, unknown> = {
    ...eventWithoutConflictingFields,
    user_id: _uid || _uidAlt,
    activity_id: _aid || _aidAlt,
    destinations: [destEnum],  // ONLY the missed destination
    pipeline_execution_id: newPipelineExecutionId,
  };

  ctx.logger.info('Constructed repost data', {
    originalDestinations: enrichedEvent.destinations,
    newDestinations: [destEnum],
    activityId,
  });

  // Wrap in CloudEvent envelope matching Go enricher format
  const cloudEvent = createCloudEvent(repostData);
  const messageData = Buffer.from(JSON.stringify(cloudEvent));
  await pubsub.topic(TOPICS.ENRICHED_ACTIVITY).publishMessage({
    data: messageData,
    attributes: {
      pipeline_execution_id: newPipelineExecutionId,
      repost_type: 'missed_destination',
      original_execution_id: activity.pipelineExecutionId,
    },
  });

  ctx.logger.info('Published re-post to router for missed destination', {
    activityId,
    destination: destKey,
    newPipelineExecutionId,
  });

  return {
    success: true,
    message: `Activity queued for sync to ${destKey}`,
    newPipelineExecutionId,
    destination: destKey,
    promptUpdatePipeline: true, // Frontend should prompt to add to pipeline
  };
}

/**
 * POST /api/repost/retry-destination
 * Re-send activity to an existing destination.
 */
async function handleRetryDestination(req: { body: RepostRequest }, ctx: FrameworkContext, userId: string): Promise<RepostResponse> {
  const { activityId, destination } = req.body;

  if (!activityId || !destination) {
    throw new HttpError(400, 'activityId and destination are required');
  }

  // Validate destination enum
  const destEnum = parseDestination(destination);
  if (destEnum === undefined) {
    throw new HttpError(400, `Invalid destination: ${destination}`);
  }

  // Get synchronized activity
  const activity = await ctx.stores.activities.getSynchronized(userId, activityId);
  if (!activity) {
    throw new HttpError(404, 'Activity not found');
  }

  // For retry, destination should exist in the original sync
  // (but we allow retry even if status was success - user might want to update)
  const destKey = getDestinationName(destEnum);

  // Try to get enriched event from PipelineRun first (new architecture)
  // Fall back to executions for backwards compatibility with existing data
  let enrichedEvent: Record<string, unknown> | null = null;

  const pipelineRun = await ctx.stores.pipelineRuns.get(userId, activity.pipelineExecutionId);
  if (pipelineRun?.enrichedEvent) {
    // Converter already parses JSON to object
    enrichedEvent = pipelineRun.enrichedEvent as unknown as Record<string, unknown>;
    ctx.logger.info('Retrieved enriched event from PipelineRun', { pipelineExecutionId: activity.pipelineExecutionId });
  }

  // Fallback to executions collection (backwards compatibility)
  if (!enrichedEvent) {
    const routerExec = await ctx.stores.executions.getRouterExecution(activity.pipelineExecutionId);
    if (routerExec?.data.inputsJson) {
      enrichedEvent = parseEnrichedActivityEvent(routerExec.data.inputsJson);
      ctx.logger.info('Retrieved enriched event from executions (fallback)', { pipelineExecutionId: activity.pipelineExecutionId });
    }
  }

  if (!enrichedEvent) {
    throw new HttpError(500, 'Unable to retrieve original activity data from execution logs');
  }

  // Generate new execution ID
  const newPipelineExecutionId = generateRepostExecutionId(activityId);

  // Check if destination already has an external ID (use update method)
  const hasExistingId = activity.destinations && activity.destinations[destKey];

  // CRITICAL: Explicitly remove fields to prevent duplicate field errors in Go (camelCase vs snake_case)
  const {
    destinations: _originalDests,
    Destinations: _originalDestsAlt,
    pipeline_execution_id: _peid,
    pipelineExecutionId: _peidAlt,
    activity_id: _aid,
    activityId: _aidAlt,
    user_id: _uid,
    userId: _uidAlt,
    ...eventWithoutConflictingFields
  } = enrichedEvent as Record<string, unknown>;

  // Update the event with ONLY the retry destination (snake_case for proto JSON)
  const repostData: Record<string, unknown> = {
    ...eventWithoutConflictingFields,
    user_id: _uid || _uidAlt,
    activity_id: _aid || _aidAlt,
    destinations: [destEnum],  // ONLY the retry destination
    pipeline_execution_id: newPipelineExecutionId,
  };

  // Wrap in CloudEvent envelope matching Go enricher format
  const cloudEvent = createCloudEvent(repostData);
  const messageData = Buffer.from(JSON.stringify(cloudEvent));
  await pubsub.topic(TOPICS.ENRICHED_ACTIVITY).publishMessage({
    data: messageData,
    attributes: {
      pipeline_execution_id: newPipelineExecutionId,
      repost_type: 'retry_destination',
      original_execution_id: activity.pipelineExecutionId,
      use_update_method: hasExistingId ? 'true' : 'false',
      existing_external_id: hasExistingId ? activity.destinations[destKey] : '',
    },
  });

  ctx.logger.info('Published re-post retry to router', {
    activityId,
    destination: destKey,
    newPipelineExecutionId,
    useUpdateMethod: !!hasExistingId,
  });

  return {
    success: true,
    message: hasExistingId
      ? `Activity queued for update on ${destKey}`
      : `Activity queued for sync to ${destKey}`,
    newPipelineExecutionId,
    destination: destKey,
  };
}

/**
 * POST /api/repost/full-pipeline
 * Re-run the entire pipeline from the beginning with bypass_dedup.
 * Now publishes directly to PIPELINE_ACTIVITY with pipelineId set.
 */
async function handleFullPipeline(req: { body: RepostRequest }, ctx: FrameworkContext, userId: string): Promise<RepostResponse> {
  const { activityId } = req.body;

  if (!activityId) {
    throw new HttpError(400, 'activityId is required');
  }

  // Get synchronized activity
  const activity = await ctx.stores.activities.getSynchronized(userId, activityId);
  if (!activity) {
    throw new HttpError(404, 'Activity not found');
  }

  // Verify the activity has a pipelineId (required for direct enricher publish)
  if (!activity.pipelineId) {
    throw new HttpError(500, 'Activity missing pipelineId - cannot re-run pipeline');
  }

  // Try to get original payload from GCS via PipelineRun (new architecture)
  // Fall back to executions for backwards compatibility with existing data
  let originalPayload: Record<string, unknown> | null = null;

  const pipelineRun = await ctx.stores.pipelineRuns.get(userId, activity.pipelineExecutionId);
  if (pipelineRun?.originalPayloadUri) {
    // Fetch from GCS using URI
    const uriMatch = pipelineRun.originalPayloadUri.match(/^gs:\/\/([^/]+)\/(.+)$/);
    if (uriMatch) {
      const [, bucket, objectPath] = uriMatch;
      try {
        const [payloadBuffer] = await storage.bucket(bucket).file(objectPath).download();
        originalPayload = JSON.parse(payloadBuffer.toString('utf-8')) as Record<string, unknown>;
        ctx.logger.info('Retrieved original payload from GCS', { uri: pipelineRun.originalPayloadUri });
      } catch (err) {
        ctx.logger.warn('Failed to fetch payload from GCS', { uri: pipelineRun.originalPayloadUri, error: err });
      }
    }
  }

  // Fallback to executions collection (backwards compatibility)
  if (!originalPayload) {
    const enricherExec = await ctx.stores.executions.getEnricherExecution(activity.pipelineExecutionId);
    if (enricherExec?.data.inputsJson) {
      try {
        originalPayload = JSON.parse(enricherExec.data.inputsJson);
        ctx.logger.info('Retrieved original payload from executions (fallback)', { pipelineExecutionId: activity.pipelineExecutionId });
      } catch {
        throw new HttpError(500, 'Failed to parse original activity payload');
      }
    }
  }

  if (!originalPayload) {
    throw new HttpError(500, 'Unable to retrieve original activity payload from execution logs');
  }

  // Generate new execution ID
  const newPipelineExecutionId = generateRepostExecutionId(activityId);

  // CRITICAL: Explicitly remove fields to prevent duplicate field errors in Go (camelCase vs snake_case)
  const {
    pipeline_execution_id: _peid,
    pipelineExecutionId: _peidAlt,
    activity_id: _aid,
    activityId: _aidAlt,
    user_id: _uid,
    userId: _uidAlt,
    pipeline_id: _pid,
    pipelineId: _pidAlt,
    ...rest
  } = originalPayload;

  // Add bypass_dedup flag, pipelineId, and new execution ID
  // Publish directly to PIPELINE_ACTIVITY (bypasses splitter since pipelineId is set)
  const repostPayload = {
    ...rest,
    user_id: _uid || _uidAlt,
    activity_id: _aid || _aidAlt,
    pipeline_id: activity.pipelineId, // USE THE ORIGINAL PIPELINE
    bypass_dedup: true,
    pipeline_execution_id: newPipelineExecutionId,
  };

  // Wrap in CloudEvent envelope matching Go enricher format
  const cloudEvent = createCloudEvent(repostPayload, 'com.fitglue.activity.pipeline');
  const messageData = Buffer.from(JSON.stringify(cloudEvent));
  await pubsub.topic(TOPICS.PIPELINE_ACTIVITY).publishMessage({
    data: messageData,
    attributes: {
      pipeline_execution_id: newPipelineExecutionId,
      repost_type: 'full_pipeline',
      original_execution_id: activity.pipelineExecutionId,
      bypass_dedup: 'true',
    },
  });

  ctx.logger.info('Published full pipeline re-execution (direct to enricher)', {
    activityId,
    pipelineId: activity.pipelineId,
    newPipelineExecutionId,
    originalExecutionId: activity.pipelineExecutionId,
  });

  return {
    success: true,
    message: 'Activity queued for full pipeline re-execution. Note: This may create duplicate activities in destinations.',
    newPipelineExecutionId,
  };
}

export const repostHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  },
  skipExecutionLogging: true
});

