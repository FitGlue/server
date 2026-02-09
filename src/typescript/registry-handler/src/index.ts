// Module-level imports for smart pruning
import { createCloudFunction, FrameworkHandler } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { getRegistry } from '@fitglue/shared/plugin';
import { initializeApp, getApps } from 'firebase-admin/app';
import { getFirestore } from 'firebase-admin/firestore';

// Get project ID for environment detection
// GOOGLE_CLOUD_PROJECT is set by Terraform in service_config.environment_variables
const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-server-dev';

/**
 * Registry Handler
 *
 * Endpoints:
 * - GET /registry: Returns all registered connections and plugins
 *
 * This endpoint is public (no auth required) to allow the frontend to fetch
 * registry metadata for page rendering before user authentication.
 *
 * When ?marketingMode=true, also returns platform stats (athlete count,
 * activities boosted) for the marketing site build.
 */

/**
 * Get the Showcase base URL based on environment.
 * - dev: dev.fitglue.tech
 * - test: test.fitglue.tech
 * - prod: fitglue.tech
 */
function getShowcaseBaseUrl(): string {
  // Project IDs: fitglue-server-dev, fitglue-server-test, fitglue-server-prod
  if (projectId.includes('-prod')) {
    return 'https://fitglue.tech';
  }
  if (projectId.includes('-test')) {
    return 'https://test.fitglue.tech';
  }
  // Dev environment (or unknown/local defaults to dev)
  return 'https://dev.fitglue.tech';
}

/**
 * Fetch platform-wide stats from Firestore for the marketing site.
 * - athleteCount: users with access_enabled == true
 * - activitiesBoostedCount: sum of activityCounts.synchronized across all users
 */
export async function getPlatformStats(logger: { info: (...args: unknown[]) => void; warn: (...args: unknown[]) => void }): Promise<{ athleteCount: number; activitiesBoostedCount: number }> {
  // Lazy-init Firebase Admin (only needed for stats)
  if (getApps().length === 0) {
    initializeApp();
  }
  const db = getFirestore();

  // Count athletes (users with access enabled)
  const athleteSnapshot = await db.collection('users')
    .where('access_enabled', '==', true)
    .count()
    .get();
  const athleteCount = athleteSnapshot.data().count;

  // Sum synchronized activity counts across all users
  // Each user document has activityCounts.synchronized as a materialized counter
  const usersSnapshot = await db.collection('users')
    .where('access_enabled', '==', true)
    .select('activityCounts.synchronized')
    .get();

  let activitiesBoostedCount = 0;
  usersSnapshot.forEach(doc => {
    const data = doc.data();
    const syncCount = data?.activityCounts?.synchronized;
    if (typeof syncCount === 'number') {
      activitiesBoostedCount += syncCount;
    }
  });

  logger.info('Platform stats fetched', { athleteCount, activitiesBoostedCount });

  return { athleteCount, activitiesBoostedCount };
}

export const handler: FrameworkHandler = async (req, ctx) => {
  const { logger } = ctx;

  // Only allow GET
  if (req.method !== 'GET') {
    throw new HttpError(405, 'Method Not Allowed');
  }

  // Get the plugin registry from shared library
  const registry = getRegistry();

  // Filter out disabled plugins unless ?showAll=true
  // Marketing mode (?marketingMode=true) shows all enabled plugins including temporarily unavailable ones
  const showAll = req.query.showAll === 'true';
  const marketingMode = req.query.marketingMode === 'true';

  // Helper to determine if a plugin should be included
  const shouldIncludePlugin = (p: { enabled?: boolean; isTemporarilyUnavailable?: boolean }) => {
    if (showAll) return p.enabled;
    if (marketingMode) return p.enabled; // Include temp unavailable in marketing
    return p.enabled && !p.isTemporarilyUnavailable; // Exclude temp unavailable in app
  };

  // Inject env-specific Showcase URL template
  const showcaseBaseUrl = getShowcaseBaseUrl();
  const destinations = registry.destinations
    .filter(shouldIncludePlugin)
    .map(d => {
      if (d.id === 'showcase') {
        return { ...d, externalUrlTemplate: `${showcaseBaseUrl}/showcase/{id}` };
      }
      return d;
    });

  const response: Record<string, unknown> = {
    sources: registry.sources.filter(shouldIncludePlugin),
    enrichers: registry.enrichers.filter(shouldIncludePlugin),
    destinations,
    integrations: registry.integrations.filter(shouldIncludePlugin),
  };

  // Include platform stats for marketing builds
  if (marketingMode) {
    try {
      response.stats = await getPlatformStats(logger);
    } catch (err) {
      logger.warn('Failed to fetch platform stats, omitting from response', { error: (err as Error).message });
      // Non-fatal: marketing site will use fallback values
    }
  }

  // Note: Cache-Control is not currently supported by SafeHandler.
  // Ideally: res.set('Cache-Control', 'public, max-age=300');

  logger.info('Plugin registry returned', {
    sourceCount: (response.sources as unknown[]).length,
    enricherCount: (response.enrichers as unknown[]).length,
    destinationCount: (response.destinations as unknown[]).length,
    integrationCount: (response.integrations as unknown[]).length,
    hasStats: !!response.stats,
  });

  return response;
};

// Export the wrapped function - no auth required (public endpoint)
export const registryHandler = createCloudFunction(handler, {
  allowUnauthenticated: true,
  skipExecutionLogging: true
});
