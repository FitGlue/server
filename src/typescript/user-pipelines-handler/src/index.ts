// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, FrameworkContext } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch } from '@fitglue/shared/routing';
import { PipelineConfig, EnricherConfig, DestinationConfig, Destination, parseActivitySource, ActivitySource, formatActivitySource } from '@fitglue/shared/types';
import { getPluginHooks, PluginLifecycleContext } from '@fitglue/shared/plugin';

export const handler: FrameworkHandler = async (req, ctx) => {
  const userId = ctx.userId;
  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  return await routeRequest(req, ctx, [
    {
      method: 'GET',
      pattern: '/api/users/me/pipelines',
      handler: async () => await handleListPipelines(userId, ctx)
    },
    {
      method: 'GET',
      pattern: '/api/users/me/pipelines/:pipelineId',
      handler: async (match: RouteMatch) => await handleGetPipeline(userId, match.params.pipelineId, ctx)
    },
    {
      method: 'POST',
      pattern: '/api/users/me/pipelines',
      handler: async () => await handleCreatePipeline(userId, req, ctx)
    },
    {
      method: 'PATCH',
      pattern: '/api/users/me/pipelines/:pipelineId',
      handler: async (match: RouteMatch) => await handleUpdatePipeline(userId, match.params.pipelineId, req, ctx)
    },
    {
      method: 'DELETE',
      pattern: '/api/users/me/pipelines/:pipelineId',
      handler: async (match: RouteMatch) => await handleDeletePipeline(userId, match.params.pipelineId, ctx)
    },
    {
      method: 'GET',
      pattern: '/api/users/me/plugin-defaults',
      handler: async () => await handleListPluginDefaults(userId, ctx)
    },
    {
      method: 'PUT',
      pattern: '/api/users/me/plugin-defaults/:pluginId',
      handler: async (match: RouteMatch) => await handleSetPluginDefault(userId, match.params.pluginId, req, ctx)
    },
    {
      method: 'DELETE',
      pattern: '/api/users/me/plugin-defaults/:pluginId',
      handler: async (match: RouteMatch) => await handleDeletePluginDefault(userId, match.params.pluginId, ctx)
    }
  ]);
};

async function handleListPipelines(userId: string, ctx: FrameworkContext) {
  const pipelines = await ctx.services.user.pipelineStore.list(userId);
  return { pipelines };
}

async function handleGetPipeline(userId: string, pipelineId: string, ctx: FrameworkContext) {
  const pipeline = await ctx.services.user.pipelineStore.get(userId, pipelineId);
  if (!pipeline) {
    throw new HttpError(404, 'Pipeline not found');
  }
  return pipeline;
}

async function handleCreatePipeline(userId: string, req: { body: Record<string, unknown> }, ctx: FrameworkContext) {
  const { logger } = ctx;
  const body = req.body;

  // Validate required fields
  if (!body.source) {
    throw new HttpError(400, 'Missing required field: source');
  }

  if (!body.destinations || !Array.isArray(body.destinations) || body.destinations.length === 0) {
    throw new HttpError(400, 'Missing required field: destinations (must be non-empty array)');
  }

  // Generate pipeline ID
  const pipelineId = `pipe_${Date.now()}`;

  const normalizedSource = normalizeSource(body.source as string);
  const destEnums = mapDestinations(body.destinations as (string | number)[]);

  const pipeline: PipelineConfig = {
    id: pipelineId,
    name: (body.name as string) || '',
    source: normalizedSource,
    enrichers: (body.enrichers as EnricherConfig[]) || [],
    destinations: destEnums,
    disabled: false,
    sourceConfig: (body.sourceConfig as Record<string, string>) || {},
    destinationConfigs: (body.destinationConfigs as Record<string, DestinationConfig>) || {},
  };

  await ctx.services.user.pipelineStore.create(userId, pipeline);

  // Run lifecycle hooks for the source plugin (e.g. GitHub webhook registration)
  const sourceRegistryId = getSourceRegistryId(normalizedSource);
  if (sourceRegistryId) {
    const hooks = getPluginHooks(sourceRegistryId);
    if (hooks?.onPipelineCreate) {
      const hookCtx: PluginLifecycleContext = {
        userId,
        pipelineId,
        config: pipeline.sourceConfig,
        getValidToken: (uid: string, provider: string) => ctx.services.user.getValidToken(uid, provider as import('@fitglue/shared/dist/infrastructure/oauth/token-source').OAuthProvider),
        logger,
      };
      // Throws on failure → pipeline creation is rolled back
      try {
        const configUpdate = await hooks.onPipelineCreate(hookCtx);
        if (configUpdate) {
          pipeline.sourceConfig = { ...pipeline.sourceConfig, ...configUpdate };
          await ctx.services.user.pipelineStore.create(userId, pipeline);
        }
      } catch (err) {
        // Roll back the pipeline we just created
        logger.error('Lifecycle hook failed, rolling back pipeline', { pipelineId, error: String(err) });
        await ctx.services.user.pipelineStore.delete(userId, pipelineId);
        throw new HttpError(500, `Pipeline source setup failed: ${err instanceof Error ? err.message : String(err)}`);
      }
    }
  }

  logger.info('Created pipeline', { userId, pipelineId });

  // Auto-save plugin defaults (first config wins - won't overwrite existing defaults)
  await savePluginDefaults(userId, pipeline, ctx);

  return { id: pipelineId };
}

