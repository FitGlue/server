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

// Mock CloudEventPublisher
const mockPublish = jest.fn().mockResolvedValue('msg-id-123');
jest.mock('@fitglue/shared/infrastructure/pubsub', () => ({
  CloudEventPublisher: jest.fn().mockImplementation(() => ({
    publish: mockPublish,
  })),
}));

// Mock @fitglue/shared/types barrel (events enums + helpers)
jest.mock('@fitglue/shared/types', () => ({
  CloudEventType: { CLOUD_EVENT_TYPE_ACTIVITY_CREATED: 1 },
  CloudEventSource: {
    CLOUD_EVENT_SOURCE_APPLE_HEALTH: 15,
    CLOUD_EVENT_SOURCE_HEALTH_CONNECT: 16,
  },
  getCloudEventSource: jest.fn((s: number) => `/integrations/${s}`),
  getCloudEventType: jest.fn((t: number) => `com.fitglue.activity.${t}`),
}));

// Mock firebase-admin/storage — avoid hoisting issues by defining mocks inside factory
jest.mock('firebase-admin/storage', () => {
  const mockSave = jest.fn().mockResolvedValue(undefined);
  const mockFile = jest.fn().mockReturnValue({ save: mockSave });
  const mockBucket = jest.fn().mockReturnValue({ file: mockFile });
  return {
    getStorage: jest.fn().mockReturnValue({ bucket: mockBucket }),
    __mocks: { mockSave, mockFile, mockBucket },
  };
});

// Get db from mocked module
import { db } from '@fitglue/shared/framework';

