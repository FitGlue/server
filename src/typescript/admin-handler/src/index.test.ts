import { Request, Response } from 'express';
import { handler } from './index';
import { ExecutionStatus } from '@fitglue/shared/dist/types/pb/execution';

// Mock @fitglue/shared
jest.mock('@fitglue/shared', () => {
  const mockDb = {
    collection: jest.fn().mockReturnThis(),
    doc: jest.fn().mockReturnThis(),
    get: jest.fn(),
    update: jest.fn(),
    count: jest.fn().mockReturnThis(),
    where: jest.fn().mockReturnThis(),
    limit: jest.fn().mockReturnThis(),
    offset: jest.fn().mockReturnThis(),
    orderBy: jest.fn().mockReturnThis(),
    batch: jest.fn(),
    withConverter: jest.fn().mockReturnThis(),
  };

  return {
    createCloudFunction: jest.fn((fn) => fn),
    FrameworkContext: jest.fn(),
    FirebaseAuthStrategy: jest.fn(),
    ForbiddenError: class ForbiddenError extends Error {
      constructor(message: string) {
        super(message);
        this.name = 'ForbiddenError';
      }
    },
    db: mockDb,
    userConverter: {
      toFirestore: jest.fn(),
      fromFirestore: jest.fn(),
    },
    UserTier: {
      USER_TIER_UNSPECIFIED: 0,
      USER_TIER_HOBBYIST: 1,
      USER_TIER_ATHLETE: 2,
    },
  };
});

// Mock firebase-admin
jest.mock('firebase-admin', () => ({
  auth: jest.fn(() => ({
    getUser: jest.fn().mockResolvedValue({ email: 'test@example.com', displayName: 'Test User' })
  }))
}));

// Get the mocked db
import { db, ForbiddenError } from '@fitglue/shared';
const mockDb = db as jest.Mocked<typeof db>;

// Helper to create mock request
function createMockRequest(overrides: Partial<Request> = {}): Request {
  return {
    method: 'GET',
    path: '/api/admin/stats',
    query: {},
    body: {},
    ...overrides,
  } as Request;
}

// Helper to create mock response
function createMockResponse(): Response {
  const res: Partial<Response> = {
    status: jest.fn().mockReturnThis(),
    json: jest.fn().mockReturnThis(),
  };
  return res as Response;
}

// Helper to create mock context
function createMockContext(overrides: Partial<any> = {}): any {
  return {
    userId: 'admin-user-123',
    logger: {
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    },
    services: {
      authorization: {
        requireAdmin: jest.fn().mockResolvedValue(undefined),
      },
      user: {
        get: jest.fn(),
        removePipeline: jest.fn(),
      },
      execution: {
        listExecutions: jest.fn().mockResolvedValue([]),
      },
    },
    stores: {
      executions: {
        listRecent: jest.fn().mockResolvedValue([]),
        listDistinctServices: jest.fn().mockResolvedValue([]),
        get: jest.fn(),
      },
      activities: {
        countSynchronized: jest.fn().mockResolvedValue(5),
      },
    },
    ...overrides,
  };
}

