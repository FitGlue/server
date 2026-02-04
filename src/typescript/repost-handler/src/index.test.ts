import { handler } from './index';

// Mock PubSub - must be before import
jest.mock('@google-cloud/pubsub', () => {
  const mockPublishMessage = jest.fn().mockResolvedValue('msg-123');
  const mockTopic = jest.fn().mockReturnValue({
    publishMessage: mockPublishMessage,
  });
  return {
    PubSub: jest.fn().mockImplementation(() => ({
      topic: mockTopic,
    })),
    __mockPublishMessage: mockPublishMessage,
    __mockTopic: mockTopic,
  };
});

// Get reference to mocks for assertions
const { __mockPublishMessage: mockPublishMessage, __mockTopic: mockTopic } = jest.requireMock('@google-cloud/pubsub');

describe('repost-handler', () => {

  let ctx: any;
  beforeEach(() => {
    mockPublishMessage.mockClear();
    mockTopic.mockClear();



    ctx = {
      userId: 'user-pro',
      logger: {
        info: jest.fn(),
        error: jest.fn(),
      },
      stores: {
        users: {
          get: jest.fn().mockResolvedValue({
            tier: 2, // USER_TIER_ATHLETE
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
        pipelineRuns: {
          get: jest.fn().mockResolvedValue(null), // Fallback to executions
          findByActivityId: jest.fn().mockResolvedValue(null),
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

      await expect(handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('returns 403 for hobbyist tier users', async () => {
      ctx.stores.users.get.mockResolvedValue({ tier: 1, isAdmin: false }); // USER_TIER_HOBBYIST

      await expect(handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 403 }));
    });

    it('allows athlete tier users', async () => {
      ctx.stores.users.get.mockResolvedValue({ tier: 2 }); // USER_TIER_ATHLETE
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [],
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
      } as any, ctx);
    });

    it('allows admin users regardless of tier', async () => {
      ctx.stores.users.get.mockResolvedValue({ tier: 1, isAdmin: true }); // USER_TIER_HOBBYIST but admin
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [],
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
      } as any, ctx);
    });

    it('allows trial users', async () => {
      const futureDate = new Date();
      futureDate.setDate(futureDate.getDate() + 7);
      ctx.stores.users.get.mockResolvedValue({ tier: 1, trialEndsAt: futureDate }); // USER_TIER_HOBBYIST with trial
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [],
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
      } as any, ctx);
    });
  });

  describe('POST /missed-destination', () => {
    beforeEach(() => {
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [{ destination: 1, externalId: 'ext-123' }], // DESTINATION_STRAVA = 1
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
      await expect(handler({
        method: 'POST',
        path: '/missed-destination',
        body: { destination: 'showcase' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 400 if destination missing', async () => {
      await expect(handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 400 for invalid destination', async () => {
      await expect(handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'invalid-dest' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 400 if destination already synced', async () => {
      // Activity already has strava synced
      await expect(handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 404 if activity not found', async () => {
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue(null);

      await expect(handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'unknown', destination: 'showcase' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('publishes message to correct topic on success', async () => {
      // Activity has strava but not showcase
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [{ destination: 1, externalId: 'ext-123' }], // strava synced
      });

      const result: any = await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'showcase' },
      } as any, ctx);

      expect(mockPublishMessage).toHaveBeenCalled();

      expect(result.success).toBe(true);
      expect(result.destination).toBe('showcase');
      expect(result.promptUpdatePipeline).toBe(true);
      expect(result.newPipelineExecutionId).toBeDefined();
    });

    it('handles snake_case field names from Go framework (activity_id/user_id)', async () => {
      // The Go framework stores CloudEvent data with snake_case field names
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [],
      });
      ctx.stores.executions.getRouterExecution.mockResolvedValue({
        id: 'exec-1',
        data: {
          // Snake_case format from Go's json.Marshal of proto
          inputsJson: JSON.stringify({
            activity_id: 'a1',
            user_id: 'user-pro',
            pipeline_id: 'pipeline-1',
            pipeline_execution_id: 'pipe-1',
            destinations: [1],
          }),
        },
      });

      const result: any = await handler({
        method: 'POST',
        path: '/missed-destination',
        body: { activityId: 'a1', destination: 'showcase' },
      } as any, ctx);

      expect(mockPublishMessage).toHaveBeenCalled();
      expect(result.success).toBe(true);
    });
  });

  describe('POST /retry-destination', () => {
    beforeEach(() => {
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [{ destination: 1, externalId: 'ext-123' }], // DESTINATION_STRAVA = 1
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
      await expect(handler({
        method: 'POST',
        path: '/retry-destination',
        body: { destination: 'strava' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('publishes message with update method flag', async () => {
      await handler({
        method: 'POST',
        path: '/retry-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, ctx);

      expect(mockPublishMessage).toHaveBeenCalled();

      // Check the message includes update marker
      const publishCall = mockPublishMessage.mock.calls[0][0];
      expect(publishCall.attributes.use_update_method).toBe('true');
      expect(publishCall.attributes.existing_external_id).toBe('ext-123');
    });

    it('works for destination without existing external ID', async () => {
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [],
      });

      await handler({
        method: 'POST',
        path: '/retry-destination',
        body: { activityId: 'a1', destination: 'strava' },
      } as any, ctx);
      const publishCall = mockPublishMessage.mock.calls[0][0];
      expect(publishCall.attributes.use_update_method).toBe('false');
    });
  });

  describe('POST /full-pipeline', () => {
    beforeEach(() => {
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        destinations: [{ destination: 1, externalId: 'ext-123' }],
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
      await expect(handler({
        method: 'POST',
        path: '/full-pipeline',
        body: {},
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 404 if activity not found', async () => {
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue(null);

      await expect(handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'unknown' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('returns 500 if enricher execution not found', async () => {
      ctx.stores.executions.getEnricherExecution.mockResolvedValue(null);

      await expect(handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'a1' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 500 }));
    });

    it('publishes message with bypass_dedup flag wrapped in CloudEvent', async () => {
      await handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'a1' },
      } as any, ctx);

      expect(mockPublishMessage).toHaveBeenCalled();

      const publishCall = mockPublishMessage.mock.calls[0][0];
      const publishedData = JSON.parse(publishCall.data.toString());

      // Should be a CloudEvent
      expect(publishedData.specversion).toBe('1.0');
      expect(publishedData.type).toBe('com.fitglue.activity.pipeline');
      expect(publishedData.data.bypass_dedup).toBe(true);
      expect(publishedData.data.activity_id).toBe('a1');

      expect(publishCall.attributes.repost_type).toBe('full_pipeline');
      expect(publishCall.attributes.bypass_dedup).toBe('true');
    });

    it('response includes warning about duplicates', async () => {
      const result: any = await handler({
        method: 'POST',
        path: '/full-pipeline',
        body: { activityId: 'a1' },
      } as any, ctx);

      expect(result.success).toBe(true);
      expect(result.message).toContain('duplicate');
    });
  });

  describe('HTTP Method Handling', () => {
    it('returns 405 for non-POST methods', async () => {
      await expect(handler({
        method: 'GET',
        path: '/missed-destination',
        body: {},
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 405 }));
    });



    it('returns 404 for unknown paths', async () => {
      await expect(handler({
        method: 'POST',
        path: '/unknown-endpoint',
        body: { activityId: 'a1' },
      } as any, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });
  });
});
