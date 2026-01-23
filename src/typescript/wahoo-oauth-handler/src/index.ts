import { createCloudFunction, FrameworkContext, validateOAuthState, storeOAuthTokens } from '@fitglue/shared';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, ctx: FrameworkContext) => {
  const { stores, logger } = ctx;

  // Extract query parameters
  const { code, state, error, error_description: errorDescription } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Wahoo authorization', { error, errorDescription });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/wahoo/error?reason=denied` } };
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/wahoo/error?reason=missing_params` } };
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/wahoo/error?reason=invalid_state` } };
  }
  const userId = validation.userId;

  logger.info('Processing Wahoo OAuth callback', { userId });

  try {
    // Exchange authorization code for tokens
    const clientId = process.env.WAHOO_CLIENT_ID;
    const clientSecret = process.env.WAHOO_CLIENT_SECRET;

    if (!clientId || !clientSecret) {
      logger.error('Missing Wahoo OAuth credentials');
      return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/wahoo/error?reason=config_error` } };
    }

    const tokenResponse = await fetch('https://api.wahooligan.com/oauth/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        client_id: clientId,
        client_secret: clientSecret,
        code,
        grant_type: 'authorization_code',
        redirect_uri: `${process.env.BASE_URL}/auth/wahoo/callback`,
      }).toString(),
    });

    if (!tokenResponse.ok) {
      const errorText = await tokenResponse.text();
      logger.error('Failed to exchange code for tokens', { status: tokenResponse.status, error: errorText });
      throw new Error(`Token exchange failed: ${tokenResponse.status}`);
    }

    const tokenData = await tokenResponse.json() as {
      access_token: string;
      refresh_token: string;
      expires_in: number;
      created_at: number;
    };
    const { access_token: accessToken, refresh_token: refreshToken, expires_in: expiresIn, created_at: createdAt } = tokenData;

    // Fetch user profile to get Wahoo user ID
    const userResponse = await fetch('https://api.wahooligan.com/v1/user', {
      headers: { 'Authorization': `Bearer ${accessToken}` },
    });

    if (!userResponse.ok) {
      logger.error('Failed to fetch Wahoo user profile');
      throw new Error('Failed to fetch user profile');
    }

    const userData = await userResponse.json() as { id: number };

    // Calculate expiration time
    const expiresAt = new Date((createdAt + expiresIn) * 1000);

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'wahoo', {
      accessToken,
      refreshToken,
      expiresAt,
      externalUserId: userData.id.toString(),
    }, stores);

    logger.info('Successfully connected Wahoo account', { userId, wahooUserId: userData.id });

    // Redirect to success page
    // Redirect to success page
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/wahoo/success` } };

  } catch (error: unknown) {
    logger.error('Error processing Wahoo OAuth callback', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/wahoo/error?reason=server_error` } };
  }
};

export const wahooOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
