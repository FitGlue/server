// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext } from '@fitglue/shared/framework';
import { validateOAuthState, storeOAuthTokens } from '@fitglue/shared/infrastructure/oauth';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, ctx: FrameworkContext) => {
  const { stores, logger } = ctx;

  // Extract query parameters
  const { code, state, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Polar authorization', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/polar/error?reason=denied` } };
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/polar/error?reason=missing_params` } };
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/polar/error?reason=invalid_state` } };
  }
  const userId = validation.userId;

  logger.info('Processing Polar OAuth callback', { userId });

  try {
    // Exchange authorization code for tokens
    // Polar uses polarremote.com for token exchange
    const clientId = process.env.POLAR_CLIENT_ID;
    const clientSecret = process.env.POLAR_CLIENT_SECRET;
    if (!clientId || !clientSecret) {
      throw new Error('Missing POLAR_CLIENT_ID or POLAR_CLIENT_SECRET environment variables');
    }

    const tokenResponse = await fetch('https://polarremote.com/v2/oauth2/token', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Accept': 'application/json;charset=UTF-8',
      },
      body: new URLSearchParams({
        code,
        grant_type: 'authorization_code',
        client_id: clientId,
        client_secret: clientSecret,
        redirect_uri: `${process.env.BASE_URL}/auth/polar/callback`,
      }),
    });

    if (!tokenResponse.ok) {
      const errorText = await tokenResponse.text();
      logger.error('Failed to exchange code for tokens', { status: tokenResponse.status, error: errorText });
      throw new Error(`Token exchange failed: ${tokenResponse.status}`);
    }

    const tokenData = await tokenResponse.json() as {
      access_token: string;
      token_type: string;
      expires_in: number;
      x_user_id: number; // Polar returns user ID as x_user_id
    };
    const { access_token: accessToken, expires_in: expiresIn, x_user_id: polarUserId } = tokenData;

    // Calculate expiration time
    const expiresAt = new Date(Date.now() + expiresIn * 1000);

    // Register user with Polar AccessLink API
    // This is required before we can access user data
    const registerResponse = await fetch('https://www.polaraccesslink.com/v3/users', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${accessToken}`,
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      body: JSON.stringify({
        'member-id': userId, // Use FitGlue userId as member-id for mapping
      }),
    });

    // 200 = newly registered, 409 = already registered (both are OK)
    if (!registerResponse.ok && registerResponse.status !== 409) {
      const errorText = await registerResponse.text();
      logger.error('Failed to register user with Polar AccessLink', { status: registerResponse.status, error: errorText });
      throw new Error(`User registration failed: ${registerResponse.status}`);
    }

    // Store tokens in Firestore
    // Note: Polar OAuth doesn't return a refresh token - tokens are long-lived
    await storeOAuthTokens(userId, 'polar', {
      accessToken,
      refreshToken: '', // Polar doesn't use refresh tokens in the same way
      expiresAt,
      externalUserId: String(polarUserId),
    }, stores);

    logger.info('Successfully connected Polar account', { userId, polarUserId });

    // Redirect to success page
    // Redirect to success page
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/polar/success` } };

  } catch (error: unknown) {
    logger.error('Error processing Polar OAuth callback', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/polar/error?reason=server_error` } };
  }
};

export const polarOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
