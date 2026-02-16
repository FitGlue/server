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



import { githubOAuthHandler } from './index';

// Helper to check redirect response
const expectRedirect = (result: any, location: string) => {
    expect(result).toBeInstanceOf(MockFrameworkResponse);
    expect(result.options.status).toBe(302);
    expect(result.options.headers?.Location).toBe(location);
};

describe('githubOAuthHandler', () => {
    let req: any;
    let ctx: any;
    let mockValidateOAuthState: jest.Mock;
    let mockStoreOAuthTokens: jest.Mock;

    beforeEach(() => {
        jest.clearAllMocks();

        const { validateOAuthState, storeOAuthTokens } = require('@fitglue/shared/infrastructure/oauth');
        mockValidateOAuthState = validateOAuthState as jest.Mock;
        mockStoreOAuthTokens = storeOAuthTokens as jest.Mock;

        req = { query: {} };

        ctx = {
            db: {},
            logger: {
                info: jest.fn(),
                warn: jest.fn(),
                error: jest.fn(),
            },
            stores: {
                users: {
                    setIntegration: jest.fn().mockResolvedValue(undefined),
                },
            },
        };

        process.env.BASE_URL = 'https://dev.fitglue.tech';
        process.env.GITHUB_CLIENT_ID = 'gh-client-id';
        process.env.GITHUB_CLIENT_SECRET = 'gh-client-secret';
    });

    afterEach(() => {
        delete process.env.BASE_URL;
        delete process.env.GITHUB_CLIENT_ID;
        delete process.env.GITHUB_CLIENT_SECRET;
    });

    it('should redirect to error page if user denies authorization', async () => {
        req.query = { error: 'access_denied' };

        const result = await (githubOAuthHandler as any)(req, ctx);

        expect(ctx.logger.warn).toHaveBeenCalledWith('User denied GitHub authorization', { error: 'access_denied' });
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/error?reason=denied');
    });

    it('should redirect to error page if code is missing', async () => {
        req.query = { state: 'valid-state' };

        const result = await (githubOAuthHandler as any)(req, ctx);

        expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/error?reason=missing_params');
    });

    it('should redirect to error page if state is missing', async () => {
        req.query = { code: 'auth-code' };

        const result = await (githubOAuthHandler as any)(req, ctx);

        expect(ctx.logger.error).toHaveBeenCalledWith('Missing required OAuth parameters');
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/error?reason=missing_params');
    });

    it('should redirect to error page if state token is invalid', async () => {
        req.query = { code: 'auth-code', state: 'invalid-state' };
        mockValidateOAuthState.mockResolvedValue({ valid: false, userId: null });

        const result = await (githubOAuthHandler as any)(req, ctx);

        expect(mockValidateOAuthState).toHaveBeenCalledWith('invalid-state');
        expect(ctx.logger.error).toHaveBeenCalledWith('Invalid or expired state token');
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/error?reason=invalid_state');
    });

    it('should redirect to error page if GitHub credentials are missing', async () => {
        req.query = { code: 'auth-code', state: 'valid-state' };
        mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });
        delete process.env.GITHUB_CLIENT_ID;

        const result = await (githubOAuthHandler as any)(req, ctx);

        expect(ctx.logger.error).toHaveBeenCalledWith('Missing GitHub OAuth credentials');
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/error?reason=config_error');
    });

    it('should successfully process OAuth callback and store tokens', async () => {
        req.query = { code: 'auth-code', state: 'valid-state' };
        mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });

        // Mock fetch: first call = token exchange, second call = user info
        global.fetch = jest.fn()
            .mockResolvedValueOnce({
                ok: true,
                json: async () => ({
                    access_token: 'gh-access-token',
                    token_type: 'bearer',
                    scope: 'repo',
                }),
            })
            .mockResolvedValueOnce({
                ok: true,
                json: async () => ({
                    id: 42,
                    login: 'janedoe',
                }),
            });

        const result = await (githubOAuthHandler as any)(req, ctx);

        // Verify token exchange request
        expect(global.fetch).toHaveBeenCalledWith(
            'https://github.com/login/oauth/access_token',
            expect.objectContaining({
                method: 'POST',
                body: JSON.stringify({
                    client_id: 'gh-client-id',
                    client_secret: 'gh-client-secret',
                    code: 'auth-code',
                }),
            })
        );

        // Verify user info request
        expect(global.fetch).toHaveBeenCalledWith(
            'https://api.github.com/user',
            expect.objectContaining({
                headers: expect.objectContaining({
                    'Authorization': 'Bearer gh-access-token',
                }),
            })
        );

        // Verify token storage with extras
        expect(mockStoreOAuthTokens).toHaveBeenCalledWith(
            'user-123',
            'github',
            expect.objectContaining({
                accessToken: 'gh-access-token',
                refreshToken: '',
                externalUserId: '42',
            }),
            ctx.stores,
            {
                githubUsername: 'janedoe',
                scope: 'repo',
            }
        );

        // Verify no separate setIntegration call (single write via storeOAuthTokens)
        expect(ctx.stores.users.setIntegration).not.toHaveBeenCalled();

        expect(ctx.logger.info).toHaveBeenCalledWith('Successfully connected GitHub account', {
            userId: 'user-123',
            githubUser: 'janedoe',
            githubId: 42,
        });
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/success');
    });

    it('should redirect to error page if token exchange fails', async () => {
        req.query = { code: 'auth-code', state: 'valid-state' };
        mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });

        global.fetch = jest.fn().mockResolvedValue({
            ok: false,
            status: 400,
            text: async () => 'Bad Request',
        });

        const result = await (githubOAuthHandler as any)(req, ctx);

        expect(ctx.logger.error).toHaveBeenCalledWith('Error processing GitHub OAuth callback', expect.anything());
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/error?reason=server_error');
    });

    it('should redirect to error page if GitHub returns token error', async () => {
        req.query = { code: 'auth-code', state: 'valid-state' };
        mockValidateOAuthState.mockResolvedValue({ valid: true, userId: 'user-123' });

        global.fetch = jest.fn().mockResolvedValue({
            ok: true,
            json: async () => ({
                error: 'bad_verification_code',
                error_description: 'The code passed is incorrect or expired.',
            }),
        });

        const result = await (githubOAuthHandler as any)(req, ctx);

        expect(ctx.logger.error).toHaveBeenCalledWith('GitHub token exchange returned error', {
            error: 'bad_verification_code',
            description: 'The code passed is incorrect or expired.',
        });
        expectRedirect(result, 'https://dev.fitglue.tech/app/connections/github/error?reason=server_error');
    });
});
