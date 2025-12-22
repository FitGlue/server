import { PubSub } from "@google-cloud/pubsub";

import { TOPICS, createCloudFunction, ActivityPayload, FrameworkContext, ActivitySource } from '@fitglue/shared';

const pubsub = new PubSub();
const TOPIC_NAME = TOPICS.RAW_ACTIVITY;

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { db, logger, userId, authScopes } = ctx;
  const timestamp = new Date().toISOString();

  // 1. Verify Authentication (Already handled by Middleware, but safe guard)
  if (!userId) {
      logger.error('Handler called without authenticated userId');
      res.status(401).send('Unauthorized');
      throw new Error('Unauthorized');
  }

  logger.info(`Authenticated user: ${userId}`);

  // 2. Extract Webhook Data
  const payload = req.body;
  const workoutId = payload.workout_id;

  if (!workoutId) {
      throw new Error('Invalid payload: Missing workout_id');
  }

  // 3. User Resolution (Egress Config Lookup)
  const userDoc = await db.collection('users').doc(userId).get();
  if (!userDoc.exists) {
      logger.error(`Authenticated user ${userId} not found in users collection`);
      res.status(500).send('User configuration error');
      throw new Error('User not found');
  }

  const userData = userDoc.data();
  // 4. Retrieve Hevy API Key for Active Fetch
  const hevyApiKey = userData?.integrations?.hevy?.apiKey;

  if (!hevyApiKey) {
      logger.error(`User ${userId} missing integrations.hevy.apiKey`);
      res.status(200).send('Configuration Error');
      return { status: 'FAILED', reason: 'Missing Hevy API Key' };
  }

  // 5. Active Fetch Decision
  let fullWorkout: any;
  const requestMock = req.headers['x-mock-fetch'] === 'true' || payload.mock_workout_data;

  if (requestMock) {
      // Secure Mock Fetch: ONLY allow if key has test scope
      if (!authScopes || !authScopes.includes('test:mock_fetch')) {
          logger.warn(`User ${userId} attempted mock fetch without scope`);
          res.status(403).send('Forbidden: Missing test scope');
          throw new Error('Missing test:mock_fetch scope');
      }

      if (!payload.mock_workout_data) {
          throw new Error('Mock fetch requested but missing mock_workout_data');
      }
      logger.info('Using mock workout data (authorized test scope)');
      fullWorkout = payload.mock_workout_data;

  } else {
      // Real Fetch
      logger.info(`Fetching workout ${workoutId} from Hevy API`);
      try {
          const response = await fetch(`https://api.hevyapp.com/v1/workouts/${workoutId}`, {
              headers: {
                  'x-api-key': hevyApiKey
              }
          });

          if (!response.ok) {
              throw new Error(`Hevy API error: ${response.status} ${response.statusText}`);
          }

          fullWorkout = await response.json();
      } catch (err: any) {
          logger.error('Failed to fetch workout from Hevy', { error: err.message, workoutId });
          throw err;
      }
  }

  // 6. Publish Fetched Data
  const messagePayload: ActivityPayload = {
      source: ActivitySource.SOURCE_HEVY,
      userId: userId,
      timestamp: timestamp,
      originalPayloadJson: JSON.stringify(fullWorkout),
      metadata: {
          'fetch_method': requestMock ? 'mock_fetch' : 'active_fetch',
          'webhook_id': workoutId
      }
  };

  const messageId = await pubsub.topic(TOPIC_NAME).publishMessage({
      json: messagePayload,
  });

  logger.info("Processed and fetched workout", { messageId, userId, workoutId });
  res.status(200).send('Processed');

  return { pubsubMessageId: messageId };
};

export const hevyWebhookHandler = createCloudFunction(handler, {
    auth: {
        strategies: ['api_key']
    }
});
