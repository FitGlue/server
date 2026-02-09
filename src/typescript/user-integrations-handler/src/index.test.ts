import { handler } from './index';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: (handler: any, _opts?: any) => handler,
  FrameworkContext: jest.fn(),
  FirebaseAuthStrategy: jest.fn(),
  FrameworkHandler: jest.fn(),
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
  return { HttpError };
});

// Mock @fitglue/shared/routing
jest.mock('@fitglue/shared/routing', () => {
  class HttpError extends Error {
    statusCode: number;
    constructor(statusCode: number, message: string) {
      super(message);
      this.statusCode = statusCode;
      this.name = 'HttpError';
    }
  }

  async function routeRequest(
    req: { method: string; path: string; query?: Record<string, unknown>; body?: unknown },
    ctx: unknown,
    routes: Array<{ method: string; pattern: string; handler: (match: { params: Record<string, string>; query: Record<string, string> }, req: unknown, ctx: unknown) => Promise<unknown> }>
  ): Promise<unknown> {
    for (const route of routes) {
      if (route.method !== req.method) continue;

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
        return await route.handler({ params, query: (req.query || {}) as Record<string, string> }, req, ctx);
      }
    }

    throw new HttpError(404, 'Not found');
  }

  return { routeRequest, RouteMatch: jest.fn() };
});

// Mock @fitglue/shared/plugin
jest.mock('@fitglue/shared/plugin', () => ({
  getRegistry: jest.fn(() => ({
    integrations: [
      { id: 'strava', authType: 'oauth2', icon: '🏃', name: 'Strava' },
      { id: 'fitbit', authType: 'oauth2', icon: '💪', name: 'Fitbit' },
      { id: 'hevy', authType: 'api_key', icon: '🏋️', name: 'Hevy' },
      { id: 'polar', authType: 'oauth2', icon: '❄️', name: 'Polar' },
      { id: 'oura', authType: 'oauth2', icon: '💍', name: 'Oura' },
      { id: 'wahoo', authType: 'oauth2', icon: '🚴', name: 'Wahoo' },
      { id: 'spotify', authType: 'oauth2', icon: '🎵', name: 'Spotify' },
      { id: 'trainingpeaks', authType: 'oauth2', icon: '📊', name: 'TrainingPeaks' },
      { id: 'google_sheets', authType: 'oauth2', icon: '📋', name: 'Google Sheets' },
    ],
  })),
}));

// Mock @fitglue/shared/types
jest.mock('@fitglue/shared/types', () => ({
  IntegrationAuthType: {
    AUTH_TYPE_UNSPECIFIED: 0,
    AUTH_TYPE_OAUTH2: 1,
    AUTH_TYPE_API_KEY: 2,
  },
}));

// Mock @fitglue/shared/infrastructure/oauth
jest.mock('@fitglue/shared/infrastructure/oauth', () => ({
  generateOAuthState: jest.fn().mockResolvedValue('mock-state-token'),
}));

// Mock @fitglue/shared/infrastructure
jest.mock('@fitglue/shared/infrastructure', () => ({
  getSecret: jest.fn().mockResolvedValue('mock-client-id'),
}));

// Mock @fitglue/shared/domain
jest.mock('@fitglue/shared/domain', () => ({
  canAddConnection: jest.fn(() => ({ allowed: true, reason: null })),
  countActiveConnections: jest.fn(() => 0),
}));

// Mock @fitglue/shared/integrations/hevy
jest.mock('@fitglue/shared/integrations/hevy', () => ({
  createHevyClient: jest.fn(() => ({
    getUser: jest.fn(),
  })),
}));

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
          hevy: { enabled: true, userId: 'hevy-user-12345', lastUsedAt: new Date() },
          github: { enabled: true, githubUserId: 'gh-12345', lastUsedAt: new Date() },
        }
      });

      const result: any = await handler(req, ctx);

      expect(result.strava.connected).toBe(true);
      expect(result.strava.externalUserId).toBe('123456');
      expect(result.hevy.connected).toBe(true);
      expect(result.hevy.externalUserId).toContain('***'); // Masked
      expect(result.github.connected).toBe(true);
      expect(result.github.externalUserId).toBe('gh-12345');
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

    it('returns OAuth URL for github', async () => {
      req.path = '/api/users/me/integrations/github/connect';
      const result: any = await handler(req, ctx);
      expect(result.url).toContain('github.com/login/oauth/authorize');
      expect(result.url).toContain('repo');
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
