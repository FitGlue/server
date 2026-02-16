/**
 * Mobile Sync Handler
 *
 * Cloud Function that receives health data from the FitGlue mobile app.
 * Accepts activities from iOS HealthKit and Android Health Connect,
 * stores them directly, and triggers async processing.
 */

// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch } from '@fitglue/shared/routing';
import { Request } from 'express';
import {
  MobileSyncRequest,
  MobileSyncResponse,
  mapMobileActivityType,
} from './types';

/**
 * Router for mobile endpoints
 */
export const handler: FrameworkHandler = async (req, ctx) => {
  const userId = ctx.userId;
  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  return await routeRequest(req, ctx, [
    {
      method: 'POST',
      pattern: '/api/mobile/sync',
      handler: async () => await handleSync(req, userId, ctx)
    },
    {
      method: 'POST',
      pattern: '/api/mobile/connect/:provider',
      handler: async (match: RouteMatch) => await handleConnect(userId, match.params.provider, ctx)
    },
  ]);
};

/**
 * Register a mobile health integration as connected.
 * Called by the mobile app after the user grants HealthKit / Health Connect permissions.
 */
async function handleConnect(userId: string, provider: string, ctx: FrameworkContext) {
  const { logger } = ctx;

  // Only accept mobile health providers
  const validProviders: Record<string, string> = {
    'apple-health': 'apple_health',
    'health-connect': 'health_connect',
  };

  const firestoreKey = validProviders[provider];
  if (!firestoreKey) {
    throw new HttpError(400, `Invalid mobile provider: ${provider}. Use 'apple-health' or 'health-connect'.`);
  }

  // Mark integration as connected in Firestore (raw update — not in typed UserIntegrations)
  await db.collection('users').doc(userId).update({
    [`integrations.${firestoreKey}.enabled`]: true,
    [`integrations.${firestoreKey}.last_used_at`]: new Date(),
  });

  logger.info('Mobile integration connected', { userId, provider: firestoreKey });
  return { message: `${provider} connected successfully` };
}

/**
 * Handle activity sync from mobile app
 */
async function handleSync(req: Request, userId: string, ctx: FrameworkContext): Promise<MobileSyncResponse> {
  const { logger, stores } = ctx;
  // Validate request body
  const syncRequest = req.body as MobileSyncRequest;

  // Validate request
  if (!syncRequest.activities || !Array.isArray(syncRequest.activities)) {
    throw new HttpError(400, 'Invalid request: activities array required');
  }

  logger.info('Mobile sync request received', {
    userId,
    activityCount: syncRequest.activities.length,
    platform: syncRequest.device?.platform,
  });

  // Check if user exists
  const user = await stores.users.get(userId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }

  const executionIds: string[] = [];
  let processedCount = 0;
  let skippedCount = 0;

  // Get reference to mobile_activities collection
  const mobileActivitiesRef = db.collection('mobile_activities');

  // Process each activity
  for (const activity of syncRequest.activities) {
    try {
      // Generate execution ID for this activity
      const pipelineExecutionId = `mobile-${Date.now()}-${Math.random().toString(36).substring(7)}`;

      // Create a minimal activity record
      const activityId = activity.externalId ||
        `${activity.source}-${new Date(activity.startTime).getTime()}`;

      // Store the raw activity for later processing
      const activityData = {
        userId,
        activityId,
        source: activity.source === 'healthkit' ? 'SOURCE_APPLE_HEALTH' : 'SOURCE_HEALTH_CONNECT',
        activityType: mapMobileActivityType(activity.activityName),
        name: activity.activityName,
        startTime: new Date(activity.startTime),
        endTime: new Date(activity.endTime),
        durationSeconds: activity.duration,
        calories: activity.calories,
        distanceMeters: activity.distance,
        heartRateSampleCount: activity.heartRateSamples?.length || 0,
        routePointCount: activity.route?.length || 0,
        createdAt: new Date(),
        pipelineExecutionId,
        status: 'pending',
      };

      // Store in mobile_activities collection
      await mobileActivitiesRef.doc(activityId).set(activityData, { merge: true });

      executionIds.push(pipelineExecutionId);
      processedCount++;

      logger.info('Activity stored', {
        pipelineExecutionId,
        activityType: activity.activityName,
        source: activity.source,
        activityId,
      });
    } catch (err) {
      logger.error('Failed to process activity', {
        error: err instanceof Error ? err.message : String(err),
        activityName: activity.activityName,
      });
      skippedCount++;
    }
  }

  // Auto-connect the mobile health integration so the pipeline wizard
  // shows it as available. Uses raw Firestore update to bypass typed
  // UserIntegrations (which doesn't have these fields in proto yet).
  if (processedCount > 0 && syncRequest.activities.length > 0) {
    const firstSource = syncRequest.activities[0].source;
    const integrationKey = firstSource === 'healthkit' ? 'apple_health' : 'health_connect';
    try {
      await db.collection('users').doc(userId).update({
        [`integrations.${integrationKey}.enabled`]: true,
        [`integrations.${integrationKey}.last_used_at`]: new Date(),
      });
      logger.info('Auto-connected mobile integration', { integrationKey });
    } catch (err) {
      // Non-fatal — integration connection is best-effort
      logger.warn('Failed to auto-connect mobile integration', {
        integrationKey,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }

  const response: MobileSyncResponse = {
    success: true,
    processedCount,
    skippedCount,
    executionIds,
    syncedAt: new Date().toISOString(),
  };

  logger.info('Mobile sync completed', {
    processedCount,
    skippedCount,
    totalReceived: syncRequest.activities.length,
  });

  return response;
};

// Export the wrapped function with Firebase Auth
export const mobileSyncHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()],
  },
});
