import * as admin from 'firebase-admin';
import { FrameworkResponse, HttpError, ShowcaseStore, UserTier } from '@fitglue/shared';

// Mock firebase-admin for user lookup
jest.mock('firebase-admin', () => {
  const mockDoc = {
    get: jest.fn(),
  };
  const mockCollection = {
    doc: jest.fn(() => mockDoc),
  };
  const mockFirestore = {
    collection: jest.fn(() => mockCollection),
  };
  return {
    apps: [],
    initializeApp: jest.fn(),
    firestore: Object.assign(jest.fn(() => mockFirestore), {
      Timestamp: {
        now: () => ({ toDate: () => new Date() }),
        fromDate: (d: Date) => ({ toDate: () => d }),
      },
    }),
  };
});

// Mock ShowcaseStore and createCloudFunction
jest.mock('@fitglue/shared', () => {
  const actual = jest.requireActual('@fitglue/shared');
  return {
    ...actual,
    createCloudFunction: (handler: any) => handler,
    ShowcaseStore: jest.fn(),
  };
});

import { showcaseHandler } from './index';

describe('showcase-handler', () => {
  let req: any;
  let ctx: any;
  let mockShowcaseStore: any;
  let mockFirestore: any;

  beforeEach(() => {
    mockFirestore = (admin.firestore as unknown as jest.Mock)();

    mockShowcaseStore = {
      get: jest.fn(),
      exists: jest.fn(),
    };
    (ShowcaseStore as unknown as jest.Mock).mockImplementation(() => mockShowcaseStore);

    req = {
      method: 'GET',
      path: '/api/showcase/test-id',
    };

    ctx = {
      stores: {
        db: mockFirestore,
      },
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        debug: jest.fn(),
      },
    };

    jest.clearAllMocks();
  });

  describe('CORS handling', () => {
    it('responds to OPTIONS with 204', async () => {
      req.method = 'OPTIONS';
      const result = await showcaseHandler(req, ctx);
      expect(result).toBeInstanceOf(FrameworkResponse);
      expect((result as unknown as FrameworkResponse).options.status).toBe(204);
    });
  });

  describe('method validation', () => {
    it('rejects non-GET/OPTIONS methods', async () => {
      req.method = 'POST';
      await expect(showcaseHandler(req, ctx)).rejects.toThrow(HttpError);
    });
  });

  describe('showcase ID extraction', () => {
    it('extracts ID from /api/showcase/{id} path', async () => {
      req.path = '/api/showcase/my-activity-id';
      mockShowcaseStore.get.mockResolvedValue(null);
      try {
        await showcaseHandler(req, ctx);
      } catch (e) {
        // Expected to throw 404
      }
      expect(mockShowcaseStore.get).toHaveBeenCalledWith('my-activity-id');
    });

    it('extracts ID from /showcase/{id} path', async () => {
      req.path = '/showcase/my-activity-id';
      mockShowcaseStore.get.mockResolvedValue(null);
      try {
        await showcaseHandler(req, ctx);
      } catch (e) {
        // Expected to throw 404
      }
      expect(mockShowcaseStore.get).toHaveBeenCalledWith('my-activity-id');
    });
  });

  describe('showcase retrieval', () => {
    it('returns 404 if showcase not found', async () => {
      mockShowcaseStore.get.mockResolvedValue(null);
      await expect(showcaseHandler(req, ctx)).rejects.toThrow(HttpError);
    });

    it('returns 410 if showcase expired', async () => {
      const pastDate = new Date(Date.now() - 86400000);
      mockShowcaseStore.get.mockResolvedValue({
        showcaseId: 'test-id',
        expiresAt: pastDate,
      });
      await expect(showcaseHandler(req, ctx)).rejects.toThrow(HttpError);
    });

    it('returns showcase data with correct format', async () => {
      const now = new Date();
      mockShowcaseStore.get.mockResolvedValue({
        showcaseId: 'test-id',
        userId: 'user-123',
        title: 'Morning Run',
        activityType: 27,
        source: 1,
        startTime: now,
        createdAt: now,
        appliedEnrichments: ['fitbit-heart-rate'],
      });

      // Mock user lookup
      const mockUserGet = jest.fn().mockResolvedValue({
        data: () => ({
          tier: UserTier.USER_TIER_ATHLETE,
        }),
      });
      mockFirestore.collection.mockReturnValue({
        doc: jest.fn().mockReturnValue({ get: mockUserGet }),
      });

      const result = await showcaseHandler(req, ctx);
      expect((result as unknown as FrameworkResponse).options.status).toBe(200);
      const responseData: any = (result as unknown as FrameworkResponse).options.body;
      expect(responseData.showcaseId).toBe('test-id');
      expect(responseData.isAthlete).toBe(true);
    });

    it('sets immutable cache headers', async () => {
      mockShowcaseStore.get.mockResolvedValue({
        showcaseId: 'test-id',
        userId: 'user-123',
        title: 'Test',
      });

      const result = await showcaseHandler(req, ctx);
      expect((result as unknown as FrameworkResponse).options.headers).toMatchObject({
        'Cache-Control': 'public, max-age=31536000, immutable',
      });
    });
  });

  describe('HTML redirect', () => {
    it('redirects to static page for /showcase/{id} paths', async () => {
      req.path = '/showcase/my-activity';
      mockShowcaseStore.get.mockResolvedValue({
        showcaseId: 'my-activity',
        userId: 'user-123',
      });

      const result = await showcaseHandler(req, ctx);
      expect((result as unknown as FrameworkResponse).options.status).toBe(302);
      expect((result as unknown as FrameworkResponse).options.headers).toMatchObject({
        'Location': '/showcase.html?id=my-activity',
      });
    });
  });

  describe('error handling', () => {
    it('throws on Firestore errors', async () => {
      mockShowcaseStore.get.mockRejectedValue(new Error('Firestore error'));
      await expect(showcaseHandler(req, ctx)).rejects.toThrow('Firestore error');
    });
  });
});
