// Module-level imports for smart pruning
import { BaseConnector, ConnectorConfig, IngestStrategy, FrameworkContext, FrameworkResponse } from '@fitglue/shared/framework';
import { StandardizedActivity, CloudEventSource, ActivitySource, ActivityType } from '@fitglue/shared/types';
import { createWahooClient } from '@fitglue/shared/integrations/wahoo';

/**
 * Wahoo Webhook Event payload structure.
 *
 * Wahoo Cloud API webhook payloads contain information about the event type
 * and the affected resource (workout).
 */
export interface WahooWebhookEvent {
  event_type: 'workout_summary' | 'workout_file';
  webhook_token: string;
  user: {
    id: number;
  };
  workout_summary?: {
    id: number;
    name: string;
    workout_token: string;
    workout_type_id: number;
    created_at: string;
    updated_at: string;
    plan_id?: number;
    starts: string;
    minutes: number;
    calories?: number;
    ascent_accum?: number;
    distance_accum?: number;
    heart_rate_avg?: number;
    power_avg?: number;
    cadence_avg?: number;
    speed_avg?: number;
  };
}

/**
 * Wahoo workout type mapping.
 * Based on Wahoo's workout_type_id values.
 */
const WAHOO_WORKOUT_TYPE_MAP: Record<number, ActivityType> = {
  0: ActivityType.ACTIVITY_TYPE_RIDE,       // Cycling
  1: ActivityType.ACTIVITY_TYPE_RUN,        // Running
  2: ActivityType.ACTIVITY_TYPE_WALK,       // Walking
  3: ActivityType.ACTIVITY_TYPE_SWIM,       // Swimming
  4: ActivityType.ACTIVITY_TYPE_WORKOUT,    // Other/Generic
  5: ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING, // Strength
  6: ActivityType.ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING, // HIIT
  7: ActivityType.ACTIVITY_TYPE_YOGA,       // Yoga
  8: ActivityType.ACTIVITY_TYPE_ROWING,     // Rowing
  9: ActivityType.ACTIVITY_TYPE_HIKE,       // Hiking
  10: ActivityType.ACTIVITY_TYPE_ELLIPTICAL, // Elliptical
};

/**
 * Map Wahoo workout type ID to ActivityType enum.
 */
export function mapWahooWorkoutType(workoutTypeId: number | undefined): ActivityType {
  if (workoutTypeId === undefined) return ActivityType.ACTIVITY_TYPE_WORKOUT;
  return WAHOO_WORKOUT_TYPE_MAP[workoutTypeId] ?? ActivityType.ACTIVITY_TYPE_WORKOUT;
}

// OAuth tokens are managed by UserService
export type WahooConnectorConfig = ConnectorConfig;

export class WahooConnector extends BaseConnector<WahooConnectorConfig> {
  readonly name = 'wahoo';
  readonly strategy: IngestStrategy = 'webhook';
  readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_WAHOO;
  readonly activitySource = ActivitySource.SOURCE_WAHOO;

  constructor(context: FrameworkContext) {
    super(context);
  }

  /**
   * Wahoo webhooks provide a workout ID in the workout_summary.id field.
   * Returns null for non-workout events.
   */
  extractId(payload: WahooWebhookEvent): string | null {
    if (!payload) return null;

    // Only process workout_summary events (new workout available)
    if (payload.event_type !== 'workout_summary') {
      this.context.logger.info('Ignoring non-workout Wahoo event', { event_type: payload.event_type });
      return null;
    }

    if (!payload.workout_summary?.id) {
      this.context.logger.warn('Wahoo workout_summary event missing workout ID');
      return null;
    }

    return payload.workout_summary.id.toString();
  }

  /**
   * Validates Wahoo configuration.
   */
  validateConfig(config: WahooConnectorConfig): void {
    super.validateConfig(config);
  }

