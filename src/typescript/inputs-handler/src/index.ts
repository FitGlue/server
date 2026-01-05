import { createCloudFunction, db, InputStore, InputService, FrameworkContext, CloudEventPublisher, getCloudEventType, CloudEventType, getCloudEventSource, CloudEventSource, ActivityPayload, FirebaseAuthStrategy, UserStore } from '@fitglue/shared';

// PubSub topic name logic via env var
const TOPIC = process.env.PUBSUB_TOPIC || 'activity-updates';


interface ResolveInputRequest {
  activity_id: string;
  input_data: Record<string, string>;
}

import { Request, Response } from 'express';

// Handler Implementation
export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {

  const inputStore = new InputStore(db);
  const inputService = new InputService(inputStore);
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
      // Omit original_payload for list view
      const responseInputs = inputs.map((i) => ({
        activity_id: i.activityId,
        user_id: i.userId,
        status: i.status,
        required_fields: i.requiredFields,
        created_at: i.createdAt,
        input_data: i.inputData
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
    if (!body.activity_id || !body.input_data) {
      res.status(400).json({ error: 'Missing activity_id or input_data' });
      return;
    }

    try {
      const input = await inputService.getPendingInput(body.activity_id);
      if (!input) {
        res.status(404).json({ error: 'Not found' });
        return;
      }

      // Service validates ownership and status
      await inputService.resolveInput(body.activity_id, ctx.userId, body.input_data);

      // Re-publish Original Payload
      // Re-fetch (or use cached if service returns updated obj, but service returns void currently)
      // Since we didn't change payload, 'input' var has it.
      if (!input.originalPayload) {
        ctx.logger.error('Original payload missing', { activityId: body.activity_id });
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

      ctx.logger.info(`Resolved and re-published activity`, { activityId: body.activity_id });
      res.status(200).json({ success: true });

    } catch (e: unknown) {
      const err = e as { message?: string };
      ctx.logger.error('Failed to resolve input', { error: e });
      // Map common errors
      if (err.message?.includes('Unauthorized')) {
        res.status(403).json({ error: 'Forbidden' });
      } else if (err.message?.includes('not found')) { // unlikely if we checked exists, but race condition
        res.status(404).json({ error: 'Not found' });
      } else if (err.message?.includes('status')) {
        res.status(409).json({ error: err.message });
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
  }
});
