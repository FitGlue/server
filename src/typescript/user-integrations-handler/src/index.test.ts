import { handler } from './index';

// Mock shared dependencies
jest.mock('@fitglue/shared', () => {
  const original = jest.requireActual('@fitglue/shared');
  return {
    ...original,
    getSecret: jest.fn().mockResolvedValue('mock-client-id'),
    generateOAuthState: jest.fn().mockResolvedValue('mock-state-token'),
  };
});

describe('user-integrations-handler', () => {
  let req: any;

  let ctx: any;
  let mockUserService: any;
  let mockUserStore: any;
  let mockApiKeysStore: any;

  beforeEach(() => {
    mockUserService = {
      get: jest.fn(),
    };
    mockApiKeysStore = {
      deleteByUserAndLabel: jest.fn().mockResolvedValue(0),
    };
    mockUserStore = {
      deleteIntegration: jest.fn().mockResolvedValue(undefined),
      setIntegration: jest.fn(),
    };

    req = {
      method: 'GET',
      body: {},
      query: {},
      path: '/api/users/me/integrations',
    };

    ctx = {
      userId: 'user-1',
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
      },
      services: {
        user: mockUserService,
      },
      stores: {
        users: mockUserStore,
        apiKeys: mockApiKeysStore,
      },
    };

    // Set env
    process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-dev';
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET / (list integrations)', () => {
    it('returns 401 if no user', async () => {
      ctx.userId = undefined;
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('returns 404 if user not found', async () => {
      mockUserService.get.mockResolvedValue(null);
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('returns integration summary', async () => {
      mockUserService.get.mockResolvedValue({
        integrations: {
          strava: { enabled: true, athleteId: 123456, lastUsedAt: new Date() },
          fitbit: { enabled: false },
          hevy: { enabled: true, userId: 'hevy-user-12345', lastUsedAt: new Date() }
        }
      });

      const result: any = await handler(req, ctx);

      expect(result.strava.connected).toBe(true);
      expect(result.strava.externalUserId).toBe('123456');
      expect(result.hevy.connected).toBe(true);
      expect(result.hevy.externalUserId).toContain('***'); // Masked
    });
  });

  describe('POST /{provider}/connect', () => {
    beforeEach(() => {
      req.method = 'POST';
      req.path = '/api/users/me/integrations/strava/connect';
      // Mock user for connection limit checks
      mockUserService.get.mockResolvedValue({
        userId: 'user-1',
        tier: 'hobbyist',
        integrations: {}
      });
    });

    it('returns 400 for invalid provider', async () => {
      req.path = '/api/users/me/integrations/invalid/connect';
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns OAuth URL for strava', async () => {
      const result: any = await handler(req, ctx);

      expect(result.url).toContain('strava.com/oauth/authorize');
      expect(result.url).toContain('mock-state-token');
    });

    it('returns OAuth URL for fitbit', async () => {
      req.path = '/api/users/me/integrations/fitbit/connect';
      const result: any = await handler(req, ctx);
      expect(result.url).toContain('fitbit.com/oauth2/authorize');
    });
  });

  describe('DELETE /{provider}', () => {
    beforeEach(() => {
      req.method = 'DELETE';
      req.path = '/api/users/me/integrations/strava';
      // Mock user lookup needed for disconnect
      mockUserService.get.mockResolvedValue({
        integrations: {
          strava: { enabled: true, athleteId: 12345, accessToken: 'tok', refreshToken: 'ref' }
        }
      });
    });

    it('returns 400 for invalid provider', async () => {
      req.path = '/api/users/me/integrations/invalid';
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));
    });

    it('returns 404 if user not found', async () => {
      mockUserService.get.mockResolvedValue(null);
      await expect(handler(req, ctx)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('disconnects integration successfully', async () => {
      await handler(req, ctx);
      expect(mockUserStore.deleteIntegration).toHaveBeenCalledWith(
        'user-1',
        'strava'
      );
    });

    it('deletes associated ingress keys for hevy', async () => {
      req.path = '/api/users/me/integrations/hevy';
      mockUserService.get.mockResolvedValue({
        integrations: {
          hevy: { enabled: true, apiKey: 'secret', userId: 'hevy-123' }
        }
      });
      mockApiKeysStore.deleteByUserAndLabel.mockResolvedValue(1);

      await handler(req, ctx);

      expect(mockUserStore.deleteIntegration).toHaveBeenCalledWith('user-1', 'hevy');
      expect(mockApiKeysStore.deleteByUserAndLabel).toHaveBeenCalledWith('user-1', 'Hevy Webhook');
    });
  });
});