describe('admin-handler', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Authentication & Authorization', () => {
    it('returns 401 if no userId', async () => {
      const req = createMockRequest();
      const res = createMockResponse();
      const ctx = createMockContext({ userId: undefined });

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(401);
      expect(res.json).toHaveBeenCalledWith({ error: 'Unauthorized' });
    });

    it('returns 403 if not admin', async () => {
      const req = createMockRequest();
      const res = createMockResponse();
      const ctx = createMockContext();
      ctx.services.authorization.requireAdmin.mockRejectedValue(new ForbiddenError('Not admin'));

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(403);
      expect(res.json).toHaveBeenCalledWith({ error: 'Admin access required' });
    });
  });

  describe('GET /api/admin/stats', () => {
    it('returns platform statistics', async () => {
      const req = createMockRequest({ path: '/api/admin/stats', method: 'GET' });
      const res = createMockResponse();
      const ctx = createMockContext();

      // Mock users collection
      (mockDb.collection as jest.Mock).mockReturnValue({
        withConverter: jest.fn().mockReturnThis(),
        get: jest.fn().mockResolvedValue({
          docs: [
            { data: () => ({ tier: 2, syncCountThisMonth: 10 }) }, // USER_TIER_ATHLETE
            { data: () => ({ tier: 1, syncCountThisMonth: 5, isAdmin: true }) }, // USER_TIER_HOBBYIST
          ],
        }),
      });

      // Mock execution stats
      ctx.stores.executions.listRecent.mockResolvedValue([
        { data: { status: ExecutionStatus.STATUS_SUCCESS } },
        { data: { status: ExecutionStatus.STATUS_FAILED } },
      ]);

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({
        totalUsers: 2,
        athleteUsers: 1,
        adminUsers: 1,
        totalSyncsThisMonth: 15,
        recentExecutions: expect.objectContaining({
          success: 1,
          failed: 1,
        }),
      }));
    });
  });

  describe('GET /api/admin/users', () => {
    it('returns enhanced user list with pagination', async () => {
      const req = createMockRequest({ path: '/api/admin/users', method: 'GET', query: {} });
      const res = createMockResponse();
      const ctx = createMockContext();

      // Mock count query
      const mockCount = jest.fn().mockReturnValue({
        get: jest.fn().mockResolvedValue({ data: () => ({ count: 1 }) }),
      });
      // Mock paginated query
      const mockGet = jest.fn().mockResolvedValue({
        docs: [
          {
            id: 'user-1',
            data: () => ({
              tier: 2, // USER_TIER_ATHLETE
              isAdmin: false,
              syncCountThisMonth: 5,
              integrations: { strava: { enabled: true } },
              pipelines: [{ id: 'p1' }],
            }),
          },
        ],
      });

      (mockDb.collection as jest.Mock).mockReturnValue({
        count: mockCount,
        withConverter: jest.fn().mockReturnThis(),
        limit: jest.fn().mockReturnValue({
          offset: jest.fn().mockReturnValue({
            get: mockGet,
          }),
        }),
      });

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(
        expect.objectContaining({
          data: expect.arrayContaining([
            expect.objectContaining({
              userId: 'user-1',
              tier: 2, // USER_TIER_ATHLETE
              integrations: ['strava'],
              pipelineCount: 1,
            }),
          ]),
          pagination: expect.objectContaining({
            page: 1,
            limit: 25,
            total: 1,
          }),
        })
      );
    });
  });

  describe('GET /api/admin/users/:id', () => {
    it('returns full user details', async () => {
      const req = createMockRequest({ path: '/api/admin/users/user-123', method: 'GET' });
      const res = createMockResponse();
      const ctx = createMockContext();

      ctx.services.user.get.mockResolvedValue({
        userId: 'user-123',
        tier: 2, // USER_TIER_ATHLETE
        isAdmin: false,
        syncCountThisMonth: 10,
        integrations: {
          hevy: { enabled: true, apiKey: 'super-secret-api-key-12345' },
        },
        pipelines: [],
      });

      // Mock pending inputs query (single where, filter in memory)
      (mockDb.collection as jest.Mock).mockReturnValue({
        where: jest.fn().mockReturnValue({
          get: jest.fn().mockResolvedValue({
            docs: [{ data: () => ({ activity_id: 'act-1', status: 1 }) }],
          }),
        }),
      });

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({
        userId: 'user-123',
        tier: 2, // USER_TIER_ATHLETE
        email: 'test@example.com',
        displayName: 'Test User',
        activityCount: 5,
        pendingInputCount: 1,
      }));
      // Verify token is masked
      const response = (res.json as jest.Mock).mock.calls[0][0];
      expect(response.integrations.hevy.apiKey).toMatch(/^\w{4}\*{4}\w{4}$/);
    });

    it('returns 404 if user not found', async () => {
      const req = createMockRequest({ path: '/api/admin/users/unknown', method: 'GET' });
      const res = createMockResponse();
      const ctx = createMockContext();
      ctx.services.user.get.mockResolvedValue(null);

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(404);
      expect(res.json).toHaveBeenCalledWith({ error: 'User not found' });
    });
  });

  describe('PATCH /api/admin/users/:id', () => {
    it('updates user tier and admin status', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123',
        method: 'PATCH',
        body: { tier: 'athlete', isAdmin: true },
      });
      const res = createMockResponse();
      const ctx = createMockContext();

      const mockUpdate = jest.fn().mockResolvedValue(undefined);
      (mockDb.collection as jest.Mock).mockReturnValue({
        doc: jest.fn().mockReturnValue({
          update: mockUpdate,
        }),
      });

      await handler(req, res, ctx);

      expect(mockUpdate).toHaveBeenCalledWith({ tier: 'athlete', is_admin: true });
      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({ success: true });
    });
  });

  describe('DELETE /api/admin/users/:id/integrations/:provider', () => {
    it('removes integration', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123/integrations/strava',
        method: 'DELETE',
      });
      const res = createMockResponse();
      const ctx = createMockContext();

      const mockUpdate = jest.fn().mockResolvedValue(undefined);
      (mockDb.collection as jest.Mock).mockReturnValue({
        doc: jest.fn().mockReturnValue({
          update: mockUpdate,
        }),
      });

      await handler(req, res, ctx);

      expect(mockUpdate).toHaveBeenCalledWith({ 'integrations.strava': null });
      expect(res.status).toHaveBeenCalledWith(200);
    });
  });

  describe('DELETE /api/admin/users/:id/pipelines/:pipelineId', () => {
    it('removes pipeline', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123/pipelines/pipe-456',
        method: 'DELETE',
      });
      const res = createMockResponse();
      const ctx = createMockContext();

      await handler(req, res, ctx);

      expect(ctx.services.user.removePipeline).toHaveBeenCalledWith('user-123', 'pipe-456');
      expect(res.status).toHaveBeenCalledWith(200);
    });
  });

  describe('DELETE /api/admin/users/:id/activities', () => {
    it('deletes all user activities in batches', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123/activities',
        method: 'DELETE',
      });
      const res = createMockResponse();
      const ctx = createMockContext();

      const mockBatch = {
        delete: jest.fn(),
        commit: jest.fn().mockResolvedValue(undefined),
      };

      // First call returns 2 docs, second call returns empty
      let callCount = 0;
      (mockDb.collection as jest.Mock).mockReturnValue({
        doc: jest.fn().mockReturnValue({
          collection: jest.fn().mockReturnValue({
            limit: jest.fn().mockReturnValue({
              get: jest.fn().mockImplementation(() => {
                callCount++;
                if (callCount === 1) {
                  return Promise.resolve({
                    empty: false,
                    size: 2,
                    docs: [{ ref: 'ref1' }, { ref: 'ref2' }],
                  });
                }
                return Promise.resolve({ empty: true, size: 0, docs: [] });
              }),
            }),
          }),
        }),
      });

      (mockDb.batch as jest.Mock).mockReturnValue(mockBatch);

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith({ success: true, deletedCount: 2 });
    });
  });

  describe('GET /api/admin/executions', () => {
    it('returns executions list with filters', async () => {
      const req = createMockRequest({
        path: '/api/admin/executions',
        method: 'GET',
        query: { service: 'enricher', limit: '10' },
      });
      const res = createMockResponse();
      const ctx = createMockContext();

      ctx.services.execution.listExecutions.mockResolvedValue([
        {
          id: 'exec-1',
          data: {
            service: 'enricher',
            status: ExecutionStatus.STATUS_SUCCESS,
            userId: 'user-1',
            timestamp: new Date().toISOString(),
          },
        },
      ]);
      ctx.stores.executions.listDistinctServices.mockResolvedValue(['enricher', 'router']);

      await handler(req, res, ctx);

      expect(ctx.services.execution.listExecutions).toHaveBeenCalledWith(
        expect.objectContaining({ service: 'enricher', limit: 10 })
      );
      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({
        availableServices: ['enricher', 'router'],
      }));
    });
  });

  describe('GET /api/admin/executions/:id', () => {
    it('returns execution details', async () => {
      const req = createMockRequest({
        path: '/api/admin/executions/exec-123',
        method: 'GET',
      });
      const res = createMockResponse();
      const ctx = createMockContext();

      ctx.stores.executions.get.mockResolvedValue({
        service: 'enricher',
        status: ExecutionStatus.STATUS_SUCCESS,
        userId: 'user-1',
        inputsJson: '{"test": true}',
        outputsJson: '{"result": "ok"}',
      });

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({
        id: 'exec-123',
        service: 'enricher',
        status: 'SUCCESS',
      }));
    });

    it('returns 404 if execution not found', async () => {
      const req = createMockRequest({
        path: '/api/admin/executions/unknown',
        method: 'GET',
      });
      const res = createMockResponse();
      const ctx = createMockContext();
      ctx.stores.executions.get.mockResolvedValue(null);

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(404);
      expect(res.json).toHaveBeenCalledWith({ error: 'Execution not found' });
    });
  });

  describe('Unknown paths', () => {
    it('returns 404 for unknown paths', async () => {
      const req = createMockRequest({ path: '/api/admin/unknown', method: 'GET' });
      const res = createMockResponse();
      const ctx = createMockContext();

      await handler(req, res, ctx);

      expect(res.status).toHaveBeenCalledWith(404);
      expect(res.json).toHaveBeenCalledWith({ error: 'Not Found' });
    });
  });
});
