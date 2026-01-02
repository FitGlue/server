import * as crypto from 'crypto';
import { ApiKeyStrategy } from './auth';
import { FrameworkContext } from './index';

// Mock dependencies
const mockGet = jest.fn();
const mockSet = jest.fn().mockResolvedValue({});
const mockUpdate = jest.fn().mockResolvedValue({});
const mockDoc = jest.fn((_) => ({
  get: mockGet,
  set: mockSet,
  withConverter: jest.fn(() => ({
    update: mockUpdate
  }))
}));
const mockCollection = jest.fn((_) => ({
  doc: mockDoc
}));

// Mock Firestore Storage Module directly
jest.mock('../storage/firestore', () => ({
  getIngressApiKeysCollection: jest.fn(() => ({
    doc: mockDoc // Returns mockDoc directly which has get/set
  }))
}));

const mockDb = {
  collection: mockCollection
} as any;

const mockLogger = {
  warn: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  debug: jest.fn(),
} as any;

const mockCtx: FrameworkContext = {
  db: mockDb,
  pubsub: {} as any,
  logger: mockLogger,
  secrets: { get: jest.fn() } as any,
  executionId: 'test-exec'
};

describe('ApiKeyStrategy', () => {
  let strategy: ApiKeyStrategy;
  const testKey = 'fg_sk_test_key_123';
  const testHash = crypto.createHash('sha256').update(testKey).digest('hex');

  beforeEach(() => {
    strategy = new ApiKeyStrategy();
    jest.clearAllMocks();
  });

  it('should authenticate with Bearer token', async () => {
    const req = {
      headers: {
        'authorization': `Bearer ${testKey}`
      }
    };

    mockGet.mockResolvedValueOnce({
      exists: true,
      data: () => ({ userId: 'user1', scopes: ['read'] })
    });

    const result = await strategy.authenticate(req, mockCtx);

    expect(result).toEqual({ userId: 'user1', scopes: ['read'] });
    expect(mockDoc).toHaveBeenCalledWith(testHash);
  });

  it('should authenticate with Raw Authorization token (Hevy style)', async () => {
    const req = {
      headers: {
        'authorization': testKey // No "Bearer " prefix
      }
    };

    mockGet.mockResolvedValueOnce({
      exists: true,
      data: () => ({ userId: 'user1', scopes: ['read'] })
    });

    const result = await strategy.authenticate(req, mockCtx);

    expect(result).toEqual({ userId: 'user1', scopes: ['read'] });
    expect(mockDoc).toHaveBeenCalledWith(testHash);
  });

  it('should authenticate with X-Api-Key header', async () => {
    const req = {
      headers: {
        'x-api-key': testKey
      }
    };

    mockGet.mockResolvedValueOnce({
      exists: true,
      data: () => ({ userId: 'user1', scopes: ['read'] })
    });

    const result = await strategy.authenticate(req, mockCtx);
    expect(result).toEqual({ userId: 'user1', scopes: ['read'] });
  });

  it('should authenticate with query parameter', async () => {
    const req = {
      headers: {},
      query: {
        key: testKey
      }
    };

    mockGet.mockResolvedValueOnce({
      exists: true,
      data: () => ({ userId: 'user1', scopes: ['read'] })
    });

    const result = await strategy.authenticate(req, mockCtx);
    expect(result).toEqual({ userId: 'user1', scopes: ['read'] });
  });

  it('should return null if no token provided', async () => {
    const req = {
      headers: {},
      query: {}
    };

    const result = await strategy.authenticate(req, mockCtx);
    expect(result).toBeNull();
    expect(mockGet).not.toHaveBeenCalled();
  });

  it('should return null if key not found in DB', async () => {
    const req = {
      headers: {
        'authorization': `Bearer ${testKey}`
      }
    };

    mockGet.mockResolvedValueOnce({
      exists: false
    });

    const result = await strategy.authenticate(req, mockCtx);
    expect(result).toBeNull();
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('Auth failed'), expect.any(Object));
  });
});
