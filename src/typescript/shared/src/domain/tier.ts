import { UserRecord, UserTier } from '../types/pb/user';

export type EffectiveTier = 'hobbyist' | 'athlete';

export const HOBBYIST_TIER_LIMITS = {
  SYNCS_PER_MONTH: 25,
  MAX_CONNECTIONS: 2,
} as const;

/**
 * Determine the effective tier for a user.
 * Priority: admin > active trial > stored tier
 */
export function getEffectiveTier(user: UserRecord): EffectiveTier {
  // Admin override always grants Athlete
  if (user.isAdmin) {
    return 'athlete';
  }

  // Active trial grants Athlete
  if (user.trialEndsAt && new Date(user.trialEndsAt) > new Date()) {
    return 'athlete';
  }

  // Fall back to stored tier (default: hobbyist)
  if (user.tier === UserTier.USER_TIER_ATHLETE) {
    return 'athlete';
  }

  return 'hobbyist';
}

/**
 * Check if a user can perform a sync (Free tier: 25/month limit)
 */
export function canSync(user: UserRecord): { allowed: boolean; reason?: string } {
  const tier = getEffectiveTier(user);

  if (tier === 'athlete') {
    return { allowed: true };
  }

  // Check monthly limit
  const count = user.syncCountThisMonth || 0;
  if (count >= HOBBYIST_TIER_LIMITS.SYNCS_PER_MONTH) {
    return {
      allowed: false,
      reason: `Hobbyist tier limit reached (${HOBBYIST_TIER_LIMITS.SYNCS_PER_MONTH}/month). Upgrade to Athlete for unlimited syncs.`,
    };
  }

  return { allowed: true };
}

/**
 * Check if a user can add a new connection (Free tier: 2 max)
 */
export function canAddConnection(user: UserRecord, currentConnectionCount: number): { allowed: boolean; reason?: string } {
  const tier = getEffectiveTier(user);

  if (tier === 'athlete') {
    return { allowed: true };
  }

  if (currentConnectionCount >= HOBBYIST_TIER_LIMITS.MAX_CONNECTIONS) {
    return {
      allowed: false,
      reason: `Hobbyist tier limited to ${HOBBYIST_TIER_LIMITS.MAX_CONNECTIONS} connections. Upgrade to Athlete for unlimited.`,
    };
  }

  return { allowed: true };
}

/**
 * Calculate trial days remaining
 */
export function getTrialDaysRemaining(user: UserRecord): number | null {
  if (!user.trialEndsAt) return null;

  const now = new Date();
  const trialEnd = new Date(user.trialEndsAt);

  if (trialEnd <= now) return 0;

  const diffMs = trialEnd.getTime() - now.getTime();
  return Math.ceil(diffMs / (1000 * 60 * 60 * 24));
}

/**
 * Count active integrations for a user
 */
export function countActiveConnections(user: UserRecord): number {
  const integrations = user.integrations || {};
  let count = 0;
  if (integrations.hevy?.enabled) count++;
  if (integrations.strava?.enabled) count++;
  if (integrations.fitbit?.enabled) count++;
  // Add mock only if in dev mode? For now, exclude from user-facing counts
  return count;
}
