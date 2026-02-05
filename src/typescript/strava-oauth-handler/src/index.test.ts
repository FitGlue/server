// Mock firebase-admin to prevent initialization
jest.mock('firebase-admin', () => ({
  apps: [],
  initializeApp: jest.fn(),
  firestore: jest.fn(() => ({
    collection: jest.fn(),
  })),
}));

// Create a simple FrameworkResponse class for testing
class MockFrameworkResponse {
  constructor(public readonly options: { status?: number; body?: unknown; headers?: Record<string, string> }) { }
}

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: (handler: any) => handler,
  FrameworkContext: jest.fn(),
  FrameworkResponse: MockFrameworkResponse,
}));

// Mock @fitglue/shared/infrastructure/oauth
jest.mock('@fitglue/shared/infrastructure/oauth', () => ({
  validateOAuthState: jest.fn(),
  storeOAuthTokens: jest.fn(),
}));

import { stravaOAuthHandler } from './index';

// Helper to check redirect response
const expectRedirect = (result: any, location: string) => {
  expect(result).toBeInstanceOf(MockFrameworkResponse);
  expect(result.options.status).toBe(302);
  expect(result.options.headers?.Location).toBe(location);
};

describe('stravaOAuthHandler', () => {
  let req: any;
  let ctx: any;
  let mockValidateOAuthState: jest.Mock;
  let mockStoreOAuthTokens: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();

    const { validateOAuthState, storeOAuthTokens } = require('@fitglue/shared/infrastructure/oauth');
    mockValidateOAuthState = validateOAuthState as jest.Mock;
    mockStoreOAuthTokens = storeOAuthTokens as jest.Mock;

    req = {
      query: {},
    };

    ctx = {
      db: {},
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
      },
      stores: {},
    };

    process.env.BASE_URL = 'https://dev.fitglue.tech';
    process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-dev';
    process.env.STRAVA_CLIENT_ID = 'client-id';
    process.env.STRAVA_CLIENT_SECRET = 'client-secret';
  });

  it('should redirect to error page if user denies authorization', async () => {
    req.query = { error: 'access_denied' };

    const result = await (stravaOAuthHandler as any)(req, ctx);

    expect(ctx.logger.warn).toHaveBeenCalledWith('User denied Strava authorization', { error: 'access_denied' });
    expectRedirect(result, 'https://dev.fitglue.tech/app/connections/strava/error?reason=denied');
  });

  it('should redirect to error page if code is missing', async () => {
    req.query = { state: 'valid-state' };

    const result = await (stravaOAuthHandler as any)(req, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
    expectRedirect(result, 'https://dev.fitglue.tech/app/connections/strava/error?reason=missing_params');
  });

  it('should redirect to error page if state is missing', async () => {
    req.query = { code: 'auth-code' };

    const result = await (stravaOAuthHandler as any)(req, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
    expectRedirect(result, 'https://dev.fitglue.tech/app/connections/strava/error?reason=missing_params');
  });

  it('should redirect to error page if state token is invalid', async () => {
    req.query = { code: 'auth-code', state: 'invalid-state' };
    mockValidateOAuthState.mockResolvedValue({ valid: false, userId: null });

    const result = await (stravaOAuthHandler as any)(req, ctx);

    expect(mockValidateOAuthState).toHaveBeenCalledWith('invalid-state');
    expect(ctx.logger.error).toHaveBeenCalledWith('Invalid or expired state token');
    expectRedirect(result, 'https://dev.fitglue.tech/app/connections/strava/error?reason=invalid_state');
  });

  it('should successfully process OAuth callback and store tokens', async () => {
    req.query = { code: 'auth-code', state: 'valid-state', scope: 'read,activity:read_all' };
    mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });

    // Mock fetch for token exchange
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: 'access-token',
        refresh_token: 'refresh-token',
        expires_at: 1234567890,
        athlete: { id: 789 },
      }),
    });

    const result = await (stravaOAuthHandler as any)(req, ctx);

    expect(mockValidateOAuthState).toHaveBeenCalledWith('valid-state');
    expect(mockStoreOAuthTokens).toHaveBeenCalledWith(
      'user-123',
      'strava',
      expect.objectContaining({
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
        externalUserId: '789',
      }),
      {}
    );
    expect(ctx.logger.info).toHaveBeenCalledWith('Successfully connected Strava account', {
      userId: 'user-123',
      athleteId: 789,
    });
    expectRedirect(result, 'https://dev.fitglue.tech/app/connections/strava/success');
  });

  it('should redirect to error page if token exchange fails', async () => {
    req.query = { code: 'auth-code', state: 'valid-state' };
    mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });

    // Mock fetch to fail
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => 'Bad Request',
    });

    const result = await (stravaOAuthHandler as any)(req, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Error processing Strava OAuth callback', expect.anything());
    expectRedirect(result, 'https://dev.fitglue.tech/app/connections/strava/error?reason=server_error');
  });
});
