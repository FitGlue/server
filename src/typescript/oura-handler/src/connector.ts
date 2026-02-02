// Module-level imports for smart pruning
import { BaseConnector, ConnectorConfig, IngestStrategy, FrameworkContext } from '@fitglue/shared/framework';
import { StandardizedActivity, CloudEventSource, ActivitySource, ActivityType } from '@fitglue/shared/types';
import { createOuraClient } from '@fitglue/shared/integrations/oura';

/**
 * Oura Webhook Event payload structure
 * https://cloud.ouraring.com/docs/webhooks
 */
export interface OuraWebhookEvent {
  event_type: 'create' | 'update' | 'delete';
  data_type: 'workout' | 'sleep' | 'daily_activity' | 'daily_readiness';
  object_id: string;
  user_id: string;
  timestamp: string;
}

/**
 * Oura Workout data structure from API (local reference)
 * https://cloud.ouraring.com/v2/docs#tag/Workout-Routes
 */
export interface OuraWorkout {
  id: string;
  activity: string;
  calories: number;
  day: string;
  distance?: number;
  end_datetime: string;
  intensity: 'easy' | 'moderate' | 'hard';
  label?: string;
  source: 'manual' | 'autodetected' | 'confirmed' | 'workout_heart_rate';
  start_datetime: string;
  average_heart_rate?: number;
  max_heart_rate?: number;
}

// OAuth tokens are managed by UserService
export type OuraConnectorConfig = ConnectorConfig;

/**
 * Map Oura activity type string to ActivityType enum.
 * Oura uses lowercase activity names.
 */
export function mapOuraActivityType(ouraActivity: string | undefined): ActivityType {
  if (!ouraActivity) return ActivityType.ACTIVITY_TYPE_WORKOUT;

  const typeMap: Record<string, ActivityType> = {
    'running': ActivityType.ACTIVITY_TYPE_RUN,
    'cycling': ActivityType.ACTIVITY_TYPE_RIDE,
    'walking': ActivityType.ACTIVITY_TYPE_WALK,
    'hiking': ActivityType.ACTIVITY_TYPE_HIKE,
    'swimming': ActivityType.ACTIVITY_TYPE_SWIM,
    'yoga': ActivityType.ACTIVITY_TYPE_YOGA,
    'pilates': ActivityType.ACTIVITY_TYPE_PILATES,
    'strength_training': ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING,
    'hiit': ActivityType.ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING,
    'elliptical': ActivityType.ACTIVITY_TYPE_ELLIPTICAL,
    'stair_climbing': ActivityType.ACTIVITY_TYPE_STAIR_STEPPER,
    'rowing': ActivityType.ACTIVITY_TYPE_ROWING,
    'dancing': ActivityType.ACTIVITY_TYPE_WORKOUT,
    'tennis': ActivityType.ACTIVITY_TYPE_TENNIS,
    'soccer': ActivityType.ACTIVITY_TYPE_SOCCER,
    'basketball': ActivityType.ACTIVITY_TYPE_WORKOUT,
    'ice_skating': ActivityType.ACTIVITY_TYPE_ICE_SKATE,
    'cross_country_skiing': ActivityType.ACTIVITY_TYPE_NORDIC_SKI,
    'downhill_skiing': ActivityType.ACTIVITY_TYPE_ALPINE_SKI,
    'snowboarding': ActivityType.ACTIVITY_TYPE_SNOWBOARD,
    'surfing': ActivityType.ACTIVITY_TYPE_SURFING,
    'golf': ActivityType.ACTIVITY_TYPE_GOLF,
    'other': ActivityType.ACTIVITY_TYPE_WORKOUT,
  };

  // Normalize to lowercase for matching
  const normalizedActivity = ouraActivity.toLowerCase().replace(/\s+/g, '_');
  return typeMap[normalizedActivity] ?? ActivityType.ACTIVITY_TYPE_WORKOUT;
}

export class OuraConnector extends BaseConnector<OuraConnectorConfig> {
  readonly name = 'oura';
  readonly strategy: IngestStrategy = 'webhook';
  readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_OURA;
  readonly activitySource = ActivitySource.SOURCE_OURA;

