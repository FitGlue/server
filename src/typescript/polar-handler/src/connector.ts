// Module-level imports for smart pruning
import { BaseConnector, ConnectorConfig, IngestStrategy, FrameworkContext, FrameworkResponse } from '@fitglue/shared/framework';
import { StandardizedActivity, CloudEventSource, ActivitySource, ActivityType } from '@fitglue/shared/types';

/**
 * Polar webhook notification payload structure.
 * Polar sends notifications for various events (EXERCISE, etc.)
 */
interface PolarWebhookNotification {
  event: 'EXERCISE' | 'SLEEP' | 'ACTIVITY';
  user_id: number;
  entity_id: string; // Exercise ID for EXERCISE events
  timestamp: string;
  url?: string; // Direct URL to fetch the resource
}

export type PolarBody = PolarWebhookNotification;

// OAuth tokens are managed by UserService
export type PolarConnectorConfig = ConnectorConfig;

/**
 * Polar AccessLink exercise data structure.
 * See: https://www.polar.com/accesslink-api/#tag/Training-data
 */
interface PolarExercise {
  id: string;
  upload_time: string;
  polar_user: string;
  device: string;
  device_id: string;
  start_time: string;
  duration: string; // ISO 8601 duration (PT1H30M)
  calories: number;
  distance: number;
  sport: string;
  has_route: boolean;
  heart_rate?: {
    average: number;
    maximum: number;
  };
  training_load?: number;
  detailed_sport_info?: string;
}

/**
 * Map Polar sport types to FitGlue ActivityType.
 */
// eslint-disable-next-line complexity
export function mapPolarSportType(sport: string | undefined): ActivityType {
  const sportLower = (sport || '').toLowerCase().trim();

  // Running
  if (sportLower.includes('running') || sportLower === 'run') {
    if (sportLower.includes('trail')) {
      return ActivityType.ACTIVITY_TYPE_TRAIL_RUN;
    }
    return ActivityType.ACTIVITY_TYPE_RUN;
  }

  // Walking
  if (sportLower.includes('walking') || sportLower === 'walk') {
    return ActivityType.ACTIVITY_TYPE_WALK;
  }

  // Cycling
  if (sportLower.includes('cycling') || sportLower.includes('biking') ||
    sportLower === 'bike' || sportLower === 'cycle') {
    return ActivityType.ACTIVITY_TYPE_RIDE;
  }

  // Swimming
  if (sportLower.includes('swimming') || sportLower === 'swim') {
    return ActivityType.ACTIVITY_TYPE_SWIM;
  }

  // Hiking
  if (sportLower.includes('hiking') || sportLower === 'hike') {
    return ActivityType.ACTIVITY_TYPE_HIKE;
  }

  // Strength Training
  if (sportLower.includes('strength') || sportLower.includes('weight') ||
    sportLower.includes('gym') || sportLower.includes('fitness')) {
    return ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING;
  }

  // Yoga
  if (sportLower.includes('yoga')) {
    return ActivityType.ACTIVITY_TYPE_YOGA;
  }

  // Pilates
  if (sportLower.includes('pilates')) {
    return ActivityType.ACTIVITY_TYPE_PILATES;
  }

  // Cross-country skiing
  if (sportLower.includes('cross-country') || sportLower.includes('nordic')) {
    return ActivityType.ACTIVITY_TYPE_NORDIC_SKI;
  }

  // Alpine skiing
  if (sportLower.includes('skiing') || sportLower.includes('downhill')) {
    return ActivityType.ACTIVITY_TYPE_ALPINE_SKI;
  }

  // Rowing
  if (sportLower.includes('rowing') || sportLower.includes('indoor rowing')) {
    return ActivityType.ACTIVITY_TYPE_ROWING;
  }

  // Default fallback
  return ActivityType.ACTIVITY_TYPE_WORKOUT;
}

/**
 * Parse ISO 8601 duration string to seconds.
 * Example: "PT1H30M45S" -> 5445 seconds
 */
