
import { PubSub, Topic } from '@google-cloud/pubsub';
import { Logger } from 'winston';

export class TypedPublisher<T> {
  private topic: Topic;

  constructor(
    private pubsub: PubSub,
    private topicName: string,
    private logger?: Logger
  ) {
    this.topic = this.pubsub.topic(this.topicName);
  }

  /**
   * Publishes a message to the topic, ensuring it matches the generic type T.
   * @param message The message payload matching type T
   * @returns The message ID
   */
  async publish(message: T): Promise<string> {
    try {
      const dataBuffer = Buffer.from(JSON.stringify(message));
      const messageId = await this.topic.publishMessage({ data: dataBuffer });

      if (this.logger) {
        this.logger.debug(`Published message to ${this.topicName}`, { messageId });
      }

      return messageId;
    } catch (error) {
      if (this.logger) {
        this.logger.error(`Failed to publish message to ${this.topicName}`, { error });
      }
      throw error;
    }
  }

  /**
   * Unwraps a raw Pub/Sub message payload into the typed object T.
   * Handles base64 decoding if necessary.
   * @param data The raw data (Buffer, string, or base64 string)
   * @returns The parsed object of type T
   */
  static unwrap<T>(data: string | Buffer | undefined | null): T | null {
    if (!data) return null;

    try {
      let jsonString: string;

      if (Buffer.isBuffer(data)) {
        jsonString = data.toString('utf-8');
      } else if (typeof data === 'string') {
        // Check if likely base64 (simple heuristic or just try parse)
        // Pub/Sub PUSH messages are often raw JSON in body, but Pull/EventArc logic varies.
        // If it's pure JSON, parse it. If it fails, try base64?
        // Actually, cloud functions usually give us the decoded body if content-type is json.
        // But if we are unwrapping the `message.data` from a Push request body, it is base64.

        // Let's assume input might be base64 if it's not looking like JSON
        if (data.trim().startsWith('{') || data.trim().startsWith('[')) {
          jsonString = data;
        } else {
          jsonString = Buffer.from(data, 'base64').toString('utf-8');
        }
      } else {
        return null;
      }

      return JSON.parse(jsonString) as T;
    } catch (err) {
      console.error('Failed to unwrap message', err);
      // Try parsing as plain JSON in case the base64 assumption was wrong
      try {
        if (typeof data === 'string') return JSON.parse(data) as T;
      } catch (ignore) { }
      throw new Error(`Failed to unwrap message: ${err}`);
    }
  }
}
