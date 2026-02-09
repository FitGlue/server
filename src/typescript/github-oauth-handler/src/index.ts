// Module-level imports for smart pruning - only includes framework and oauth modules
import { createCloudFunction, FrameworkContext, FrameworkResponse } from '@fitglue/shared/framework';
import { validateOAuthState, storeOAuthTokens } from '@fitglue/shared/infrastructure/oauth';
import { GitHubIntegration } from '@fitglue/shared/types';

// Helper to create redirect responses
const redirect = (url: string) => new FrameworkResponse({ status: 302, headers: { Location: url } });

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, ctx: FrameworkContext) => {
    const { stores, logger } = ctx;

    // Extract query parameters
    const { code, state, error: authError } = req.query;

    // Handle authorization denial
    if (authError) {
        logger.warn('User denied GitHub authorization', { error: authError });
        return redirect(`${process.env.BASE_URL}/app/connections/github/error?reason=denied`);
    }

    // Validate required parameters
    if (!code || !state) {
        logger.error('Missing required OAuth parameters');
        return redirect(`${process.env.BASE_URL}/app/connections/github/error?reason=missing_params`);
    }

    // Validate state token (CSRF protection)
    const validation = await validateOAuthState(state);
    if (!validation.valid || !validation.userId) {
        logger.error('Invalid or expired state token');
        return redirect(`${process.env.BASE_URL}/app/connections/github/error?reason=invalid_state`);
    }
    const userId = validation.userId;

    logger.info('Processing GitHub OAuth callback', { userId });

    try {
        // Exchange authorization code for access token
        const clientId = process.env.GITHUB_CLIENT_ID;
        const clientSecret = process.env.GITHUB_CLIENT_SECRET;

        if (!clientId || !clientSecret) {
            logger.error('Missing GitHub OAuth credentials');
            return redirect(`${process.env.BASE_URL}/app/connections/github/error?reason=config_error`);
        }

        const tokenResponse = await fetch('https://github.com/login/oauth/access_token', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            },
            body: JSON.stringify({
                client_id: clientId,
                client_secret: clientSecret,
                code,
            }),
        });

        if (!tokenResponse.ok) {
            const errorText = await tokenResponse.text();
            logger.error('Failed to exchange code for token', { status: tokenResponse.status, error: errorText });
            throw new Error(`Token exchange failed: ${tokenResponse.status}`);
        }

        const tokenData = await tokenResponse.json() as {
            access_token: string;
            token_type: string;
            scope: string;
            error?: string;
            error_description?: string;
        };

        if (tokenData.error) {
            logger.error('GitHub token exchange returned error', {
                error: tokenData.error,
                description: tokenData.error_description
            });
            throw new Error(`Token error: ${tokenData.error}`);
        }

        const { access_token: accessToken, scope } = tokenData;

        // Fetch user info to get GitHub username
        const userResponse = await fetch('https://api.github.com/user', {
            headers: {
                'Authorization': `Bearer ${accessToken}`,
                'Accept': 'application/vnd.github.v3+json',
                'User-Agent': 'FitGlue/1.0',
            },
        });

        if (!userResponse.ok) {
            throw new Error(`Failed to fetch GitHub user: ${userResponse.status}`);
        }

        const githubUser = await userResponse.json() as {
            id: number;
            login: string;
        };

        // Store tokens in Firestore
        // Note: GitHub OAuth tokens don't expire (no refresh token needed)
        await storeOAuthTokens(userId, 'github', {
            accessToken,
            refreshToken: '', // GitHub tokens don't expire or use refresh tokens
            expiresAt: new Date('2099-12-31'), // Never expires
            externalUserId: githubUser.id.toString(),
        }, stores);

        // Store additional GitHub-specific metadata (username, granted scopes)
        // Uses setIntegration with merge to add fields alongside the tokens stored above
        await stores.users.setIntegration(userId, 'github', {
            enabled: true,
            githubUsername: githubUser.login,
            scope,
        } as Partial<GitHubIntegration> as GitHubIntegration);

        logger.info('Successfully connected GitHub account', {
            userId,
            githubUser: githubUser.login,
            githubId: githubUser.id,
        });

        // Redirect to success page
        return redirect(`${process.env.BASE_URL}/app/connections/github/success`);

    } catch (error: unknown) {
        logger.error('Error processing GitHub OAuth callback', { error });
        return redirect(`${process.env.BASE_URL}/app/connections/github/error?reason=server_error`);
    }
};

export const githubOAuthHandler = createCloudFunction(handler, {
    allowUnauthenticated: true, // Public OAuth callback endpoint
    skipExecutionLogging: true
});
