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

  // Map URL param to typed integration key
  const validProviders: Record<string, 'appleHealth' | 'healthConnect'> = {
    'apple-health': 'appleHealth',
    'health-connect': 'healthConnect',
  };

  const integrationKey = validProviders[provider];
  if (!integrationKey) {
    throw new HttpError(400, `Invalid mobile provider: ${provider}. Use 'apple-health' or 'health-connect'.`);
  }

  // Mark integration as connected using typed setIntegration
  await ctx.stores.users.setIntegration(userId, integrationKey, {
    enabled: true,
    lastUsedAt: new Date(),
  });

  logger.info('Mobile integration connected', { userId, provider: integrationKey });
  return { message: `${provider} connected successfully` };
}

/**
 * Handle activity sync from mobile app
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any, complexity
async function handleSync(req: any, userId: string, ctx: FrameworkContext): Promise<MobileSyncResponse> {
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

  // Update last_used_at on the mobile health integration
  if (processedCount > 0 && syncRequest.activities.length > 0) {
    const firstSource = syncRequest.activities[0].source;
    const integrationKey = firstSource === 'healthkit' ? 'appleHealth' : 'healthConnect' as const;
    try {
      await stores.users.setIntegration(userId, integrationKey, {
        enabled: true,
        lastUsedAt: new Date(),
      });
      logger.info('Updated mobile integration last_used_at', { integrationKey });
    } catch (err) {
      // Non-fatal — integration tracking is best-effort
      logger.warn('Failed to update mobile integration', {
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
}

// Export the wrapped function with Firebase Auth
export const mobileSyncHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()],
  },
});