// Access GCS mocks from factory
const storageMocks = () => require('firebase-admin/storage').__mocks as {
  mockSave: jest.Mock;
  mockFile: jest.Mock;
  mockBucket: jest.Mock;
};

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

    // Mock user sub-collection: db.collection('users').doc(userId).collection('mobile_activities')
    const mockUserDoc = {
      collection: jest.fn().mockReturnValue(mockMobileActivitiesCollection),
    };
    const mockUsersCollection = {
      doc: jest.fn().mockReturnValue(mockUserDoc),
    };
    (db.collection as jest.Mock).mockReturnValue(mockUsersCollection);

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
      pubsub: {},
    };

    jest.clearAllMocks();
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

  it('processes activities and stores to user sub-collection', async () => {
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

    // Verify sub-collection path: db.collection('users').doc(userId).collection('mobile_activities')
    expect(db.collection).toHaveBeenCalledWith('users');

    // Verify both activities were stored
    expect(mockDocRef.set).toHaveBeenCalledTimes(2);

    // Verify first activity mapping
    const firstCallData = mockDocRef.set.mock.calls[0][0];
    expect(firstCallData.source).toBe('SOURCE_APPLE_HEALTH');
    expect(firstCallData.activityType).toBe('Run');
    expect(firstCallData.status).toBe('stored');

    // Verify second activity mapping
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

  it('publishes to topic-mobile-activity for each activity', async () => {
    const activities = [
      {
        externalId: 'ext-1',
        source: 'healthkit',
        activityName: 'Running',
        startTime: '2026-01-01T12:00:00Z',
        endTime: '2026-01-01T12:30:00Z',
        duration: 1800,
      },
    ];

    await handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities },
    } as any), ctx);

    // Verify CloudEventPublisher was called with topic-mobile-activity
    const { CloudEventPublisher } = require('@fitglue/shared/infrastructure/pubsub');
    expect(CloudEventPublisher).toHaveBeenCalledWith(
      ctx.pubsub,
      'topic-mobile-activity',
      expect.any(String),
      expect.any(String),
      ctx.logger
    );

    // Verify publish was called with the correct message
    expect(mockPublish).toHaveBeenCalledWith(
      expect.objectContaining({
        userId: 'user-1',
        activityId: 'ext-1',
        source: 'healthkit',
      }),
      'ext-1'
    );
  });

  it('offloads telemetry to GCS when HR samples present', async () => {
    const activities = [
      {
        externalId: 'ext-hr',
        source: 'healthkit',
        activityName: 'Running',
        startTime: '2026-01-01T12:00:00Z',
        endTime: '2026-01-01T12:30:00Z',
        duration: 1800,
        heartRateSamples: [
          { timestamp: '2026-01-01T12:00:00Z', bpm: 120 },
          { timestamp: '2026-01-01T12:01:00Z', bpm: 135 },
        ],
      },
    ];

    await handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities },
    } as any), ctx);

    // Verify GCS upload was called
    const { mockBucket, mockFile, mockSave } = storageMocks();
    expect(mockBucket).toHaveBeenCalled();
    expect(mockFile).toHaveBeenCalledWith('mobile_activities/user-1/ext-hr.json');
    expect(mockSave).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ contentType: 'application/json' })
    );

    // Verify the saved content contains HR samples
    const savedContent = JSON.parse(mockSave.mock.calls[0][0]);
    expect(savedContent.heartRateSamples).toHaveLength(2);
    expect(savedContent.heartRateSamples[0].bpm).toBe(120);

    // Verify Firestore doc has telemetry_uri reference
    const storedData = mockDocRef.set.mock.calls[0][0];
    expect(storedData.telemetryUri).toContain('gs://');
    expect(storedData.telemetryUri).toContain('mobile_activities/user-1/ext-hr.json');
    expect(storedData.heartRateSampleCount).toBe(2);
  });

  it('offloads telemetry to GCS when GPS route present', async () => {
    const activities = [
      {
        externalId: 'ext-gps',
        source: 'health_connect',
        activityName: 'Running',
        startTime: '2026-01-01T12:00:00Z',
        endTime: '2026-01-01T12:30:00Z',
        duration: 1800,
        route: [
          { timestamp: '2026-01-01T12:00:00Z', latitude: 52.95, longitude: -1.15, altitude: 50 },
          { timestamp: '2026-01-01T12:01:00Z', latitude: 52.96, longitude: -1.16, altitude: 52 },
        ],
      },
    ];

    await handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities },
    } as any), ctx);

    // Verify GCS upload was called with route data
    const { mockSave } = storageMocks();
    const savedContent = JSON.parse(mockSave.mock.calls[0][0]);
    expect(savedContent.route).toHaveLength(2);
    expect(savedContent.route[0].latitude).toBe(52.95);
    expect(savedContent.heartRateSamples).toHaveLength(0);

    // Verify Firestore doc tracks route count
    const storedData = mockDocRef.set.mock.calls[0][0];
    expect(storedData.routePointCount).toBe(2);
    expect(storedData.telemetryUri).toContain('gs://');
  });

  it('skips GCS upload when no telemetry data', async () => {
    const activities = [
      {
        externalId: 'ext-basic',
        source: 'healthkit',
        activityName: 'WeightTraining',
        startTime: '2026-01-01T12:00:00Z',
        endTime: '2026-01-01T13:00:00Z',
        duration: 3600,
      },
    ];

    await handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities },
    } as any), ctx);

    // No GCS upload should happen
    const { mockSave } = storageMocks();
    expect(mockSave).not.toHaveBeenCalled();

    // Firestore doc should have null telemetry_uri
    const storedData = mockDocRef.set.mock.calls[0][0];
    expect(storedData.telemetryUri).toBeNull();
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

  it('generates deterministic activity ID when externalId missing', async () => {
    const activities = [
      {
        source: 'health_connect',
        activityName: 'Walk',
        startTime: '2026-01-01T13:00:00Z',
        endTime: '2026-01-01T14:00:00Z',
        duration: 3600,
      }
    ];

    await handler(({
      method: 'POST',
      path: '/api/mobile/sync',
      body: { activities },
    } as any), ctx);

    // Should generate ID from source + timestamp
    expect(mockMobileActivitiesCollection.doc).toHaveBeenCalledWith(
      expect.stringContaining('health_connect-')
    );
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
