import { createCloudFunction, FrameworkContext, validateOAuthState, storeOAuthTokens, getSecret } from '@fitglue/shared';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, ctx: FrameworkContext) => {
  const { stores, logger } = ctx;

  // Extract query parameters
  const { code, state, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Oura authorization', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/oura/error?reason=denied` } };
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/oura/error?reason=missing_params` } };
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/oura/error?reason=invalid_state` } };
  }
  const userId = validation.userId;

  logger.info('Processing Oura OAuth callback', { userId });

  try {
    // Exchange authorization code for tokens
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
    const clientId = await getSecret(projectId, 'oura-client-id');
    const clientSecret = await getSecret(projectId, 'oura-client-secret');

    // Oura uses standard OAuth2 token exchange
    const tokenResponse = await fetch('https://api.ouraring.com/oauth/token', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        code,
        client_id: clientId,
        client_secret: clientSecret,
        redirect_uri: `${process.env.BASE_URL}/auth/oura/callback`,
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
      token_type: string;
    };
    const { access_token, refresh_token, expires_in } = tokenData;

    // Calculate expiration time
    const expiresAt = new Date(Date.now() + expires_in * 1000);

    // Fetch user info to get external user ID
    const userInfoResponse = await fetch('https://api.ouraring.com/v2/usercollection/personal_info', {
      headers: {
        'Authorization': `Bearer ${access_token}`,
      },
    });

    let externalUserId = 'unknown';
    if (userInfoResponse.ok) {
      const userInfo = await userInfoResponse.json() as { id?: string };
      externalUserId = userInfo.id || 'unknown';
    }

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'oura', {
      accessToken: access_token,
      refreshToken: refresh_token,
      expiresAt,
      externalUserId,
    }, stores);

    logger.info('Successfully connected Oura account', { userId, externalUserId });

    // Redirect to success page
    // Redirect to success page
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/oura/success` } };

  } catch (error: unknown) {
    logger.error('Error processing Oura OAuth callback', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/oura/error?reason=server_error` } };
  }
};

export const ouraOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
