import { BaseConnector, ConnectorConfig, IngestStrategy, StandardizedActivity, CloudEventSource, ActivitySource, createFitbitClient, mapTCXToStandardized, FrameworkContext, ActivityType } from '@fitglue/shared';

interface FitbitNotification {
  collectionType: string;
  date: string;
  ownerId: string;
  ownerType: string;
  subscriptionId: string;
}

export type FitbitBody = FitbitNotification[];

export interface FitbitConnectorConfig extends ConnectorConfig {
  // OAuth tokens are managed by UserService via createFitbitClient
}

/**
 * Map Fitbit activityParentName to ActivityType enum.
 * Fitbit has 500+ activity types, but we map common ones to Strava-compatible types.
 * This mapping covers the most common activity categories from Fitbit.
 */
export function mapFitbitActivityType(activityParentName: string | undefined): ActivityType {
  const name = (activityParentName || '').toLowerCase().trim();

  // Running variations (including trail, virtual)
  if (name.includes('run') || name === 'treadmill' || name === 'jogging' || name === 'jog' ||
    name.includes('structured workout')) {
    // Check for trail run specifically
    if (name.includes('trail')) {
      return ActivityType.ACTIVITY_TYPE_TRAIL_RUN;
    }
    // Check for virtual run
    if (name.includes('virtual')) {
      return ActivityType.ACTIVITY_TYPE_VIRTUAL_RUN;
    }
    return ActivityType.ACTIVITY_TYPE_RUN;
  }

  // Walking
  if (name.includes('walk')) {
    return ActivityType.ACTIVITY_TYPE_WALK;
  }

  // Cycling variations (including spinning, indoor cycling)
  if (name.includes('bike') || name.includes('cycling') || name.includes('biking') ||
    name.includes('spinning') || name.includes('spin class') || name.includes('indoor cycle') ||
    name === 'spin' || name === 'cycle') {
    // Check for virtual ride
    if (name.includes('virtual')) {
      return ActivityType.ACTIVITY_TYPE_VIRTUAL_RIDE;
    }
    return ActivityType.ACTIVITY_TYPE_RIDE;
  }

  // Swimming
  if (name.includes('swim')) {
    return ActivityType.ACTIVITY_TYPE_SWIM;
  }

  // Weight Training
  if (name.includes('weight') || name === 'weights' || name.includes('strength') ||
    name.includes('resistance') || name.includes('lifting') || name.includes('dumbell') ||
    name.includes('barbell') || name.includes('kettlebell')) {
    return ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING;
  }

  // Yoga
  if (name.includes('yoga')) {
    return ActivityType.ACTIVITY_TYPE_YOGA;
  }

  // Pilates
  if (name.includes('pilates')) {
    return ActivityType.ACTIVITY_TYPE_PILATES;
  }

  // Hiking
  if (name.includes('hike') || name.includes('hiking')) {
    return ActivityType.ACTIVITY_TYPE_HIKE;
  }

  // Elliptical / Cross Trainer
  if (name.includes('elliptical') || name.includes('cross trainer') || name.includes('cross-trainer')) {
    return ActivityType.ACTIVITY_TYPE_ELLIPTICAL;
  }

  // Rowing
  if (name.includes('row') || name.includes('rowing') || name.includes('erg')) {
    return ActivityType.ACTIVITY_TYPE_ROWING;
  }

  // CrossFit
  if (name.includes('crossfit') || name.includes('cross fit')) {
    return ActivityType.ACTIVITY_TYPE_CROSSFIT;
  }

  // HIIT / High Intensity Interval Training
  if (name.includes('hiit') || name.includes('high intensity') || name.includes('interval training') ||
    name.includes('tabata') || name.includes('circuit') || name.includes('bootcamp') ||
    name.includes('boot camp') || name.includes('functional training')) {
    return ActivityType.ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING;
  }

  // Tennis / Racquet Sports
  if (name.includes('tennis') || name.includes('racquetball') || name.includes('squash') ||
    name.includes('badminton') || name.includes('pickleball') || name.includes('paddle')) {
    return ActivityType.ACTIVITY_TYPE_TENNIS;
  }

  // Soccer / Football
  if (name.includes('soccer') || name.includes('football') || name.includes('futsal')) {
    return ActivityType.ACTIVITY_TYPE_SOCCER;
  }

  // Golf
  if (name.includes('golf')) {
    return ActivityType.ACTIVITY_TYPE_GOLF;
  }

  // Skiing
  if (name.includes('ski') && !name.includes('water ski')) {
    if (name.includes('cross country') || name.includes('nordic')) {
      return ActivityType.ACTIVITY_TYPE_NORDIC_SKI;
    }
    if (name.includes('backcountry')) {
      return ActivityType.ACTIVITY_TYPE_BACKCOUNTRY_SKI;
    }
    return ActivityType.ACTIVITY_TYPE_ALPINE_SKI;
  }

  // Snowboarding
  if (name.includes('snowboard')) {
    return ActivityType.ACTIVITY_TYPE_SNOWBOARD;
  }

  // Skateboarding
  if (name.includes('skateboard')) {
    return ActivityType.ACTIVITY_TYPE_SKATEBOARD;
  }

  // Surfing / Water Sports
  if (name.includes('surf') || name.includes('standup paddle') || name.includes('sup') ||
    name.includes('paddleboard') || name === 'paddle') {
    return ActivityType.ACTIVITY_TYPE_SURFING;
  }

  // Stair Climbing / Stair Stepper
  if (name.includes('stair') || name.includes('step machine') || name.includes('stepper')) {
    return ActivityType.ACTIVITY_TYPE_STAIR_STEPPER;
  }

  // Martial Arts / Combat Sports
  if (name.includes('martial art') || name.includes('boxing') || name.includes('kickbox') ||
    name.includes('karate') || name.includes('judo') || name.includes('taekwondo') ||
    name.includes('mma') || name.includes('wrestling') || name.includes('jiu jitsu') ||
    name.includes('muay thai') || name.includes('fencing')) {
    return ActivityType.ACTIVITY_TYPE_WORKOUT; // No specific type, use workout
  }

  // Dance / Aerobics
  if (name.includes('dance') || name.includes('zumba') || name.includes('aerobic') ||
    name.includes('jazzercise') || name.includes('barre')) {
    return ActivityType.ACTIVITY_TYPE_WORKOUT;
  }

  // Rock Climbing
  if (name.includes('climb') && (name.includes('rock') || name.includes('boulder') || name.includes('wall'))) {
    return ActivityType.ACTIVITY_TYPE_ROCK_CLIMBING;
  }

  // Kayaking / Canoeing
  if (name.includes('kayak') || name.includes('canoe') || name.includes('paddling')) {
    return ActivityType.ACTIVITY_TYPE_KAYAKING;
  }

  // Wheelchair
  if (name.includes('wheelchair')) {
    return ActivityType.ACTIVITY_TYPE_WHEELCHAIR;
  }

  // Handcycle
  if (name.includes('handcycle') || name.includes('hand cycle')) {
    return ActivityType.ACTIVITY_TYPE_HANDCYCLE;
  }

  // Ice Skating
  if (name.includes('ice skat') || name.includes('hockey')) {
    return ActivityType.ACTIVITY_TYPE_ICE_SKATE;
  }

  // Inline Skating / Rollerblading
  if (name.includes('inline') || name.includes('rollerblade') || name.includes('roller skat')) {
    return ActivityType.ACTIVITY_TYPE_INLINE_SKATE;
  }

  // E-Bike
  if (name.includes('e-bike') || name.includes('ebike') || name.includes('electric bike')) {
    return ActivityType.ACTIVITY_TYPE_EBIKE_RIDE;
  }

  // Default fallback for unrecognized activities
  return ActivityType.ACTIVITY_TYPE_WORKOUT;
}

