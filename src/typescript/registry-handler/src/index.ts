// Module-level imports for smart pruning
import { createCloudFunction, FrameworkHandler } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { getRegistry } from '@fitglue/shared/plugin';

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

  const response = {
    sources: registry.sources.filter(shouldIncludePlugin),
    enrichers: registry.enrichers.filter(shouldIncludePlugin),
    destinations,
    integrations: registry.integrations.filter(shouldIncludePlugin), // Apply same logic to integrations
  };

  // Note: Cache-Control is not currently supported by SafeHandler.
  // Ideally: res.set('Cache-Control', 'public, max-age=300');

  logger.info('Plugin registry returned', {
    sourceCount: response.sources.length,
    enricherCount: response.enrichers.length,
    destinationCount: response.destinations.length,
    integrationCount: response.integrations.length,
  });

  return response;
};

// Export the wrapped function - no auth required (public endpoint)
export const registryHandler = createCloudFunction(handler, {
  allowUnauthenticated: true,
  skipExecutionLogging: true
});
