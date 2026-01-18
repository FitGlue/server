import * as admin from 'firebase-admin';

// Mock firebase-admin
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
    firestore: jest.fn(() => mockFirestore),
  };
});

// Mock @google-cloud/functions-framework
jest.mock('@google-cloud/functions-framework', () => ({
  http: jest.fn(),
}));

import { showcaseHandler } from './index';

describe('showcase-handler', () => {
  let req: any;
  let res: any;
  let mockDocGet: jest.Mock;
  let mockFirestore: any;

  beforeEach(() => {
    // Get the mocked firestore
    mockFirestore = (admin.firestore as jest.Mock)();
    mockDocGet = mockFirestore.collection().doc().get;

    req = {
      method: 'GET',
      path: '/api/showcase/test-id',
    };

    res = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn(),
      send: jest.fn(),
      set: jest.fn(),
      redirect: jest.fn(),
    };

    jest.clearAllMocks();
  });

  describe('CORS handling', () => {
    it('responds to OPTIONS with 204', async () => {
      req.method = 'OPTIONS';
      await showcaseHandler(req, res);
      expect(res.status).toHaveBeenCalledWith(204);
      expect(res.send).toHaveBeenCalledWith('');
    });

    it('sets CORS headers on all requests', async () => {
      mockDocGet.mockResolvedValue({ exists: false });
      await showcaseHandler(req, res);
      expect(res.set).toHaveBeenCalledWith('Access-Control-Allow-Origin', '*');
      expect(res.set).toHaveBeenCalledWith('Access-Control-Allow-Methods', 'GET, OPTIONS');
    });
  });

  describe('method validation', () => {
    it('rejects non-GET/OPTIONS methods', async () => {
      req.method = 'POST';
      await showcaseHandler(req, res);
      expect(res.status).toHaveBeenCalledWith(405);
      expect(res.json).toHaveBeenCalledWith({ error: 'Method Not Allowed' });
    });
  });

  describe('showcase ID extraction', () => {
    it('extracts ID from /api/showcase/{id} path', async () => {
      req.path = '/api/showcase/my-activity-id';
      mockDocGet.mockResolvedValue({ exists: false });
      await showcaseHandler(req, res);
      expect(mockFirestore.collection).toHaveBeenCalledWith('showcased_activities');
      expect(mockFirestore.collection().doc).toHaveBeenCalledWith('my-activity-id');
    });

    it('extracts ID from /showcase/{id} path', async () => {
      req.path = '/showcase/my-activity-id';
      mockDocGet.mockResolvedValue({ exists: false });
      await showcaseHandler(req, res);
      expect(mockFirestore.collection().doc).toHaveBeenCalledWith('my-activity-id');
    });

    it('returns 400 if no showcase ID provided', async () => {
      req.path = '/api/showcase/';
      await showcaseHandler(req, res);
      expect(res.status).toHaveBeenCalledWith(400);
      expect(res.json).toHaveBeenCalledWith({ error: 'Missing showcase ID' });
    });
  });

  describe('showcase retrieval', () => {
    it('returns 404 if showcase not found', async () => {
      mockDocGet.mockResolvedValue({ exists: false });
      await showcaseHandler(req, res);
      expect(res.status).toHaveBeenCalledWith(404);
      expect(res.json).toHaveBeenCalledWith({ error: 'Showcase not found' });
    });

    it('returns 410 if showcase expired', async () => {
      const pastDate = new Date(Date.now() - 86400000); // 1 day ago
      mockDocGet.mockResolvedValue({
        exists: true,
        data: () => ({
          showcaseId: 'test-id',
          expiresAt: { toDate: () => pastDate },
        }),
      });
      await showcaseHandler(req, res);
      expect(res.status).toHaveBeenCalledWith(410);
      expect(res.json).toHaveBeenCalledWith({ error: 'This showcase has expired' });
    });

    it('returns showcase data with correct format', async () => {
      const now = new Date();
      const futureDate = new Date(Date.now() + 86400000); // 1 day from now
      mockDocGet.mockResolvedValue({
        exists: true,
        data: () => ({
          showcaseId: 'test-id',
          title: 'Morning Run',
          description: 'A nice run',
          activityType: 27, // RUN
          source: 1, // FITBIT
          startTime: { toDate: () => now },
          createdAt: { toDate: () => now },
          expiresAt: { toDate: () => futureDate },
          appliedEnrichments: ['fitbit-heart-rate'],
          enrichmentMetadata: { hr: 'true' },
          tags: ['running'],
          // These should be stripped
          userId: 'should-not-appear',
          activityId: 'should-not-appear',
        }),
      });

      await showcaseHandler(req, res);

      expect(res.status).toHaveBeenCalledWith(200);
      const responseData = res.json.mock.calls[0][0];
      expect(responseData.showcaseId).toBe('test-id');
      expect(responseData.title).toBe('Morning Run');
      expect(responseData).not.toHaveProperty('userId');
      expect(responseData).not.toHaveProperty('activityId');
    });

    it('sets immutable cache headers', async () => {
      mockDocGet.mockResolvedValue({
        exists: true,
        data: () => ({
          showcaseId: 'test-id',
          title: 'Test',
          description: '',
          activityType: 0,
          source: 0,
        }),
      });

      await showcaseHandler(req, res);

      expect(res.set).toHaveBeenCalledWith(
        'Cache-Control',
        'public, max-age=31536000, immutable'
      );
    });
  });

  describe('HTML redirect', () => {
    it('redirects to static page for /showcase/{id} paths', async () => {
      req.path = '/showcase/my-activity';
      mockDocGet.mockResolvedValue({
        exists: true,
        data: () => ({
          showcaseId: 'my-activity',
          title: 'Test',
        }),
      });

      await showcaseHandler(req, res);

      expect(res.redirect).toHaveBeenCalledWith(302, '/showcase.html?id=my-activity');
    });
  });

  describe('error handling', () => {
    it('returns 500 on Firestore errors', async () => {
      mockDocGet.mockRejectedValue(new Error('Firestore error'));
      await showcaseHandler(req, res);
      expect(res.status).toHaveBeenCalledWith(500);
      expect(res.json).toHaveBeenCalledWith({ error: 'Internal Server Error' });
    });
  });
});