export class FitbitConnector extends BaseConnector<FitbitConnectorConfig> {
  readonly name = 'fitbit';
  readonly strategy: IngestStrategy = 'webhook';
  readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_FITBIT_WEBHOOK;
  readonly activitySource = ActivitySource.SOURCE_FITBIT;

  constructor(context: FrameworkContext) {
    super(context);
  }

  /**
   * Fitbit webhooks provide a date, not an activity ID.
   * We extract the date from the notification payload.
   * Returns null for non-activity notifications to skip processing.
   */
  extractId(payload: FitbitBody): string | null {
    if (!payload) return null;

    const fitbitActivitiesSubscription = payload.find((p) => p.subscriptionId === 'fitglue-activities');
    if (!fitbitActivitiesSubscription) {
      return null;
    }

    // Skip non-activity notifications
    if (fitbitActivitiesSubscription.collectionType && fitbitActivitiesSubscription.collectionType !== 'activities') {
      return null;
    }

    return fitbitActivitiesSubscription.date || null;
  }

  /**
   * Validates Fitbit configuration.
   */
  validateConfig(config: FitbitConnectorConfig): void {
    super.validateConfig(config);
  }

  /**
   * Handles Fitbit-specific request verification:
   * - GET requests for webhook verification
   * - POST signature validation
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async verifyRequest(req: any, context: FrameworkContext): Promise<{ handled: boolean; response?: any } | undefined> {
    const { logger } = context;

    // Handle GET verification requests
    if (req.method === 'GET') {
      const verifyCode = req.query.verify;
      if (!verifyCode) {
        logger.warn('Missing verify code in GET request');
        // Return 404 with body
        const { FrameworkResponse } = await import('@fitglue/shared');
        return {
          handled: true,
          response: new FrameworkResponse({ status: 404, body: 'Not Found' })
        };
      }

      const expectedCode = process.env['FITBIT_VERIFICATION_CODE'];
      if (verifyCode === expectedCode) {
        logger.info('Fitbit verification successful');
        // Return 204 No Content
        const { FrameworkResponse } = await import('@fitglue/shared');
        return {
          handled: true,
          response: new FrameworkResponse({ status: 204 })
        };
      } else {
        logger.warn('Invalid verification code');
        // Return 404 with body
        const { FrameworkResponse } = await import('@fitglue/shared');
        return {
          handled: true,
          response: new FrameworkResponse({ status: 404, body: 'Not Found' })
        };
      }
    }

    // Handle POST signature validation
    if (req.method === 'POST') {
      const signature = req.headers['x-fitbit-signature'];
      if (!signature) {
        logger.warn('Missing X-Fitbit-Signature header');
        const { FrameworkResponse } = await import('@fitglue/shared');
        // Use FrameworkResponse for 400 error
        return {
          handled: true,
          response: new FrameworkResponse({ status: 400, body: 'Missing Signature' })
        };
      }

      const clientSecret = process.env['FITBIT_CLIENT_SECRET'];
      if (!clientSecret) {
        logger.error('Missing FITBIT_CLIENT_SECRET env var');
        // Throwing error will be caught by SafeHandler and result in 500
        throw new Error('Missing FITBIT_CLIENT_SECRET env var');
      }

      const { createHmac } = await import('crypto');
      const rawBody = req.rawBody;
      if (!rawBody) {
        logger.error('Raw body not available for verification');
        throw new Error('Raw body not available');
      }

      const hmac = createHmac('sha1', `${clientSecret}&`);
      hmac.update(rawBody);
      const expectedSignature = hmac.digest('base64');

      if (signature !== expectedSignature) {
        logger.warn('Invalid Fitbit signature');
        const { FrameworkResponse } = await import('@fitglue/shared');
        // Use FrameworkResponse for 404 error
        return {
          handled: true,
          response: new FrameworkResponse({ status: 404, body: 'Not Found' })
        };
      }

      // Signature valid - continue to normal processing
      logger.info('Fitbit signature verified');
    }

    // Continue to normal webhook processing
    return undefined;
  }

  /**
   * Resolves user ID from Fitbit webhook payload.
   * Maps Fitbit's ownerId to our internal userId.
   */
  async resolveUser(payload: FitbitBody, context: FrameworkContext): Promise<string | null> {
    const { logger, services } = context;

    // payload is the body of the request, which for Fitbit webhooks is the notification payload
    // which is AN ARRAY of objects, with collection types and ownerIds etc.
    // we need to find the ownerId of the first object in the array that is
    // for `subscriptionId: fitglue-activities`

    const fitbitActivitiesSubscription = payload.find((p) => p.subscriptionId === 'fitglue-activities');
    if (!fitbitActivitiesSubscription) {
      logger.warn('Fitbit payload missing fitglue-activities subscription');
      return null;
    }

    const fitbitUserId = fitbitActivitiesSubscription.ownerId;
    if (!fitbitUserId) {
      logger.warn('Fitbit payload missing ownerId');
      return null;
    }

    const user = await services.user.findByFitbitId(fitbitUserId);
    if (!user) {
      logger.warn(`No user found for Fitbit ID: ${fitbitUserId}`);
      return null;
    }

    return user.id;
  }

