import { handler } from './index';
import { InputService, CloudEventPublisher } from '@fitglue/shared';

// Mock shared dependencies
jest.mock('@fitglue/shared', () => {
  const original = jest.requireActual('@fitglue/shared');
  return {
    ...original,
    InputStore: jest.fn(),
    InputService: jest.fn(),
    CloudEventPublisher: jest.fn(),
    db: {}, // Mock db object
  };
});

describe('inputs-handler', () => {
  let req: any;
  let res: any;
  let ctx: any;
  let mockInputService: any;
  let mockPublish: any;

  beforeEach(() => {
    mockInputService = {
      listPendingInputs: jest.fn(),
      getPendingInput: jest.fn(),
      resolveInput: jest.fn(),
    };
    (InputService as any).mockImplementation(() => mockInputService);

    mockPublish = jest.fn();
    (CloudEventPublisher as any).mockImplementation(() => ({
      publish: mockPublish
    }));

    req = {
      method: 'GET',
      body: {},
      query: {},
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
      pubsub: {}, // Mock pubsub object (CloudEventPublisher uses it, but we mocked the class)
      services: {},
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
          originalPayload: { some: 'data' } // Should be omitted
        }
      ]);

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({
        inputs: [{
          activity_id: 'a1',
          user_id: 'u1',
          status: 1,
          required_fields: ['title'],
          created_at: { seconds: 100 },
          input_data: {},
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
        activity_id: 'act-1',
        input_data: { title: 'New Title' }
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
      // Mock existing input with payload
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
  });
});
