import * as crypto from 'crypto';
import { getSecret } from '../../infrastructure/secrets';

/**
 * Generate a signed OAuth state token containing the user ID
 * @param userId - The FitGlue user ID
 * @returns Base64-encoded signed state token
 */
export async function generateOAuthState(userId: string): Promise<string> {
  if (!process.env.OAUTH_STATE_SECRET && !process.env.GOOGLE_CLOUD_PROJECT) {
    throw new Error('Missing configuration: OAUTH_STATE_SECRET or GOOGLE_CLOUD_PROJECT environment variable is required');
  }
  const secret = process.env.OAUTH_STATE_SECRET || await getSecret(process.env.GOOGLE_CLOUD_PROJECT!, 'oauth-state-secret');
  const timestamp = Date.now();
  const expiresAt = timestamp + 10 * 60 * 1000; // 10 minutes

  const payload = JSON.stringify({ userId, expiresAt });
  const hmac = crypto.createHmac('sha256', secret);
  hmac.update(payload);
  const signature = hmac.digest('hex');

  const state = { payload, signature };
  return Buffer.from(JSON.stringify(state)).toString('base64url');
}

/**
 * Validate and extract user ID from OAuth state token
 * @param state - Base64-encoded state token
 * @returns User ID if valid, null otherwise
 */
export async function validateOAuthState(state: string): Promise<string | null> {
  // Fetch secret first - let this throw if configuration is missing (infrastructure error)
  if (!process.env.OAUTH_STATE_SECRET && !process.env.GOOGLE_CLOUD_PROJECT) {
    throw new Error('Missing configuration: OAUTH_STATE_SECRET or GOOGLE_CLOUD_PROJECT environment variable is required');
  }
  const secret = process.env.OAUTH_STATE_SECRET || await getSecret(process.env.GOOGLE_CLOUD_PROJECT!, 'oauth-state-secret');

  try {
    const decoded = JSON.parse(Buffer.from(state, 'base64url').toString());
    const { payload, signature } = decoded;

    // Verify signature
    const hmac = crypto.createHmac('sha256', secret);
    hmac.update(payload);
    const expectedSignature = hmac.digest('hex');

    if (signature !== expectedSignature) {
      console.warn('OAuth state signature mismatch');
      return null;
    }

    // Check expiration
    const { userId, expiresAt } = JSON.parse(payload);
    if (Date.now() > expiresAt) {
      console.warn('OAuth state expired');
      return null;
    }

    return userId;
  } catch (error) {
    console.warn('Error validating OAuth state:', error);
    return null;
  }
}

/**
 * Store OAuth tokens for a user integration
 */
export async function storeOAuthTokens(
  db: FirebaseFirestore.Firestore,
  userId: string,
  provider: 'strava' | 'fitbit',
  tokens: {
    accessToken: string;
    refreshToken: string;
    expiresAt: Date;
    externalUserId: string;
  }
): Promise<void> {
  const batch = db.batch();

  // Update user's integration
  const userRef = db.collection('users').doc(userId);
  // Write using snake_case (Source of Truth: Proto/Go)
  batch.update(userRef, {
    [`integrations.${provider}.enabled`]: true,
    [`integrations.${provider}.access_token`]: tokens.accessToken,
    [`integrations.${provider}.refresh_token`]: tokens.refreshToken,
    [`integrations.${provider}.expires_at`]: tokens.expiresAt,
    [`integrations.${provider}.${provider === 'strava' ? 'athlete_id' : 'fitbit_user_id'}`]: provider === 'strava' ? Number(tokens.externalUserId) : tokens.externalUserId,
  });

  // Create identity mapping
  const identityRef = db.collection('integrations').doc(provider).collection('ids').doc(tokens.externalUserId);
  batch.set(identityRef, {
    userId,
    createdAt: new Date(),
  });

  await batch.commit();
}

/**
 * Refresh tokens with the provider using the refresh token
 */
export async function refreshOAuthToken(
  provider: 'strava' | 'fitbit',
  refreshToken: string
): Promise<{ accessToken: string; refreshToken: string; expiresAt: Date }> {
  const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
  // Note: getSecret falls back to env vars if project not set or local
  const clientId = await getSecret(projectId, `${provider}-client-id`);
  const clientSecret = await getSecret(projectId, `${provider}-client-secret`);

  let url = '';
  const body = new URLSearchParams();

  if (provider === 'strava') {
    url = 'https://www.strava.com/oauth/token';
    body.append('client_id', clientId);
    body.append('client_secret', clientSecret);
    body.append('grant_type', 'refresh_token');
    body.append('refresh_token', refreshToken);
  } else if (provider === 'fitbit') {
    url = 'https://api.fitbit.com/oauth2/token';
    body.append('grant_type', 'refresh_token');
    body.append('refresh_token', refreshToken);
  }

  const headers: Record<string, string> = {
    'Content-Type': 'application/x-www-form-urlencoded',
  };

  if (provider === 'fitbit') {
    const credentials = Buffer.from(`${clientId}:${clientSecret}`).toString('base64');
    headers['Authorization'] = `Basic ${credentials}`;
  }

  const response = await fetch(url, {
    method: 'POST',
    headers,
    body,
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Token refresh failed for ${provider}: ${response.status} ${errorText}`);
  }

  const data = await response.json() as any;

  // Normalize response
  let accessToken = '';
  let newRefreshToken = '';
  let expiresAt = new Date();

  if (provider === 'strava') {
    accessToken = data.access_token;
    newRefreshToken = data.refresh_token;
    // Strava usually returns expires_at (seconds since epoch) and expires_in (seconds from now)
    if (data.expires_at) {
      expiresAt = new Date(data.expires_at * 1000);
    } else {
      expiresAt = new Date(Date.now() + data.expires_in * 1000);
    }
  } else if (provider === 'fitbit') {
    accessToken = data.access_token;
    newRefreshToken = data.refresh_token;
    expiresAt = new Date(Date.now() + data.expires_in * 1000);
  }

  return { accessToken, refreshToken: newRefreshToken, expiresAt };
}
