/**
 * Mobile Source Handler
 *
 * Pub/Sub-triggered Cloud Function that processes mobile health activities.
 * Consumes messages from topic-mobile-activity (published by mobile-sync-handler),
 * fetches Firestore metadata + GCS telemetry, maps to StandardizedActivity,
 * and publishes to topic-raw-activity for the main pipeline.
 *
 * Flow:
 * 1. Receive MobileActivityMessage from topic-mobile-activity
 * 2. Fetch activity metadata from Firestore (users/{userId}/mobile_activities/{activityId})
 * 3. If telemetryUri present, fetch telemetry blob from GCS
 * 4. Map to StandardizedActivity using mapper
 * 5. Publish ActivityPayload to topic-raw-activity
 * 6. Update Firestore status to 'published'
 */

// Module-level imports for smart pruning
import { createCloudFunction, FrameworkHandler, db } from '@fitglue/shared/framework';
import { CloudEventPublisher } from '@fitglue/shared/infrastructure/pubsub';
import {
    ActivityPayload,
    ActivitySource,
    CloudEventType,
    CloudEventSource,
    getCloudEventSource,
    getCloudEventType,
} from '@fitglue/shared/types';
import { getStorage } from 'firebase-admin/storage';

import { mapToStandardizedActivity, MobileActivityMetadata, TelemetryData } from './mapper';

/**
 * Message shape published by mobile-sync-handler to topic-mobile-activity
 */
interface MobileActivityMessage {
    userId: string;
    activityId: string;
    source: 'healthkit' | 'health_connect';
    telemetryUri?: string;
}

/**
 * Parse the Pub/Sub message from the request body.
 *
 * For CloudEvent triggers, the framework unwraps the Pub/Sub envelope
 * and CloudEvent wrapper, so req.body should contain the direct payload.
 * Falls back to manual Pub/Sub decoding for edge cases.
 */
function parseMessage(req: { body: unknown }): MobileActivityMessage {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const body = req.body as any;

    // Direct payload (framework already unwrapped)
    if (body && body.userId && body.activityId && body.source) {
        return body as MobileActivityMessage;
    }

    // Fallback: manual Pub/Sub envelope decoding
    if (body?.message?.data && typeof body.message.data === 'string') {
        try {
            const decoded = Buffer.from(body.message.data, 'base64').toString('utf-8');
            const parsed = JSON.parse(decoded);

            // Handle CloudEvent wrapper
            if (parsed.specversion && parsed.data) {
                return parsed.data as MobileActivityMessage;
            }

            return parsed as MobileActivityMessage;
        } catch {
            // Fall through to error
        }
    }

    throw new Error(
        'Invalid message format: missing userId, activityId, or source. ' +
        `Got keys: ${body ? Object.keys(body).join(', ') : 'null'}`
    );
}

/**
 * Fetch telemetry data from GCS
 */
async function fetchTelemetryFromGcs(telemetryUri: string): Promise<TelemetryData | undefined> {
    if (!telemetryUri || !telemetryUri.startsWith('gs://')) {
        return undefined;
    }

    const withoutProtocol = telemetryUri.replace('gs://', '');
    const slashIndex = withoutProtocol.indexOf('/');
    const bucketName = withoutProtocol.substring(0, slashIndex);
    const filePath = withoutProtocol.substring(slashIndex + 1);

    const storage = getStorage();
    const file = storage.bucket(bucketName).file(filePath);
    const [exists] = await file.exists();

    if (!exists) {
        return undefined;
    }

    const [buffer] = await file.download();
    return JSON.parse(buffer.toString('utf-8')) as TelemetryData;
}

/**
 * Main handler — receives Pub/Sub message from topic-mobile-activity
 */
