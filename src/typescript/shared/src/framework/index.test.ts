
import { createCloudFunction, FrameworkContext } from './index';


// Mock dependencies
jest.mock('firebase-admin', () => ({
  apps: [],
  initializeApp: jest.fn(),
  firestore: jest.fn(() => ({
    collection: jest.fn(() => ({
      doc: jest.fn(() => ({
        set: jest.fn(),
        update: jest.fn(),
        get: jest.fn(),
      })),
    })),
  })),
}));

jest.mock('@google-cloud/pubsub', () => ({
  PubSub: jest.fn().mockImplementation(() => ({
    topic: jest.fn(),
  })),
}));

jest.mock('winston', () => ({
  createLogger: jest.fn().mockImplementation(() => ({
    child: jest.fn().mockReturnThis(),
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  })),
  format: {
    json: jest.fn(),
    combine: jest.fn(),
    timestamp: jest.fn(),
    printf: jest.fn(),
  },
  transports: {
    Console: jest.fn(),
  },
}));

jest.mock('../execution/logger', () => ({
  logExecutionStart: jest.fn().mockResolvedValue('exec-123'),
  logExecutionSuccess: jest.fn().mockResolvedValue(undefined),
  logExecutionFailure: jest.fn().mockResolvedValue(undefined),
}));

describe('createCloudFunction', () => {
  let mockReq: any;
  let mockRes: any;
  let handler: jest.Mock;
  let sendSpy: jest.Mock;
  let jsonSpy: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mockReq = {
      method: 'POST',
      path: '/test',
      body: { user_id: 'user-1' },
      headers: {},
      query: {},
    };
    sendSpy = jest.fn();
    jsonSpy = jest.fn();
    mockRes = {
      status: jest.fn().mockReturnThis(),
      send: sendSpy,
      json: jsonSpy,
      set: jest.fn(),
    };
    handler = jest.fn().mockResolvedValue({ success: true });
  });

  it('should execute handler and log success', async () => {
    const cloudFunction = createCloudFunction(handler, { allowUnauthenticated: true });
    await cloudFunction(mockReq, mockRes);

    expect(handler).toHaveBeenCalled();
    expect(mockRes.set).toHaveBeenCalledWith('x-execution-id', expect.stringMatching(/^unknown-function-\d+$/));
  });

  it('should handle errors and log failure', async () => {
    handler.mockRejectedValue(new Error('Test Error'));
    const cloudFunction = createCloudFunction(handler, { allowUnauthenticated: true });

    await cloudFunction(mockReq, mockRes);

    expect(mockRes.status).toHaveBeenCalledWith(500);
    expect(sendSpy).toHaveBeenCalledWith('Internal Server Error');
  });

  it('should extract user_id from body', async () => {
    const cloudFunction = createCloudFunction(handler, { allowUnauthenticated: true });
    await cloudFunction(mockReq, mockRes);

    const ctx = handler.mock.calls[0][2] as FrameworkContext;
    expect(ctx.userId).toBe('user-1');
  });

  it('should log execution failure on HTTP 400', async () => {
    handler.mockImplementation(async (req, res) => {
      res.status(400).json({ error: 'Bad Request' });
    });
    const cloudFunction = createCloudFunction(handler, { allowUnauthenticated: true });
    await cloudFunction(mockReq, mockRes);

    expect(mockRes.status).toHaveBeenCalledWith(400);
    expect(jsonSpy).toHaveBeenCalledWith({ error: 'Bad Request' });
  });

  it('should throw if no auth strategies and not allowUnauthenticated', () => {
    expect(() => createCloudFunction(handler)).toThrow('Security: Auth required');
    expect(() => createCloudFunction(handler, {})).toThrow('Security: Auth required');
    expect(() => createCloudFunction(handler, { auth: { strategies: [] } })).toThrow('Security: Auth required');
  });

});
