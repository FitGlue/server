import { FrameworkContext } from './index';
import { Connector, ConnectorConfig } from './connector';
import { ActivityPayload, ActivitySource } from '../types/pb/activity';
import { StandardizedActivity } from '../types/pb/standardized_activity';
import { CloudEventPublisher } from '../infrastructure/pubsub/cloud-event-publisher';
import { CloudEventType } from '../types/pb/events';
import { getCloudEventSource, getCloudEventType } from '../types/events-helper';
import { TOPICS } from '../config';

/**
 * ConnectorConstructor defines the static shape of a Connector class.
 */
export interface ConnectorConstructor<TConfig extends ConnectorConfig, TRaw> {
  new(context: FrameworkContext): Connector<TConfig, TRaw>;
}

/**
 * createWebhookProcessor creates a standardized Cloud Function handler for webhooks.
 * It enforces the Extract -> Dedup -> Fetch -> Publish lifecycle.
 */
export function createWebhookProcessor<TConfig extends ConnectorConfig, TRaw>(
  ConnectorClass: ConnectorConstructor<TConfig, TRaw>
) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return async (req: any, res: any, ctx: FrameworkContext) => {
    const { logger, userId } = ctx;
    const timestamp = new Date();

    // 0. Instantiate Connector with Context
    const connector = new ConnectorClass(ctx);

    // 1. Verify Authentication
    logger.info('Webhook processor received context', { userId, hasUserId: !!userId, ctxUserId: ctx.userId });
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
    const user = await ctx.services.user.get(userId);
    if (!user) {
      logger.error(`User ${userId} not found`);
      res.status(500).send('User configuration error');
      throw new Error('User not found');
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const connectorConfig = (user.integrations as any)?.[connector.name];

    if (!connectorConfig || !connectorConfig.enabled) {
      logger.warn(`User ${userId} has not enabled integration ${connector.name} or config missing`);
      res.status(200).send('Integration disabled or unconfigured');
      return { status: 'Skipped', reason: 'Integration disabled' };
    }

    const fullConfig = { ...connectorConfig, userId } as unknown as TConfig;

    // 4. Validate Config
    try {
      connector.validateConfig(fullConfig);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (err: any) {
      logger.error(`Invalid configuration for user ${userId}`, { error: err.message });
      res.status(200).send(`Configuration Error: ${err.message}`);
      return { status: 'Failed', reason: 'Configuration Error' };
    }

    // 5. Pipeline Check
    // Bail early if user has no pipeline configured for this source.
    // This prevents phantom pipeline executions and the legacy fallback in the enricher.
    const sourceEnumName = ActivitySource[connector.activitySource]; // e.g., "SOURCE_HEVY"
    const hasPipelineForSource = user.pipelines?.some(p => p.source === sourceEnumName) ?? false;
    if (!hasPipelineForSource) {
      logger.info(`User ${userId} has no pipeline configured for source ${sourceEnumName}. Skipping.`);
      res.status(200).send('No pipeline configured for this source');
      return { status: 'Skipped', reason: `No pipeline for source ${sourceEnumName}` };
    }

    // 6. Loop Prevention Check
    // Check if the incoming external ID exists as a destination in any synchronized activity.
    // This prevents infinite loops (e.g., Hevy → Strava → Hevy → ...)
    const isLoopActivity = await ctx.services.user.checkDestinationExists(userId, connector.name, externalId);
    if (isLoopActivity) {
      logger.info(`Loop detected: Activity ${externalId} was already posted by this system. Skipping.`);
      res.status(200).send('Skipped: Loop prevention');
      return { status: 'Skipped', reason: 'Loop prevention - activity was already posted as destination' };
    }

    // 7. Deduplication Check
    const alreadyProcessed = await ctx.services.user.hasProcessedActivity(userId, connector.name, externalId);
    if (alreadyProcessed) {
      logger.info(`Activity ${externalId} already processed for user ${userId}`);
      res.status(200).send('Already processed');
      return { status: 'Skipped', reason: 'Already processed' };
    }

    // 8. Fetch & Map Activities
    let standardizedActivities: StandardizedActivity[];
    try {
      standardizedActivities = await connector.fetchAndMap(externalId, fullConfig);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (err: any) {
      logger.error(`Failed to fetch/map activity ${externalId}`, { error: err.message });
      res.status(500).send('Failed to process activity');
      throw err;
    }

    logger.info(`Processing ${standardizedActivities.length} activities`);

    // 9. Publishing (loop for batch support)
    const publishedIds: string[] = [];

    for (const standardizedActivity of standardizedActivities) {
      const activityExternalId = standardizedActivity.externalId;

      // Check dedup for each activity in batch
      const isActivityProcessed = await ctx.services.user.hasProcessedActivity(userId, connector.name, activityExternalId);
      if (isActivityProcessed) {
        logger.info(`[${connector.name}] Activity ${activityExternalId} already processed, skipping`);
        continue;
      }

      // Generate unique pipelineExecutionId for each activity
      // This ensures separate trace grouping even when multiple activities come from one webhook
      const activityPipelineExecutionId = `${ctx.executionId}-${activityExternalId}`;

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
        standardizedActivity: standardizedActivity,
        pipelineExecutionId: activityPipelineExecutionId, // Unique per activity
        // Resume mode fields (false for initial processing)
        isResume: false,
        resumeOnlyEnrichers: [],
        useUpdateMethod: false,
      };

      const publisher = new CloudEventPublisher<ActivityPayload>(
        ctx.pubsub,
        TOPICS.RAW_ACTIVITY,
        getCloudEventSource(connector.cloudEventSource),
        getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_CREATED),
        logger
      );

      const messageId = await publisher.publish(messagePayload, activityExternalId);

      // Mark activity as processed
      await ctx.services.user.markActivityAsProcessed(userId, connector.name, standardizedActivity.externalId, {
        processedAt: new Date(),
        source: connector.name,
        externalId: standardizedActivity.externalId
      });

      publishedIds.push(activityExternalId);
      logger.info(`[${connector.name}] Published activity ${activityExternalId}`, { messageId });
    }

    logger.info(`[${connector.name}] Successfully processed ${publishedIds.length}/${standardizedActivities.length} activities for ${externalId}`);
    res.status(200).json({
      status: 'Success',
      published: publishedIds.length,
      total: standardizedActivities.length
    });

    return { status: 'Processed', externalId, publishedCount: publishedIds.length, publishedIds };
  };
}