async function handleUpdatePipeline(userId: string, pipelineId: string, req: { body: Record<string, unknown> }, ctx: FrameworkContext) {
  const { logger } = ctx;
  const body = req.body;

  // Check if this is a partial update (disabled field present, no source = toggle request)
  const hasDisabled = Object.prototype.hasOwnProperty.call(body, 'disabled');
  const hasSource = Object.prototype.hasOwnProperty.call(body, 'source');

  logger.info('PATCH pipeline request received', { userId, pipelineId, hasDisabled, hasSource });

  if (hasDisabled && !hasSource) {
    // Toggle disabled state only
    await ctx.services.user.pipelineStore.toggleDisabled(userId, pipelineId, body.disabled as boolean);
    logger.info('Toggled pipeline disabled state', { userId, pipelineId, disabled: body.disabled });
    return { message: 'Pipeline disabled state updated' };
  }

  // Full replacement - validate required fields first
  if (!body.source) {
    throw new HttpError(400, 'Missing required field: source');
  }

  if (!body.destinations || !Array.isArray(body.destinations) || body.destinations.length === 0) {
    throw new HttpError(400, 'Missing required field: destinations (must be non-empty array)');
  }

  const normalizedSource = normalizeSource(body.source as string);
  const destEnums = mapDestinations(body.destinations as (string | number)[]);

  const pipeline: PipelineConfig = {
    id: pipelineId,
    name: (body.name as string) || '',
    source: normalizedSource,
    enrichers: (body.enrichers as EnricherConfig[]) || [],
    destinations: destEnums,
    disabled: (body.disabled as boolean) || false,
    sourceConfig: (body.sourceConfig as Record<string, string>) || {},
    destinationConfigs: (body.destinationConfigs as Record<string, DestinationConfig>) || {},
  };

  await ctx.services.user.pipelineStore.create(userId, pipeline); // create() is idempotent (set operation)
  logger.info('Updated pipeline', { userId, pipelineId });

  // Auto-save plugin defaults (first config wins - won't overwrite existing defaults)
  await savePluginDefaults(userId, pipeline, ctx);

  return { message: 'Pipeline updated' };
}

async function handleDeletePipeline(userId: string, pipelineId: string, ctx: FrameworkContext) {
  const { logger } = ctx;

  // Fetch pipeline to get sourceConfig before deletion (for webhook cleanup)
  const pipeline = await ctx.services.user.pipelineStore.get(userId, pipelineId);
  if (pipeline) {
    const sourceRegistryId = getSourceRegistryId(pipeline.source);
    if (sourceRegistryId) {
      const hooks = getPluginHooks(sourceRegistryId);
      if (hooks?.onPipelineDelete) {
        const hookCtx: PluginLifecycleContext = {
          userId,
          pipelineId,
          config: pipeline.sourceConfig || {},
          getValidToken: (uid: string, provider: string) => ctx.services.user.getValidToken(uid, provider as import('@fitglue/shared/dist/infrastructure/oauth/token-source').OAuthProvider),
          logger,
        };
        try {
          await hooks.onPipelineDelete(hookCtx);
        } catch (err) {
          logger.warn('Lifecycle hook onPipelineDelete failed (best-effort)', { pipelineId, error: String(err) });
        }
      }
    }
  }

  await ctx.services.user.pipelineStore.delete(userId, pipelineId);
  logger.info('Deleted pipeline', { userId, pipelineId });
  return { message: 'Pipeline deleted' };
}

// --- Plugin defaults handlers ---

async function handleListPluginDefaults(userId: string, ctx: FrameworkContext) {
  const defaults = await ctx.services.user.pluginDefaultsStore.list(userId);
  return { defaults };
}

