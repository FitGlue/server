import { FrameworkContext } from './index';
import { Connector, ConnectorConfig } from './connector';
import { UserService } from '../domain/services/user';
import { ActivityPayload } from '../types/pb/activity';
import { StandardizedActivity } from '../types/pb/standardized_activity';
import { CloudEventPublisher } from '../infrastructure/pubsub/cloud-event-publisher';
import { CloudEventType } from '../types/pb/events';
import { getCloudEventSource, getCloudEventType } from '../types/events-helper';
import { TOPICS } from '../config';

/**
 * createWebhookProcessor creates a standardized Cloud Function handler for webhooks.
 * It enforces the Extract -> Dedup -> Fetch -> Publish lifecycle.
 */
export function createWebhookProcessor<TConfig extends ConnectorConfig, TRaw>(
  connector: Connector<TConfig, TRaw>
) {
  return async (req: any, res: any, ctx: FrameworkContext) => {
    const { db, logger, userId } = ctx;
    const timestamp = new Date();

    // 1. Verify Authentication
    if (!userId) {
      logger.error('Handler called without authenticated userId');
      res.status(401).send('Unauthorized');
      throw new Error('Unauthorized');
    }

    logger.info(`Webhook Processor [${connector.name}] for user: ${userId}`);

    // 1.5. Custom Request Verification
    const verificationResult = await connector.verifyRequest(req, res, ctx);
    if (verificationResult?.handled) {
      return verificationResult.response || { status: 'Handled by connector verification' };
    }

    // 2. Extract Webhook Data
    const body = req.body || {};
    const externalId = connector.extractId(body);

    if (!externalId) {
      logger.error(`[${connector.name}] Invalid payload: Missing external ID`, {
        preview: JSON.stringify(body).substring(0, 200)
      });
      throw new Error('Invalid payload: Missing external ID');
    }

    // 3. User Resolution & Config Lookup
    // We assume standard path: users/{userId} -> integrations -> {connector.name}
    const { storage } = await import('../index');
    const userDoc = await storage.getUsersCollection().doc(userId).get();

    if (!userDoc.exists) {
      logger.error(`User ${userId} not found`);
      res.status(500).send('User configuration error');
      throw new Error('User not found');
    }

    const userData = userDoc.data();
    const connectorConfig = userData?.integrations?.[connector.name as keyof typeof userData.integrations];

    if (!connectorConfig || !connectorConfig.enabled) {
      logger.warn(`User ${userId} has not enabled integration ${connector.name} or config missing`);
      res.status(200).send('Integration disabled or unconfigured');
      return { status: 'Skipped', reason: 'Integration disabled' };
    }

    const fullConfig = { ...connectorConfig, userId } as unknown as TConfig;

    // Validate Config
    try {
      connector.validateConfig(fullConfig);
    } catch (e: any) {
      logger.error(`[${connector.name}] Configuration validation failed`, { error: e.message });
      res.status(200).send(`Configuration Error: ${e.message}`);
      return { status: 'Failed', reason: 'Configuration Invalid' };
    }

    // 4. Deduplication
    const userService = new UserService(db);
    const isProcessed = await userService.hasProcessedActivity(userId, connector.name, externalId);

    if (isProcessed) {
      logger.info(`[${connector.name}] Activity ${externalId} already processed, skipping`);
      res.status(200).json({ status: 'Skipped', reason: 'Already processed' });
      return { status: 'Skipped', reason: 'Already processed', externalId };
    }

    // 5. Active Fetch & Map
    logger.info(`[${connector.name}] Fetching and mapping ${externalId}`);

    let activities: StandardizedActivity[];
    try {
      activities = await connector.fetchAndMap(externalId, fullConfig);
    } catch (err: any) {
      logger.error(`[${connector.name}] Fetch/Map failed`, { error: err });
      throw err;
    }

    logger.info(`[${connector.name}] Processing ${activities.length} activities`);

    // 6. Publishing (loop for batch support)
    const publishedIds: string[] = [];

    for (const standardizedActivity of activities) {
      const activityExternalId = standardizedActivity.externalId;

      // Check dedup for each activity in batch
      const isActivityProcessed = await userService.hasProcessedActivity(userId, connector.name, activityExternalId);
      if (isActivityProcessed) {
        logger.info(`[${connector.name}] Activity ${activityExternalId} already processed, skipping`);
        continue;
      }

      const messagePayload: ActivityPayload = {
        source: connector.activitySource,
        userId: userId,
        timestamp: timestamp,
        originalPayloadJson: JSON.stringify({ id: externalId, activityId: activityExternalId, note: "Fetched via Generic Connector" }),
        metadata: {
          'fetch_method': 'active_fetch_connector',
          'webhook_id': externalId,
          'activity_id': activityExternalId,
          'connector': connector.name
        },
        standardizedActivity: standardizedActivity
      };

      const publisher = new CloudEventPublisher<ActivityPayload>(
        ctx.pubsub,
        TOPICS.RAW_ACTIVITY,
        getCloudEventSource(connector.cloudEventSource),
        getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_CREATED),
        logger
      );

      const messageId = await publisher.publish(messagePayload, activityExternalId);

      // 7. Mark Processed
      await userService.markActivityAsProcessed(userId, connector.name, activityExternalId);

      publishedIds.push(activityExternalId);
      logger.info(`[${connector.name}] Published activity ${activityExternalId}`, { messageId });
    }

    logger.info(`[${connector.name}] Successfully processed ${publishedIds.length}/${activities.length} activities for ${externalId}`);
    res.status(200).json({ status: 'Processed', publishedCount: publishedIds.length, publishedIds });

    return { status: 'Processed', externalId, publishedCount: publishedIds.length, publishedIds };
  };
}
