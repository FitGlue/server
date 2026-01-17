// Mock firebase-admin to prevent initialization
jest.mock('firebase-admin', () => ({
  apps: [],
  initializeApp: jest.fn(),
  firestore: jest.fn(() => ({
    collection: jest.fn(),
  })),
}));

// Mock the shared package
jest.mock('@fitglue/shared', () => ({
  createCloudFunction: (handler: any) => handler,
  validateOAuthState: jest.fn(),
  storeOAuthTokens: jest.fn(),
  getSecret: jest.fn(),
}));

import { stravaOAuthHandler } from './index';

describe('stravaOAuthHandler', () => {
  let req: any;
  let res: any;
  let ctx: any;
  let mockValidateOAuthState: jest.Mock;
  let mockStoreOAuthTokens: jest.Mock;
  let mockGetSecret: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();

    const { validateOAuthState, storeOAuthTokens, getSecret } = require('@fitglue/shared');
    mockValidateOAuthState = validateOAuthState as jest.Mock;
    mockStoreOAuthTokens = storeOAuthTokens as jest.Mock;
    mockGetSecret = getSecret as jest.Mock;

    req = {
      query: {},
    };

    res = {
      redirect: jest.fn(),
      status: jest.fn().mockReturnThis(),
      send: jest.fn(),
      set: jest.fn(),
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
  });

  it('should redirect to error page if user denies authorization', async () => {
    req.query = { error: 'access_denied' };

    await (stravaOAuthHandler as any)(req, res, ctx);

    expect(ctx.logger.warn).toHaveBeenCalledWith('User denied Strava authorization', { error: 'access_denied' });
    expect(res.redirect).toHaveBeenCalledWith('https://dev.fitglue.tech/app/connections/strava/error?reason=denied');
  });

  it('should redirect to error page if code is missing', async () => {
    req.query = { state: 'valid-state' };

    await (stravaOAuthHandler as any)(req, res, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
    expect(res.redirect).toHaveBeenCalledWith('https://dev.fitglue.tech/app/connections/strava/error?reason=missing_params');
  });

  it('should redirect to error page if state is missing', async () => {
    req.query = { code: 'auth-code' };

    await (stravaOAuthHandler as any)(req, res, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
    expect(res.redirect).toHaveBeenCalledWith('https://dev.fitglue.tech/app/connections/strava/error?reason=missing_params');
  });

  it('should redirect to error page if state token is invalid', async () => {
    req.query = { code: 'auth-code', state: 'invalid-state' };
    mockValidateOAuthState.mockResolvedValue({ valid: false, userId: null });

    await (stravaOAuthHandler as any)(req, res, ctx);

    expect(mockValidateOAuthState).toHaveBeenCalledWith('invalid-state');
    expect(ctx.logger.error).toHaveBeenCalledWith('Invalid or expired state token');
    expect(res.redirect).toHaveBeenCalledWith('https://dev.fitglue.tech/app/connections/strava/error?reason=invalid_state');
  });

  it('should successfully process OAuth callback and store tokens', async () => {
    req.query = { code: 'auth-code', state: 'valid-state', scope: 'read,activity:read_all' };
    mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });
    mockGetSecret.mockImplementation((projectId: string, secretName: string) => {
      if (secretName === 'strava-client-id') return Promise.resolve('client-id');
      if (secretName === 'strava-client-secret') return Promise.resolve('client-secret');
      return Promise.resolve('');
    });

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

    await (stravaOAuthHandler as any)(req, res, ctx);

    expect(mockValidateOAuthState).toHaveBeenCalledWith('valid-state');
    expect(mockGetSecret).toHaveBeenCalledWith('fitglue-server-dev', 'strava-client-id');
    expect(mockGetSecret).toHaveBeenCalledWith('fitglue-server-dev', 'strava-client-secret');
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
    expect(res.redirect).toHaveBeenCalledWith('https://dev.fitglue.tech/app/connections/strava/success');
  });

  it('should redirect to error page if token exchange fails', async () => {
    req.query = { code: 'auth-code', state: 'valid-state' };
    mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });
    mockGetSecret.mockResolvedValue('secret');

    // Mock fetch to fail
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => 'Bad Request',
    });

    await (stravaOAuthHandler as any)(req, res, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Error processing Strava OAuth callback', expect.anything());
    expect(res.redirect).toHaveBeenCalledWith('https://dev.fitglue.tech/app/connections/strava/error?reason=server_error');
  });
});