  /**
   * Handles Wahoo-specific request verification.
   * GET requests for webhook endpoint validation.
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async verifyRequest(req: any, context: FrameworkContext): Promise<{ handled: boolean; response?: any } | undefined> {
    const { logger } = context;

    // Handle GET verification requests
    if (req.method === 'GET') {
      logger.info('Wahoo verification request received');

      return {
        handled: true,
        // 200 OK + Text Body
        response: new FrameworkResponse({ status: 200, body: 'OK' })
      };
    }

    // Continue to normal webhook processing
    return undefined;
  }

  /**
   * Resolves user ID from Wahoo webhook payload.
   * Maps Wahoo's user.id to our internal userId via IntegrationIdentityStore.
   */
  async resolveUser(payload: WahooWebhookEvent, context: FrameworkContext): Promise<string | null> {
    const { logger, stores } = context;

    const wahooUserId = payload.user?.id;
    if (!wahooUserId) {
      logger.warn('Wahoo payload missing user.id');
      return null;
    }

    // Use IntegrationIdentityStore to find user by external ID
    const userId = await stores.integrationIdentities.findUserByExternalId('wahoo', wahooUserId.toString());
    if (!userId) {
      logger.warn(`No user found for Wahoo User ID: ${wahooUserId}`);
      return null;
    }

    return userId;
  }

  /**
   * Fetches workout details from Wahoo using typed client.
   * Maps to StandardizedActivity.
   *
   * @param workoutId - The Wahoo workout ID
   * @param config - Wahoo connector config with userId injected
   */
  async fetchAndMap(workoutId: string, config: WahooConnectorConfig): Promise<StandardizedActivity[]> {
    const userId = (config as unknown as { userId: string }).userId;
    if (!userId) {
      throw new Error('userId missing in connector config');
    }

    const { logger, services } = this.context;

    // Create typed Wahoo client with automatic token refresh
    const client = createWahooClient(services.user, userId, { usageTracking: true });

    // Fetch workout details using typed client
    const { data: workout, error, response } = await client.GET('/v1/workouts/{workoutId}', {
      params: { path: { workoutId } }
    });

    if (error || !response.ok || !workout) {
      throw new Error(`Wahoo API Error: ${response.status} ${response.statusText}`);
    }

    logger.debug(`Wahoo Workout Response for ${workoutId}`, { workout });

    // Download FIT file if available (for future implementation)
    try {
      const { response: fitResponse } = await client.GET('/v1/workouts/{workoutId}/file', {
        params: { path: { workoutId } }
      });

      if (fitResponse.ok) {
        // FIT file download successful - would store in GCS
        // The FIT file data is available for future enricher processing
        logger.info('Wahoo FIT file available for download', { workoutId });
      } else {
        logger.warn('FIT file not available for Wahoo workout', { workoutId, status: fitResponse.status });
      }
    } catch (err) {
      logger.warn('Failed to download Wahoo FIT file', { workoutId, error: err });
    }

    // Map to StandardizedActivity
    const startTime = workout.starts ? new Date(workout.starts) : new Date();
    const standardized: StandardizedActivity = {
      source: 'WAHOO',
      externalId: workoutId,
      userId: userId,
      startTime,
      timeMarkers: [],
      name: workout.name || 'Wahoo Workout',
      type: mapWahooWorkoutType(workout.workout_type_id),
      description: '',
      tags: [],
      notes: '',
      sessions: [{
        startTime,
        totalElapsedTime: (workout.minutes || 0) * 60, // Convert minutes to seconds
        totalDistance: workout.distance_accum || 0,
        laps: [],
        strengthSets: []
      }]
    };

    return [standardized];
  }

  /**
   * Not used for Wahoo (we use fetchAndMap directly).
   */
  async mapActivity(_rawPayload: unknown, _context?: unknown): Promise<StandardizedActivity> {
    throw new Error('mapActivity not implemented for WahooConnector - use fetchAndMap instead');
  }
}

