import * as admin from 'firebase-admin';
import { UserStore, IntegrationIdentityStore } from '../../storage/firestore';

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
  await userStore.update(userId, {
    [`integrations.${provider}`]: {
      enabled: true,
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
      expiresAt: tokens.expiresAt
    }
  });

  // Map external user ID to our user ID
  await identityStore.mapIdentity(provider, tokens.externalUserId, userId);
}
