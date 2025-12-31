import { createCloudFunction, FrameworkContext, TOPICS, createFitbitClient, UserService, ActivitySource } from '@fitglue/shared';
import { mapTCXToStandardized } from './mapper';

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { logger, db, pubsub } = ctx;

  // 1. Parse Pub/Sub Message
  // This function is triggered by Pub/Sub message from fitbit-webhook-handler
  // The payload is the Fitbit "notification" object: { collectionType, date, ownerId, ownerType, subscriptionId }
  let notification: any = req.body;
  if (req.body && req.body.message && req.body.message.data) {
    try {
      const dataBuffer = Buffer.from(req.body.message.data, 'base64');
      notification = JSON.parse(dataBuffer.toString());
    } catch (e) {
      logger.error('Failed to parse Pub/Sub message data');
      return;
    }
  }

  const { ownerId, date, collectionType } = notification;

  if (collectionType !== 'activities') {
    logger.info(`Skipping non-activity update`, { collectionType });
    // This is not an error, just a skip. We can return success with skipped status.
    return { status: 'skipped', reason: 'non_activity_update', collectionType };
  }

  logger.info(`Processing Fitbit update`, { ownerId, date });

  // 2. Resolve User
  const usersSnapshot = await db.collection('users')
    .where('integrations.fitbit.fitbit_user_id', '==', ownerId)
    .limit(1)
    .get();

  if (usersSnapshot.empty) {
    logger.warn(`No user found for Fitbit ID: ${ownerId}`);
    // This IS an error condition for the system logic
    throw new Error(`No user found for Fitbit ID: ${ownerId}`);
  }

  const userDoc = usersSnapshot.docs[0];
  const userId = userDoc.id;
  const userService = new UserService(db);

  // 3. Initialize Fitbit Client
  const client = await createFitbitClient(userService, userId);

  // 4. Fetch Activity Logs for the Date
  // Fitbit only gives us the Date. We have to fetch all activities for that date
  const { data: activityList, error: listError } = await client.GET("/1/user/-/activities/date/{date}.json", {
    params: {
      path: { date: date }
    }
  });

  if (listError || !activityList || !activityList.activities) {
    logger.error('Failed to fetch activity list', { error: listError });
    throw new Error('Fitbit API Error');
  }

  const activities = activityList.activities;
  logger.info(`Found ${activities.length} activities for date ${date}`);

  let publishedCount = 0;
  let errors = 0;
  const publishedIds: any[] = [];

  // 5. Process Activities
  for (const act of activities) {
    // We cannot check for `tcxLink` as it is missing from the Swagger definition.
    // We will attempt to fetch TCX for all activities.

    // Path parameter in Swagger for TCX endpoint is 'log-id' (kebab-case)
    const logIdStr = act.logId?.toString();
    if (!logIdStr) continue;

    logger.info(`Fetching TCX for activity ${act.logId}`);

    // Fetch TCX
    // Using correct path and parseAs: 'text' for XML response
    const { data: tcxData, error: tcxError } = await client.GET("/1/user/-/activities/{log-id}.tcx", {
      params: { path: { 'log-id': logIdStr } },
      parseAs: 'text'
    });

    if (tcxError || !tcxData) {
      // It's expected that many activities (manual logs, auto-detected walks) won't have TCX.
      // We log info instead of error to avoid noise.
      logger.info(`No TCX data for activity ${act.logId} (or fetch failed)`);
      continue;
    }

    // 6. Map to Standardized Activity
    try {
      // tcxData is string (XML)
      const standardized = mapTCXToStandardized(tcxData as string, act, userId);

      // 7. Publish to Enrichment Pipeline
      const messageId = await pubsub.topic(TOPICS.RAW_ACTIVITY).publishMessage({
        json: {
          source: ActivitySource.SOURCE_FITBIT,
          userId: userId,
          timestamp: new Date().toISOString(),
          standardizedActivity: standardized,
          originalPayloadJson: JSON.stringify(act),
          metadata: {
            fitbitLogId: act.logId,
            date: date
          }
        }
      });

      logger.info(`Published activity ${act.logId} to enrichment pipeline`, { messageId });
      publishedCount++;
      publishedIds.push(act.logId);

    } catch (mapErr) {
      logger.error(`Failed to map/publish activity ${act.logId}`, { error: mapErr });
      errors++;
    }
  }

  return {
    action: 'ingest',
    date,
    userId,
    foundActivities: activities.length,
    publishedCount,
    publishedIds,
    errors
  };
};

export const fitbitIngest = createCloudFunction(handler);
