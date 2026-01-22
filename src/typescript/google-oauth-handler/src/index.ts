import { createCloudFunction, FrameworkContext, validateOAuthState, storeOAuthTokens, getSecret } from '@fitglue/shared';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, ctx: FrameworkContext) => {
  const { stores, logger } = ctx;

  // Extract query parameters
  const { code, state, scope, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Google authorization', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/google/error?reason=denied` } };
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/google/error?reason=missing_params` } };
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/google/error?reason=invalid_state` } };
  }
  const userId = validation.userId;

  logger.info('Processing Google OAuth callback', { userId, scope });

  try {
    // Exchange authorization code for tokens
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
    const clientId = await getSecret(projectId, 'google-oauth-client-id');
    const clientSecret = await getSecret(projectId, 'google-oauth-client-secret');

    const tokenResponse = await fetch('https://oauth2.googleapis.com/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        client_id: clientId,
        client_secret: clientSecret,
        code,
        grant_type: 'authorization_code',
        redirect_uri: `${process.env.BASE_URL}/auth/google/callback`,
      }),
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
      scope: string;
    };
    const { access_token, refresh_token, expires_in } = tokenData;

    // Calculate expiration time
    const expiresAt = new Date(Date.now() + expires_in * 1000);

    // Get user info from Google to get external user ID
    const userInfoResponse = await fetch('https://www.googleapis.com/oauth2/v2/userinfo', {
      headers: {
        'Authorization': `Bearer ${access_token}`,
      },
    });

    let externalUserId = '';
    if (userInfoResponse.ok) {
      const userInfo = await userInfoResponse.json() as { id?: string };
      externalUserId = userInfo.id || '';
    }

    if (!externalUserId) {
      logger.error('Could not retrieve Google user ID');
      throw new Error('Could not retrieve user ID');
    }

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'google', {
      accessToken: access_token,
      refreshToken: refresh_token,
      expiresAt,
      externalUserId,
    }, stores);

    logger.info('Successfully connected Google account', { userId, googleUserId: externalUserId });

    // Redirect to success page
    // Redirect to success page
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/google/success` } };

  } catch (error: unknown) {
    logger.error('Error processing Google OAuth callback', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/google/error?reason=server_error` } };
  }
};

export const googleOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
