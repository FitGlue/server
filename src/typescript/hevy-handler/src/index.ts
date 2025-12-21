import { PubSub } from "@google-cloud/pubsub";
import * as crypto from "crypto";
// Using build-injected shared modules (Protobuf Generated)


// Retrieve secret from environment (secret manager injection)
import { TOPICS, createCloudFunction, getSecret, ActivityPayload, FrameworkContext, ActivitySource } from '@fitglue/shared';

const pubsub = new PubSub();
const TOPIC_NAME = TOPICS.RAW_ACTIVITY;

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { db, logger } = ctx;
  const timestamp = new Date().toISOString();

  // Cache check or fetch
  let signingSecret = process.env.HEVY_SIGNING_SECRET;
  if (!signingSecret) {
      try {
          // Fallback to project ID or default
          const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-project';
          signingSecret = await getSecret(projectId, 'hevy-signing-secret');
      } catch (e) {
          logger.warn('Could not fetch secret from GSM', { error: e });
      }
  }

  // 1. Signature Validation
  const signature = req.headers['x-hevy-signature'] as string;

  // In production, we MUST return 401 if secret is configured but signature missing/invalid.
  if (signingSecret) {
       // Cast req to any to access rawBody (provided by Google Cloud Functions framework)
       const rawBody = (req as any).rawBody || req.body;
       if (!verifySignature(rawBody, signature, signingSecret)) {
          logger.warn(`Invalid signature attempt. Sig: ${signature}`);
          res.status(401).send('Unauthorized');
          throw new Error('Invalid X-Hevy-Signature');
      }
  } else {
      logger.warn('HEVY_SIGNING_SECRET not set. Skipping signature verification (unsafe).');
  }

  // 2. Extract Payload
  const workoutData = req.body;
  if (!workoutData || !workoutData.workout) {
      throw new Error('Invalid payload: Missing workout data');
  }

  // 3. User Resolution (Multi-Tenancy)
  const hevyUserId = workoutData.user_id;
  if (!hevyUserId) {
      throw new Error('Invalid payload: Missing user_id');
  }

  // Lookup user by Hevy ID
  const userSnapshot = await db.collection('users')
      .where('integrations.hevy.userId', '==', hevyUserId)
      .limit(1)
      .get();

  if (userSnapshot.empty) {
      logger.warn(`Webhook received for unknown Hevy user: ${hevyUserId}`);
      res.status(200).send('User not configured');
      return { status: 'SKIPPED', reason: 'User not found' };
  }

  const userId = userSnapshot.docs[0].id;

  // 4. Publish to Pub/Sub with Protobuf Typing
  const messagePayload: ActivityPayload = {
      source: ActivitySource.SOURCE_HEVY,
      userId: userId,
      timestamp: timestamp,
      originalPayloadJson: JSON.stringify(workoutData),
      metadata: {}
  };

  const messageId = await pubsub.topic(TOPIC_NAME).publishMessage({
      json: messagePayload,
  });

  logger.info("Processed workout", { messageId, userId });
  res.status(200).send('Processed');

  return { pubsubMessageId: messageId };
};

export const hevyWebhookHandler = createCloudFunction(handler);

function verifySignature(body: Buffer | any, signature: string | undefined, secret: string): boolean {
    if (!signature) return false;

    // Use rawBody (Buffer) if available, otherwise fallback to JSON (for local/testing)
    const hmac = crypto.createHmac('sha256', secret);
    const content = Buffer.isBuffer(body) ? body : JSON.stringify(body);
    const digest = hmac.update(content).digest('hex');

    // Constant time comparison to prevent timing attacks
    return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(digest));
}
