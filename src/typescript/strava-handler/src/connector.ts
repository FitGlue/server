import {
  BaseConnector,
  ConnectorConfig,
  IngestStrategy,
  StandardizedActivity,
  CloudEventSource,
  ActivitySource,
  createStravaClient,
  FrameworkContext,
  ActivityType
} from '@fitglue/shared';

/**
 * Strava Webhook Event payload structure
 * https://developers.strava.com/docs/webhooks/
 */
export interface StravaWebhookEvent {
  object_type: 'activity' | 'athlete';
  object_id: number;
  aspect_type: 'create' | 'update' | 'delete';
  updates?: Record<string, unknown>;
  owner_id: number;
  subscription_id: number;
  event_time: number;
}

export interface StravaConnectorConfig extends ConnectorConfig {
  // OAuth tokens are managed by UserService via createStravaClient
}

/**
 * Map Strava activity type string to ActivityType enum.
 */
export function mapStravaActivityType(stravaType: string | undefined): ActivityType {
  if (!stravaType) return ActivityType.ACTIVITY_TYPE_WORKOUT;

  // Strava uses PascalCase type names that match our enum strava_name extensions
  const typeMap: Record<string, ActivityType> = {
    'AlpineSki': ActivityType.ACTIVITY_TYPE_ALPINE_SKI,
    'BackcountrySki': ActivityType.ACTIVITY_TYPE_BACKCOUNTRY_SKI,
    'Canoeing': ActivityType.ACTIVITY_TYPE_CANOEING,
    'Crossfit': ActivityType.ACTIVITY_TYPE_CROSSFIT,
    'EBikeRide': ActivityType.ACTIVITY_TYPE_EBIKE_RIDE,
    'Elliptical': ActivityType.ACTIVITY_TYPE_ELLIPTICAL,
    'Golf': ActivityType.ACTIVITY_TYPE_GOLF,
    'GravelRide': ActivityType.ACTIVITY_TYPE_GRAVEL_RIDE,
    'Handcycle': ActivityType.ACTIVITY_TYPE_HANDCYCLE,
    'HighIntensityIntervalTraining': ActivityType.ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING,
    'Hike': ActivityType.ACTIVITY_TYPE_HIKE,
    'IceSkate': ActivityType.ACTIVITY_TYPE_ICE_SKATE,
    'InlineSkate': ActivityType.ACTIVITY_TYPE_INLINE_SKATE,
    'Kayaking': ActivityType.ACTIVITY_TYPE_KAYAKING,
    'Kitesurf': ActivityType.ACTIVITY_TYPE_KITESURF,
    'MountainBikeRide': ActivityType.ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE,
    'NordicSki': ActivityType.ACTIVITY_TYPE_NORDIC_SKI,
    'Pilates': ActivityType.ACTIVITY_TYPE_PILATES,
    'Ride': ActivityType.ACTIVITY_TYPE_RIDE,
    'RockClimbing': ActivityType.ACTIVITY_TYPE_ROCK_CLIMBING,
    'RollerSki': ActivityType.ACTIVITY_TYPE_ROLLER_SKI,
    'Rowing': ActivityType.ACTIVITY_TYPE_ROWING,
    'Run': ActivityType.ACTIVITY_TYPE_RUN,
    'Sail': ActivityType.ACTIVITY_TYPE_SAIL,
    'Skateboard': ActivityType.ACTIVITY_TYPE_SKATEBOARD,
    'Snowboard': ActivityType.ACTIVITY_TYPE_SNOWBOARD,
    'Snowshoe': ActivityType.ACTIVITY_TYPE_SNOWSHOE,
    'Soccer': ActivityType.ACTIVITY_TYPE_SOCCER,
    'StairStepper': ActivityType.ACTIVITY_TYPE_STAIR_STEPPER,
    'StandUpPaddling': ActivityType.ACTIVITY_TYPE_STAND_UP_PADDLING,
    'Surfing': ActivityType.ACTIVITY_TYPE_SURFING,
    'Swim': ActivityType.ACTIVITY_TYPE_SWIM,
    'Tennis': ActivityType.ACTIVITY_TYPE_TENNIS,
    'TrailRun': ActivityType.ACTIVITY_TYPE_TRAIL_RUN,
    'VirtualRide': ActivityType.ACTIVITY_TYPE_VIRTUAL_RIDE,
    'VirtualRun': ActivityType.ACTIVITY_TYPE_VIRTUAL_RUN,
    'Walk': ActivityType.ACTIVITY_TYPE_WALK,
    'WeightTraining': ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING,
    'Wheelchair': ActivityType.ACTIVITY_TYPE_WHEELCHAIR,
    'Windsurf': ActivityType.ACTIVITY_TYPE_WINDSURF,
    'Workout': ActivityType.ACTIVITY_TYPE_WORKOUT,
    'Yoga': ActivityType.ACTIVITY_TYPE_YOGA,
  };

  return typeMap[stravaType] ?? ActivityType.ACTIVITY_TYPE_WORKOUT;
}

