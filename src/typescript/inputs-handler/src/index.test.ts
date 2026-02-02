import { handler } from './index';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: jest.fn((handler: any) => handler),
  FirebaseAuthStrategy: jest.fn(),
  FrameworkHandler: jest.fn(),
  db: {},
}));

// Mock @fitglue/shared/errors
jest.mock('@fitglue/shared/errors', () => {
  class HttpError extends Error {
    statusCode: number;
    constructor(statusCode: number, message: string) {
      super(message);
      this.statusCode = statusCode;
      this.name = 'HttpError';
    }
  }
  class ForbiddenError extends Error {
    statusCode: number;
    constructor(message: string = 'Access denied') {
      super(message);
      this.name = 'ForbiddenError';
      this.statusCode = 403;
    }
  }
  return { HttpError, ForbiddenError };
});

// Mock @fitglue/shared/storage
jest.mock('@fitglue/shared/storage', () => ({
  InputStore: jest.fn(),
  UserStore: jest.fn(),
}));

// Mock @fitglue/shared/domain/services
jest.mock('@fitglue/shared/domain/services', () => ({
  InputService: jest.fn().mockImplementation(() => ({
    listPendingInputs: jest.fn(),
    getPendingInput: jest.fn(),
    resolveInput: jest.fn(),
    dismissInput: jest.fn(),
  })),
}));

// Mock @fitglue/shared/infrastructure/pubsub
jest.mock('@fitglue/shared/infrastructure/pubsub', () => ({
  CloudEventPublisher: jest.fn(),
}));

// Mock @fitglue/shared/types
jest.mock('@fitglue/shared/types', () => ({
  ActivityPayload: {},
  CloudEventType: { ACTIVITY_ENRICHMENT: 1 },
  CloudEventSource: { FITGLUE_INPUT_RESUME: 1 },
}));

// Mock @fitglue/shared/dist/types/events-helper
jest.mock('@fitglue/shared/dist/types/events-helper', () => ({
  getCloudEventType: jest.fn(() => 'activity-enrichment'),
  getCloudEventSource: jest.fn(() => 'fitglue-input-resume'),
}));

// Mock @fitglue/shared/dist/config
jest.mock('@fitglue/shared/dist/config', () => ({
  TOPICS: { PIPELINE_ACTIVITY: 'pipeline-activity' },
}));

import { CloudEventPublisher } from '@fitglue/shared/infrastructure/pubsub';

// Mock GCS for payload fetching - use factory pattern to avoid Jest hoisting
const mockGcsDownload = jest.fn();
jest.mock('@google-cloud/storage', () => {
  return {
    Storage: jest.fn().mockImplementation(() => ({
      bucket: jest.fn().mockReturnValue({
        file: jest.fn().mockReturnValue({
          download: jest.fn().mockImplementation(() => mockGcsDownload())
        })
      })
    }))
  };
});

