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

  class HttpError extends Error {
    statusCode: number;
    details?: unknown;
    constructor(statusCode: number, message: string, details?: unknown) {
      super(message);
      this.statusCode = statusCode;
      this.details = details;
      this.name = 'HttpError';
      Object.setPrototypeOf(this, HttpError.prototype);
    }
  }

  // Simple route matching implementation for tests
  async function routeRequest(
    req: { method: string; path: string; query: Record<string, unknown> },
    ctx: unknown,
    routes: Array<{ method: string; pattern: string; handler: (match: { params: Record<string, string>; query: Record<string, string> }, req: unknown, ctx: unknown) => Promise<unknown> }>
  ): Promise<unknown> {
    for (const route of routes) {
      if (route.method !== req.method) continue;

      // Simple pattern matching
      const patternParts = route.pattern.split('/').filter(Boolean);
      const pathParts = req.path.split('/').filter(Boolean);

      if (patternParts.length !== pathParts.length) continue;

      const params: Record<string, string> = {};
      let matched = true;

      for (let i = 0; i < patternParts.length; i++) {
        if (patternParts[i].startsWith(':')) {
          params[patternParts[i].slice(1)] = pathParts[i];
        } else if (patternParts[i] !== pathParts[i]) {
          matched = false;
          break;
        }
      }

      if (matched) {
        return await route.handler({ params, query: req.query as Record<string, string> }, req, ctx);
      }
    }

    throw new HttpError(404, 'Not found');
  }

  return {
    createCloudFunction: jest.fn((fn) => fn),
    FrameworkContext: jest.fn(),
    FrameworkHandler: jest.fn(),
    FirebaseAuthStrategy: jest.fn(),
    ForbiddenError: class ForbiddenError extends Error {
      constructor(message: string) {
        super(message);
        this.name = 'ForbiddenError';
      }
    },
    HttpError,
    routeRequest,
    RouteMatch: jest.fn(),
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
    formatExecutionStatus: (status: number | string | undefined | null) => {
      if (status === 2) return 'SUCCESS';
      if (status === 3) return 'FAILED';
      if (status === 1) return 'STARTED';
      return 'UNKNOWN';
    },
    formatDestination: (dest: unknown) => String(dest),
    ExecutionStatus: {
      STATUS_UNKNOWN: 0,
      STATUS_STARTED: 1,
      STATUS_SUCCESS: 2,
      STATUS_FAILED: 3,
      STATUS_PENDING: 4,
      STATUS_WAITING: 5,
    },
    PendingInput_Status: {
      STATUS_UNSPECIFIED: 0,
      STATUS_WAITING: 1,
      STATUS_COMPLETED: 2,
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

// Helper to create mock request - casts to handler's expected request type
function createMockRequest(overrides: Record<string, unknown> = {}): Parameters<typeof handler>[0] {
  return {
    method: 'GET',
    path: '/api/admin/stats',
    query: {},
    body: {},
    ...overrides,
  } as Parameters<typeof handler>[0];
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
        pipelineStore: {
          list: jest.fn().mockResolvedValue([]),
          delete: jest.fn().mockResolvedValue(undefined),
          toggleDisabled: jest.fn().mockResolvedValue(undefined),
        },
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
      const ctx = createMockContext({ userId: undefined });

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('returns 403 if not admin', async () => {
      const req = createMockRequest();
      const ctx = createMockContext();
      ctx.services.authorization.requireAdmin.mockRejectedValue(new ForbiddenError('Not admin'));

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 403 }));
    });
  });

  describe('GET /api/admin/stats', () => {
    it('returns platform statistics', async () => {
      const req = createMockRequest({ path: '/api/admin/stats', method: 'GET' });
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

      const result = await handler(req, ctx);

      expect(result).toEqual(expect.objectContaining({
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

      const result = await handler(req, ctx);

      expect(result).toEqual({
        data: [
          {
            userId: 'user-1',
            createdAt: undefined,
            tier: 2, // USER_TIER_ATHLETE
            trialEndsAt: undefined,
            isAdmin: false,
            accessEnabled: false,
            syncCountThisMonth: 5,
            stripeCustomerId: null,
            preventedSyncCount: 0,
            integrations: ['strava'],
          },
        ],
        pagination: {
          page: 1,
          limit: 25,
          total: 1,
          hasMore: false,
        },
      });
    });
  });

  describe('GET /api/admin/users/:id', () => {
    it('returns full user details', async () => {
      const req = createMockRequest({ path: '/api/admin/users/user-123', method: 'GET' });
      const ctx = createMockContext();

      ctx.services.user.get.mockResolvedValue({
        userId: 'user-123',
        tier: 2, // USER_TIER_ATHLETE
        isAdmin: false,
        syncCountThisMonth: 10,
        integrations: {
          hevy: { enabled: true, apiKey: 'super-secret-api-key-12345' },
        },
      });

      // Mock pipelineStore.list to return empty array
      ctx.services.user.pipelineStore.list.mockResolvedValue([]);

      // Mock pending inputs query (single where, filter in memory)
      (mockDb.collection as jest.Mock).mockReturnValue({
        where: jest.fn().mockReturnValue({
          get: jest.fn().mockResolvedValue({
            docs: [{ data: () => ({ activity_id: 'act-1', status: 1 }) }],
          }),
        }),
      });

      const result: any = await handler(req, ctx);

      expect(result).toEqual(expect.objectContaining({
        userId: 'user-123',
        tier: 2, // USER_TIER_ATHLETE
        email: 'test@example.com',
        displayName: 'Test User',
        activityCount: 5,
        pendingInputCount: 1,
      }));
      // Verify token is masked
      expect(result.integrations.hevy.apiKey).toMatch(/^\w{4}\*{4}\w{4}$/);
    });

    it('returns 404 if user not found', async () => {
      const req = createMockRequest({ path: '/api/admin/users/unknown', method: 'GET' });
      const ctx = createMockContext();
      ctx.services.user.get.mockResolvedValue(null);

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });
  });

  describe('PATCH /api/admin/users/:id', () => {
    it('updates user tier and admin status', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123',
        method: 'PATCH',
        body: { tier: 'athlete', isAdmin: true },
      });
      const ctx = createMockContext();

      const mockUpdate = jest.fn().mockResolvedValue(undefined);
      (mockDb.collection as jest.Mock).mockReturnValue({
        doc: jest.fn().mockReturnValue({
          update: mockUpdate,
        }),
      });

      const result = await handler(req, ctx);

      expect(mockUpdate).toHaveBeenCalledWith({ tier: 'athlete', is_admin: true });
      expect(result).toEqual({ success: true });
    });
  });

  describe('DELETE /api/admin/users/:id/integrations/:provider', () => {
    it('removes integration', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123/integrations/strava',
        method: 'DELETE',
      });
      const ctx = createMockContext();

      const mockUpdate = jest.fn().mockResolvedValue(undefined);
      (mockDb.collection as jest.Mock).mockReturnValue({
        doc: jest.fn().mockReturnValue({
          update: mockUpdate,
        }),
      });

      const result = await handler(req, ctx);

      expect(mockUpdate).toHaveBeenCalledWith({ 'integrations.strava': null });
      expect(result).toEqual({ success: true });
    });
  });

  describe('DELETE /api/admin/users/:id/pipelines/:pipelineId', () => {
    it('removes pipeline', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123/pipelines/pipe-456',
        method: 'DELETE',
      });
      const ctx = createMockContext();

      const result = await handler(req, ctx);

      expect(ctx.services.user.pipelineStore.delete).toHaveBeenCalledWith('user-123', 'pipe-456');
      expect(result).toEqual({ success: true });
    });
  });

  describe('DELETE /api/admin/users/:id/activities', () => {
    it('deletes all user activities in batches', async () => {
      const req = createMockRequest({
        path: '/api/admin/users/user-123/activities',
        method: 'DELETE',
      });
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

      const result = await handler(req, ctx);

      expect(result).toEqual({ success: true, deletedCount: 2 });
    });
  });

  describe('GET /api/admin/executions', () => {
    it('returns executions list with filters', async () => {
      const req = createMockRequest({
        path: '/api/admin/executions',
        method: 'GET',
        query: { service: 'enricher', limit: '10' },
      });
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

      const result = await handler(req, ctx);

      expect(ctx.services.execution.listExecutions).toHaveBeenCalledWith(
        expect.objectContaining({ service: 'enricher', limit: 10 })
      );
      expect(result).toEqual(expect.objectContaining({
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
      const ctx = createMockContext();

      ctx.stores.executions.get.mockResolvedValue({
        service: 'enricher',
        status: ExecutionStatus.STATUS_SUCCESS,
        userId: 'user-1',
        inputsJson: '{"test": true}',
        outputsJson: '{"result": "ok"}',
      });

      const result = await handler(req, ctx);

      expect(result).toEqual(expect.objectContaining({
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
      const ctx = createMockContext();
      ctx.stores.executions.get.mockResolvedValue(null);

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });
  });

  describe('Unknown paths', () => {
    it('returns 404 for unknown paths', async () => {
      const req = createMockRequest({ path: '/api/admin/unknown', method: 'GET' });
      const ctx = createMockContext();

      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });
  });
});
