import { createCloudFunction, FrameworkContext, validateOAuthState, storeOAuthTokens, getSecret } from '@fitglue/shared';

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { db, logger } = ctx;

  // Extract query parameters
  const { code, state, scope, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Strava authorization', { error });
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

  logger.info('Processing Strava OAuth callback', { userId, scope });

  try {
    // Exchange authorization code for tokens
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
    const clientId = await getSecret(projectId, 'strava-client-id');
    const clientSecret = await getSecret(projectId, 'strava-client-secret');

    const tokenResponse = await fetch('https://www.strava.com/api/v3/oauth/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        client_id: clientId,
        client_secret: clientSecret,
        code,
        grant_type: 'authorization_code',
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
      expires_at: number;
      athlete: { id: number };
    };
    const { access_token, refresh_token, expires_at, athlete } = tokenData;

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'strava', {
      accessToken: access_token,
      refreshToken: refresh_token,
      expiresAt: new Date(expires_at * 1000),
      externalUserId: athlete.id.toString(),
    });

    logger.info('Successfully connected Strava account', { userId, athleteId: athlete.id });

    // Redirect to success page
    res.redirect(`${process.env.BASE_URL}/auth/success?provider=strava`);

  } catch (error) {
    logger.error('Error processing Strava OAuth callback', { error });
    res.redirect(`${process.env.BASE_URL}/auth/error?reason=server_error`);
  }
};

export const stravaOAuthHandler = createCloudFunction(handler, {
  auth: {
    strategies: [], // Public endpoint
  },
});
