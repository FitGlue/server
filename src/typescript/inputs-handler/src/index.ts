// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError, ForbiddenError } from '@fitglue/shared/errors';
import { InputStore, UserStore } from '@fitglue/shared/storage';
import { InputService } from '@fitglue/shared/domain/services';
import { CloudEventPublisher } from '@fitglue/shared/infrastructure/pubsub';
import { ActivityPayload, CloudEventType, CloudEventSource, getCloudEventType, getCloudEventSource } from '@fitglue/shared/types';
import { Storage } from '@google-cloud/storage';

const storage = new Storage();

// PubSub topic for resume: Pipeline activity topic (bypasses splitter since pipelineId is already set)
const RESUME_TOPIC = 'topic-pipeline-activity';


interface ResolveInputRequest {
  activityId: string;
  inputData: Record<string, string>;
}


// Handler Implementation
// eslint-disable-next-line complexity
export const handler: FrameworkHandler = async (req, ctx) => {

  const inputStore = new InputStore(db);
  const inputService = new InputService(inputStore, ctx.services.authorization);
  const userStore = new UserStore(db);

  const path = req.path;

  // --- Handlers ---

  // Handle FCM Token Registration FIRST specific paths
  if (req.method === 'POST' && (path === '/fcm-token' || path.endsWith('/fcm-token'))) {
    if (!ctx.userId) {
      throw new HttpError(401, 'Unauthorized');
    }

    const { token } = req.body;
    if (!token) {
      throw new HttpError(400, 'Missing token');
    }

    await userStore.addFcmToken(ctx.userId, token);
    ctx.logger.info('Registered FCM token', { userId: ctx.userId });
    return { success: true };
  }

  if (req.method === 'GET') {
    // User ID is guaranteed by Auth middleware in createCloudFunction
    if (!ctx.userId) {
      throw new HttpError(401, 'Unauthorized');
    }

    const inputs = await inputService.listPendingInputs(ctx.userId);
    // Use standard camelCase (DTO matches Service object now)
    const responseInputs = inputs.map((i) => ({
      id: i.activityId, // Added id alias for frontend if needed, or just keep activityId
      activityId: i.activityId,
      userId: i.userId,
      status: i.status,
      requiredFields: i.requiredFields,
      createdAt: i.createdAt,
      inputData: i.inputData,
      // Additional fields for proper UI display
      pipelineId: i.pipelineId,
      enricherProviderId: i.enricherProviderId,
      autoPopulated: i.autoPopulated,
      autoDeadline: i.autoDeadline,
      linkedActivityId: i.linkedActivityId
    }));
    return { inputs: responseInputs };
  }

  if (req.method === 'POST') {
    if (!ctx.userId) {
      throw new HttpError(401, 'Unauthorized');
    }

    const body = req.body as ResolveInputRequest;
    if (!body.activityId || !body.inputData) {
      throw new HttpError(400, 'Missing activityId or inputData');
    }

    try {
      const input = await inputService.getPendingInput(ctx.userId, body.activityId);
      if (!input) {
        throw new HttpError(404, 'Not found');
      }

      // Service validates ownership and status
      await inputService.resolveInput(ctx.userId, body.activityId, ctx.userId, body.inputData);

      // Fetch Original Payload from GCS for re-publish
      if (!input.originalPayloadUri) {
        ctx.logger.error('Original payload URI missing', { activityId: body.activityId });
        throw new HttpError(500, 'Original payload URI missing, cannot resume');
      }

      // Parse GCS URI and fetch payload
      const uriMatch = input.originalPayloadUri.match(/^gs:\/\/([^/]+)\/(.+)$/);
      if (!uriMatch) {
        throw new HttpError(500, `Invalid GCS URI: ${input.originalPayloadUri}`);
      }
      const [, bucket, objectPath] = uriMatch;

      const [payloadBuffer] = await storage.bucket(bucket).file(objectPath).download();
      const payload = JSON.parse(payloadBuffer.toString('utf-8')) as ActivityPayload;

      // Set resume mode flags so the enricher calls EnrichResume instead of Enrich
      // Without these, the enricher runs in normal mode and recreates the pending input
      payload.isResume = true;
      payload.resumePendingInputId = body.activityId;

      // Transfer linkedActivityId to activityId - the enricher requires this in resume mode
      // to look up the existing activity record created during initial enrichment
      if (!input.linkedActivityId) {
        ctx.logger.error('Missing linkedActivityId on pending input - cannot resume', {
          activityId: body.activityId,
          pipelineId: input.pipelineId
        });
        throw new HttpError(500, 'Pending input missing linkedActivityId, cannot resume');
      }
      payload.activityId = input.linkedActivityId;

      // Re-publish using CloudEventPublisher
      const publisher = new CloudEventPublisher<ActivityPayload>(
        ctx.pubsub,
        RESUME_TOPIC,
        getCloudEventSource(CloudEventSource.CLOUD_EVENT_SOURCE_INPUTS_HANDLER), // Source
        getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_INPUT_RESOLVED), // Type
        ctx.logger
      );

      await publisher.publish(payload);

      ctx.logger.info('Resolved and re-published activity to enricher', { activityId: body.activityId, topic: RESUME_TOPIC });
      return { success: true };

    } catch (e: unknown) {
      const err = e as { message?: string };
      // Map common errors
      if (e instanceof ForbiddenError || err.message?.includes('Unauthorized')) {
        throw new HttpError(403, e instanceof ForbiddenError ? e.message : 'Forbidden');
      } else if (err.message?.includes('not found')) {
        throw new HttpError(404, 'Not found');
      } else if (err.message?.includes('status')) {
        throw new HttpError(409, err.message);
      }
      // Bubble others
      throw e;
    }
  }

  if (req.method === 'DELETE') {
    if (!ctx.userId) {
      throw new HttpError(401, 'Unauthorized');
    }

    // Path is like /:activityId for delete, or possibly /api/inputs/:activityId depending on environment
    // Robustly extract the last non-empty segment
    const segments = path.split('/').filter(s => s.length > 0);
    const rawId = segments.pop();

    if (!rawId) {
      throw new HttpError(400, 'Missing activityId');
    }

    const activityId = decodeURIComponent(rawId);

    await inputService.dismissInput(ctx.userId, activityId, ctx.userId);
    ctx.logger.info('Dismissed input', { activityId });
    return { success: true };
  }

  // --- User Handlers ---
  // (Moved to top priority check)

  throw new HttpError(405, 'Method Not Allowed');
};

// Export the wrapped function
export const inputsHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  },
  skipExecutionLogging: true
});
