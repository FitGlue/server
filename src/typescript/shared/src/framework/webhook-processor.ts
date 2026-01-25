import { FrameworkContext } from './index';
import { Connector, ConnectorConfig } from './connector';
import { ActivityPayload, ActivitySource } from '../types/pb/activity';
import { StandardizedActivity } from '../types/pb/standardized_activity';
import { CloudEventPublisher } from '../infrastructure/pubsub/cloud-event-publisher';
import { CloudEventType } from '../types/pb/events';
import { ExecutionStatus } from '../types/pb/execution';
import { getCloudEventSource, getCloudEventType, getCorrespondingDestination } from '../types/events-helper';
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
// eslint-disable-next-line complexity
export function createWebhookProcessor<TConfig extends ConnectorConfig, TRaw>(
  ConnectorClass: ConnectorConstructor<TConfig, TRaw>
) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any, complexity
  return async (req: any, ctx: FrameworkContext) => {
    const { logger, userId } = ctx;
    const timestamp = new Date();

    // 0. Instantiate Connector with Context
    const connector = new ConnectorClass(ctx);

    // 1. Verify Authentication
    logger.info('Webhook processor received context', { userId, hasUserId: !!userId, ctxUserId: ctx.userId });
    if (!userId) {
      logger.error('Handler called without authenticated userId');
      // res.status(401).send('Unauthorized'); // REPLACE
      const err = new Error('Unauthorized') as Error & { statusCode: number };
      err.statusCode = 401;
      throw err;
    }

    logger.info(`Webhook Processor [${connector.name}] for user: ${userId}`);

    // 1.5. Custom Request Verification
    const verificationResult = await connector.verifyRequest(req, ctx);
    if (verificationResult?.handled) {
      // Return the response object (FrameworkResponse or plain object) directly
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
      // res.status(500).send('User configuration error');
      throw new Error('User not found');
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const connectorConfig = (user.integrations as any)?.[connector.name];

    if (!connectorConfig || !connectorConfig.enabled) {
      logger.warn(`User ${userId} has not enabled integration ${connector.name} or config missing`);
      // res.status(200).send('Integration disabled or unconfigured');
      return { status: 'Skipped', reason: 'Integration disabled' };
    }

    const fullConfig = { ...connectorConfig, userId } as unknown as TConfig;

    // 4. Validate Config
    try {
      connector.validateConfig(fullConfig);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (err: any) {
      logger.error(`Invalid configuration for user ${userId}`, { error: err.message });
      // res.status(200).send(`Configuration Error: ${err.message}`);
      return { status: 'Failed', reason: 'Configuration Error', error: err.message };
    }

    // 5. Pipeline Check
    // Bail early if user has no pipeline configured for this source.
    // This prevents phantom pipeline executions and the legacy fallback in the enricher.
    const sourceEnumName = ActivitySource[connector.activitySource]; // e.g., "SOURCE_HEVY"
    const userPipelines = await ctx.services.user.pipelineStore.list(userId);
    const hasPipelineForSource = userPipelines.some(p => p.source === sourceEnumName);
    if (!hasPipelineForSource) {
      logger.info(`User ${userId} has no pipeline configured for source ${sourceEnumName}. Skipping.`);
      // res.status(200).send('No pipeline configured for this source');
      return { status: 'Skipped', reason: `No pipeline for source ${sourceEnumName}` };
    }

    // 6. Source-Level Bounceback Check (Primary Loop Prevention)
    // Check if this activity was uploaded by us - if so, this webhook is a "bounceback"
    // This uses the uploaded_activities collection which tracks our successful uploads.
    // We query by destination enum + destinationId (the externalId from webhook IS the destination's ID).
    const correspondingDestination = getCorrespondingDestination(connector.activitySource);
    if (correspondingDestination !== undefined) {
      const isBounceback = await ctx.services.user.isBounceback(userId, correspondingDestination, externalId);
      if (isBounceback) {
        logger.info(`Bounceback detected: Activity ${externalId} was uploaded by us to ${connector.name}. Skipping.`);
        return { status: 'Skipped', reason: 'Bounceback - activity was uploaded by FitGlue' };
      }
    }

    // 7. Loop Prevention Check (Fallback)
    // Check if the incoming external ID exists as a destination in any synchronized activity.
    // This prevents infinite loops (e.g., Hevy → Strava → Hevy → ...)
    const isLoopActivity = await ctx.services.user.checkDestinationExists(userId, connector.name, externalId);
    if (isLoopActivity) {
      logger.info(`Loop detected: Activity ${externalId} was already posted by this system. Skipping.`);
      return { status: 'Skipped', reason: 'Loop prevention - activity was already posted as destination' };
    }

    // 8. Deduplication Check
    const alreadyProcessed = await ctx.services.user.hasProcessedActivity(userId, connector.name, externalId);
    if (alreadyProcessed) {
      logger.info(`Activity ${externalId} already processed for user ${userId}`);
      return { status: 'Skipped', reason: 'Already processed' };
    }

    // 8. Fetch & Map Activities
    let standardizedActivities: StandardizedActivity[];
    try {
      standardizedActivities = await connector.fetchAndMap(externalId, fullConfig);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (err: any) {
      logger.error(`Failed to fetch/map activity ${externalId}`, { error: err.message });
      // res.status(500).send('Failed to process activity');
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

      // Log virtual source execution so it appears in the trace
      // Since the root execution is attached to ctx.executionId, it won't show up in the trace for activityPipelineExecutionId
      // We manually create a record here to bridge that gap.
      const sourceExecutionId = `${ctx.executionId}-${activityExternalId}-source`;

      try {
        await ctx.services.execution.create(sourceExecutionId, {
          executionId: sourceExecutionId,
          pipelineExecutionId: activityPipelineExecutionId,
          service: connector.name,
          status: ExecutionStatus.STATUS_SUCCESS,
          triggerType: 'webhook',
          timestamp: timestamp,
          startTime: timestamp,
          endTime: new Date(),
          userId: userId,
          inputsJson: JSON.stringify({ id: externalId, activityId: activityExternalId }),
          outputsJson: JSON.stringify({ published: true, activityId: activityExternalId })
        });
      } catch (err) {
        logger.warn(`Failed to log source execution for ${activityExternalId}`, { error: err });
        // Proceed anyway, this is just for tracing visibility
      }

      const messagePayload: ActivityPayload = {
        source: connector.activitySource,
        userId: userId,
        timestamp: timestamp,
        originalPayloadJson: JSON.stringify({ id: externalId, activityId: activityExternalId, note: 'Fetched via Generic Connector' }),
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

    // Return standard success response (200 OK)
    return {
      status: 'Success',
      published: publishedIds.length,
      total: standardizedActivities.length
    };
  };
}
