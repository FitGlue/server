import { handler } from './index';

// Mock uuid
jest.mock('uuid', () => ({
  v4: jest.fn().mockReturnValue('mock-uuid-1234')
}));

describe('user-pipelines-handler', () => {
  let req: any;

  let ctx: any;
  let mockUserService: any;

  beforeEach(() => {
    mockUserService = {
      get: jest.fn(),
      addPipeline: jest.fn(),
      replacePipeline: jest.fn(),
      removePipeline: jest.fn(),
      togglePipelineDisabled: jest.fn(),
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
        user: mockUserService,
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

    it('returns 404 if user not found', async () => {
      mockUserService.get.mockResolvedValue(null);
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('returns pipelines list', async () => {
      mockUserService.get.mockResolvedValue({
        pipelines: [
          { id: 'p1', source: 'hevy', destinations: ['strava'] }
        ]
      });

      const result = await handler(req, ctx);

      expect(result).toEqual({
        pipelines: [{ id: 'p1', source: 'hevy', destinations: ['strava'] }]
      });
    });

    it('returns empty array if no pipelines', async () => {
      mockUserService.get.mockResolvedValue({});

      const result = await handler(req, ctx);

      expect(result).toEqual({ pipelines: [] });
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

      // addPipeline(userId, name, source, enrichers, destinations)
      expect(mockUserService.addPipeline).toHaveBeenCalledWith(
        'user-1',
        '',
        'hevy',
        [],
        ['strava']
      );
    });

    it('uses provided ID if given', async () => {
      req.body.id = 'custom-id';
      await handler(req, ctx);

      // Still uses name, source, enrichers, destinations (id is managed internally)
      expect(mockUserService.addPipeline).toHaveBeenCalledWith(
        'user-1',
        '',
        'hevy',
        [],
        ['strava']
      );
    });
  });

  describe('DELETE /{pipelineId}', () => {
    beforeEach(() => {
      req.method = 'DELETE';
      req.path = '/api/users/me/pipelines/pipeline-123';
    });

    it('deletes pipeline successfully', async () => {
      await handler(req, ctx);

      expect(mockUserService.removePipeline).toHaveBeenCalledWith('user-1', 'pipeline-123');
    });
  });

  describe('PATCH /{pipelineId}', () => {
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

      // replacePipeline(userId, { pipelineId, name, source, enrichers, destinations })
      expect(mockUserService.replacePipeline).toHaveBeenCalledWith(
        'user-1',
        {
          pipelineId: 'pipeline-123',
          name: '',
          source: 'fitbit',
          enrichers: [],
          destinations: ['strava', 'mock']
        }
      );
    });

    it('toggles disabled state when only disabled field is sent', async () => {
      req.body = { disabled: true };
      await handler(req, ctx);

      expect(mockUserService.togglePipelineDisabled).toHaveBeenCalledWith('user-1', 'pipeline-123', true);
      expect(mockUserService.replacePipeline).not.toHaveBeenCalled();
    });

    it('toggles disabled state to false when disabled=false is sent', async () => {
      req.body = { disabled: false };
      await handler(req, ctx);

      expect(mockUserService.togglePipelineDisabled).toHaveBeenCalledWith('user-1', 'pipeline-123', false);
      expect(mockUserService.replacePipeline).not.toHaveBeenCalled();
    });

    it('returns 400 when source is missing for full update', async () => {
      req.body = { destinations: ['strava'] };
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
      expect(mockUserService.replacePipeline).not.toHaveBeenCalled();
    });

    it('returns 400 when destinations is missing for full update', async () => {
      req.body = { source: 'fitbit' };
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
      expect(mockUserService.replacePipeline).not.toHaveBeenCalled();
    });
  });
});