describe('inputs-handler', () => {
  let req: any;

  let ctx: any;
  let mockPublish: any;
  let mockInputService: any;

  beforeEach(() => {
    // Get mocked InputService and reset it
    const { InputService: MockedInputService } = jest.requireMock('@fitglue/shared/domain/services');
    mockInputService = {
      listPendingInputs: jest.fn(),
      getPendingInput: jest.fn(),
      resolveInput: jest.fn(),
      dismissInput: jest.fn(),
    };
    MockedInputService.mockImplementation(() => mockInputService);

    mockPublish = jest.fn();
    (CloudEventPublisher as any).mockImplementation(() => ({
      publish: mockPublish
    }));

    req = {
      method: 'GET',
      body: {},
      query: {},
      path: '/api/inputs',
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
      req.path = '/api/inputs';
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

    it('resolves and republishes successfully with linkedActivityId', async () => {
      const mockPayload = { source: 'HEVY' };
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        linkedActivityId: 'linked-activity-uuid',
        originalPayloadUri: 'gs://test-bucket/pending/user-1/act-1.json'
      });
      mockGcsDownload.mockResolvedValue([Buffer.from(JSON.stringify(mockPayload))]);

      const result = await handler(req, ctx);

      expect(mockInputService.getPendingInput).toHaveBeenCalledWith('user-1', 'act-1');
      expect(mockInputService.resolveInput).toHaveBeenCalledWith('user-1', 'act-1', 'user-1', { title: 'New Title' });
      expect(mockGcsDownload).toHaveBeenCalled();
      // Verify linkedActivityId is transferred to activityId for resume mode
      expect(mockPublish).toHaveBeenCalledWith({
        ...mockPayload,
        isResume: true,
        resumePendingInputId: 'act-1',
        activityId: 'linked-activity-uuid',
      });
      expect(result).toEqual({ success: true });
    });

    it('returns 500 if linkedActivityId is missing', async () => {
      const mockPayload = { source: 'HEVY' };
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        // No linkedActivityId - this should now error
        originalPayloadUri: 'gs://test-bucket/pending/user-1/act-1.json'
      });
      mockGcsDownload.mockResolvedValue([Buffer.from(JSON.stringify(mockPayload))]);

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 500 }));
      expect(ctx.logger.error).toHaveBeenCalledWith(
        'Missing linkedActivityId on pending input - cannot resume',
        expect.objectContaining({ activityId: 'act-1' })
      );
      expect(mockPublish).not.toHaveBeenCalled();
    });

    it('returns 500 if original payload URI missing', async () => {
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        originalPayloadUri: null
      });

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 500 }));
      expect(mockInputService.resolveInput).toHaveBeenCalled();
      expect(mockPublish).not.toHaveBeenCalled();
    });

    it('handles conflict errors', async () => {
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        originalPayloadUri: 'gs://test-bucket/pending/user-1/act-1.json'
      });
      mockInputService.resolveInput.mockRejectedValue(new Error('Wait status required'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 409 }));
    });

    it('handles ForbiddenError from authorization', async () => {
      const { ForbiddenError } = jest.requireMock('@fitglue/shared/errors');
      mockInputService.getPendingInput.mockResolvedValue({
        activityId: 'act-1',
        originalPayloadUri: 'gs://test-bucket/pending/user-1/act-1.json'
      });
      mockInputService.resolveInput.mockRejectedValue(new ForbiddenError('You do not have permission'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 403 }));
    });
  });

  describe('DELETE /:activityId', () => {
    beforeEach(() => {
      req.method = 'DELETE';
      req.path = '/api/inputs/act-1';
    });

    // Note: DELETE /api/inputs without ID will match FCM token endpoint and succeed
    // This test is not valid for the current implementation

    it('calls dismissInput and returns success', async () => {
      const result = await handler(req, ctx);
      expect(mockInputService.dismissInput).toHaveBeenCalledWith('user-1', 'act-1', 'user-1');
      expect(result).toEqual({ success: true });
    });

    it('handles generic errors', async () => {
      mockInputService.dismissInput.mockRejectedValue(new Error('Some error'));
      await expect(handler(req, ctx)).rejects.toThrow('Some error');
    });

    it('handles encoded IDs', async () => {
      req.path = '/api/inputs/FITBIT%3A123';
      const result = await handler(req, ctx);
      expect(mockInputService.dismissInput).toHaveBeenCalledWith('user-1', 'FITBIT:123', 'user-1');
      expect(result).toEqual({ success: true });
    });

    it('handles ForbiddenError from authorization', async () => {
      const { ForbiddenError } = jest.requireMock('@fitglue/shared/errors');
      const error = new ForbiddenError('Access denied');
      mockInputService.dismissInput.mockRejectedValue(error);

      // ForbiddenError is caught and re-thrown as HttpError with statusCode 403
      await expect(handler(req, ctx)).rejects.toThrow('Access denied');
    });
  });
});
