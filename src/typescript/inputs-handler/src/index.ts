import { createCloudFunction, db, InputStore, InputService, FrameworkContext, CloudEventPublisher, getCloudEventType, CloudEventType, getCloudEventSource, CloudEventSource, ActivityPayload, FirebaseAuthStrategy, UserStore, ForbiddenError } from '@fitglue/shared';

// PubSub topic name logic via env var
const TOPIC = process.env.PUBSUB_TOPIC || 'activity-updates';


interface ResolveInputRequest {
  activityId: string;
  inputData: Record<string, string>;
}

import { Request, Response } from 'express';

// Handler Implementation
export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {

  const inputStore = new InputStore(db);
  const inputService = new InputService(inputStore, ctx.services.authorization);
  const userStore = new UserStore(db);

  const path = req.path;

  // --- Handlers ---

  // Handle FCM Token Registration FIRST specific paths
  if (req.method === 'POST' && (path === '/fcm-token' || path.endsWith('/fcm-token'))) {
    if (!ctx.userId) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    const { token } = req.body;
    if (!token) {
      res.status(400).json({ error: 'Missing token' });
      return;
    }

    try {
      await userStore.addFcmToken(ctx.userId, token);
      ctx.logger.info('Registered FCM token', { userId: ctx.userId });
      res.status(200).json({ success: true });
    } catch (e) {
      ctx.logger.error('Failed to register FCM token', { error: e });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  if (req.method === 'GET') {
    // User ID is guaranteed by Auth middleware in createCloudFunction
    if (!ctx.userId) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    try {
      const inputs = await inputService.listPendingInputs(ctx.userId);
      // Use standard camelCase (DTO matches Service object now)
      const responseInputs = inputs.map((i) => ({
        id: i.activityId, // Added id alias for frontend if needed, or just keep activityId
        activityId: i.activityId,
        userId: i.userId,
        status: i.status,
        requiredFields: i.requiredFields,
        createdAt: i.createdAt,
        inputData: i.inputData
      }));
      res.status(200).json({ inputs: responseInputs });
    } catch (e) {
      ctx.logger.error('Failed to list inputs', { error: e });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  if (req.method === 'POST') {
    if (!ctx.userId) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    const body = req.body as ResolveInputRequest;
    if (!body.activityId || !body.inputData) {
      res.status(400).json({ error: 'Missing activityId or inputData' });
      return;
    }

    try {
      const input = await inputService.getPendingInput(body.activityId);
      if (!input) {
        res.status(404).json({ error: 'Not found' });
        return;
      }

      // Service validates ownership and status
      await inputService.resolveInput(body.activityId, ctx.userId, body.inputData);

      // Re-publish Original Payload
      // Re-fetch (or use cached if service returns updated obj, but service returns void currently)
      // Since we didn't change payload, 'input' var has it.
      if (!input.originalPayload) {
        ctx.logger.error('Original payload missing', { activityId: body.activityId });
        res.status(500).json({ error: 'Original payload missing, cannot resume' });
        return;
      }

      // Re-publish using CloudEventPublisher
      const publisher = new CloudEventPublisher<ActivityPayload>(
        ctx.pubsub,
        TOPIC,
        getCloudEventSource(CloudEventSource.CLOUD_EVENT_SOURCE_INPUTS_HANDLER), // Source
        getCloudEventType(CloudEventType.CLOUD_EVENT_TYPE_INPUT_RESOLVED), // Type
        ctx.logger
      );

      await publisher.publish(input.originalPayload);

      ctx.logger.info(`Resolved and re-published activity`, { activityId: body.activityId });
      res.status(200).json({ success: true });

    } catch (e: unknown) {
      const err = e as { message?: string };
      ctx.logger.error('Failed to resolve input', { error: e });
      // Map common errors
      if (e instanceof ForbiddenError || err.message?.includes('Unauthorized')) {
        res.status(403).json({ error: e instanceof ForbiddenError ? e.message : 'Forbidden' });
      } else if (err.message?.includes('not found')) {
        res.status(404).json({ error: 'Not found' });
      } else if (err.message?.includes('status')) {
        res.status(409).json({ error: err.message });
      } else {
        res.status(500).json({ error: 'Internal Server Error' });
      }
    }
    return;
  }

  if (req.method === 'DELETE') {
    if (!ctx.userId) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    // Path is like /:activityId for delete, or possibly /api/inputs/:activityId depending on environment
    // Robustly extract the last non-empty segment
    const segments = path.split('/').filter(s => s.length > 0);
    const rawId = segments.pop();

    if (!rawId) {
      res.status(400).json({ error: 'Missing activityId' });
      return;
    }

    const activityId = decodeURIComponent(rawId);

    try {
      await inputService.dismissInput(activityId, ctx.userId);
      ctx.logger.info('Dismissed input', { activityId });
      res.status(200).json({ success: true });
    } catch (e: unknown) {
      const err = e as { message?: string };
      ctx.logger.error('Failed to dismiss input', { error: e, activityId });
      if (e instanceof ForbiddenError || err.message?.includes('Unauthorized')) {
        res.status(403).json({ error: e instanceof ForbiddenError ? e.message : 'Forbidden' });
      } else {
        res.status(500).json({ error: 'Internal Server Error' });
      }
    }
    return;
  }

  // --- User Handlers ---
  // (Moved to top priority check)

  res.status(405).send('Method Not Allowed');
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
