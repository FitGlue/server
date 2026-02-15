/**
 * repost-handler
 *
 * Handles re-post mechanisms for synchronized activities:
 * - POST /api/repost/missed-destination - Send to new destination
 * - POST /api/repost/retry-destination  - Retry existing destination
 * - POST /api/repost/full-pipeline      - Full pipeline re-execution
 */

// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, FrameworkHandler } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RoutableRequest } from '@fitglue/shared/routing';
import { getEffectiveTier } from '@fitglue/shared/domain';
import { parseDestination, getDestinationName } from '@fitglue/shared/types';
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

/**
 * Fetch enriched event from GCS using URI.
 * @param gcsUri - GCS URI in format gs://bucket/path
 * @returns Parsed enriched event object or null if fetch failed
 */
async function fetchEnrichedEventFromGCS(gcsUri: string): Promise<Record<string, unknown> | null> {
  const uriMatch = gcsUri.match(/^gs:\/\/([^/]+)\/(.+)$/);
  if (!uriMatch) {
    return null;
  }

  const [, bucket, objectPath] = uriMatch;
  try {
    const [buffer] = await storage.bucket(bucket).file(objectPath).download();
    return JSON.parse(buffer.toString('utf-8')) as Record<string, unknown>;
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
  return await routeRequest(req as RoutableRequest, ctx, [
    {
      method: 'POST',
      pattern: '*/missed-destination',
      handler: async () => handleMissedDestination(req, ctx, userId),
    },
    {
      method: 'POST',
      pattern: '*/retry-destination',
      handler: async () => handleRetryDestination(req, ctx, userId),
    },
    {
      method: 'POST',
      pattern: '*/full-pipeline',
      handler: async () => handleFullPipeline(req, ctx, userId),
    },
  ]);
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

  // Validate destination enum (parseDestination returns DESTINATION_UNSPECIFIED for unknown inputs)
  const destEnum = parseDestination(destination);
  if (!destEnum) {
    throw new HttpError(400, `Invalid destination: ${destination}`);
  }

  // Get pipeline run for this activity
  const pipelineRun = await ctx.stores.pipelineRuns.findByActivityId(userId, activityId);
  if (!pipelineRun) {
    throw new HttpError(404, 'Activity not found');
  }

  // Check destination isn't already synced
  const destKey = getDestinationName(destEnum);
  const existingDest = pipelineRun.destinations?.find(d => d.destination === destEnum);
  if (existingDest?.externalId) {
    throw new HttpError(400, `Activity already synced to ${destKey}`);
  }

  // Try to get enriched event from GCS via PipelineRun
  let enrichedEvent: Record<string, unknown> | null = null;

  if (pipelineRun.enrichedEventUri) {
    // Fetch from GCS using URI (avoids 1MB Firestore limit)
    enrichedEvent = await fetchEnrichedEventFromGCS(pipelineRun.enrichedEventUri);
    if (enrichedEvent) {
      ctx.logger.info('Retrieved enriched event from GCS', { uri: pipelineRun.enrichedEventUri });
    } else {
      ctx.logger.warn('Failed to fetch enriched event from GCS', { uri: pipelineRun.enrichedEventUri });
    }
  }

  // Fallback to executions collection (backwards compatibility)
  if (!enrichedEvent) {
    const routerExec = await ctx.stores.executions.getRouterExecution(pipelineRun.id);
    if (routerExec?.data.inputsJson) {
      enrichedEvent = parseEnrichedActivityEvent(routerExec.data.inputsJson);
      ctx.logger.info('Retrieved enriched event from executions (fallback)', { pipelineExecutionId: pipelineRun.id });
    }
  }

  if (!enrichedEvent) {
    throw new HttpError(500, 'Unable to retrieve original activity data from execution logs');
  }

  // Generate new execution ID
  const newPipelineExecutionId = generateRepostExecutionId(activityId);

  // CRITICAL: Explicitly remove fields to prevent duplicate field errors in Go (camelCase vs snake_case)
  // protojson treats both casings as the same proto field, so having both causes "duplicate field" unmarshal errors
  const {
    destinations: _originalDests,
    Destinations: _originalDestsAlt,
    pipeline_execution_id: _peid,
    pipelineExecutionId: _peidAlt,
    activity_id: _aid,
    activityId: _aidAlt,
    user_id: _uid,
    userId: _uidAlt,
    enrichment_metadata: _em,
    enrichmentMetadata: _emAlt,
    ...eventWithoutConflictingFields
  } = enrichedEvent as Record<string, unknown>;

  // Merge both casing variants of enrichment_metadata into a single canonical snake_case field
  const mergedMetadata: Record<string, string> = {
    ...(_emAlt as Record<string, string> || {}),
    ...(_em as Record<string, string> || {}),
  };

  // Inject user's default config for the missed destination.
  // Without this, destinations like Google Sheets would fail with "spreadsheet_id not configured"
  // because the original pipeline didn't include this destination.
  try {
    const destRegistryId = destKey.toLowerCase().replace(/\s+/g, '');
    const pluginDefault = await ctx.services.user.pluginDefaultsStore.get(userId, destRegistryId);
    if (pluginDefault?.config && Object.keys(pluginDefault.config).length > 0) {
      for (const [k, v] of Object.entries(pluginDefault.config)) {
        mergedMetadata[`${destRegistryId}_${k}`] = String(v);
      }
      ctx.logger.info('Injected plugin defaults for missed destination', { destination: destRegistryId });
    }
  } catch (err) {
    // Best-effort: don't block repost if defaults lookup fails
    ctx.logger.warn('Failed to fetch plugin defaults for missed destination (best-effort)', { error: String(err) });
  }

  // Update the event with ONLY the new destination (snake_case for proto JSON)
  // Use the ORIGINAL pipeline run ID so uploaders update the correct PipelineRun document
  const repostData: Record<string, unknown> = {
    ...eventWithoutConflictingFields,
    user_id: _uid || _uidAlt,
    activity_id: _aid || _aidAlt,
    destinations: [destEnum],  // ONLY the missed destination
    pipeline_execution_id: pipelineRun.id,
    enrichment_metadata: mergedMetadata,
  };

  ctx.logger.info('Constructed repost data', {
    originalDestinations: enrichedEvent.destinations,
    newDestinations: [destEnum],
    activityId,
  });

  // Wrap in CloudEvent envelope matching Go enricher format
  const cloudEvent = createCloudEvent(repostData);
  const messageData = Buffer.from(JSON.stringify(cloudEvent));
  await pubsub.topic('topic-enriched-activity').publishMessage({
    data: messageData,
    attributes: {
      pipeline_execution_id: newPipelineExecutionId,
      repost_type: 'missed_destination',
      original_execution_id: pipelineRun.id,
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

  // Validate destination enum (parseDestination returns DESTINATION_UNSPECIFIED for unknown inputs)
  const destEnum = parseDestination(destination);
  if (!destEnum) {
    throw new HttpError(400, `Invalid destination: ${destination}`);
  }

  // Get pipeline run for this activity
  const pipelineRun = await ctx.stores.pipelineRuns.findByActivityId(userId, activityId);
  if (!pipelineRun) {
    throw new HttpError(404, 'Activity not found');
  }

  // For retry, destination should exist in the original sync
  // (but we allow retry even if status was success - user might want to update)
  const destKey = getDestinationName(destEnum);

  // Try to get enriched event from GCS via PipelineRun
  let enrichedEvent: Record<string, unknown> | null = null;

  if (pipelineRun.enrichedEventUri) {
    // Fetch from GCS using URI (avoids 1MB Firestore limit)
    enrichedEvent = await fetchEnrichedEventFromGCS(pipelineRun.enrichedEventUri);
    if (enrichedEvent) {
      ctx.logger.info('Retrieved enriched event from GCS', { uri: pipelineRun.enrichedEventUri });
    } else {
      ctx.logger.warn('Failed to fetch enriched event from GCS', { uri: pipelineRun.enrichedEventUri });
    }
  }

  // Fallback to executions collection (backwards compatibility)
  if (!enrichedEvent) {
    const routerExec = await ctx.stores.executions.getRouterExecution(pipelineRun.id);
    if (routerExec?.data.inputsJson) {
      enrichedEvent = parseEnrichedActivityEvent(routerExec.data.inputsJson);
      ctx.logger.info('Retrieved enriched event from executions (fallback)', { pipelineExecutionId: pipelineRun.id });
    }
  }

  if (!enrichedEvent) {
    throw new HttpError(500, 'Unable to retrieve original activity data from execution logs');
  }

  // Generate new execution ID
  const newPipelineExecutionId = generateRepostExecutionId(activityId);

  // Check if destination already has an external ID (use update method)
  const existingDest = pipelineRun.destinations?.find(d => d.destination === destEnum);
  const hasExistingId = !!existingDest?.externalId;

  // CRITICAL: Explicitly remove fields to prevent duplicate field errors in Go (camelCase vs snake_case)
  // protojson treats both casings as the same proto field, so having both causes "duplicate field" unmarshal errors
  const {
    destinations: _originalDests,
    Destinations: _originalDestsAlt,
    pipeline_execution_id: _peid,
    pipelineExecutionId: _peidAlt,
    activity_id: _aid,
    activityId: _aidAlt,
    user_id: _uid,
    userId: _uidAlt,
    enrichment_metadata: _em,
    enrichmentMetadata: _emAlt,
    ...eventWithoutConflictingFields
  } = enrichedEvent as Record<string, unknown>;

  // Merge both casing variants into a single canonical snake_case field
  const mergedMetadata: Record<string, string> = {
    ...(_emAlt as Record<string, string> || {}),
    ...(_em as Record<string, string> || {}),
  };

  // Update the event with ONLY the retry destination (snake_case for proto JSON)
  // Use the ORIGINAL pipeline run ID so uploaders update the correct PipelineRun document
  const repostData: Record<string, unknown> = {
    ...eventWithoutConflictingFields,
    user_id: _uid || _uidAlt,
    activity_id: _aid || _aidAlt,
    destinations: [destEnum],  // ONLY the retry destination
    pipeline_execution_id: pipelineRun.id,
    enrichment_metadata: mergedMetadata,
  };

  // Wrap in CloudEvent envelope matching Go enricher format
  const cloudEvent = createCloudEvent(repostData);
  const messageData = Buffer.from(JSON.stringify(cloudEvent));
  await pubsub.topic('topic-enriched-activity').publishMessage({
    data: messageData,
    attributes: {
      pipeline_execution_id: newPipelineExecutionId,
      repost_type: 'retry_destination',
      original_execution_id: pipelineRun.id,
      use_update_method: hasExistingId ? 'true' : 'false',
      existing_external_id: existingDest?.externalId || '',
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
 * 
 * Supports re-running even when all destinations failed (activity not synced).
 */
async function handleFullPipeline(req: { body: RepostRequest }, ctx: FrameworkContext, userId: string): Promise<RepostResponse> {
  const { activityId } = req.body;

  if (!activityId) {
    throw new HttpError(400, 'activityId is required');
  }

  // Find pipeline run by activity ID
  const pipelineRun = await ctx.stores.pipelineRuns.findByActivityId(userId, activityId);
  if (!pipelineRun) {
    throw new HttpError(404, 'Activity not found');
  }

  const pipelineId = pipelineRun.pipelineId;
  const pipelineExecutionId = pipelineRun.id;
  ctx.logger.info('Found pipeline run', { activityId, pipelineId, pipelineExecutionId });

  // Try to get original payload from GCS via PipelineRun
  let originalPayload: Record<string, unknown> | null = null;

  if (pipelineRun.originalPayloadUri) {
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
    const enricherExec = await ctx.stores.executions.getEnricherExecution(pipelineExecutionId);
    if (enricherExec?.data.inputsJson) {
      try {
        originalPayload = JSON.parse(enricherExec.data.inputsJson);
        ctx.logger.info('Retrieved original payload from executions (fallback)', { pipelineExecutionId });
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
  // protojson treats both casings as the same proto field, so having both causes "duplicate field" unmarshal errors
  const {
    pipeline_execution_id: _peid,
    pipelineExecutionId: _peidAlt,
    activity_id: _aid,
    activityId: _aidAlt,
    user_id: _uid,
    userId: _uidAlt,
    pipeline_id: _pid,
    pipelineId: _pidAlt,
    enrichment_metadata: _em,
    enrichmentMetadata: _emAlt,
    ...rest
  } = originalPayload;

  // Merge both casing variants into a single canonical snake_case field
  const mergedMetadata: Record<string, string> = {
    ...(_emAlt as Record<string, string> || {}),
    ...(_em as Record<string, string> || {}),
  };

  // Add bypass_dedup flag, pipelineId, and new execution ID
  // Publish directly to PIPELINE_ACTIVITY (bypasses splitter since pipelineId is set)
  const repostPayload = {
    ...rest,
    user_id: _uid || _uidAlt,
    activity_id: _aid || _aidAlt,
    pipeline_id: pipelineId, // USE THE ORIGINAL PIPELINE
    bypass_dedup: true,
    pipeline_execution_id: newPipelineExecutionId,
    enrichment_metadata: mergedMetadata,
  };

  // Wrap in CloudEvent envelope matching Go enricher format
  const cloudEvent = createCloudEvent(repostPayload, 'com.fitglue.activity.pipeline');
  const messageData = Buffer.from(JSON.stringify(cloudEvent));
  await pubsub.topic('topic-pipeline-activity').publishMessage({
    data: messageData,
    attributes: {
      pipeline_execution_id: newPipelineExecutionId,
      repost_type: 'full_pipeline',
      original_execution_id: pipelineExecutionId,
      bypass_dedup: 'true',
    },
  });

  ctx.logger.info('Published full pipeline re-execution (direct to enricher)', {
    activityId,
    pipelineId,
    newPipelineExecutionId,
    originalExecutionId: pipelineExecutionId,
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

