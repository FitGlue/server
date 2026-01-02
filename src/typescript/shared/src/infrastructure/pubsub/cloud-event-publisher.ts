import { PubSub, Topic } from '@google-cloud/pubsub';
import { Logger } from 'winston';
import { CloudEvent } from 'cloudevents';

export class CloudEventPublisher<T> {
  private topic: Topic;

  constructor(
    private pubsub: PubSub,
    private topicName: string,
    private source: string, // CloudEvent 'source' (URI-reference)
    private type: string,   // CloudEvent 'type' (com.example.object.action)
    private logger?: Logger
  ) {
    this.topic = this.pubsub.topic(this.topicName);
  }

  /**
   * Publishes a message wrapped in a CloudEvent envelope.
   * @param data The payload data of type T
   * @param subject Optional subject (e.g. resource ID)
   * @returns The Pub/Sub message ID
   */
  async publish(data: T, subject?: string): Promise<string> {
    const ce = new CloudEvent({
      type: this.type,
      source: this.source,
      subject,
      data,
      datacontenttype: 'application/json',
    });

    try {
      // Serialize to JSON directly. CloudFunctions/PubSub bindings often handle
      // the JSON body as the message data.
      // We explicitly serialize the WHOLE event as the data payload of the Pub/Sub message.
      const messageBuffer = Buffer.from(JSON.stringify(ce));

      const messageId = await this.topic.publishMessage({ data: messageBuffer });

      if (this.logger) {
        this.logger.debug(`Published CloudEvent to ${this.topicName}`, {
          messageId,
          ceType: this.type,
          ceSource: this.source,
          ceId: ce.id
        });
      }

      return messageId;
    } catch (error) {
      if (this.logger) {
        this.logger.error(`Failed to publish CloudEvent to ${this.topicName}`, { error });
      }
      throw error;
    }
  }

  /**
    * Static helper to unwrap a CloudEvent from a raw payload (string/buffer).
    * This attempts to parse the input as a JSON CloudEvent and return the data field.
    *
    * @param raw The raw message payload (usually from req.body.message.data or similar)
    */
  static unwrap<R>(raw: string | Buffer | object | undefined | null): R | null {
    if (!raw) return null;

    try {
      // 1. If it's already an object, assume it's the parsed JSON event
      if (typeof raw === 'object' && !Buffer.isBuffer(raw)) {
        // rudimentary check
        if ('specversion' in raw && 'data' in raw) {
          return (raw as any).data as R;
        }
        // fallback: maybe it's just the data? No, strict mode means we expect envelope.
        // But for now let's try to be helpful.
        // Actually, if we are strictly enforcing CloudEventPublisher usage, we expect an Envelope.
        // Return raw if it fails check? No, safer to return null or throw.
        // Let's assume strict CloudEvent structure for now.
        return (raw as any).data || null;
      }

      // 2. Buffer/String parsing
      let jsonString: string;
      if (Buffer.isBuffer(raw)) {
        jsonString = raw.toString('utf-8');
      } else {
        jsonString = raw as string;
      }

      // Handle Base64?
      if (!jsonString.trim().startsWith('{')) {
        try {
          jsonString = Buffer.from(jsonString, 'base64').toString('utf-8');
        } catch (e) {
          // ignore, maybe properly valid JSON string that doesn't start with {?
        }
      }

      const parsed = JSON.parse(jsonString);
      return parsed.data as R;

    } catch (error) {
      console.error('Failed to unwrap CloudEvent', error);
      return null;
    }
  }
}
