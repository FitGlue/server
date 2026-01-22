import { handler } from './index';

// Mock shared dependencies
jest.mock('@fitglue/shared', () => {
  const original = jest.requireActual('@fitglue/shared');
  return {
    ...original,
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
    UserStore: jest.fn(),
    UserService: jest.fn(),
    ActivityStore: jest.fn(),
    ForbiddenError: class ForbiddenError extends Error {
      constructor(message: string = 'Access denied') {
        super(message);
        this.name = 'ForbiddenError';
      }
    }
  };
});

import { UserService } from '@fitglue/shared';

describe('user-profile-handler', () => {
  let req: any;

  let ctx: any;
  let mockUserService: any;
  let mockAuthorizationService: any;

  beforeEach(() => {
    mockUserService = {
      get: jest.fn(),
      deleteUser: jest.fn()
    };
    mockAuthorizationService = {
      requireAdmin: jest.fn(),
      requireAccess: jest.fn(),
      isAdmin: jest.fn(),
      canAccessUser: jest.fn()
    };
    (UserService as any).mockImplementation(() => mockUserService);

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
        },
        pipelines: [
          { id: 'p1', source: 'hevy', enrichers: [], destinations: [1] }
        ]
      });

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
      const { ForbiddenError: FE } = jest.requireMock('@fitglue/shared');
      mockAuthorizationService.requireAdmin.mockRejectedValue(new FE('Admin access required'));
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 403 }));
    });

    it('returns user list for admin', async () => {
      mockAuthorizationService.requireAdmin.mockResolvedValue(undefined);
      const { db } = jest.requireMock('@fitglue/shared');
      db.get.mockResolvedValue({
        docs: [
          { id: 'user-1', data: () => ({ tier: 2, isAdmin: true }) }, // USER_TIER_ATHLETE
          { id: 'user-2', data: () => ({ tier: 1, isAdmin: false }) } // USER_TIER_HOBBYIST
        ]
      });

      const result: any = await handler(req, ctx);

      expect(mockAuthorizationService.requireAdmin).toHaveBeenCalledWith('user-1');
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
      const { db } = jest.requireMock('@fitglue/shared');
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

