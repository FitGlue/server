// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkContext, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError, ForbiddenError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch, RoutableRequest } from '@fitglue/shared/routing';
import { InputStore, UserStore, PipelineRunStore } from '@fitglue/shared/storage';
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
export const handler: FrameworkHandler = async (req, ctx) => {

  const inputStore = new InputStore(db);
  const pipelineRunStore = new PipelineRunStore(db);
  const inputService = new InputService(inputStore, ctx.services.authorization, pipelineRunStore);
  const userStore = new UserStore(db);

  if (!ctx.userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  const userId = ctx.userId;

  return await routeRequest(req as RoutableRequest, ctx, [
    {
      method: 'POST',
      pattern: '*/fcm-token',
      handler: async () => handleFcmToken(userId, req.body, userStore, ctx.logger),
    },
    {
      method: 'GET',
      pattern: '*',
      handler: async () => handleListInputs(userId, inputService),
    },
    {
      method: 'POST',
      pattern: '*',
      handler: async () => handleResolveInput(userId, req.body as ResolveInputRequest, inputService, ctx),
    },
    {
      method: 'DELETE',
      pattern: '*/inputs/:activityId',
      handler: async (match: RouteMatch) => handleDismissInput(
        userId, match.params.activityId, inputService, ctx.logger
      ),
    },
  ]);
};

// --- FCM Token Registration ---
async function handleFcmToken(
  userId: string,
  body: Record<string, unknown>,
  userStore: UserStore,
  logger: { info: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<{ success: boolean }> {
  const { token } = body;
  if (!token) {
    throw new HttpError(400, 'Missing token');
  }

  await userStore.addFcmToken(userId, token as string);
  logger.info('Registered FCM token', { userId });
  return { success: true };
}

// --- List Pending Inputs ---
async function handleListInputs(
  userId: string,
  inputService: InputService
): Promise<{ inputs: Record<string, unknown>[] }> {
  const inputs = await inputService.listPendingInputs(userId);
  const responseInputs = inputs.map((i) => ({
    id: i.activityId,
    activityId: i.activityId,
    userId: i.userId,
    status: i.status,
    requiredFields: i.requiredFields,
    createdAt: i.createdAt,
    inputData: i.inputData,
    pipelineId: i.pipelineId,
    enricherProviderId: i.enricherProviderId,
    autoPopulated: i.autoPopulated,
    autoDeadline: i.autoDeadline,
    linkedActivityId: i.linkedActivityId
  }));
  return { inputs: responseInputs };
}

// --- Resolve Input ---
async function handleResolveInput(
  userId: string,
  body: ResolveInputRequest,
  inputService: InputService,
  ctx: FrameworkContext
): Promise<{ success: boolean }> {
  if (!body.activityId || !body.inputData) {
    throw new HttpError(400, 'Missing activityId or inputData');
  }

  try {
    const input = await inputService.getPendingInput(userId, body.activityId);
    if (!input) {
      throw new HttpError(404, 'Not found');
    }

    // Service validates ownership and status
    await inputService.resolveInput(userId, body.activityId, userId, body.inputData);

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
    payload.isResume = true;
    payload.resumePendingInputId = body.activityId;

    // Transfer linkedActivityId to activityId
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
      getCloudEventSource(CloudEventSource.CLOUD_EVENT_SOURCE_INPUTS_HANDLER),
      getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_INPUT_RESOLVED),
      ctx.logger
    );

    await publisher.publish(payload);

    ctx.logger.info('Resolved and re-published activity to enricher', { activityId: body.activityId, topic: RESUME_TOPIC });
    return { success: true };

  } catch (e: unknown) {
    const err = e as { message?: string };
    if (e instanceof ForbiddenError || err.message?.includes('Unauthorized')) {
      throw new HttpError(403, e instanceof ForbiddenError ? e.message : 'Forbidden');
    } else if (err.message?.includes('not found')) {
      throw new HttpError(404, 'Not found');
    } else if (err.message?.includes('status')) {
      throw new HttpError(409, err.message);
    }
    throw e;
  }
}

// --- Dismiss Input ---
async function handleDismissInput(
  userId: string,
  activityId: string,
  inputService: InputService,
  logger: { info: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<{ success: boolean }> {
  await inputService.dismissInput(userId, decodeURIComponent(activityId), userId);
  logger.info('Dismissed input', { activityId });
  return { success: true };
}
export const inputsHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  },
  skipExecutionLogging: true
});