  constructor(context: FrameworkContext) {
    super(context);
  }

  /**
   * Oura webhooks provide a workout ID in the object_id field.
   * Returns null for non-workout events or delete events.
   */
  extractId(payload: OuraWebhookEvent): string | null {
    if (!payload) return null;

    // Only process workout events
    if (payload.data_type !== 'workout') {
      this.context.logger.info('Ignoring non-workout Oura event', { data_type: payload.data_type });
      return null;
    }

    if (payload.event_type === 'delete') {
      this.context.logger.info('Ignoring workout delete event');
      return null;
    }

    // For update events, we could optionally re-process, but for now skip
    if (payload.event_type === 'update') {
      this.context.logger.info('Ignoring workout update event');
      return null;
    }

    return payload.object_id || null;
  }

  /**
   * Validates Oura configuration.
   */
  validateConfig(config: OuraConnectorConfig): void {
    super.validateConfig(config);
  }

  /**
   * Resolves user ID from Oura webhook payload.
   * Maps Oura's user_id to our internal userId.
   */
  async resolveUser(payload: OuraWebhookEvent, context: FrameworkContext): Promise<string | null> {
    const { logger, services } = context;

    const ouraUserId = payload.user_id;
    if (!ouraUserId) {
      logger.warn('Oura payload missing user_id');
      return null;
    }

    const user = await services.user.findByOuraId(ouraUserId);
    if (!user) {
      logger.warn(`No user found for Oura User ID: ${ouraUserId}`);
      return null;
    }

    return user.id;
  }

  /**
   * Fetches workout details from Oura using typed client.
   * Maps to StandardizedActivity.
   *
   * @param workoutId - The Oura workout ID
   * @param config - Oura connector config with userId injected
   */
  async fetchAndMap(workoutId: string, config: OuraConnectorConfig): Promise<StandardizedActivity[]> {
    const userId = (config as unknown as { userId: string }).userId;
    if (!userId) {
      throw new Error('userId missing in connector config');
    }

    // Create typed Oura client with automatic token refresh
    const client = createOuraClient(this.context.services.user, userId, { usageTracking: true });

    // Fetch workout details from Oura API using typed client
    const { data: workout, error, response } = await client.GET('/v2/usercollection/workout/{document_id}', {
      params: { path: { document_id: workoutId } }
    });

    if (error || !response.ok || !workout) {
      const errorText = error ? JSON.stringify(error) : response.statusText;
      throw new Error(`Oura API Error (${response.status}): ${errorText}`);
    }

    this.context.logger.debug(`Oura Workout Response for ${workoutId}`, { workout });

    // Calculate duration in seconds
    const startTime = new Date(workout.start_datetime);
    const endTime = new Date(workout.end_datetime);
    const durationSeconds = Math.floor((endTime.getTime() - startTime.getTime()) / 1000);

    // Map to StandardizedActivity
    const standardized: StandardizedActivity = {
      source: 'OURA',
      externalId: workoutId,
      userId: userId,
      startTime: startTime,
      timeMarkers: [],
      name: workout.label || `${this.formatActivityName(workout.activity)} Workout`,
      type: mapOuraActivityType(workout.activity),
      description: '',
      tags: workout.intensity ? [`intensity:${workout.intensity}`] : [],
      notes: '',
      sessions: [{
        startTime: startTime,
        totalElapsedTime: durationSeconds,
        totalDistance: workout.distance ?? 0,
        totalCalories: workout.calories ?? undefined,
        // Note: Heart rate data not available from Oura workout API
        laps: [],
        strengthSets: []
      }]
    };

    return [standardized];
  }

  /**
   * Format activity name from Oura's snake_case to Title Case
   */
  private formatActivityName(activity: string): string {
    return activity
      .split('_')
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
  }

  /**
   * Not used for Oura (we use fetchAndMap directly).
   */
  async mapActivity(_rawPayload: unknown, _context?: unknown): Promise<StandardizedActivity> {
    throw new Error('mapActivity not implemented for OuraConnector - use fetchAndMap instead');
  }
}
