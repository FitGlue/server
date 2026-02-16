import { handler } from './index';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => {
  class HttpError extends Error {
    statusCode: number;
    constructor(statusCode: number, message: string) {
      super(message);
      this.statusCode = statusCode;
      this.name = 'HttpError';
    }
  }
  return {
    createCloudFunction: jest.fn((handler: any) => handler),
    FrameworkContext: jest.fn(),
    FirebaseAuthStrategy: jest.fn(),
    HttpError,
    db: {
      collection: jest.fn(),
    },
  };
});

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

// Mock @fitglue/shared/routing
jest.mock('@fitglue/shared/routing', () => {
  const actual = jest.requireActual('@fitglue/shared/routing');
  return actual;
});

// Get db from mocked module
import { db } from '@fitglue/shared/framework';

describe('mobile-sync-handler', () => {

  let ctx: any;
  let mockMobileActivitiesCollection: any;
  let mockDocRef: any;

  beforeEach(() => {
    mockDocRef = {
      set: jest.fn().mockResolvedValue(undefined),
    };
    mockMobileActivitiesCollection = {
      doc: jest.fn().mockReturnValue(mockDocRef),
    };

    (db.collection as jest.Mock).mockReturnValue(mockMobileActivitiesCollection);

    ctx = {
      userId: 'user-1',
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
      },
      stores: {
        users: {
          get: jest.fn().mockResolvedValue({ id: 'user-1' }),
          setIntegration: jest.fn().mockResolvedValue(undefined),
        },
      },
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  it('returns 401 if no user', async () => {
    ctx.userId = undefined;
    await expect(handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities: [] },
    } as any), ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
  });

  it('returns 404 for unmatched route', async () => {
    await expect(handler(({
      method: 'GET',
      path: '/api/mobile/unknown',
    } as any), ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
  });

  it('returns 400 if activities missing', async () => {
    await expect(handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: {},
    } as any), ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
  });

  it('returns 404 if user not found', async () => {
    ctx.stores.users.get.mockResolvedValue(null);
    await expect(handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities: [] },
    } as any), ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
  });

  it('processes activities successfully', async () => {
    const activities = [
      {
        externalId: 'ext-1',
        source: 'healthkit',
        activityName: 'Running',
        startTime: '2026-01-01T12:00:00Z',
        endTime: '2026-01-01T12:30:00Z',
        duration: 1800,
        calories: 300,
        distance: 5000,
      },
      {
        source: 'health_connect',
        activityName: 'WeightTraining',
        startTime: '2026-01-01T13:00:00Z',
        endTime: '2026-01-01T14:00:00Z',
        duration: 3600,
      }
    ];

    const result: any = await handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities, device: { platform: 'ios' } },
    } as any), ctx);

    expect(result).toEqual(expect.objectContaining({
      success: true,
      processedCount: 2,
      skippedCount: 0,
    }));

    expect(mockMobileActivitiesCollection.doc).toHaveBeenCalledWith('ext-1');
    expect(mockMobileActivitiesCollection.doc).toHaveBeenCalledWith(expect.stringContaining('health_connect-'));
    expect(mockDocRef.set).toHaveBeenCalledTimes(2);

    // Verify mapping
    const firstCallData = mockDocRef.set.mock.calls[0][0];
    expect(firstCallData.source).toBe('SOURCE_APPLE_HEALTH');
    expect(firstCallData.activityType).toBe('Run');

    const secondCallData = mockDocRef.set.mock.calls[1][0];
    expect(secondCallData.source).toBe('SOURCE_HEALTH_CONNECT');
    expect(secondCallData.activityType).toBe('WeightTraining');

    // Verify integration was auto-updated
    expect(ctx.stores.users.setIntegration).toHaveBeenCalledWith(
      'user-1',
      'appleHealth',
      expect.objectContaining({ enabled: true })
    );
  });

  it('handles individual activity processing errors', async () => {
    mockDocRef.set.mockRejectedValueOnce(new Error('firestore error'));

    const activities = [
      {
        externalId: 'ext-1',
        source: 'healthkit',
        activityName: 'Running',
        startTime: '2026-01-01T12:00:00Z',
        endTime: '2026-01-01T12:30:00Z',
        duration: 1800,
      },
      {
        externalId: 'ext-2',
        source: 'health_connect',
        activityName: 'WeightTraining',
        startTime: '2026-01-01T13:00:00Z',
        endTime: '2026-01-01T14:00:00Z',
        duration: 3600,
      }
    ];

    const result: any = await handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities },
    } as any), ctx);

    expect(result).toEqual(expect.objectContaining({
      success: true,
      processedCount: 1,
      skippedCount: 1,
    }));
  });

  describe('connect endpoint', () => {
    it('connects health-connect integration', async () => {
      const result: any = await handler(({
        method: 'POST',
        path: '/api/mobile/connect/health-connect',
      } as any), ctx);

      expect(result).toEqual({ message: 'health-connect connected successfully' });
      expect(ctx.stores.users.setIntegration).toHaveBeenCalledWith(
        'user-1',
        'healthConnect',
        expect.objectContaining({ enabled: true })
      );
    });

    it('connects apple-health integration', async () => {
      const result: any = await handler(({
        method: 'POST',
        path: '/api/mobile/connect/apple-health',
      } as any), ctx);

      expect(result).toEqual({ message: 'apple-health connected successfully' });
      expect(ctx.stores.users.setIntegration).toHaveBeenCalledWith(
        'user-1',
        'appleHealth',
        expect.objectContaining({ enabled: true })
      );
    });

    it('returns 400 for invalid provider', async () => {
      await expect(handler(({
        method: 'POST',
        path: '/api/mobile/connect/garmin',
      } as any), ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });
  });
});
