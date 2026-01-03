import { BaseConnector, ConnectorConfig, IngestStrategy, StandardizedActivity, CloudEventSource, ActivitySource, createFitbitClient, mapTCXToStandardized, FrameworkContext } from '@fitglue/shared';

interface FitbitNotification {
  collectionType: string;
  date: string;
  ownerId: string;
  ownerType: string;
  subscriptionId: string;
}

type FitbitBody = FitbitNotification[];

export interface FitbitConnectorConfig extends ConnectorConfig {
  fitbit_user_id: string;
  // OAuth tokens are managed by UserService via createFitbitClient
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
   * Requires fitbit_user_id to be present.
   */
  validateConfig(config: FitbitConnectorConfig): void {
    super.validateConfig(config);
    if (!config.fitbit_user_id) {
      throw new Error(`Connector ${this.name}: 'fitbit_user_id' is missing`);
    }
  }

  /**
   * Handles Fitbit-specific request verification:
   * - GET requests for webhook verification
   * - POST signature validation
   */
  async verifyRequest(req: any, res: any, context: any): Promise<{ handled: boolean; response?: any } | undefined> {
    const { logger } = context;

    // Handle GET verification requests
    if (req.method === 'GET') {
      const verifyCode = req.query.verify;
      if (!verifyCode) {
        logger.warn('Missing verify code in GET request');
        res.status(404).send('Not Found');
        return { handled: true, response: { action: 'verification', status: 'failed' } };
      }

      const expectedCode = process.env['FITBIT_VERIFICATION_CODE'];
      if (verifyCode === expectedCode) {
        logger.info('Fitbit verification successful');
        res.status(204).send();
        return { handled: true, response: { action: 'verification', status: 'success' } };
      } else {
        logger.warn('Invalid verification code');
        res.status(404).send('Not Found');
        return { handled: true, response: { action: 'verification', status: 'invalid' } };
      }
    }

    // Handle POST signature validation
    if (req.method === 'POST') {
      const signature = req.headers['x-fitbit-signature'];
      if (!signature) {
        logger.warn('Missing X-Fitbit-Signature header');
        res.status(400).send('Missing Signature');
        throw new Error('Missing X-Fitbit-Signature header');
      }

      const clientSecret = process.env['FITBIT_CLIENT_SECRET'];
      if (!clientSecret) {
        logger.error('Missing FITBIT_CLIENT_SECRET env var');
        res.status(500).send('Configuration Error');
        throw new Error('Missing FITBIT_CLIENT_SECRET env var');
      }

      const { createHmac } = await import('crypto');
      const rawBody = (req as any).rawBody;
      if (!rawBody) {
        logger.error('Raw body not available for verification');
        res.status(500).send('Internal Server Error');
        throw new Error('Raw body not available');
      }

      const hmac = createHmac('sha1', `${clientSecret}&`);
      hmac.update(rawBody);
      const expectedSignature = hmac.digest('base64');

      if (signature !== expectedSignature) {
        logger.warn('Invalid Fitbit signature');
        res.status(404).send('Not Found');
        throw new Error('Invalid Signature');
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
  async resolveUser(payload: FitbitBody, context: any): Promise<string | null> {
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
    const userId = (config as any).userId;
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

      // Map TCX to StandardizedActivity
      try {
        const standardized = mapTCXToStandardized(tcxData as string, act, userId, 'FITBIT');
        standardizedActivities.push(standardized);
      } catch (mapErr) {
        console.error(`Failed to map activity ${logIdStr}:`, mapErr);
        // Continue processing other activities
      }
    }

    return standardizedActivities;
  }

  /**
   * Not used for Fitbit (we use fetchAndMap directly).
   */
  async mapActivity(_rawPayload: any, _context?: any): Promise<StandardizedActivity> {
    throw new Error('mapActivity not implemented for FitbitConnector - use fetchAndMap instead');
  }
}
