import * as crypto from 'crypto';
import { getSecret } from './secrets';

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
  batch.update(userRef, {
    [`integrations.${provider}.enabled`]: true,
    [`integrations.${provider}.access_token`]: tokens.accessToken,
    [`integrations.${provider}.refresh_token`]: tokens.refreshToken,
    [`integrations.${provider}.expires_at`]: tokens.expiresAt,
    [`integrations.${provider}.${provider === 'strava' ? 'athlete_id' : 'fitbit_user_id'}`]: tokens.externalUserId,
  });

  // Create identity mapping
  const identityRef = db.collection('integrations').doc(provider).collection('ids').doc(tokens.externalUserId);
  batch.set(identityRef, {
    userId,
    createdAt: new Date(),
  });

  await batch.commit();
}