function parseIsoDuration(duration: string): number {
  const match = duration.match(/PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?/);
  if (!match) return 0;

  const hours = parseInt(match[1] || '0', 10);
  const minutes = parseInt(match[2] || '0', 10);
  const seconds = parseInt(match[3] || '0', 10);

  return hours * 3600 + minutes * 60 + seconds;
}

export class PolarConnector extends BaseConnector<PolarConnectorConfig> {
  readonly name = 'polar';
  readonly strategy: IngestStrategy = 'webhook';
  readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_POLAR_WEBHOOK;
  readonly activitySource = ActivitySource.SOURCE_POLAR;

  constructor(context: FrameworkContext) {
    super(context);
  }

  /**
   * Extract exercise ID from Polar webhook notification.
   * Returns null for non-exercise events.
   */
  extractId(payload: PolarBody): string | null {
    if (!payload || payload.event !== 'EXERCISE') {
      return null;
    }
    return payload.entity_id || null;
  }

  /**
   * Validates Polar configuration.
   */
  validateConfig(config: PolarConnectorConfig): void {
    super.validateConfig(config);
  }

  /**
   * Handle Polar-specific request verification.
   * Polar uses HMAC-SHA256 signatures in the Polar-Webhook-Signature header.
   */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  async verifyRequest(req: any, context: FrameworkContext): Promise<{ handled: boolean; response?: any } | undefined> {
    const { logger } = context;

    // Handle GET verification requests (webhook setup)
    if (req.method === 'GET') {
      logger.info('Polar webhook verification GET request');

      return {
        handled: true,
        response: new FrameworkResponse({ status: 200, body: 'OK' })
      };
    }

    // Verify POST webhook signature
    if (req.method === 'POST') {
      const signature = req.headers['polar-webhook-signature'];
      if (!signature) {
        logger.warn('Missing Polar-Webhook-Signature header');
        // Allow through for now - signature validation can be added when key is configured
        return undefined;
      }

      // Signature validation would go here when configured
      logger.info('Polar webhook POST received');
    }

    return undefined;
  }

  /**
   * Resolve FitGlue user ID from Polar user ID.
   */
  async resolveUser(payload: PolarBody, context: FrameworkContext): Promise<string | null> {
    const { logger, services } = context;

    if (!payload || !payload.user_id) {
      logger.warn('Polar payload missing user_id');
      return null;
    }

    const polarUserId = String(payload.user_id);
    const user = await services.user.findByPolarId(polarUserId);

    if (!user) {
      logger.warn(`No user found for Polar ID: ${polarUserId}`);
      return null;
    }

    return user.id;
  }

