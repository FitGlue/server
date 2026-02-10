import { handler, getPlatformStats } from './index';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: jest.fn((handler: any) => handler),
  FrameworkHandler: jest.fn(),
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
  return { HttpError };
});

// Mock @fitglue/shared/plugin
jest.mock('@fitglue/shared/plugin', () => ({
  getRegistry: jest.fn(() => ({
    sources: [
      { id: 'hevy', name: 'Hevy', enabled: true },
      { id: 'mock', name: 'Mock', enabled: false },
    ],
    enrichers: [
      { id: 'ai-description', name: 'AI Description', enabled: true },
    ],
    destinations: [
      { id: 'strava', name: 'Strava', enabled: true },
    ],
    integrations: [
      { id: 'hevy', name: 'Hevy', enabled: true },
      { id: 'fitbit', name: 'Fitbit', enabled: true },
    ],
  })),
}));

// --- Firebase Admin Mocks ---
const mockCountGet = jest.fn();
const mockCollectionGroupCountGet = jest.fn();
const mockWhere = jest.fn();
const mockCollectionGroupWhere = jest.fn();

jest.mock('firebase-admin/app', () => ({
  getApps: jest.fn(() => [{ name: 'mock' }]), // Pretend already initialized
  initializeApp: jest.fn(),
}));

jest.mock('firebase-admin/firestore', () => ({
  getFirestore: jest.fn(() => ({
    collection: jest.fn(() => ({
      where: mockWhere,
    })),
    collectionGroup: jest.fn(() => ({
      where: mockCollectionGroupWhere,
    })),
  })),
}));

// Import FrameworkContext type
import type { FrameworkContext } from '@fitglue/shared/framework';

describe('registry-handler', () => {
  let mockReq: Parameters<typeof handler>[0];

  let mockCtx: FrameworkContext;

  beforeEach(() => {
    mockReq = {
      method: 'GET',
      query: {},
      path: '/api/registry',
    } as Parameters<typeof handler>[0];

    mockCtx = {
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
        debug: jest.fn(),
      },
    } as unknown as FrameworkContext;

    process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-dev';

    // Reset Firebase mocks
    mockWhere.mockReset();
    mockCountGet.mockReset();
    mockCollectionGroupWhere.mockReset();
    mockCollectionGroupCountGet.mockReset();
  });

  it('returns 405 for non-GET requests', async () => {
    mockReq.method = 'POST';
    await expect(handler(mockReq, mockCtx)).rejects.toThrow(expect.objectContaining({ statusCode: 405 }));
  });

  it('returns filtered registry (enabled only)', async () => {
    const response: any = await handler(mockReq, mockCtx);

    expect(response.sources).toHaveLength(1); // Only enabled
    expect(response.sources[0].id).toBe('hevy');
    expect(response.enrichers).toHaveLength(1);
    expect(response.destinations).toHaveLength(1);
    expect(response.integrations).toHaveLength(2);
  });

  it('returns all enabled plugins (showAll=true does not include disabled)', async () => {
    // Current implementation of showAll still filters by p.enabled
    mockReq.query = { showAll: 'true' };
    const response: any = await handler(mockReq, mockCtx);

    expect(response.sources).toHaveLength(1);
  });

  it('does not include stats for normal requests', async () => {
    const response: any = await handler(mockReq, mockCtx);
    expect(response.stats).toBeUndefined();
  });

  it('includes stats when marketingMode=true', async () => {
    // Setup Firestore mocks for getPlatformStats
    // Users collection: count query for athletes
    mockWhere.mockImplementation(() => ({
      count: () => ({ get: mockCountGet }),
    }));
    mockCountGet.mockResolvedValue({ data: () => ({ count: 15 }) });

    // collectionGroup: count query for synced pipeline runs
    mockCollectionGroupWhere.mockImplementation(() => ({
      count: () => ({ get: mockCollectionGroupCountGet }),
    }));
    mockCollectionGroupCountGet.mockResolvedValue({ data: () => ({ count: 150 }) });

    mockReq.query = { marketingMode: 'true' };
    const response: any = await handler(mockReq, mockCtx);

    expect(response.stats).toEqual({
      athleteCount: 15,
      activitiesBoostedCount: 150,
    });
  });

  it('omits stats gracefully when Firestore query fails', async () => {
    mockWhere.mockImplementation(() => {
      throw new Error('Firestore unavailable');
    });

    mockReq.query = { marketingMode: 'true' };
    const response: any = await handler(mockReq, mockCtx);

    // Stats should be absent, not cause a crash
    expect(response.stats).toBeUndefined();
    expect(mockCtx.logger.warn).toHaveBeenCalled();
  });

  it('logs registry response', async () => {
    await handler(mockReq, mockCtx);

    expect(mockCtx.logger.info).toHaveBeenCalledWith('Plugin registry returned', {
      sourceCount: 1,
      enricherCount: 1,
      destinationCount: 1,
      integrationCount: 2,
      hasStats: false,
    });
  });
});

describe('getPlatformStats', () => {
  const mockLogger = { info: jest.fn(), warn: jest.fn() };

  beforeEach(() => {
    mockWhere.mockReset();
    mockCountGet.mockReset();
    mockCollectionGroupWhere.mockReset();
    mockCollectionGroupCountGet.mockReset();
  });

  it('returns correct counts from Firestore', async () => {
    // Users collection: count query for athletes
    mockWhere.mockImplementation(() => ({
      count: () => ({ get: mockCountGet }),
    }));
    mockCountGet.mockResolvedValue({ data: () => ({ count: 8 }) });

    // collectionGroup: count query for synced pipeline runs
    mockCollectionGroupWhere.mockImplementation(() => ({
      count: () => ({ get: mockCollectionGroupCountGet }),
    }));
    mockCollectionGroupCountGet.mockResolvedValue({ data: () => ({ count: 347 }) });

    const stats = await getPlatformStats(mockLogger);

    expect(stats).toEqual({ athleteCount: 8, activitiesBoostedCount: 347 });
    expect(mockLogger.info).toHaveBeenCalledWith('Platform stats fetched', { athleteCount: 8, activitiesBoostedCount: 347 });
  });
});
