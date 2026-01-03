import * as admin from 'firebase-admin';
import { UserStore, IntegrationIdentityStore } from '../../storage/firestore';
import * as crypto from 'crypto';

/**
 * Store OAuth tokens for a user integration.
 * This function is used by OAuth handlers to save tokens after successful authentication.
 */
export async function storeOAuthTokens(
  userId: string,
  provider: 'strava' | 'fitbit',
  tokens: {
    accessToken: string;
    refreshToken: string;
    expiresAt: Date;
    externalUserId: string;
  },
  stores?: { users: UserStore; integrationIdentities: IntegrationIdentityStore }
): Promise<void> {
  // If stores not provided, create them (for backward compatibility)
  const db = admin.firestore();
  const userStore = stores?.users || new UserStore(db);
  const identityStore = stores?.integrationIdentities || new IntegrationIdentityStore(db);

  // Update user's integration tokens
  // Update user's integration tokens (using new typed method)
  await userStore.setIntegration(userId, provider, {
    enabled: true,
    accessToken: tokens.accessToken,
    refreshToken: tokens.refreshToken,
    expiresAt: tokens.expiresAt,
    createdAt: new Date(),
    lastUsedAt: new Date(),
    // Add provider-specific ID fields
    ...(provider === 'strava' ? { athleteId: Number(tokens.externalUserId) } : {}),
    ...(provider === 'fitbit' ? { fitbitUserId: tokens.externalUserId } : {})
  } as any); // Cast as any because setIntegration expects strict union checks that are hard to satisfy generically here without complex logic

  // Map external user ID to our user ID
  await identityStore.mapIdentity(provider, tokens.externalUserId, userId);
}

/**
 * Generate a random state token for OAuth.
 * In production this should be signed/encrypted.
 */
export async function generateOAuthState(userId: string): Promise<string> {
  const random = crypto.randomBytes(16).toString('hex');
  return `${userId}:${random}`;
}

export async function validateOAuthState(state: string): Promise<{ userId: string; valid: boolean }> {
  const parts = state.split(':');
  if (parts.length < 2) return { userId: '', valid: false };
  return { userId: parts[0], valid: true };
}
