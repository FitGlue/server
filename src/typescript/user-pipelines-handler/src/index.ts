import { createCloudFunction, FirebaseAuthStrategy, HttpError, routeRequest, RouteMatch, FrameworkHandler, FrameworkContext } from '@fitglue/shared';
import { PipelineConfig, EnricherConfig } from '@fitglue/shared/dist/types/pb/user';
import { Destination } from '@fitglue/shared/dist/types/pb/events';

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
    disabled: false
  };

  await ctx.services.user.pipelineStore.create(userId, pipeline);
  logger.info('Created pipeline', { userId, pipelineId });
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
    disabled: (body.disabled as boolean) || false
  };

  await ctx.services.user.pipelineStore.create(userId, pipeline); // create() is idempotent (set operation)
  logger.info('Updated pipeline', { userId, pipelineId });
  return { message: 'Pipeline updated' };
}

async function handleDeletePipeline(userId: string, pipelineId: string, ctx: FrameworkContext) {
  const { logger } = ctx;
  await ctx.services.user.pipelineStore.delete(userId, pipelineId);
  logger.info('Deleted pipeline', { userId, pipelineId });
  return { message: 'Pipeline deleted' };
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
 * Maps registry IDs (e.g., 'hevy') to Go-expected format (e.g., 'SOURCE_HEVY').
 */
function normalizeSource(source: string): string {
  // Mapping from registry IDs to protobuf enum strings
  const sourceMap: Record<string, string> = {
    'hevy': 'SOURCE_HEVY',
    'fitbit': 'SOURCE_FITBIT',
    'mock': 'SOURCE_TEST',
    'apple-health': 'SOURCE_APPLE_HEALTH',
    'health-connect': 'SOURCE_HEALTH_CONNECT',
    'file_upload': 'SOURCE_FILE_UPLOAD',
  };

  // If already in protobuf format, return as-is
  if (source.startsWith('SOURCE_')) {
    return source;
  }

  // Map from registry ID to protobuf format
  return sourceMap[source.toLowerCase()] ?? `SOURCE_${source.toUpperCase()}`;
}

export const userPipelinesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
