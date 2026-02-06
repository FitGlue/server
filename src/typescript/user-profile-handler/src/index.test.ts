import { handler } from './index';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: jest.fn((handler: any) => handler),
  FirebaseAuthStrategy: jest.fn(),
  FrameworkHandler: jest.fn(),
  db: {
    collection: jest.fn().mockReturnThis(),
    doc: jest.fn().mockReturnThis(),
    where: jest.fn().mockReturnThis(),
    get: jest.fn().mockResolvedValue({ empty: true, size: 0, forEach: jest.fn(), docs: [] }),
    batch: jest.fn().mockReturnValue({
      delete: jest.fn(),
      commit: jest.fn().mockResolvedValue(undefined)
    }),
    update: jest.fn().mockResolvedValue(undefined)
  },
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
  class ForbiddenError extends Error {
    statusCode: number;
    constructor(message: string = 'Access denied') {
      super(message);
      this.name = 'ForbiddenError';
      this.statusCode = 403;
    }
  }
  return { HttpError, ForbiddenError };
});

// Mock @fitglue/shared/types
jest.mock('@fitglue/shared/types', () => ({
  INTEGRATIONS: {
    strava: { authType: 1, externalUserIdField: 'athleteId' },
    fitbit: { authType: 1, externalUserIdField: 'userId' },
    hevy: { authType: 2, externalUserIdField: 'apiKey' },
    polar: { authType: 1, externalUserIdField: 'userId' },
    oura: { authType: 1, externalUserIdField: 'userId' },
    wahoo: { authType: 1, externalUserIdField: 'userId' },
    spotify: { authType: 1, externalUserIdField: 'userId' },
  },
  IntegrationAuthType: {
    INTEGRATION_AUTH_TYPE_UNSPECIFIED: 0,
    INTEGRATION_AUTH_TYPE_OAUTH2: 1,
    INTEGRATION_AUTH_TYPE_API_KEY: 2,
  },
}));

describe('user-profile-handler', () => {
  let req: any;

  let ctx: any;
  let mockUserService: any;
  let mockAuthorizationService: any;

  beforeEach(() => {
    mockUserService = {
      get: jest.fn(),
      deleteUser: jest.fn(),
      pipelineStore: {
        list: jest.fn().mockResolvedValue([])
      }
    };
    mockAuthorizationService = {
      requireAdmin: jest.fn(),
      requireAccess: jest.fn(),
      isAdmin: jest.fn(),
      canAccessUser: jest.fn()
    };

    req = {
      method: 'GET',
      body: {},
      path: '/users/me'
    };

    ctx = {
      userId: 'user-1',
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn()
      },
      services: {
        user: mockUserService,
        authorization: mockAuthorizationService
      }
    };
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET /users/me', () => {
    it('returns 401 if no user', async () => {
      ctx.userId = undefined;
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('returns 404 if user not found', async () => {
      mockUserService.get.mockResolvedValue(null);
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('returns user profile with integrations and pipelines', async () => {
      mockUserService.get.mockResolvedValue({
        userId: 'user-1',
        createdAt: new Date('2024-01-01'),
        integrations: {
          strava: { enabled: true, athleteId: 123456 },
          fitbit: { enabled: false },
          hevy: { enabled: true, apiKey: 'abc123xyz789' }
        }
      });

      mockUserService.pipelineStore.list.mockResolvedValue([
        { id: 'p1', source: 'hevy', enrichers: [], destinations: [1] }
      ]);

      const result: any = await handler(req, ctx);

      expect(result.userId).toBe('user-1');
      expect(result.integrations.strava.connected).toBe(true);
      expect(result.integrations.hevy.connected).toBe(true);
      expect(result.pipelines).toHaveLength(1);
    });
  });

  describe('GET /admin/users', () => {
    beforeEach(() => {
      req.path = '/admin/users';
    });

    it('returns 403 if not admin', async () => {
      const { ForbiddenError: FE } = jest.requireMock('@fitglue/shared/errors');
      mockAuthorizationService.requireAdmin.mockRejectedValue(new FE('Admin access required'));
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 403 }));
    });

    it('returns user list for admin', async () => {
      mockAuthorizationService.requireAdmin.mockResolvedValue(undefined);
      mockUserService.listUsers = jest.fn().mockResolvedValue([
        { userId: 'user-1', tier: 2, isAdmin: true, createdAt: new Date(), trialEndsAt: null, syncCountThisMonth: 0, stripeCustomerId: null },
        { userId: 'user-2', tier: 1, isAdmin: false, createdAt: new Date(), trialEndsAt: null, syncCountThisMonth: 0, stripeCustomerId: null }
      ]);

      const result: any = await handler(req, ctx);

      expect(mockAuthorizationService.requireAdmin).toHaveBeenCalledWith('user-1');
      expect(mockUserService.listUsers).toHaveBeenCalled();
      expect(result).toHaveLength(2);
    });
  });

  describe('PATCH /users/me', () => {
    beforeEach(() => {
      req.method = 'PATCH';
    });

    it('returns success (currently no-op)', async () => {
      const result = await handler(req, ctx);
      expect(result).toEqual({ success: true });
    });
  });

  describe('DELETE /users/me', () => {
    beforeEach(() => {
      req.method = 'DELETE';
      req.path = '/users/me';
      // Reset db.get to return empty collections for cascade delete
      const { db } = jest.requireMock('@fitglue/shared/framework');
      db.get.mockResolvedValue({ empty: true, size: 0, forEach: jest.fn(), docs: [] });
    });

    it('returns 401 if no user', async () => {
      ctx.userId = undefined;
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('cascade deletes user and returns success', async () => {
      const result = await handler(req, ctx);
      expect(mockUserService.deleteUser).toHaveBeenCalledWith('user-1');
      expect(result).toEqual({ success: true });
    });
  });

  describe('Unsupported methods', () => {
    it('returns 405 for POST', async () => {
      req.method = 'POST';
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 405 }));
    });
  });
});

