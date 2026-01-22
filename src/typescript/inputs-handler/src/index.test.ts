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
      // Should throw HttpError (or similar)

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
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

      const result = await handler(req, ctx);

      expect(result).toEqual({
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
      await expect(handler(req, ctx)).rejects.toThrow('db error');
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
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 404 if input not found', async () => {
      mockInputService.getPendingInput.mockResolvedValue(null);
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('resolves and republishes successfully', async () => {
      const mockPayload = { source: 'HEVY' };
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        originalPayload: mockPayload
      });

      const result = await handler(req, ctx);

      expect(mockInputService.resolveInput).toHaveBeenCalledWith('act-1', 'user-1', { title: 'New Title' });
      expect(mockPublish).toHaveBeenCalledWith(mockPayload);
      expect(result).toEqual({ success: true });
    });

    it('returns 500 if original payload missing', async () => {
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        originalPayload: null
      });

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 500 }));
      expect(mockInputService.resolveInput).toHaveBeenCalled();
      expect(mockPublish).not.toHaveBeenCalled();
    });

    it('handles conflict errors', async () => {
      mockInputService.getPendingInput.mockResolvedValue({ activityId: 'act-1' });
      mockInputService.resolveInput.mockRejectedValue(new Error('Wait status required'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 409 }));
    });

    it('handles ForbiddenError from authorization', async () => {
      const { ForbiddenError } = jest.requireMock('@fitglue/shared');
      mockInputService.getPendingInput.mockResolvedValue({ activityId: 'act-1' });
      mockInputService.resolveInput.mockRejectedValue(new ForbiddenError('You do not have permission'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 403 }));
    });
  });

  describe('DELETE /:activityId', () => {
    beforeEach(() => {
      req.method = 'DELETE';
      req.path = '/act-1';
    });

    it('returns 400 if missing activityId', async () => {
      req.path = '/';
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('calls dismissInput and returns success', async () => {
      const result = await handler(req, ctx);
      expect(mockInputService.dismissInput).toHaveBeenCalledWith('act-1', 'user-1');
      expect(result).toEqual({ success: true });
    });

    it('handles generic errors', async () => {
      mockInputService.dismissInput.mockRejectedValue(new Error('Some error'));
      await expect(handler(req, ctx)).rejects.toThrow('Some error');
    });

    it('handles encoded IDs', async () => {
      req.path = '/api/inputs/FITBIT%3A123';
      const result = await handler(req, ctx);
      expect(mockInputService.dismissInput).toHaveBeenCalledWith('FITBIT:123', 'user-1');
      expect(result).toEqual({ success: true });
    });

    it('handles ForbiddenError from authorization', async () => {
      const { ForbiddenError } = jest.requireMock('@fitglue/shared');
      mockInputService.dismissInput.mockRejectedValue(new ForbiddenError('Access denied'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 403 }));
    });
  });
});
