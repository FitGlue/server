import { createCloudFunction, FrameworkContext, validateOAuthState, storeOAuthTokens, getSecret } from '@fitglue/shared';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { stores, logger } = ctx;

  // Extract query parameters
  const { code, state, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Spotify authorization', { error });
    res.redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=denied`);
    return;
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    res.redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=missing_params`);
    return;
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    res.redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=invalid_state`);
    return;
  }
  const userId = validation.userId;

  logger.info('Processing Spotify OAuth callback', { userId });

  try {
    // Exchange authorization code for tokens
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
    const clientId = await getSecret(projectId, 'spotify-client-id');
    const clientSecret = await getSecret(projectId, 'spotify-client-secret');

    const tokenResponse = await fetch('https://accounts.spotify.com/api/token', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Authorization': 'Basic ' + Buffer.from(`${clientId}:${clientSecret}`).toString('base64'),
      },
      body: new URLSearchParams({
        code,
        grant_type: 'authorization_code',
        redirect_uri: `${process.env.BASE_URL}/spotify-oauth-callback`,
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
      scope: string;
    };
    const { access_token, refresh_token, expires_in } = tokenData;

    // Calculate expires_at from expires_in (seconds)
    const expiresAt = new Date(Date.now() + expires_in * 1000);

    // Fetch Spotify user profile to get user ID
    const profileResponse = await fetch('https://api.spotify.com/v1/me', {
      headers: {
        'Authorization': `Bearer ${access_token}`,
      },
    });

    if (!profileResponse.ok) {
      logger.error('Failed to fetch Spotify profile', { status: profileResponse.status });
      throw new Error(`Profile fetch failed: ${profileResponse.status}`);
    }

    const profile = await profileResponse.json() as { id: string };

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'spotify', {
      accessToken: access_token,
      refreshToken: refresh_token,
      expiresAt,
      externalUserId: profile.id,
    }, stores);

    logger.info('Successfully connected Spotify account', { userId, spotifyUserId: profile.id });

    // Redirect to success page
    res.redirect(`${process.env.BASE_URL}/app/connections/spotify/success`);

  } catch (error: unknown) {
    logger.error('Error processing Spotify OAuth callback', { error });
    res.redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=server_error`);
  }
};

export const spotifyOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
