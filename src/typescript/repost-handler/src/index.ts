/**
 * repost-handler
 *
 * Handles re-post mechanisms for synchronized activities:
 * - POST /api/repost/missed-destination - Send to new destination
 * - POST /api/repost/retry-destination  - Retry existing destination
 * - POST /api/repost/full-pipeline      - Full pipeline re-execution
 */

import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, getEffectiveTier } from '@fitglue/shared';
import { TOPICS } from '@fitglue/shared/dist/config';
import { parseDestination, getDestinationName } from '@fitglue/shared/dist/types/events-helper';
import { Request, Response } from 'express';
import { PubSub } from '@google-cloud/pubsub';
import { v4 as uuidv4 } from 'uuid';

const pubsub = new PubSub();

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

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  // CORS headers
  res.set('Access-Control-Allow-Origin', '*');
  res.set('Access-Control-Allow-Methods', 'POST, OPTIONS');
  res.set('Access-Control-Allow-Headers', 'Content-Type, Authorization');

  if (req.method === 'OPTIONS') {
    res.status(204).send('');
    return;
  }

  if (req.method !== 'POST') {
    res.status(405).json({ error: 'Method Not Allowed' });
    return;
  }

  // Auth check
  if (!ctx.userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  // Check tier (Pro/Athlete only)
  try {
    const user = await ctx.stores.users.get(ctx.userId);
    if (!user) {
      res.status(401).json({ error: 'User not found' });
      return;
    }

    const effectiveTier = getEffectiveTier(user);
    const hasPro = effectiveTier === 'athlete' || user.isAdmin ||
      (user.trialEndsAt && user.trialEndsAt > new Date());

    if (!hasPro) {
      res.status(403).json({ error: 'Athlete tier required for re-post features' });
      return;
    }
  } catch (err) {
    ctx.logger.error('Failed to check user tier', { error: err });
    res.status(500).json({ error: 'Internal Server Error' });
    return;
  }

  // Route to appropriate handler
  const path = req.path;

  try {
    if (path.endsWith('/missed-destination')) {
      await handleMissedDestination(req, res, ctx);
    } else if (path.endsWith('/retry-destination')) {
      await handleRetryDestination(req, res, ctx);
    } else if (path.endsWith('/full-pipeline')) {
      await handleFullPipeline(req, res, ctx);
    } else {
      res.status(404).json({ error: 'Not Found' });
    }
  } catch (err) {
    ctx.logger.error('Re-post handler error', { error: err, path });
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

/**
 * POST /api/repost/missed-destination
 * Send activity to a new destination that wasn't in the original pipeline.
 */
async function handleMissedDestination(req: Request, res: Response, ctx: FrameworkContext): Promise<void> {
  const { activityId, destination } = req.body as RepostRequest;

  if (!activityId || !destination) {
    res.status(400).json({ error: 'activityId and destination are required' });
    return;
  }

  // Validate destination enum
  const destEnum = parseDestination(destination);
  if (destEnum === undefined) {
    res.status(400).json({ error: `Invalid destination: ${destination}` });
    return;
  }

  // Get synchronized activity
  const activity = await ctx.stores.activities.getSynchronized(ctx.userId!, activityId);
  if (!activity) {
    res.status(404).json({ error: 'Activity not found' });
    return;
  }

  // Check destination isn't already synced
  const destKey = getDestinationName(destEnum);
  if (activity.destinations && activity.destinations[destKey]) {
    res.status(400).json({
      error: `Activity already synced to ${destKey}`,
      existingExternalId: activity.destinations[destKey]
    });
    return;
  }

  // Get router execution to retrieve EnrichedActivityEvent
  const routerExec = await ctx.stores.executions.getRouterExecution(activity.pipelineExecutionId);
  if (!routerExec || !routerExec.data.inputsJson) {
    res.status(500).json({ error: 'Unable to retrieve original activity data from execution logs' });
    return;
  }

  const enrichedEvent = parseEnrichedActivityEvent(routerExec.data.inputsJson);
  if (!enrichedEvent) {
    res.status(500).json({ error: 'Failed to parse activity data from execution logs' });
    return;
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

  const response: RepostResponse = {
    success: true,
    message: `Activity queued for sync to ${destKey}`,
    newPipelineExecutionId,
    destination: destKey,
    promptUpdatePipeline: true, // Frontend should prompt to add to pipeline
  };

  res.status(200).json(response);
}

/**
 * POST /api/repost/retry-destination
 * Re-send activity to an existing destination.
 */
async function handleRetryDestination(req: Request, res: Response, ctx: FrameworkContext): Promise<void> {
  const { activityId, destination } = req.body as RepostRequest;

  if (!activityId || !destination) {
    res.status(400).json({ error: 'activityId and destination are required' });
    return;
  }

  // Validate destination enum
  const destEnum = parseDestination(destination);
  if (destEnum === undefined) {
    res.status(400).json({ error: `Invalid destination: ${destination}` });
    return;
  }

  // Get synchronized activity
  const activity = await ctx.stores.activities.getSynchronized(ctx.userId!, activityId);
  if (!activity) {
    res.status(404).json({ error: 'Activity not found' });
    return;
  }

  // For retry, destination should exist in the original sync
  // (but we allow retry even if status was success - user might want to update)
  const destKey = getDestinationName(destEnum);

  // Get router execution to retrieve EnrichedActivityEvent
  const routerExec = await ctx.stores.executions.getRouterExecution(activity.pipelineExecutionId);
  if (!routerExec || !routerExec.data.inputsJson) {
    res.status(500).json({ error: 'Unable to retrieve original activity data from execution logs' });
    return;
  }

  const enrichedEvent = parseEnrichedActivityEvent(routerExec.data.inputsJson);
  if (!enrichedEvent) {
    res.status(500).json({ error: 'Failed to parse activity data from execution logs' });
    return;
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

  const response: RepostResponse = {
    success: true,
    message: hasExistingId
      ? `Activity queued for update on ${destKey}`
      : `Activity queued for sync to ${destKey}`,
    newPipelineExecutionId,
    destination: destKey,
  };

  res.status(200).json(response);
}

/**
 * POST /api/repost/full-pipeline
 * Re-run the entire pipeline from the beginning with bypass_dedup.
 */
async function handleFullPipeline(req: Request, res: Response, ctx: FrameworkContext): Promise<void> {
  const { activityId } = req.body as RepostRequest;

  if (!activityId) {
    res.status(400).json({ error: 'activityId is required' });
    return;
  }

  // Get synchronized activity
  const activity = await ctx.stores.activities.getSynchronized(ctx.userId!, activityId);
  if (!activity) {
    res.status(404).json({ error: 'Activity not found' });
    return;
  }

  // Get enricher execution to retrieve original ActivityPayload/inputs
  const enricherExec = await ctx.stores.executions.getEnricherExecution(activity.pipelineExecutionId);
  if (!enricherExec || !enricherExec.data.inputsJson) {
    res.status(500).json({ error: 'Unable to retrieve original activity payload from execution logs' });
    return;
  }

  let originalPayload;
  try {
    originalPayload = JSON.parse(enricherExec.data.inputsJson);
  } catch {
    res.status(500).json({ error: 'Failed to parse original activity payload' });
    return;
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
    ...rest
  } = originalPayload;

  // Add bypass_dedup flag and new execution ID
  const repostPayload = {
    ...rest,
    user_id: _uid || _uidAlt,
    activity_id: _aid || _aidAlt,
    bypass_dedup: true,
    pipeline_execution_id: newPipelineExecutionId,
  };

  // Publish to raw activity topic (beginning of pipeline)
  // Wrap in CloudEvent envelope matching Go enricher format
  const cloudEvent = createCloudEvent(repostPayload, 'com.fitglue.activity.created');
  const messageData = Buffer.from(JSON.stringify(cloudEvent));
  await pubsub.topic(TOPICS.RAW_ACTIVITY).publishMessage({
    data: messageData,
    attributes: {
      pipeline_execution_id: newPipelineExecutionId,
      repost_type: 'full_pipeline',
      original_execution_id: activity.pipelineExecutionId,
      bypass_dedup: 'true',
    },
  });

  ctx.logger.info('Published full pipeline re-execution', {
    activityId,
    newPipelineExecutionId,
    originalExecutionId: activity.pipelineExecutionId,
  });

  const response: RepostResponse = {
    success: true,
    message: 'Activity queued for full pipeline re-execution. Note: This may create duplicate activities in destinations.',
    newPipelineExecutionId,
  };

  res.status(200).json(response);
}

export const repostHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  },
  skipExecutionLogging: true
});