async function handleSetPluginDefault(userId: string, pluginId: string, req: { body: Record<string, unknown> }, ctx: FrameworkContext) {
  const { config } = req.body;
  if (!config || typeof config !== 'object') {
    throw new HttpError(400, 'config is required and must be an object');
  }

  const now = new Date();
  const pluginDefault = {
    pluginId,
    config: config as Record<string, string>,
    createdAt: now,
    updatedAt: now,
  };

  await ctx.services.user.pluginDefaultsStore.set(userId, pluginDefault);
  ctx.logger.info('Set plugin default', { userId, pluginId });
  return pluginDefault;
}

async function handleDeletePluginDefault(userId: string, pluginId: string, ctx: FrameworkContext) {
  await ctx.services.user.pluginDefaultsStore.delete(userId, pluginId);
  ctx.logger.info('Deleted plugin default', { userId, pluginId });
  return { success: true };
}

/**
 * Auto-save source and destination configs as user-level plugin defaults.
 * Uses setIfNotExists: the first pipeline config for a given plugin becomes
 * that user's default. Subsequent pipelines don't overwrite it.
 * This runs fire-and-forget style — failures are logged but don't fail the request.
 */
async function savePluginDefaults(userId: string, pipeline: PipelineConfig, ctx: FrameworkContext) {
  const { logger } = ctx;
  const store = ctx.services.user.pluginDefaultsStore;

  try {
    // Save source config as default
    if (pipeline.sourceConfig && Object.keys(pipeline.sourceConfig).length > 0) {
      const sourcePluginId = getSourceRegistryId(pipeline.source);
      if (sourcePluginId) {
        const created = await store.setIfNotExists(userId, {
          pluginId: sourcePluginId,
          config: pipeline.sourceConfig,
          createdAt: new Date(),
          updatedAt: new Date(),
        });
        if (created) {
          logger.info('Saved source plugin default', { plugin: sourcePluginId });
        }
      }
    }

    // Save destination configs as defaults
    if (pipeline.destinationConfigs) {
      for (const [destId, destCfg] of Object.entries(pipeline.destinationConfigs) as [string, DestinationConfig][]) {
        if (destCfg?.config && Object.keys(destCfg.config).length > 0) {
          const created = await store.setIfNotExists(userId, {
            pluginId: destId,
            config: destCfg.config,
            createdAt: new Date(),
            updatedAt: new Date(),
          });
          if (created) {
            logger.info('Saved destination plugin default', { plugin: destId });
          }
        }
      }
    }
  } catch (err) {
    // Best-effort: don't fail the pipeline operation if defaults save fails
    logger.warn('Failed to save plugin defaults (best-effort)', { error: String(err) });
  }
}

// Helper functions

function mapDestinations(dests: (string | number)[]): Destination[] {
  return dests.map(d => {
    // If already a number, validate and return as Destination
    if (typeof d === 'number') {
      return Object.values(Destination).includes(d) ? d : Destination.DESTINATION_UNSPECIFIED;
    }

    // String lookup: check against proto enum names (e.g., 'DESTINATION_STRAVA' or just 'strava')
    const normalized = d.toUpperCase();
    const enumKey = normalized.startsWith('DESTINATION_') ? normalized : `DESTINATION_${normalized}`;

    // Find matching enum value from Destination enum
    const enumValue = Destination[enumKey as keyof typeof Destination];
    if (typeof enumValue === 'number') {
      return enumValue;
    }

    return Destination.DESTINATION_UNSPECIFIED;
  });
}

/**
 * Normalize source ID from registry format to protobuf enum string format.
 * Uses the generated enum formatter to map registry IDs (e.g., 'hevy', 'github')
 * to Go-expected format (e.g., 'SOURCE_HEVY', 'SOURCE_GITHUB').
 */
function normalizeSource(source: string): string {
  // If already in protobuf format, return as-is
  if (source.startsWith('SOURCE_')) {
    return source;
  }

  const parsed = parseActivitySource(source);
  if (parsed === ActivitySource.SOURCE_UNKNOWN) {
    throw new HttpError(400, `Unknown source: ${source}`);
  }
  return ActivitySource[parsed];
}

/**
 * Reverse-map a protobuf source string (e.g. 'SOURCE_GITHUB') back to a
 * registry ID (e.g. 'github') for plugin hook lookup.
 */
function getSourceRegistryId(protoSource: string): string | null {
  // formatActivitySource turns 'SOURCE_GITHUB' → 'GitHub' etc.
  // We lowercase to get the registry ID ('github').
  const parsed = parseActivitySource(protoSource);
  if (parsed === ActivitySource.SOURCE_UNKNOWN) {
    return null;
  }
  const formatted = formatActivitySource(parsed);
  return formatted ? formatted.toLowerCase().replace(/\s+/g, '-') : null;
}

export const userPipelinesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
