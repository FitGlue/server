import { createCloudFunction, getRegistry, FrameworkContext } from '@fitglue/shared';
import { Request, Response } from 'express';

/**
 * Registry Handler
 *
 * Endpoints:
 * - GET /registry: Returns all registered connections and plugins
 *
 * This endpoint is public (no auth required) to allow the frontend to fetch
 * registry metadata for page rendering before user authentication.
 */

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const { logger } = ctx;

  // Only allow GET
  if (req.method !== 'GET') {
    res.status(405).json({ error: 'Method Not Allowed' });
    return;
  }

  try {
    // Get the plugin registry from shared library
    const registry = getRegistry();

    // Filter out disabled plugins unless ?showAll=true
    const showAll = req.query.showAll === 'true';

    const response = {
      sources: registry.sources.filter(p => showAll || p.enabled),
      enrichers: registry.enrichers.filter(p => showAll || p.enabled),
      destinations: registry.destinations.filter(p => showAll || p.enabled),
      integrations: registry.integrations, // Already filtered in getRegistry()
    };

    // Cache for 5 minutes (plugin list rarely changes)
    res.set('Cache-Control', 'public, max-age=300');
    res.status(200).json(response);

    logger.info('Plugin registry returned', {
      sourceCount: response.sources.length,
      enricherCount: response.enrichers.length,
      destinationCount: response.destinations.length,
      integrationCount: response.integrations.length,
    });
  } catch (e) {
    logger.error('Failed to get plugin registry', { error: e });
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

// Export the wrapped function - no auth required (public endpoint)
export const registryHandler = createCloudFunction(handler, {
  allowUnauthenticated: true,
  skipExecutionLogging: true
});

