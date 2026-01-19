import { handler } from './index';

// Mock PubSub - must be before import
jest.mock('@google-cloud/pubsub', () => {
  const mockPublishMessage = jest.fn().mockResolvedValue('msg-123');
  return {
    PubSub: jest.fn().mockImplementation(() => ({
      topic: jest.fn().mockReturnValue({
        publishMessage: mockPublishMessage,
      }),
    })),
    __mockPublishMessage: mockPublishMessage,
  };
});

// Get reference to mock for assertions
const { __mockPublishMessage: mockPublishMessage } = jest.requireMock('@google-cloud/pubsub');

describe('repost-handler', () => {
  let res: any;
  let ctx: any;

  beforeEach(() => {
    mockPublishMessage.mockClear();

    res = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn(),
      send: jest.fn(),
      set: jest.fn(),
    };

    ctx = {
      userId: 'user-pro',
      logger: {
        info: jest.fn(),
        error: jest.fn(),
      },
      stores: {
        users: {
          get: jest.fn().mockResolvedValue({
            tier: 'pro',
            isAdmin: false,
          }),
        },
        activities: {
          getSynchronized: jest.fn(),
          updateDestination: jest.fn(),
        },
        executions: {
          getRouterExecution: jest.fn(),
          getEnricherExecution: jest.fn(),
        },
      },
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('Authentication & Authorization', () => {
    it('returns 401 if no user', async () => {
      ctx.userId = undefined;

      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(401);
      expect(res.json).toHaveBeenCalledWith({ error: 'Unauthorized' });
    });

    it('returns 403 for free tier users', async () => {
      ctx.stores.users.get.mockResolvedValue({ tier: 'free', isAdmin: false });

      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(403);
      expect(res.json).toHaveBeenCalledWith({ error: 'Pro tier required for re-post features' });
    });

    it('allows pro tier users', async () => {
      ctx.stores.users.get.mockResolvedValue({ tier: 'pro' });
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: {},
      });
      ctx.stores.executions.getRouterExecution.mockResolvedValue({
        id: 'exec-1',
        data: {
          inputsJson: JSON.stringify({ activityId: 'a1', userId: 'user-pro' }),
        },
      });

      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
    });

    it('allows admin users regardless of tier', async () => {
      ctx.stores.users.get.mockResolvedValue({ tier: 'free', isAdmin: true });
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: {},
      });
      ctx.stores.executions.getRouterExecution.mockResolvedValue({
        id: 'exec-1',
        data: {
          inputsJson: JSON.stringify({ activityId: 'a1', userId: 'user-pro' }),
        },
      });

      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
    });

    it('allows trial users', async () => {
      const futureDate = new Date();
      futureDate.setDate(futureDate.getDate() + 7);
      ctx.stores.users.get.mockResolvedValue({ tier: 'free', trialEndsAt: futureDate });
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: {},
      });
      ctx.stores.executions.getRouterExecution.mockResolvedValue({
        id: 'exec-1',
        data: {
          inputsJson: JSON.stringify({ activityId: 'a1', userId: 'user-pro' }),
        },
      });

      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
    });
  });

  describe('POST /missed-destination', () => {
    beforeEach(() => {
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: { strava: 'ext-123' },
      });
      ctx.stores.executions.getRouterExecution.mockResolvedValue({
        id: 'exec-1',
        data: {
          inputsJson: JSON.stringify({
            activityId: 'a1',
            userId: 'user-pro',
            destinations: [1], // DESTINATION_STRAVA
          }),
        },
      });
    });

    it('returns 400 if activityId missing', async () => {
      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { destination: 'showcase' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(400);
      expect(res.json).toHaveBeenCalledWith({ error: 'activityId and destination are required' });
    });

    it('returns 400 if destination missing', async () => {
      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(400);
    });

    it('returns 400 for invalid destination', async () => {
      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'invalid-dest' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(400);
      expect(res.json).toHaveBeenCalledWith({ error: 'Invalid destination: invalid-dest' });
    });

    it('returns 400 if destination already synced', async () => {
      // Activity already has strava synced
      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(400);
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({
        error: 'Activity already synced to strava',
      }));
    });

    it('returns 404 if activity not found', async () => {
      ctx.stores.activities.getSynchronized.mockResolvedValue(null);

      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'unknown', destination: 'showcase' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(404);
    });

    it('publishes message to correct topic on success', async () => {
      // Activity has strava but not showcase
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: { strava: 'ext-123' },
      });

      await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'showcase' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(mockPublishMessage).toHaveBeenCalled();

      const response = res.json.mock.calls[0][0];
      expect(response.success).toBe(true);
      expect(response.destination).toBe('showcase');
      expect(response.promptUpdatePipeline).toBe(true);
      expect(response.newPipelineExecutionId).toBeDefined();
    });
  });

  describe('POST /retry-destination', () => {
    beforeEach(() => {
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: { strava: 'ext-123' },
      });
      ctx.stores.executions.getRouterExecution.mockResolvedValue({
        id: 'exec-1',
        data: {
          inputsJson: JSON.stringify({
            activityId: 'a1',
            userId: 'user-pro',
          }),
        },
      });
    });

    it('returns 400 if activityId missing', async () => {
      await handler({
        method: 'POST',
        path: '/retry-destination',
        body: { destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(400);
    });

    it('publishes message with update method flag', async () => {
      await handler({
        method: 'POST',
        path: '/retry-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(mockPublishMessage).toHaveBeenCalled();

      // Check the message includes update marker
      const publishCall = mockPublishMessage.mock.calls[0][0];
      expect(publishCall.attributes.use_update_method).toBe('true');
      expect(publishCall.attributes.existing_external_id).toBe('ext-123');
    });

    it('works for destination without existing external ID', async () => {
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: {},
      });

      await handler({
        method: 'POST',
        path: '/retry-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      const publishCall = mockPublishMessage.mock.calls[0][0];
      expect(publishCall.attributes.use_update_method).toBe('false');
    });
  });

  describe('POST /full-pipeline', () => {
    beforeEach(() => {
      ctx.stores.activities.getSynchronized.mockResolvedValue({
        activityId: 'a1',
        pipelineExecutionId: 'pipe-1',
        destinations: { strava: 'ext-123' },
      });
      ctx.stores.executions.getEnricherExecution.mockResolvedValue({
        id: 'exec-1',
        data: {
          inputsJson: JSON.stringify({
            activityId: 'a1',
            userId: 'user-pro',
            source: 1, // SOURCE_HEVY
          }),
        },
      });
    });

    it('returns 400 if activityId missing', async () => {
      await handler({
        method: 'POST',
        path: '/full-pipeline',
        body: {},
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(400);
      expect(res.json).toHaveBeenCalledWith({ error: 'activityId is required' });
    });

    it('returns 404 if activity not found', async () => {
      ctx.stores.activities.getSynchronized.mockResolvedValue(null);

      await handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'unknown' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(404);
    });

    it('returns 500 if enricher execution not found', async () => {
      ctx.stores.executions.getEnricherExecution.mockResolvedValue(null);

      await handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'a1' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(500);
      expect(res.json).toHaveBeenCalledWith({
        error: 'Unable to retrieve original activity payload from execution logs'
      });
    });

    it('publishes message with bypass_dedup flag', async () => {
      await handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'a1' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(mockPublishMessage).toHaveBeenCalled();

      const publishCall = mockPublishMessage.mock.calls[0][0];
      expect(publishCall.attributes.repost_type).toBe('full_pipeline');
      expect(publishCall.attributes.bypass_dedup).toBe('true');
    });

    it('response includes warning about duplicates', async () => {
      await handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'a1' },
      } as any, res, ctx);

      const response = res.json.mock.calls[0][0];
      expect(response.success).toBe(true);
      expect(response.message).toContain('duplicate');
    });
  });

  describe('HTTP Method Handling', () => {
    it('returns 405 for non-POST methods', async () => {
      await handler({
        method: 'GET',
        path: '/missed-destination',
        body: {},
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(405);
    });

    it('returns 204 for OPTIONS (CORS preflight)', async () => {
      await handler({
        method: 'OPTIONS',
        path: '/missed-destination',
        body: {},
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(204);
    });

    it('returns 404 for unknown paths', async () => {
      await handler({
        method: 'POST',
        path: '/unknown-endpoint',
        body: { activityId: 'a1' },
      } as any, res, ctx);

      expect(res.status).toHaveBeenCalledWith(404);
    });
  });
});
