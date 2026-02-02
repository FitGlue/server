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
    logger.warn('User denied TrainingPeaks authorization', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/trainingpeaks/error?reason=denied` } };
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/trainingpeaks/error?reason=missing_params` } };
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/trainingpeaks/error?reason=invalid_state` } };
  }
  const userId = validation.userId;

  logger.info('Processing TrainingPeaks OAuth callback', { userId });

  try {
    // Exchange authorization code for tokens
    const clientId = process.env.TRAININGPEAKS_CLIENT_ID;
    const clientSecret = process.env.TRAININGPEAKS_CLIENT_SECRET;
    if (!clientId || !clientSecret) {
      throw new Error('Missing TRAININGPEAKS_CLIENT_ID or TRAININGPEAKS_CLIENT_SECRET environment variables');
    }

    const tokenResponse = await fetch('https://oauth.trainingpeaks.com/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        client_id: clientId,
        client_secret: clientSecret,
        code,
        grant_type: 'authorization_code',
        redirect_uri: `${process.env.BASE_URL}/auth/trainingpeaks/callback`,
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
      athlete_id?: string;
    };
    const { access_token: accessToken, refresh_token: refreshToken, expires_in: expiresIn } = tokenData;

    // Calculate expiration time
    const expiresAt = new Date(Date.now() + expiresIn * 1000);

    // Get athlete ID from token response or fetch from API
    let athleteId = tokenData.athlete_id;
    if (!athleteId) {
      // Fetch athlete info from TrainingPeaks API
      const athleteResponse = await fetch('https://api.trainingpeaks.com/v1/athlete/profile', {
        headers: {
          'Authorization': `Bearer ${accessToken}`,
        },
      });
      if (athleteResponse.ok) {
        const profile = await athleteResponse.json() as { Id?: string; id?: string };
        athleteId = profile.Id || profile.id || '';
      }
    }

    if (!athleteId) {
      logger.error('Could not retrieve TrainingPeaks athlete ID');
      throw new Error('Could not retrieve athlete ID');
    }

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'trainingpeaks', {
      accessToken,
      refreshToken,
      expiresAt,
      externalUserId: athleteId,
    }, stores);

    logger.info('Successfully connected TrainingPeaks account', { userId, athleteId });

    // Redirect to success page
    // Redirect to success page
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/trainingpeaks/success` } };

  } catch (error: unknown) {
    logger.error('Error processing TrainingPeaks OAuth callback', { error });
    return { statusCode: 302, headers: { Location: `${process.env.BASE_URL}/app/connections/trainingpeaks/error?reason=server_error` } };
  }
};

export const trainingPeaksOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
