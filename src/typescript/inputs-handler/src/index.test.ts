import { handler } from './index';
import { CloudEventPublisher } from '@fitglue/shared';

// Mock shared dependencies
jest.mock('@fitglue/shared', () => {
  const original = jest.requireActual('@fitglue/shared');

  // Create a mock InputService class that accepts authorization
  const MockInputService = jest.fn().mockImplementation(() => ({
    listPendingInputs: jest.fn(),
    getPendingInput: jest.fn(),
    resolveInput: jest.fn(),
    dismissInput: jest.fn(),
  }));

  return {
    ...original,
    InputStore: jest.fn(),
    InputService: MockInputService,
    CloudEventPublisher: jest.fn(),
    db: {}, // Mock db object
    ForbiddenError: class ForbiddenError extends Error {
      constructor(message: string = 'Access denied') {
        super(message);
        this.name = 'ForbiddenError';
      }
    }
  };
});

describe('inputs-handler', () => {
  let req: any;
  let res: any;
  let ctx: any;
  let mockPublish: any;
  let mockInputService: any;

  beforeEach(() => {
    // Get mocked InputService and reset it
    const { InputService } = jest.requireMock('@fitglue/shared');
    mockInputService = {
      listPendingInputs: jest.fn(),
      getPendingInput: jest.fn(),
      resolveInput: jest.fn(),
      dismissInput: jest.fn(),
    };
    InputService.mockImplementation(() => mockInputService);

    mockPublish = jest.fn();
    (CloudEventPublisher as any).mockImplementation(() => ({
      publish: mockPublish
    }));

    req = {
      method: 'GET',
      body: {},
      query: {},
      path: '',
    };
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
      pubsub: {},
      services: {
        authorization: {
          requireAdmin: jest.fn(),
          requireAccess: jest.fn(),
          canAccessUser: jest.fn().mockResolvedValue(true),
          isAdmin: jest.fn().mockResolvedValue(false),
        }
      },
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET /', () => {
    it('returns 401 if no user', async () => {
      ctx.userId = undefined;
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(401);
    });

    it('returns list of inputs', async () => {
      mockInputService.listPendingInputs.mockResolvedValue([
        {
          activityId: 'a1',
          userId: 'u1',
          status: 1,
          requiredFields: ['title'],
          createdAt: { seconds: 100 },
          inputData: {},
          originalPayload: { some: 'data' }
        }
      ]);

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({
        inputs: [{
          id: 'a1',
          activityId: 'a1',
          userId: 'u1',
          status: 1,
          requiredFields: ['title'],
          createdAt: { seconds: 100 },
          inputData: {},
        }]
      });
    });

    it('handles errors', async () => {
      mockInputService.listPendingInputs.mockRejectedValue(new Error('db error'));
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(500);
    });
  });

  describe('POST /', () => {
    beforeEach(() => {
      req.method = 'POST';
      req.body = {
        activityId: 'act-1',
        inputData: { title: 'New Title' }
      };
    });

    it('returns 400 if missing fields', async () => {
      req.body = {};
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(400);
    });

    it('returns 404 if input not found', async () => {
      mockInputService.getPendingInput.mockResolvedValue(null);
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(404);
    });

    it('resolves and republishes successfully', async () => {
      const mockPayload = { source: 'HEVY' };
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        originalPayload: mockPayload
      });

      await handler(req, res, ctx);

      expect(mockInputService.resolveInput).toHaveBeenCalledWith('act-1', 'user-1', { title: 'New Title' });
      expect(mockPublish).toHaveBeenCalledWith(mockPayload);
      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({ success: true });
    });

    it('returns 500 if original payload missing', async () => {
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        originalPayload: null
      });

      await handler(req, res, ctx);

      expect(mockInputService.resolveInput).toHaveBeenCalled();
      expect(mockPublish).not.toHaveBeenCalled();
      expect(res.status).toHaveBeenCalledWith(500);
    });

    it('handles conflict errors', async () => {
      mockInputService.getPendingInput.mockResolvedValue({ activityId: 'act-1' });
      mockInputService.resolveInput.mockRejectedValue(new Error('Wait status required'));

      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(409);
    });

    it('handles ForbiddenError from authorization', async () => {
      const { ForbiddenError } = jest.requireMock('@fitglue/shared');
      mockInputService.getPendingInput.mockResolvedValue({ activityId: 'act-1' });
      mockInputService.resolveInput.mockRejectedValue(new ForbiddenError('You do not have permission'));

      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(403);
      expect(res.json).toHaveBeenCalledWith({ error: 'You do not have permission' });
    });
  });

  describe('DELETE /:activityId', () => {
    beforeEach(() => {
      req.method = 'DELETE';
      req.path = '/act-1';
    });

    it('returns 400 if missing activityId', async () => {
      req.path = '/';
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(400);
    });

    it('calls dismissInput and returns success', async () => {
      await handler(req, res, ctx);
      expect(mockInputService.dismissInput).toHaveBeenCalledWith('act-1', 'user-1');
      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({ success: true });
    });

    it('handles generic errors', async () => {
      mockInputService.dismissInput.mockRejectedValue(new Error('Some error'));
      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(500);
    });

    it('handles encoded IDs', async () => {
      req.path = '/api/inputs/FITBIT%3A123';
      await handler(req, res, ctx);
      expect(mockInputService.dismissInput).toHaveBeenCalledWith('FITBIT:123', 'user-1');
      expect(res.status).toHaveBeenCalledWith(200);
    });

    it('handles ForbiddenError from authorization', async () => {
      const { ForbiddenError } = jest.requireMock('@fitglue/shared');
      mockInputService.dismissInput.mockRejectedValue(new ForbiddenError('Access denied'));

      await handler(req, res, ctx);
      expect(res.status).toHaveBeenCalledWith(403);
      expect(res.json).toHaveBeenCalledWith({ error: 'Access denied' });
    });
  });
});