export class StravaConnector extends BaseConnector<StravaConnectorConfig> {
  readonly name = 'strava';
  readonly strategy: IngestStrategy = 'webhook';
  readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_STRAVA;
  readonly activitySource = ActivitySource.SOURCE_STRAVA;

  constructor(context: FrameworkContext) {
    super(context);
  }

  /**
   * Strava webhooks provide an activity ID in the object_id field.
   * Returns null for non-activity events or delete events.
   */
  extractId(payload: StravaWebhookEvent): string | null {
    if (!payload) return null;

    // Only process activity create events
    if (payload.object_type !== 'activity') {
      this.context.logger.info('Ignoring non-activity Strava event', { object_type: payload.object_type });
      return null;
    }

    if (payload.aspect_type === 'delete') {
      this.context.logger.info('Ignoring activity delete event');
      return null;
    }

    // For update events, we could optionally re-process, but for now skip
    if (payload.aspect_type === 'update') {
      this.context.logger.info('Ignoring activity update event');
      return null;
    }

    return payload.object_id?.toString() || null;
  }

  /**
   * Validates Strava configuration.
   */
  validateConfig(config: StravaConnectorConfig): void {
    super.validateConfig(config);
  }

  /**
   * Handles Strava-specific request verification.
   * GET requests for webhook subscription validation.
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async verifyRequest(req: any, res: any, context: FrameworkContext): Promise<{ handled: boolean; response?: Record<string, unknown> } | undefined> {
    const { logger } = context;

    // Handle GET verification requests (Strava subscription validation)
    if (req.method === 'GET') {
      const hubChallenge = req.query['hub.challenge'];
      const hubVerifyToken = req.query['hub.verify_token'];

      if (!hubChallenge) {
        logger.warn('Missing hub.challenge in GET request');
        res.status(400).send('Missing hub.challenge');
        return { handled: true, response: { action: 'verification', status: 'failed' } };
      }

      // Verify the token matches our expected value
      const expectedToken = process.env['STRAVA_VERIFY_TOKEN'];
      if (hubVerifyToken !== expectedToken) {
        logger.warn('Invalid verify_token');
        res.status(403).send('Forbidden');
        return { handled: true, response: { action: 'verification', status: 'invalid_token' } };
      }

      logger.info('Strava verification successful');
      res.status(200).json({ 'hub.challenge': hubChallenge });
      return { handled: true, response: { action: 'verification', status: 'success' } };
    }

    // Continue to normal webhook processing
    return undefined;
  }

  /**
   * Resolves user ID from Strava webhook payload.
   * Maps Strava's owner_id (athlete ID) to our internal userId.
   */
  async resolveUser(payload: StravaWebhookEvent, context: FrameworkContext): Promise<string | null> {
    const { logger, services } = context;

    const stravaAthleteId = payload.owner_id;
    if (!stravaAthleteId) {
      logger.warn('Strava payload missing owner_id');
      return null;
    }

    const user = await services.user.findByStravaId(stravaAthleteId);
    if (!user) {
      logger.warn(`No user found for Strava Athlete ID: ${stravaAthleteId}`);
      return null;
    }

    return user.id;
  }

  /**
   * Fetches activity details from Strava and maps to StandardizedActivity.
   *
   * @param activityId - The Strava activity ID
   * @param config - Strava connector config with userId injected
   */
  async fetchAndMap(activityId: string, config: StravaConnectorConfig): Promise<StandardizedActivity[]> {
    const userId = (config as unknown as { userId: string }).userId;
    if (!userId) {
      throw new Error("userId missing in connector config");
    }

    const userService = this.context.services.user;
    const client = createStravaClient(userService, userId);

    // Fetch activity details
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const { data: activity, error } = await client.GET('/activities/{id}' as any, {
      params: { path: { id: activityId } }
    });

    if (error || !activity) {
      throw new Error(`Strava API Error: ${error}`);
    }

    this.context.logger.debug(`Strava Activity Response for ${activityId}`, { activity });

    // Map to StandardizedActivity
    const standardized: StandardizedActivity = {
      source: 'STRAVA',
      externalId: activityId,
      userId: userId,
      startTime: activity.start_date ? new Date(activity.start_date) : new Date(),
      name: activity.name || 'Strava Activity',
      type: mapStravaActivityType(activity.type),
      description: activity.description || '',
      tags: [],
      notes: '',
      sessions: [{
        startTime: activity.start_date ? new Date(activity.start_date) : new Date(),
        totalElapsedTime: activity.elapsed_time || 0,
        totalDistance: activity.distance || 0,
        laps: [],
        strengthSets: []
      }]
    };

    return [standardized];
  }

  /**
   * Not used for Strava (we use fetchAndMap directly).
   */
  async mapActivity(_rawPayload: unknown, _context?: unknown): Promise<StandardizedActivity> {
    throw new Error('mapActivity not implemented for StravaConnector - use fetchAndMap instead');
  }
}
