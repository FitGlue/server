import { handler } from './index';

// Mock uuid
jest.mock('uuid', () => ({
  v4: jest.fn().mockReturnValue('mock-uuid-1234')
}));

describe('user-pipelines-handler', () => {
  let req: any;
  let ctx: any;
  let mockPipelineStore: any;

  beforeEach(() => {
    mockPipelineStore = {
      get: jest.fn(),
      list: jest.fn(),
      create: jest.fn(),
      update: jest.fn(),
      delete: jest.fn(),
      toggleDisabled: jest.fn(),
    };

    req = {
      method: 'GET',
      body: {},
      query: {},
      path: '/api/users/me/pipelines',
    };

    ctx = {
      userId: 'user-1',
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
      },
      services: {
        user: {
          pipelineStore: mockPipelineStore
        },
      },
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET / (list pipelines)', () => {
    it('returns 401 if no user', async () => {
      ctx.userId = undefined;
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('returns pipelines list', async () => {
      mockPipelineStore.list.mockResolvedValue([
        { id: 'p1', source: 'SOURCE_HEVY', destinations: [1] }
      ]);

      const result = await handler(req, ctx);

      expect(mockPipelineStore.list).toHaveBeenCalledWith('user-1');
      expect(result).toEqual({
        pipelines: [{ id: 'p1', source: 'SOURCE_HEVY', destinations: [1] }]
      });
    });

    it('returns empty array if no pipelines', async () => {
      mockPipelineStore.list.mockResolvedValue([]);

      const result = await handler(req, ctx);

      expect(result).toEqual({ pipelines: [] });
    });
  });

  describe('GET /:pipelineId', () => {
    beforeEach(() => {
      req.method = 'GET';
      req.path = '/api/users/me/pipelines/pipeline-123';
    });

    it('returns pipeline if found', async () => {
      mockPipelineStore.get.mockResolvedValue({
        id: 'pipeline-123',
        source: 'SOURCE_HEVY',
        destinations: [1]
      });

      const result = await handler(req, ctx);

      expect(mockPipelineStore.get).toHaveBeenCalledWith('user-1', 'pipeline-123');
      expect(result).toEqual({
        id: 'pipeline-123',
        source: 'SOURCE_HEVY',
        destinations: [1]
      });
    });

    it('returns 404 if pipeline not found', async () => {
      mockPipelineStore.get.mockResolvedValue(null);

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });
  });

  describe('POST / (create pipeline)', () => {
    beforeEach(() => {
      req.method = 'POST';
      req.path = '/api/users/me/pipelines';
      req.body = {
        source: 'hevy',
        destinations: ['strava']
      };
    });

    it('returns 400 if missing source', async () => {
      req.body = { destinations: ['strava'] };
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 400 if missing destinations', async () => {
      req.body = { source: 'hevy' };
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('creates pipeline with generated ID', async () => {
      await handler(req, ctx);

      expect(mockPipelineStore.create).toHaveBeenCalledWith(
        'user-1',
        expect.objectContaining({
          id: expect.stringContaining('pipe_'),
          name: '',
          source: 'SOURCE_HEVY',
          enrichers: [],
          destinations: expect.any(Array),
          disabled: false,
          sourceConfig: {},
          destinationConfigs: {},
        })
      );
    });

    it('passes through sourceConfig and destinationConfigs on create', async () => {
      req.body = {
        source: 'hevy',
        destinations: ['strava'],
        sourceConfig: { repo: 'my-org/my-repo', folder: 'workouts' },
        destinationConfigs: { googlesheets: { config: { spreadsheet_id: 'abc123' } } },
      };

      await handler(req, ctx);

      expect(mockPipelineStore.create).toHaveBeenCalledWith(
        'user-1',
        expect.objectContaining({
          sourceConfig: { repo: 'my-org/my-repo', folder: 'workouts' },
          destinationConfigs: { googlesheets: { config: { spreadsheet_id: 'abc123' } } },
        })
      );
    });
  });

  describe('DELETE /:pipelineId', () => {
    beforeEach(() => {
      req.method = 'DELETE';
      req.path = '/api/users/me/pipelines/pipeline-123';
    });

    it('deletes pipeline successfully', async () => {
      await handler(req, ctx);

      expect(mockPipelineStore.delete).toHaveBeenCalledWith('user-1', 'pipeline-123');
    });
  });

  describe('PATCH /:pipelineId', () => {
    beforeEach(() => {
      req.method = 'PATCH';
      req.path = '/api/users/me/pipelines/pipeline-123';
      req.body = {
        source: 'fitbit',
        destinations: ['strava', 'mock']
      };
    });

    it('updates pipeline successfully', async () => {
      await handler(req, ctx);

      expect(mockPipelineStore.create).toHaveBeenCalledWith(
        'user-1',
        expect.objectContaining({
          id: 'pipeline-123',
          source: 'SOURCE_FITBIT',
          destinations: expect.any(Array),
          sourceConfig: {},
          destinationConfigs: {},
        })
      );
    });

    it('passes through sourceConfig and destinationConfigs on update', async () => {
      req.body = {
        source: 'fitbit',
        destinations: ['strava'],
        sourceConfig: { folder: 'data' },
        destinationConfigs: { strava: { config: { activity_type: 'run' } } },
      };

      await handler(req, ctx);

      expect(mockPipelineStore.create).toHaveBeenCalledWith(
        'user-1',
        expect.objectContaining({
          sourceConfig: { folder: 'data' },
          destinationConfigs: { strava: { config: { activity_type: 'run' } } },
        })
      );
    });

    it('toggles disabled state when only disabled field is sent', async () => {
      req.body = { disabled: true };
      await handler(req, ctx);

      expect(mockPipelineStore.toggleDisabled).toHaveBeenCalledWith('user-1', 'pipeline-123', true);
      expect(mockPipelineStore.create).not.toHaveBeenCalled();
    });

    it('toggles disabled state to false when disabled=false is sent', async () => {
      req.body = { disabled: false };
      await handler(req, ctx);

      expect(mockPipelineStore.toggleDisabled).toHaveBeenCalledWith('user-1', 'pipeline-123', false);
      expect(mockPipelineStore.create).not.toHaveBeenCalled();
    });

    it('returns 400 when source is missing for full update', async () => {
      req.body = { destinations: ['strava'] };
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
      expect(mockPipelineStore.create).not.toHaveBeenCalled();
    });

    it('returns 400 when destinations is missing for full update', async () => {
      req.body = { source: 'fitbit' };
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
      expect(mockPipelineStore.create).not.toHaveBeenCalled();
    });
  });
});
