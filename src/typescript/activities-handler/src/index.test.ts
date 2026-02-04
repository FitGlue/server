import { handler } from './index';
import { CloudEventPublisher } from '@fitglue/shared/dist/infrastructure/pubsub';

// Mock shared dependencies
jest.mock('@fitglue/shared/dist/infrastructure/pubsub', () => {
  return {
    CloudEventPublisher: jest.fn(),
  };
});

describe('activities-handler', () => {

  let ctx: any;
  let mockPublish: any;

  beforeEach(() => {
    mockPublish = jest.fn();
    (CloudEventPublisher as any).mockImplementation(() => ({
      publish: mockPublish
    }));


    ctx = {
      userId: 'user-1',
      logger: {
        info: jest.fn(),
        error: jest.fn(),
      },
      pubsub: {}, // Mock pubsub object (CloudEventPublisher uses it, but we mocked the class)
      services: {
        execution: {
          listByPipeline: jest.fn(),
        }
      },
      stores: {
        activities: {
          countSynchronized: jest.fn(),
          listSynchronized: jest.fn(),
          getSynchronized: jest.fn(),
          getSynchronizedPipelineIds: jest.fn(),
        },
        pipelineRuns: {
          list: jest.fn(),
          countSynced: jest.fn(),
          listSynced: jest.fn(),
          findByActivityId: jest.fn(),
        },
        executions: {
          listDistinctPipelines: jest.fn(),
          batchListByPipelines: jest.fn(),
        }
      }
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET', () => {
    it('/ returns 401 if no user', async () => {
      ctx.userId = undefined;
      // Should throw HttpError
      await expect(handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '/api/activities',
      } as any), ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('/ returns list of synchronized activities', async () => {
      ctx.stores.pipelineRuns.listSynced.mockResolvedValue([{
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        title: 'Activity 1',
        description: 'Description 1',
        type: 5, // ACTIVITY_TYPE_CROSSFIT
        source: 'SOURCE_HEVY',
        destinations: [],
      }]);

      const result: any = await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '/api/activities',
      } as any), ctx);

      expect(result.activities).toHaveLength(1);
      expect(result.activities[0].activityId).toBe('a1');
      expect(result.activities[0].title).toBe('Activity 1');
      expect(result.activities[0].type).toBe('Crossfit');
      expect(result.activities[0].source).toBe('Hevy');
    });

    it('/ with includeExecution=true returns activities with pipelineExecution', async () => {
      ctx.stores.pipelineRuns.listSynced.mockResolvedValue([{
        id: 'pipeline-123',
        activityId: 'a1',
        pipelineId: 'pipeline-def-1',
        title: 'Enhanced Workout',
        type: 46,
        source: 'SOURCE_HEVY',
        destinations: [],
      }]);

      const executionsMap = new Map();
      executionsMap.set('pipeline-123', [{
        id: 'exec-1',
        data: {
          service: 'enricher',
          status: 2, // STATUS_SUCCESS
          timestamp: new Date('2026-01-15T10:00:00Z'),
          outputsJson: JSON.stringify({ provider_executions: [{ ProviderName: 'muscle-heatmap', Status: 'SUCCESS' }] }),
        },
      }]);
      ctx.stores.executions.batchListByPipelines.mockResolvedValue(executionsMap);

      const result: any = await handler(({
        method: 'GET',
        body: {},
        query: { includeExecution: 'true' },
        path: '/api/activities',
      } as any), ctx);

      expect(result.activities).toHaveLength(1);
      expect(result.activities[0].pipelineExecution).toBeDefined();
      expect(result.activities[0].pipelineExecution).toHaveLength(1);
      expect(result.activities[0].pipelineExecution[0].service).toBe('enricher');
    });

    it('/stats returns a count of', async () => {
      ctx.stores.pipelineRuns.countSynced.mockResolvedValue(1);

      const result = await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '/api/activities/stats',
      } as any), ctx);

      expect(result).toEqual({
        synchronizedCount: 1,
        totalSynced: 1,
        monthlySynced: 1,
        weeklySynced: 1,
      });
    });

    it('/:id returns single activity', async () => {
      ctx.stores.pipelineRuns.findByActivityId.mockResolvedValue({
        id: 'pipe-1',
        activityId: 'a1',
        pipelineId: 'pipeline-123',
        title: 'Activity 1',
        description: 'Description 1',
        type: 46, // ACTIVITY_TYPE_WEIGHT_TRAINING
        source: 'SOURCE_FITBIT',
        destinations: [],
      });
      ctx.services.execution.listByPipeline.mockResolvedValue([]);

      const result: any = await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '/api/activities/a1',
      } as any), ctx);

      expect(result.activity.activityId).toBe('a1');
      expect(result.activity.title).toBe('Activity 1');
      expect(result.activity.type).toBe('Weight Training');
      expect(result.activity.source).toBe('Fitbit');
    });

    it('handles errors', async () => {
      ctx.stores.pipelineRuns.listSynced.mockRejectedValue(new Error('db error'));
      await expect(handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '/api/activities',
      } as any), ctx)).rejects.toThrow('db error');
    });

    describe('/unsynchronized', () => {
      beforeEach(() => {
        // Add executions store and execution service mocks
        ctx.stores.executions = {
          listDistinctPipelines: jest.fn(),
        };
        ctx.services = {
          execution: {
            listByPipeline: jest.fn(),
          },
        };
        ctx.stores.activities.getSynchronizedPipelineIds = jest.fn();
      });

      it('/unsynchronized returns list of unsynchronized executions', async () => {
        // Mock: one pipeline run that is failed
        ctx.stores.pipelineRuns.list.mockResolvedValue([{
          id: 'pipeline-1',
          title: 'Morning Run',
          type: 27, // ACTIVITY_TYPE_RUN
          source: 'Fitbit',
          status: 4, // PIPELINE_RUN_STATUS_FAILED
          statusMessage: 'Activity filter rejected',
          destinations: [],
          createdAt: new Date('2026-01-01T12:00:00Z'),
        }]);

        const result = await handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/api/activities/unsynchronized',
        } as any), ctx);

        expect(result).toEqual({
          executions: [{
            pipelineExecutionId: 'pipeline-1',
            title: 'Morning Run',
            activityType: 'Run',
            source: 'Fitbit',
            status: 'Failed',
            errorMessage: 'Activity filter rejected',
            timestamp: '2026-01-01T12:00:00.000Z',
          }],
        });
      });

      it('/unsynchronized filters out successful pipelines', async () => {
        // Mock: successful pipeline runs should be excluded
        ctx.stores.pipelineRuns.list.mockResolvedValue([{
          id: 'pipeline-synced',
          title: 'Synced Activity',
          type: 27,
          source: 'Fitbit',
          status: 2, // PIPELINE_RUN_STATUS_SUCCESS
          destinations: [],
          createdAt: new Date(),
        }]);

        const result = await handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/api/activities/unsynchronized',
        } as any), ctx);

        expect(result).toEqual({ executions: [] });
      });

      it('/unsynchronized/:id returns pipeline trace', async () => {
        ctx.services.execution.listByPipeline.mockResolvedValue([{
          id: 'exec-1',
          data: {
            service: 'fitbit-handler',
            status: 1,
            timestamp: new Date('2026-01-01T12:00:00Z'),
          },
        }, {
          id: 'exec-2',
          data: {
            service: 'enricher',
            status: 7,
            timestamp: new Date('2026-01-01T12:00:01Z'),
            errorMessage: 'Skipped by filter',
          },
        }]);

        const result = await handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/api/activities/unsynchronized/pipeline-1',
        } as any), ctx);

        expect(result).toEqual({
          pipelineExecutionId: 'pipeline-1',
          pipelineExecution: expect.arrayContaining([
            expect.objectContaining({ service: 'fitbit-handler', status: 'Started' }),
            expect.objectContaining({ service: 'enricher', status: 'Skipped', errorMessage: 'Skipped by filter' }),
          ]),
        });
      });

      it('/unsynchronized/:id returns 404 if not found', async () => {
        ctx.services.execution.listByPipeline.mockResolvedValue([]);

        await expect(handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/api/activities/unsynchronized/unknown',
        } as any), ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
      });
    });
  });
});
