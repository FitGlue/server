import { handler } from './index';

// Mock uuid
jest.mock('uuid', () => ({
  v4: jest.fn().mockReturnValue('mock-uuid-1234')
}));

describe('user-pipelines-handler', () => {
  let req: any;
  let res: any;
  let ctx: any;
  let mockUserService: any;

  beforeEach(() => {
    mockUserService = {
      get: jest.fn(),
      addPipeline: jest.fn(),
      replacePipeline: jest.fn(),
      removePipeline: jest.fn(),
    };

    req = {
      method: 'GET',
      body: {},
      query: {},
      path: '',
    };
    res = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn(),
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
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(401);
    });

    it('returns 404 if user not found', async () => {
      mockUserService.get.mockResolvedValue(null);
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(404);
    });

    it('returns pipelines list', async () => {
      mockUserService.get.mockResolvedValue({
        pipelines: [
          { id: 'p1', source: 'hevy', destinations: ['strava'] }
        ]
      });

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({
        pipelines: [{ id: 'p1', source: 'hevy', destinations: ['strava'] }]
      });
    });

    it('returns empty array if no pipelines', async () => {
      mockUserService.get.mockResolvedValue({});

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({ pipelines: [] });
    });
  });

  describe('POST / (create pipeline)', () => {
    beforeEach(() => {
      req.method = 'POST';
      req.body = {
        source: 'hevy',
        destinations: ['strava']
      };
    });

    it('returns 400 if missing source', async () => {
      req.body = { destinations: ['strava'] };
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(400);
    });

    it('returns 400 if missing destinations', async () => {
      req.body = { source: 'hevy' };
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(400);
    });

    it('creates pipeline with generated ID', async () => {
      await handler(req, res, ctx);

      // addPipeline(userId, name, source, enrichers, destinations)
      expect(mockUserService.addPipeline).toHaveBeenCalledWith(
        'user-1',
        '',
        'hevy',
        [],
        ['strava']
      );
      expect(res.status).toHaveBeenCalledWith(200);
    });

    it('uses provided ID if given', async () => {
      req.body.id = 'custom-id';
      await handler(req, res, ctx);

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
      req.path = '/pipeline-123';
    });

    it('deletes pipeline successfully', async () => {
      await handler(req, res, ctx);

      expect(mockUserService.removePipeline).toHaveBeenCalledWith('user-1', 'pipeline-123');
      expect(res.status).toHaveBeenCalledWith(200);
    });
  });

  describe('PATCH /{pipelineId}', () => {
    beforeEach(() => {
      req.method = 'PATCH';
      req.path = '/pipeline-123';
      req.body = {
        source: 'fitbit',
        destinations: ['strava', 'mock']
      };
    });

    it('updates pipeline successfully', async () => {
      await handler(req, res, ctx);

      // replacePipeline(userId, pipelineId, name, source, enrichers, destinations)
      expect(mockUserService.replacePipeline).toHaveBeenCalledWith(
        'user-1',
        'pipeline-123',
        '',
        'fitbit',
        [],
        ['strava', 'mock']
      );
      expect(res.status).toHaveBeenCalledWith(200);
    });
  });
});
