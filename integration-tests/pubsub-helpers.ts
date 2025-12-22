import { PubSub } from '@google-cloud/pubsub';
import { config } from './config';

let pubsubClient: PubSub | null = null;

/**
 * Get or create PubSub client for the configured project
 */
function getPubSubClient(): PubSub {
  if (!pubsubClient) {
    pubsubClient = new PubSub({ projectId: config.projectId });
  }
  return pubsubClient;
}

/**
 * Publish a message to the raw-activity topic
 * Triggers the Enricher function
 */
export async function publishRawActivity(payload: {
  source: number;
  user_id: string;
  timestamp: string;
  original_payload_json: string;
  metadata: Record<string, any>;
}): Promise<string> {
  if (!config.topics) {
    throw new Error('Pub/Sub topics not configured for this environment');
  }

  const pubsub = getPubSubClient();
  const topic = pubsub.topic(config.topics.rawActivity);

  const dataBuffer = Buffer.from(JSON.stringify(payload));
  const messageId = await topic.publishMessage({ data: dataBuffer });

  return messageId;
}

/**
 * Publish a message to the enriched-activity topic
 * Triggers the Router function
 */
export async function publishEnrichedActivity(payload: {
  user_id: string;
  activity_id: string;
  gcs_uri: string;
  description: string;
  metadata_json: string;
}): Promise<string> {
  if (!config.topics) {
    throw new Error('Pub/Sub topics not configured for this environment');
  }

  const pubsub = getPubSubClient();
  const topic = pubsub.topic(config.topics.enrichedActivity);

  const dataBuffer = Buffer.from(JSON.stringify(payload));
  const messageId = await topic.publishMessage({ data: dataBuffer });

  return messageId;
}

/**
 * Publish a message to the upload-strava topic
 * Triggers the Strava Uploader function
 */
export async function publishUploadJob(payload: {
  user_id: string;
  activity_id: string;
  gcs_uri: string;
  description: string;
}): Promise<string> {
  if (!config.topics) {
    throw new Error('Pub/Sub topics not configured for this environment');
  }

  const pubsub = getPubSubClient();
  const topic = pubsub.topic(config.topics.uploadStrava);

  const dataBuffer = Buffer.from(JSON.stringify(payload));
  const messageId = await topic.publishMessage({ data: dataBuffer });

  return messageId;
}