  /**
   * Fetches all activities for a given date and maps them to StandardizedActivity[].
   *
   * @param activityId - The date string (YYYY-MM-DD) from the webhook
   * @param config - Fitbit connector config with userId injected
   */
  async fetchAndMap(activityId: string, config: FitbitConnectorConfig): Promise<StandardizedActivity[]> {
    const userId = (config as unknown as { userId: string }).userId;
    if (!userId) {
      throw new Error("userId missing in connector config");
    }


    // Use UserService from context
    const userService = this.context.services.user;

    const client = createFitbitClient(userService, userId, { usageTracking: true });
    const date = activityId; // The "activityId" is actually a date for Fitbit

    // Fetch activity list for the date
    const { data: activityList, error: listError } = await client.GET("/1/user/-/activities/date/{date}.json", {
      params: {
        path: { date: date }
      }
    });

    if (listError || !activityList || !activityList.activities) {
      throw new Error(`Fitbit API Error: ${listError}`);
    }

    this.context.logger.debug(`Activity List Response for ${date}`, { activityList });

    const activities = activityList.activities;
    const standardizedActivities: StandardizedActivity[] = [];

    // Process each activity
    for (const act of activities) {
      const logIdStr = act.logId?.toString();
      if (!logIdStr) continue;

      // Fetch TCX for the activity
      const { data: tcxData, error: tcxError, response } = await client.GET("/1/user/-/activities/{log-id}.tcx", {
        params: { path: { 'log-id': logIdStr } },
        parseAs: 'text'
      });

      if (tcxError || !tcxData) {
        const status = response.status;

        // Skip activities without TCX (manual, auto-detected, non-GPS)
        if (status === 404 || status === 204) {
          continue;
        }

        // Throw on transient errors to trigger retry
        if (status === 429 || status >= 500) {
          throw new Error(`Transient Fitbit API Error: ${status}`);
        }

        // Skip other errors (e.g. 403 permission issues)
        continue;
      }

      // Fetch detailed activity data to get the correct type (TCX doesn't have it, and daily summary is insufficient)
      // See: https://dev.fitbit.com/build/reference/web-api/activity/get-activity-log/
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const { data: activityDetail, error: detailError } = await client.GET("/1/user/-/activities/{log-id}.json" as any, {
        params: { path: { 'log-id': logIdStr } }
      });

      if (detailError) {
        this.context.logger.warn(`Failed to fetch activity detail for ${logIdStr}`, { error: detailError });
        // We will continue with best-effort mapping from the summary data
      } else {
        this.context.logger.debug(`Activity Detail Response for ${logIdStr}`, { activityDetail });
      }

      // Map TCX to StandardizedActivity
      try {
        const standardized = mapTCXToStandardized(tcxData as string, act, userId, 'FITBIT');

        // Override type using detailed activity data if available, otherwise fall back to summary data
        // The detailed response has `activityLog` property
        const detailedActivity = activityDetail?.activityLog;

        const nameToUse = detailedActivity?.name || detailedActivity?.activityParentName || act.name || act.activityParentName;
        const mappedType = mapFitbitActivityType(nameToUse);

        this.context.logger.debug(`Activity type mapping: id=${logIdStr}`, {
          nameToUse,
          mappedType,
          mappedTypeString: ActivityType[mappedType],
          originalName: act.name,
          originalParentName: act.activityParentName
        });

        standardized.type = mappedType;
        standardizedActivities.push(standardized);
      } catch (mapErr) {
        this.context.logger.error(`Failed to map activity ${logIdStr}`, { error: mapErr });
        // Continue processing other activities
      }
    }

    return standardizedActivities;
  }

  /**
   * Not used for Fitbit (we use fetchAndMap directly).
   */
  async mapActivity(_rawPayload: unknown, _context?: unknown): Promise<StandardizedActivity> {
    throw new Error('mapActivity not implemented for FitbitConnector - use fetchAndMap instead');
  }
}
