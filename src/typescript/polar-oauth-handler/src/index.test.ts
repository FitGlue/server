// Mock firebase-admin to prevent initialization
jest.mock('firebase-admin', () => ({
  apps: [],
  initializeApp: jest.fn(),
  firestore: jest.fn(() => ({
    collection: jest.fn(),
  })),
}));

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: (handler: any) => handler,
  FrameworkContext: jest.fn(),
}));

// Mock @fitglue/shared/infrastructure/oauth
jest.mock('@fitglue/shared/infrastructure/oauth', () => ({
  validateOAuthState: jest.fn(),
  storeOAuthTokens: jest.fn(),
}));

import { polarOAuthHandler } from './index';

describe('polarOAuthHandler', () => {
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
    process.env.POLAR_CLIENT_ID = 'test-client-id';
    process.env.POLAR_CLIENT_SECRET = 'test-client-secret';
  });

  afterEach(() => {
    delete process.env.POLAR_CLIENT_ID;
    delete process.env.POLAR_CLIENT_SECRET;
  });

  it('should redirect to error page if user denies authorization', async () => {
    req.query = { error: 'access_denied' };

    const result = await (polarOAuthHandler as any)(req, ctx);

    expect(ctx.logger.warn).toHaveBeenCalledWith('User denied Polar authorization', { error: 'access_denied' });
    expect(result).toEqual({
      statusCode: 302,
      headers: { Location: 'https://dev.fitglue.tech/app/connections/polar/error?reason=denied' }
    });
  });

  it('should redirect to error page if code is missing', async () => {
    req.query = { state: 'valid-state' };

    const result = await (polarOAuthHandler as any)(req, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
    expect(result).toEqual({
      statusCode: 302,
      headers: { Location: 'https://dev.fitglue.tech/app/connections/polar/error?reason=missing_params' }
    });
  });

  it('should redirect to error page if state is missing', async () => {
    req.query = { code: 'auth-code' };

    const result = await (polarOAuthHandler as any)(req, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
    expect(result).toEqual({
      statusCode: 302,
      headers: { Location: 'https://dev.fitglue.tech/app/connections/polar/error?reason=missing_params' }
    });
  });

  it('should redirect to error page if state token is invalid', async () => {
    req.query = { code: 'auth-code', state: 'invalid-state' };
    mockValidateOAuthState.mockResolvedValue({ valid: false, userId: null });

    const result = await (polarOAuthHandler as any)(req, ctx);

    expect(mockValidateOAuthState).toHaveBeenCalledWith('invalid-state');
    expect(ctx.logger.error).toHaveBeenCalledWith('Invalid or expired state token');
    expect(result).toEqual({
      statusCode: 302,
      headers: { Location: 'https://dev.fitglue.tech/app/connections/polar/error?reason=invalid_state' }
    });
  });

  it('should successfully process OAuth callback and store tokens', async () => {
    req.query = { code: 'auth-code', state: 'valid-state' };
    mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });

    // Mock fetch for token exchange and user registration
    global.fetch = jest.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: 'access-token',
          token_type: 'Bearer',
          expires_in: 31536000,
          x_user_id: 12345,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
      });

    const result = await (polarOAuthHandler as any)(req, ctx);

    expect(mockValidateOAuthState).toHaveBeenCalledWith('valid-state');
    expect(mockStoreOAuthTokens).toHaveBeenCalledWith(
      'user-123',
      'polar',
      expect.objectContaining({
        accessToken: 'access-token',
        refreshToken: '',
        externalUserId: '12345',
      }),
      {}
    );
    expect(ctx.logger.info).toHaveBeenCalledWith('Successfully connected Polar account', {
      userId: 'user-123',
      polarUserId: 12345,
    });
    expect(result).toEqual({
      statusCode: 302,
      headers: { Location: 'https://dev.fitglue.tech/app/connections/polar/success' }
    });
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

    const result = await (polarOAuthHandler as any)(req, ctx);

    expect(ctx.logger.error).toHaveBeenCalledWith('Error processing Polar OAuth callback', expect.anything());
    expect(result).toEqual({
      statusCode: 302,
      headers: { Location: 'https://dev.fitglue.tech/app/connections/polar/error?reason=server_error' }
    });
  });
});
