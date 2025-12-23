import { TOPICS, createCloudFunction, ActivityPayload, FrameworkContext, ActivitySource, createHevyClient } from '@fitglue/shared';

// Removed local PubSub instantiation
const TOPIC_NAME = TOPICS.RAW_ACTIVITY;

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { db, logger, userId } = ctx;
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

  // 5. Active Fetch
  logger.info(`Fetching workout ${workoutId} from Hevy API`);

  const client = createHevyClient({ apiKey: hevyApiKey });
  const { data, error, response } = await client.GET("/v1/workouts/{workoutId}", {
      params: {
          path: { workoutId }
      }
  });

  if (error || !data) {
       logger.error('Failed to fetch workout from Hevy', { error, status: response.status });
       throw new Error(`Hevy API error: ${response.status} ${response.statusText}`);
  }

  // 6. Runtime Validation & Publishing
  // Relying on static types from openapi-fetch as requested
  const fullWorkout = data;

  const messagePayload: ActivityPayload = {
      source: ActivitySource.SOURCE_HEVY,
      userId: userId,
      timestamp: timestamp,
      originalPayloadJson: JSON.stringify(fullWorkout),
      metadata: {
          'fetch_method': 'active_fetch',
          'webhook_id': workoutId
      }
  };

  // Uses injected PubSub client
  const messageId = await ctx.pubsub.topic(TOPIC_NAME).publishMessage({
      json: messagePayload,
  });

  logger.info("Processed and fetched workout", { messageId, userId, workoutId });

  // Return richer execution result for logging
  const executionResult = {
      status: 'Processed',
      pubsubMessageId: messageId,
      workoutId,
      fullWorkout // This will be captured by logExecutionSuccess
  };

  res.status(200).json({ executionId: ctx.executionId, status: 'Processed' });

  return executionResult;
};

export const hevyWebhookHandler = createCloudFunction(handler, {
    auth: {
        strategies: ['api_key']
    }
});
