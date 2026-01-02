import { fitbitIngest } from './index';

// Mocks
const mockPublish = jest.fn().mockResolvedValue('msg-id-123');
const mockMarkProcessed = jest.fn().mockResolvedValue(undefined);
const mockHasProcessed = jest.fn().mockResolvedValue(false);
const mockFitbitGet = jest.fn();

const mockCtx = {
  logger: {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
  },
  db: {
    collection: jest.fn(),
  },
  pubsub: {},
};

jest.mock('@fitglue/shared', () => {
  return {
    createCloudFunction: (handler: any) => handler,
    createFitbitClient: jest.fn(() => ({
      GET: mockFitbitGet
    })),
    TypedPublisher: jest.fn().mockImplementation(() => ({
      unwrap: (data: any) => data,
      publish: mockPublish
    })),
    storage: {
      getUsersCollection: jest.fn(() => mockCtx.db.collection('users')),
    },
    // Static method workaround if TypedPublisher has static unwrap
    // But here we are mocking the class constructor and unwrap usage in code is `TypedPublisher.unwrap`.
    // We'll handle static unwrap below if needed, or assume it's used as instance (check code: `TypedPublisher.unwrap` is likely static)
  };
});

// Since TypedPublisher.unwrap is static, we need to mock it on the object itself not just instance
const shared = require('@fitglue/shared');
shared.TypedPublisher.unwrap = jest.fn((data: any) => data);
shared.UserService = jest.fn().mockImplementation(() => ({
  hasProcessedActivity: mockHasProcessed,
  markActivityAsProcessed: mockMarkProcessed,
}));
shared.TOPICS = { RAW_ACTIVITY: 'raw-activity' };
shared.ActivitySource = { SOURCE_FITBIT: 'fitbit' };


jest.mock('./mapper', () => ({
  mapTCXToStandardized: jest.fn().mockReturnValue({ sessions: [] })
}));

describe('Fitbit Ingest', () => {

  beforeEach(() => {
    jest.clearAllMocks();
    mockFitbitGet.mockReset();

    // Default Mock DB behavior for "User Found"
    // Helper to create a chainable query mock
    const createQueryMock = () => {
      const queryMock: any = {
        limit: jest.fn().mockReturnValue({
          get: jest.fn().mockResolvedValue({
            empty: false,
            docs: [{ id: 'test-user-id' }]
          })
        })
      };
      // Allow chaining .where().where()
      queryMock.where = jest.fn().mockReturnValue(queryMock);
      return queryMock;
    };

    (mockCtx.db.collection as jest.Mock).mockReturnValue(createQueryMock());
  });

  it('should skip non-activity updates', async () => {
    const req = { body: { collectionType: 'sleep', ownerId: 'fitbit-user', date: '2023-01-01' } };
    const result = await (fitbitIngest as any)(req, {}, mockCtx);

    expect(result).toEqual({ status: 'skipped', reason: 'non_activity_update', collectionType: 'sleep' });
    expect(mockFitbitGet).not.toHaveBeenCalled();
  });

  it('should throw error if user not found', async () => {
    // Mock DB empty result
    // Mock DB empty result
    const emptyQueryMock: any = {
      limit: jest.fn().mockReturnValue({
        get: jest.fn().mockResolvedValue({
          empty: true
        })
      })
    };
    emptyQueryMock.where = jest.fn().mockReturnValue(emptyQueryMock);

    (mockCtx.db.collection as jest.Mock).mockReturnValue(emptyQueryMock);

    const req = { body: { collectionType: 'activities', ownerId: 'unknown-user', date: '2023-01-01' } };

    await expect((fitbitIngest as any)(req, {}, mockCtx)).rejects.toThrow('No user found for Fitbit ID: unknown-user');
  });

  it('should process activities successfully', async () => {
    const req = { body: { collectionType: 'activities', ownerId: 'fitbit-user', date: '2023-01-01' } };

    // Mock Fitbit Activities List
    mockFitbitGet.mockResolvedValueOnce({
      data: {
        activities: [
          { logId: 111, name: 'Run', tcxLink: 'http://link' },
          { logId: 222, name: 'Walk', tcxLink: 'http://link' }
        ]
      }
    });

    // Mock TCX Fetch for each activity
    // 1st call (Run) -> Success
    mockFitbitGet.mockResolvedValueOnce({
      data: '<xml>TCX1</xml>',
      response: { status: 200 }
    });
    // 2nd call (Walk) -> Success
    mockFitbitGet.mockResolvedValueOnce({
      data: '<xml>TCX2</xml>',
      response: { status: 200 }
    });

    const result = await (fitbitIngest as any)(req, {}, mockCtx);

    // Verify fetching list
    expect(mockFitbitGet).toHaveBeenCalledWith("/1/user/-/activities/date/{date}.json", expect.objectContaining({
      params: { path: { date: '2023-01-01' } }
    }));

    // Verify processing count
    expect(result.publishedCount).toBe(2);
    expect(result.publishedIds).toEqual([111, 222]);

    // Verify Publish
    expect(mockPublish).toHaveBeenCalledTimes(2);

    // Verify Mark Processed
    expect(mockMarkProcessed).toHaveBeenCalledWith('test-user-id', 'fitbit', "111");
    expect(mockMarkProcessed).toHaveBeenCalledWith('test-user-id', 'fitbit', "222");
  });

  it('should skip already processed activities', async () => {
    const req = { body: { collectionType: 'activities', ownerId: 'fitbit-user', date: '2023-01-01' } };

    mockHasProcessed.mockResolvedValueOnce(true); // First activity processed

    mockFitbitGet.mockResolvedValueOnce({
      data: { activities: [{ logId: 111 }] }
    });

    const result = await (fitbitIngest as any)(req, {}, mockCtx);

    expect(result.publishedCount).toBe(0);
    expect(mockPublish).not.toHaveBeenCalled();
  });
});
