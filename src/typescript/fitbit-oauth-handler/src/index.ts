import { createCloudFunction, FrameworkContext, validateOAuthState, storeOAuthTokens, getSecret } from '@fitglue/shared';

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { db, logger } = ctx;

  // Extract query parameters
  const { code, state, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Fitbit authorization', { error });
    res.redirect(`${process.env.BASE_URL}/auth/error?reason=denied`);
    return;
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    res.redirect(`${process.env.BASE_URL}/auth/error?reason=missing_params`);
    return;
  }

  // Validate state token (CSRF protection)
  const userId = await validateOAuthState(state);
  if (!userId) {
    logger.error('Invalid or expired state token');
    res.redirect(`${process.env.BASE_URL}/auth/error?reason=invalid_state`);
    return;
  }

  logger.info('Processing Fitbit OAuth callback', { userId });

  try {
    // Exchange authorization code for tokens
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
    const clientId = await getSecret(projectId, 'fitbit-client-id');
    const clientSecret = await getSecret(projectId, 'fitbit-client-secret');

    // Fitbit requires Basic Auth for token exchange
    const credentials = Buffer.from(`${clientId}:${clientSecret}`).toString('base64');

    const tokenResponse = await fetch('https://api.fitbit.com/oauth2/token', {
      method: 'POST',
      headers: {
        'Authorization': `Basic ${credentials}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      body: new URLSearchParams({
        client_id: clientId,
        code,
        grant_type: 'authorization_code',
        redirect_uri: `${process.env.BASE_URL}/auth/fitbit/callback`,
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
      user_id: string;
    };
    const { access_token, refresh_token, expires_in, user_id } = tokenData;

    // Calculate expiration time
    const expiresAt = new Date(Date.now() + expires_in * 1000);

    // Store tokens in Firestore
    await storeOAuthTokens(db, userId, 'fitbit', {
      accessToken: access_token,
      refreshToken: refresh_token,
      expiresAt,
      externalUserId: user_id,
    });

    logger.info('Successfully connected Fitbit account', { userId, fitbitUserId: user_id });

    // Redirect to success page
    res.redirect(`${process.env.BASE_URL}/auth/success?provider=fitbit`);

  } catch (error) {
    logger.error('Error processing Fitbit OAuth callback', { error });
    res.redirect(`${process.env.BASE_URL}/auth/error?reason=server_error`);
  }
};

export const fitbitOAuthHandler = createCloudFunction(handler, {
  auth: {
    strategies: [], // Public endpoint
  },
});
