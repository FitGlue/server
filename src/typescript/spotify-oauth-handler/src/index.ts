// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext, FrameworkResponse } from '@fitglue/shared/framework';
import { validateOAuthState, storeOAuthTokens } from '@fitglue/shared/infrastructure/oauth';

// Helper to create redirect responses
const redirect = (url: string) => new FrameworkResponse({ status: 302, headers: { Location: url } });

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const handler = async (req: any, ctx: FrameworkContext) => {
  const { stores, logger } = ctx;

  // Extract query parameters
  const { code, state, error } = req.query;

  // Handle authorization denial
  if (error) {
    logger.warn('User denied Spotify authorization', { error });
    return redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=denied`);
  }

  // Validate required parameters
  if (!code || !state) {
    logger.error('Missing required OAuth parameters');
    return redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=missing_params`);
  }

  // Validate state token (CSRF protection)
  const validation = await validateOAuthState(state);
  if (!validation.valid || !validation.userId) {
    logger.error('Invalid or expired state token');
    return redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=invalid_state`);
  }
  const userId = validation.userId;

  logger.info('Processing Spotify OAuth callback', { userId });

  try {
    // Exchange authorization code for tokens
    const clientId = process.env.SPOTIFY_CLIENT_ID;
    const clientSecret = process.env.SPOTIFY_CLIENT_SECRET;

    if (!clientId || !clientSecret) {
      logger.error('Missing Spotify OAuth credentials');
      return redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=config_error`);
    }

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
    const { access_token: accessToken, refresh_token: refreshToken, expires_in: expiresIn } = tokenData;

    // Calculate expires_at from expires_in (seconds)
    const expiresAt = new Date(Date.now() + expiresIn * 1000);

    // Fetch Spotify user profile to get user ID
    const profileResponse = await fetch('https://api.spotify.com/v1/me', {
      headers: {
        'Authorization': `Bearer ${accessToken}`,
      },
    });

    if (!profileResponse.ok) {
      logger.error('Failed to fetch Spotify profile', { status: profileResponse.status });
      throw new Error(`Profile fetch failed: ${profileResponse.status}`);
    }

    const profile = await profileResponse.json() as { id: string };

    // Store tokens in Firestore
    await storeOAuthTokens(userId, 'spotify', {
      accessToken,
      refreshToken,
      expiresAt,
      externalUserId: profile.id,
    }, stores);

    logger.info('Successfully connected Spotify account', { userId, spotifyUserId: profile.id });

    // Redirect to success page
    return redirect(`${process.env.BASE_URL}/app/connections/spotify/success`);

  } catch (error: unknown) {
    logger.error('Error processing Spotify OAuth callback', { error });
    return redirect(`${process.env.BASE_URL}/app/connections/spotify/error?reason=server_error`);
  }
};

export const spotifyOAuthHandler = createCloudFunction(handler, {
  allowUnauthenticated: true, // Public OAuth callback endpoint
  skipExecutionLogging: true
});
