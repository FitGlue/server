import { handler } from './index';
import { CloudEventPublisher } from '@fitglue/shared';

// Mock shared dependencies
jest.mock('@fitglue/shared', () => {
  const original = jest.requireActual('@fitglue/shared');
  return {
    ...original,
    CloudEventPublisher: jest.fn(),
  };
});

describe('activities-handler', () => {
  let res: any;
  let ctx: any;
  let mockPublish: any;

  beforeEach(() => {
    mockPublish = jest.fn();
    (CloudEventPublisher as any).mockImplementation(() => ({
      publish: mockPublish
    }));

    res = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn(),
      send: jest.fn(),
    };
    ctx = {
      userId: 'user-1',
      logger: {
        info: jest.fn(),
        error: jest.fn(),
      },
      pubsub: {}, // Mock pubsub object (CloudEventPublisher uses it, but we mocked the class)
      services: {},
      stores: {
        activities: {
          countSynchronized: jest.fn(),
          listSynchronized: jest.fn(),
          getSynchronized: jest.fn(),
          getSynchronizedPipelineIds: jest.fn(),
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
      await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '',
      } as any), res, ctx);
      expect(res.status).toHaveBeenCalledWith(401);
    });

    it('/ returns list of synchronized activities', async () => {
      ctx.stores.activities.listSynchronized.mockResolvedValue([{
        activityId: 'a1',
        title: 'Activity 1',
        description: 'Description 1',
        type: 5, // ACTIVITY_TYPE_CROSSFIT
        source: 'SOURCE_HEVY',
      }]);

      await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '',
      } as any), res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({
        activities: [{
          activityId: 'a1',
          title: 'Activity 1',
          description: 'Description 1',
          type: 'Crossfit',
          source: 'Hevy',
        }]
      });
    });

    it('/ with includeExecution=true returns activities with pipelineExecution', async () => {
      ctx.stores.activities.listSynchronized.mockResolvedValue([{
        activityId: 'a1',
        title: 'Enhanced Workout',
        type: 46,
        source: 'SOURCE_HEVY',
        pipelineExecutionId: 'pipeline-123',
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

      await handler(({
        method: 'GET',
        body: {},
        query: { includeExecution: 'true' },
        path: '',
      } as any), res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      const response = res.json.mock.calls[0][0];
      expect(response.activities).toHaveLength(1);
      expect(response.activities[0].pipelineExecution).toBeDefined();
      expect(response.activities[0].pipelineExecution).toHaveLength(1);
      expect(response.activities[0].pipelineExecution[0].service).toBe('enricher');
      expect(response.activities[0].pipelineExecutionId).toBe('pipeline-123');
    });

    it('/stats returns a count of', async () => {
      ctx.stores.activities.countSynchronized.mockResolvedValue(1);

      await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '/stats',
      } as any), res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({
        synchronizedCount: 1,
        totalSynced: 1,
        monthlySynced: 1,
        weeklySynced: 1,
      });
    });

    it('/:id returns single activity', async () => {
      const activity = {
        activityId: 'a1',
        title: 'Activity 1',
        description: 'Description 1',
        type: 46, // ACTIVITY_TYPE_WEIGHT_TRAINING
        source: 'SOURCE_FITBIT',
      }
      ctx.stores.activities.getSynchronized.mockResolvedValue(activity);

      await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '/a1',
      } as any), res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({
        activity: {
          activityId: 'a1',
          title: 'Activity 1',
          description: 'Description 1',
          type: 'Weight Training',
          source: 'Fitbit',
        }
      });
    });

    it('handles errors', async () => {
      ctx.stores.activities.listSynchronized.mockRejectedValue(new Error('db error'));
      await handler(({
        method: 'GET',
        body: {},
        query: {},
        path: '',
      } as any), res, ctx);
      expect(res.status).toHaveBeenCalledWith(500);
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
        // Mock: one pipeline execution that is NOT synchronized
        ctx.stores.executions.listDistinctPipelines.mockResolvedValue([{
          id: 'exec-1',
          data: {
            pipelineExecutionId: 'pipeline-1',
            service: 'enricher',
            status: 7, // STATUS_SKIPPED
            timestamp: new Date('2026-01-01T12:00:00Z'),
            errorMessage: 'Activity filter rejected',
            inputsJson: JSON.stringify({ activity: { title: 'Morning Run', type: 27, source: 'SOURCE_FITBIT' } }),
          },
        }]);
        ctx.stores.activities.getSynchronizedPipelineIds.mockResolvedValue(new Set());

        await handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/unsynchronized',
        } as any), res, ctx);

        expect(res.status).toHaveBeenCalledWith(200);
        expect(res.json).toHaveBeenCalledWith({
          executions: [{
            pipelineExecutionId: 'pipeline-1',
            title: 'Morning Run',
            activityType: 'Run',
            source: 'Fitbit',
            status: 'SKIPPED',
            errorMessage: 'Activity filter rejected',
            timestamp: '2026-01-01T12:00:00.000Z',
          }],
        });
      });

      it('/unsynchronized filters out synchronized pipelines', async () => {
        ctx.stores.executions.listDistinctPipelines.mockResolvedValue([{
          id: 'exec-1',
          data: {
            pipelineExecutionId: 'pipeline-synced',
            service: 'enricher',
            status: 2, // STATUS_SUCCESS
            timestamp: new Date(),
          },
        }]);
        // This pipeline is synchronized
        ctx.stores.activities.getSynchronizedPipelineIds.mockResolvedValue(new Set(['pipeline-synced']));

        await handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/unsynchronized',
        } as any), res, ctx);

        expect(res.status).toHaveBeenCalledWith(200);
        expect(res.json).toHaveBeenCalledWith({ executions: [] });
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

        await handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/unsynchronized/pipeline-1',
        } as any), res, ctx);

        expect(res.status).toHaveBeenCalledWith(200);
        expect(res.json).toHaveBeenCalledWith({
          pipelineExecutionId: 'pipeline-1',
          pipelineExecution: expect.arrayContaining([
            expect.objectContaining({ service: 'fitbit-handler', status: 'STARTED' }),
            expect.objectContaining({ service: 'enricher', status: 'SKIPPED', errorMessage: 'Skipped by filter' }),
          ]),
        });
      });

      it('/unsynchronized/:id returns 404 if not found', async () => {
        ctx.services.execution.listByPipeline.mockResolvedValue([]);

        await handler(({
          method: 'GET',
          body: {},
          query: {},
          path: '/unsynchronized/unknown',
        } as any), res, ctx);

        expect(res.status).toHaveBeenCalledWith(404);
      });
    });
  });
});
