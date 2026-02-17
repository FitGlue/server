/**
 * Mobile Sync Handler
 *
 * Cloud Function that receives health data from the FitGlue mobile app.
 * Accepts activities from iOS HealthKit and Android Health Connect.
 *
 * Flow:
 * 1. Stores activity summary in user sub-collection (users/{id}/mobile_activities/{id})
 * 2. Offloads structured telemetry (HR samples, GPS routes) to GCS
 * 3. Publishes per-activity messages to topic-mobile-activity for downstream processing
 */

// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch } from '@fitglue/shared/routing';
import { CloudEventPublisher } from '@fitglue/shared/infrastructure/pubsub';
import { CloudEventType, CloudEventSource, getCloudEventSource, getCloudEventType } from '@fitglue/shared/types';
import { getStorage } from 'firebase-admin/storage';

import {
  MobileSyncRequest,
  MobileSyncResponse,
  MobileActivity,
  mapMobileActivityType,
  getMobileSourceId,
} from './types';

/**
 * Message payload published to topic-mobile-activity.
 * The downstream mobile-source-handler consumes this to build StandardizedActivity.
 */
export interface MobileActivityMessage {
  userId: string;
  activityId: string;
  source: 'healthkit' | 'health_connect';
  telemetryUri?: string;
}

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
 * Upload telemetry data (HR samples + GPS routes) to GCS.
 * Returns the gs:// URI of the uploaded blob.
 */
async function uploadTelemetryToGcs(
  userId: string,
  activityId: string,
  activity: MobileActivity
): Promise<string | undefined> {
  const hasHr = activity.heartRateSamples && activity.heartRateSamples.length > 0;
  const hasRoute = activity.route && activity.route.length > 0;

  if (!hasHr && !hasRoute) {
    return undefined;
  }

  const telemetry = {
    heartRateSamples: activity.heartRateSamples || [],
    route: activity.route || [],
  };

  const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
  const bucketName = `${projectId}-artifacts`;
  const gcsPath = `mobile_activities/${userId}/${activityId}.json`;

  const storage = getStorage();
  const file = storage.bucket(bucketName).file(gcsPath);
  await file.save(JSON.stringify(telemetry), {
    contentType: 'application/json',
    metadata: { cacheControl: 'no-cache' },
  });

  return `gs://${bucketName}/${gcsPath}`;
}

/**
 * Handle activity sync from mobile app.
 *
 * 1. Store summary + telemetry_uri in Firestore (user sub-collection)
 * 2. Upload structured telemetry to GCS
 * 3. Publish to topic-mobile-activity for pipeline processing
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

  // User sub-collection for mobile activities
  const mobileActivitiesRef = db.collection('users').doc(userId).collection('mobile_activities');

  // Process each activity: store → GCS offload → Pub/Sub publish
  for (const activity of syncRequest.activities) {
    try {
      const activityId = activity.externalId ||
        `${activity.source}-${new Date(activity.startTime).getTime()}`;

      // 1. Upload telemetry to GCS (HR samples + GPS routes)
      const telemetryUri = await uploadTelemetryToGcs(userId, activityId, activity);

      // 2. Store summary in Firestore (user sub-collection)
      const activityData = {
        userId,
        activityId,
        source: getMobileSourceId(activity.source),
        activityType: mapMobileActivityType(activity.activityName),
        name: activity.activityName,
        startTime: new Date(activity.startTime),
        endTime: new Date(activity.endTime),
        durationSeconds: activity.duration,
        calories: activity.calories || null,
        distanceMeters: activity.distance || null,
        heartRateSampleCount: activity.heartRateSamples?.length || 0,
        routePointCount: activity.route?.length || 0,
        telemetryUri: telemetryUri || null,
        createdAt: new Date(),
        status: 'stored',
      };

      await mobileActivitiesRef.doc(activityId).set(activityData, { merge: true });

      // 3. Publish to topic-mobile-activity for downstream processing
      const cloudEventSource = activity.source === 'healthkit'
        ? CloudEventSource.CLOUD_EVENT_SOURCE_APPLE_HEALTH
        : CloudEventSource.CLOUD_EVENT_SOURCE_HEALTH_CONNECT;

      const publisher = new CloudEventPublisher<MobileActivityMessage>(
        ctx.pubsub,
        'topic-mobile-activity',
        getCloudEventSource(cloudEventSource),
        getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_CREATED),
        logger
      );

      const message: MobileActivityMessage = {
        userId,
        activityId,
        source: activity.source,
        telemetryUri,
      };

      await publisher.publish(message, activityId);

      executionIds.push(activityId);
      processedCount++;

      logger.info('Activity stored and published', {
        activityType: activity.activityName,
        source: activity.source,
        activityId,
        hasTelemetry: !!telemetryUri,
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
  skipExecutionLogging: true,
});