export const handler: FrameworkHandler = async (req, ctx) => {
    const { logger } = ctx;

    // 1. Parse the incoming message
    const message = parseMessage(req);
    const { userId, activityId, source, telemetryUri } = message;

    logger.info('Processing mobile activity', { userId, activityId, source });

    // 2. Pipeline Check — bail early if no pipeline is configured for this source
    // Mirrors the 'Step 5: Pipeline Check' in createWebhookProcessor (hevy-handler etc.)
    const sourceEnumName = source === 'healthkit'
        ? ActivitySource[ActivitySource.SOURCE_APPLE_HEALTH]
        : ActivitySource[ActivitySource.SOURCE_HEALTH_CONNECT];

    const pipelinesSnapshot = await db
        .collection('users')
        .doc(userId)
        .collection('pipelines')
        .where('source', '==', sourceEnumName)
        .where('disabled', '==', false)
        .limit(1)
        .get();

    if (pipelinesSnapshot.empty) {
        logger.info('No pipeline configured for source, skipping', { userId, source, sourceEnumName });
        return { status: 'skipped', reason: 'no_pipeline_for_source', source: sourceEnumName };
    }

    // 3. Fetch activity metadata from Firestore
    const activityDoc = await db
        .collection('users')
        .doc(userId)
        .collection('mobile_activities')
        .doc(activityId)
        .get();

    if (!activityDoc.exists) {
        logger.error('Activity not found in Firestore', { userId, activityId });
        return { status: 'skipped', reason: 'activity_not_found' };
    }

    const metadata = activityDoc.data() as MobileActivityMetadata;

    // 4. Fetch telemetry from GCS if available
    let telemetry: TelemetryData | undefined;
    const effectiveTelemetryUri = telemetryUri || metadata.telemetryUri;
    if (effectiveTelemetryUri) {
        try {
            telemetry = await fetchTelemetryFromGcs(effectiveTelemetryUri);
            logger.info('Telemetry fetched from GCS', {
                activityId,
                hrSamples: telemetry?.heartRateSamples?.length || 0,
                routePoints: telemetry?.route?.length || 0,
            });
        } catch (err) {
            logger.warn('Failed to fetch telemetry from GCS, proceeding without', {
                activityId,
                error: err instanceof Error ? err.message : String(err),
            });
        }
    }

    // 5. Map to StandardizedActivity
    const standardizedActivity = mapToStandardizedActivity(metadata, telemetry);

    // 6. Build ActivityPayload and publish to topic-raw-activity
    const activitySource = source === 'healthkit'
        ? ActivitySource.SOURCE_APPLE_HEALTH
        : ActivitySource.SOURCE_HEALTH_CONNECT;

    const cloudEventSource = source === 'healthkit'
        ? CloudEventSource.CLOUD_EVENT_SOURCE_APPLE_HEALTH
        : CloudEventSource.CLOUD_EVENT_SOURCE_HEALTH_CONNECT;

    const pipelineExecutionId = `mobile-${activityId}-${Date.now()}`;

    const payload: ActivityPayload = {
        source: activitySource,
        userId,
        originalPayloadJson: JSON.stringify(metadata),
        metadata: {
            received_at: new Date().toISOString(),
            source_handler: 'mobile-source-handler',
            activity_id: activityId,
            mobile_source: source,
        },
        standardizedActivity,
        pipelineExecutionId,
        isResume: false,
        resumeOnlyEnrichers: [],
        useUpdateMethod: false,
    };

    const publisher = new CloudEventPublisher<ActivityPayload>(
        ctx.pubsub,
        'topic-raw-activity',
        getCloudEventSource(cloudEventSource),
        getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_CREATED),
        logger
    );

    const messageId = await publisher.publish(payload, activityId);

    // 7. Update Firestore status to 'published'
    await db
        .collection('users')
        .doc(userId)
        .collection('mobile_activities')
        .doc(activityId)
        .update({
            status: 'published',
            publishedAt: new Date(),
            pipelineExecutionId,
        });

    logger.info('Mobile activity published to pipeline', {
        activityId,
        messageId,
        pipelineExecutionId,
        type: standardizedActivity.type,
        hasTelemetry: !!telemetry,
    });

    return { status: 'published', activityId, messageId, pipelineExecutionId };
};

// Export the wrapped function — Pub/Sub triggered, no user auth needed
export const mobileSourceHandler = createCloudFunction(handler, {
    allowUnauthenticated: true,
});