  /**
   * Fetch exercise data from Polar AccessLink using transaction-based API.
   *
   * Polar's Transaction Lifecycle:
   * 1. Start Transaction: POST /v3/users/{userId}/exercise-transactions
   * 2. List Exercises: GET transaction exercises
   * 3. Fetch Exercise Details: GET individual exercise
   * 4. COMMIT Transaction: PUT to acknowledge receipt
   *
   * IMPORTANT: If transaction is not committed, exercises reappear in next request.
   */
  async fetchAndMap(activityId: string, config: PolarConnectorConfig): Promise<StandardizedActivity[]> {
    const userId = (config as unknown as { userId: string }).userId;
    if (!userId) {
      throw new Error('userId missing in connector config');
    }

    const userService = this.context.services.user;
    const user = await userService.getUser(userId);

    if (!user?.integrations?.polar?.enabled) {
      throw new Error('Polar integration not enabled for user');
    }

    const accessToken = user.integrations.polar.accessToken;
    const polarUserId = user.integrations.polar.polarUserId;

    if (!accessToken || !polarUserId) {
      throw new Error('Missing Polar access token or user ID');
    }

    // Step 1: Start a transaction
    const transactionResponse = await fetch(
      `https://www.polaraccesslink.com/v3/users/${polarUserId}/exercise-transactions`,
      {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${accessToken}`,
          'Accept': 'application/json',
        },
      }
    );

    // 204 = No new exercises
    if (transactionResponse.status === 204) {
      this.context.logger.info('No new exercises available from Polar');
      return [];
    }

    if (!transactionResponse.ok) {
      throw new Error(`Failed to start Polar transaction: ${transactionResponse.status}`);
    }

    const transactionData = await transactionResponse.json() as {
      'transaction-id': number;
      'resource-uri': string;
    };
    const transactionId = transactionData['transaction-id'];

    this.context.logger.info('Started Polar exercise transaction', { transactionId });

    try {
      // Step 2: List exercises in this transaction
      const exercisesResponse = await fetch(
        `https://www.polaraccesslink.com/v3/users/${polarUserId}/exercise-transactions/${transactionId}`,
        {
          method: 'GET',
          headers: {
            'Authorization': `Bearer ${accessToken}`,
            'Accept': 'application/json',
          },
        }
      );

      if (!exercisesResponse.ok) {
        throw new Error(`Failed to list Polar exercises: ${exercisesResponse.status}`);
      }

      const exercisesData = await exercisesResponse.json() as {
        exercises: string[]; // Array of exercise URLs
      };

      const standardizedActivities: StandardizedActivity[] = [];

      // Step 3: Fetch each exercise
      for (const exerciseUrl of exercisesData.exercises) {
        try {
          const exerciseResponse = await fetch(exerciseUrl, {
            method: 'GET',
            headers: {
              'Authorization': `Bearer ${accessToken}`,
              'Accept': 'application/json',
            },
          });

          if (!exerciseResponse.ok) {
            this.context.logger.warn(`Failed to fetch exercise: ${exerciseUrl}`);
            continue;
          }

          const exercise = await exerciseResponse.json() as PolarExercise;

          // Map to StandardizedActivity
          const standardized = this.mapExerciseToStandardized(exercise, userId);
          standardizedActivities.push(standardized);
        } catch (err) {
          this.context.logger.error(`Error fetching exercise: ${exerciseUrl}`, { error: err });
        }
      }

      // Step 4: COMMIT the transaction (CRITICAL!)
      await this.commitTransaction(accessToken, polarUserId, transactionId);

      return standardizedActivities;

    } catch (err) {
      // Still try to commit on error to prevent exercises getting stuck
      try {
        await this.commitTransaction(accessToken, polarUserId, transactionId);
      } catch (commitErr) {
        this.context.logger.error('Failed to commit transaction after error', { error: commitErr });
      }
      throw err;
    }
  }

  /**
   * Commit a Polar transaction to acknowledge receipt of exercises.
   */
  private async commitTransaction(accessToken: string, polarUserId: string, transactionId: number): Promise<void> {
    const commitResponse = await fetch(
      `https://www.polaraccesslink.com/v3/users/${polarUserId}/exercise-transactions/${transactionId}`,
      {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${accessToken}`,
        },
      }
    );

    if (!commitResponse.ok) {
      throw new Error(`Failed to commit Polar transaction: ${commitResponse.status}`);
    }

    this.context.logger.info('Committed Polar exercise transaction', { transactionId });
  }

  /**
   * Map a Polar exercise to StandardizedActivity.
   */
  private mapExerciseToStandardized(exercise: PolarExercise, userId: string): StandardizedActivity {
    const durationSeconds = parseIsoDuration(exercise.duration);
    const startTime = new Date(exercise.start_time);

    return {
      source: 'POLAR',
      externalId: exercise.id,
      userId,
      type: mapPolarSportType(exercise.sport),
      name: exercise.detailed_sport_info || exercise.sport || 'Polar Activity',
      description: `Polar ${exercise.sport || 'Activity'} - ${exercise.device || 'Unknown Device'}`,
      startTime: startTime,
      timeMarkers: [],
      tags: ['polar', exercise.sport?.toLowerCase() || 'workout'].filter(Boolean),
      notes: '',
      sessions: [{
        startTime: startTime,
        totalElapsedTime: durationSeconds,
        totalDistance: exercise.distance || 0,
        laps: [],
        strengthSets: [],
      }],
    };
  }

  /**
   * Map raw payload - not used for Polar (we use fetchAndMap).
   */
  async mapActivity(_rawPayload: unknown, _context?: unknown): Promise<StandardizedActivity> {
    throw new Error('mapActivity not implemented for PolarConnector - use fetchAndMap instead');
  }
}
