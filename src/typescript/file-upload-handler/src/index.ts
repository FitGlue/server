import {
  createCloudFunction,
  FrameworkContext,
  FirebaseAuthStrategy,
} from '@fitglue/shared';
import { CloudEventPublisher } from '@fitglue/shared/dist/infrastructure/pubsub/cloud-event-publisher';
import { ActivityPayload, ActivitySource } from '@fitglue/shared/dist/types/pb/activity';
import { CloudEventSource, CloudEventType } from '@fitglue/shared/dist/types/pb/events';
import { StandardizedActivity } from '@fitglue/shared/dist/types/pb/standardized_activity';
import { getCloudEventSource, getCloudEventType } from '@fitglue/shared/dist/types/events-helper';
import { TOPICS } from '@fitglue/shared/dist/config';
import { Request, Response } from 'express';
import { v4 as uuidv4 } from 'uuid';

/**
 * File Upload Handler
 *
 * Accepts a StandardizedActivity JSON payload (already parsed client-side using fit-inspect)
 * and publishes it to the raw-activity topic for enrichment and routing.
 *
 * The FIT file parsing happens on the client side (using the fit-inspect WASM module),
 * so this handler receives the already-parsed activity.
 */

interface FileUploadRequest {
  /** StandardizedActivity JSON (parsed client-side from FIT file) */
  activity: StandardizedActivity;
  /** Optional user-provided title override */
  title?: string;
  /** Optional user-provided description */
  description?: string;
}

const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  if (req.method !== 'POST') {
    res.status(405).json({ error: 'Method not allowed' });
    return;
  }

  if (!ctx.userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  const { logger, userId, pubsub } = ctx;

  try {
    const { activity, title, description } = req.body as FileUploadRequest;

    if (!activity) {
      res.status(400).json({ error: 'activity is required' });
      return;
    }

    // Validate basic activity structure
    if (!activity.startTime) {
      res.status(400).json({ error: 'activity.startTime is required' });
      return;
    }

    // Check if user has a pipeline for file-upload source
    const user = await ctx.services.user.get(userId);
    if (!user) {
      res.status(404).json({ error: 'User not found' });
      return;
    }

    const hasPipeline = user.pipelines?.some(p => p.source === 'SOURCE_FILE_UPLOAD') ?? false;
    if (!hasPipeline) {
      res.status(400).json({
        error: 'No pipeline configured for File Upload source. Create a pipeline first.',
      });
      return;
    }

    // Generate unique external ID for this upload
    const externalId = `upload_${uuidv4()}`;

    // Apply user overrides
    const enrichedActivity: StandardizedActivity = {
      ...activity,
      userId: userId,
      source: 'FILE_UPLOAD',
      externalId: externalId,
      name: title || activity.name || 'Uploaded Activity',
      description: description || activity.description || '',
    };

    // Create activity payload
    const payload: ActivityPayload = {
      source: ActivitySource.SOURCE_FILE_UPLOAD,
      userId: userId,
      timestamp: new Date(),
      originalPayloadJson: JSON.stringify({ uploadedAt: new Date().toISOString() }),
      metadata: {
        connector: 'file-upload',
        uploadMethod: 'direct',
      },
      standardizedActivity: enrichedActivity,
      pipelineExecutionId: ctx.executionId,
      isResume: false,
      resumeOnlyEnrichers: [],
      useUpdateMethod: false,
    };

    // Publish to raw-activity topic
    const publisher = new CloudEventPublisher<ActivityPayload>(
      pubsub,
      TOPICS.RAW_ACTIVITY,
      getCloudEventSource(CloudEventSource.CLOUD_EVENT_SOURCE_FILE_UPLOAD),
      getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_ACTIVITY_CREATED),
      logger
    );

    const messageId = await publisher.publish(payload, externalId);

    logger.info('File upload published', { messageId, externalId, userId });

    res.status(200).json({
      success: true,
      message: 'Activity uploaded and queued for processing',
      activityId: externalId,
    });
  } catch (error) {
    ctx.logger.error('File upload failed', { error, userId: ctx.userId });
    res.status(500).json({ error: 'Failed to process uploaded file' });
  }
};

export const fileUploadHandler = createCloudFunction(handler, {
  auth: { strategies: [new FirebaseAuthStrategy()] },
});
