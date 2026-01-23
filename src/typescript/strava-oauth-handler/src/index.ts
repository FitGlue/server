import { createCloudFunction, FrameworkContext, validateOAuthState, storeOAuthTokens } from '@fitglue/shared';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, ctx: FrameworkContext) => {
  const { stores, logger } = ctx;

  // Extract query parameters
  const { code, state, scope, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Strava authorization', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/strava/error?reason=denied` } };
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/strava/error?reason=missing_params` } };
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/strava/error?reason=invalid_state` } };
  }
  const userId = validation.userId;

  logger.info('Processing Strava OAuth callback', { userId, scope });

  try {
    // Exchange authorization code for tokens
    const clientId = process.env.STRAVA_CLIENT_ID;
    const clientSecret = process.env.STRAVA_CLIENT_SECRET;

    if (!clientId || !clientSecret) {
      logger.error('Missing Strava OAuth credentials');
      return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/strava/error?reason=config_error` } };
    }

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
    const { access_token: accessToken, refresh_token: refreshToken, expires_at: expiresAtSeconds, athlete } = tokenData;

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'strava', {
      accessToken,
      refreshToken,
      expiresAt: new Date(expiresAtSeconds * 1000),
      externalUserId: athlete.id.toString(),
    }, stores); // Pass stores directly

    logger.info('Successfully connected Strava account', { userId, athleteId: athlete.id });

    // Redirect to success page
    // Redirect to success page
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/strava/success` } };

  } catch (error: unknown) {
    logger.error('Error processing Strava OAuth callback', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/strava/error?reason=server_error` } };
  }
};

export const stravaOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
