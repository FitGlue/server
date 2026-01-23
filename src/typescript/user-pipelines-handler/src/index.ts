import { createCloudFunction, FirebaseAuthStrategy, HttpError, routeRequest, RouteMatch, FrameworkHandler, FrameworkContext, EnricherConfig } from '@fitglue/shared';
import { v4 as uuidv4 } from 'uuid';

// ... imports

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
  const user = await ctx.services.user.get(userId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }

  return { pipelines: user.pipelines || [] };
}

async function handleGetPipeline(userId: string, pipelineId: string, ctx: FrameworkContext) {
  const user = await ctx.services.user.get(userId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }

  const pipeline = (user.pipelines || []).find((p: { id: string }) => p.id === pipelineId);
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
  const pipelineId = body.id || uuidv4();

  const pipeline = {
    id: pipelineId,
    name: body.name || '', // Optional name field
    source: body.source,
    enrichers: body.enrichers || [],
    destinations: body.destinations
  };

  // addPipeline(userId, name, source, enrichers, destinations) returns generated ID
  const generatedId = await ctx.services.user.addPipeline(
    userId,
    pipeline.name as string,
    pipeline.source as string,
    pipeline.enrichers as EnricherConfig[],
    pipeline.destinations as string[]
  );
  logger.info('Created pipeline', { userId, pipelineId: generatedId });
  return { id: generatedId };
}

async function handleUpdatePipeline(userId: string, pipelineId: string, req: { body: Record<string, unknown> }, ctx: FrameworkContext) {
  const { logger } = ctx;
  const body = req.body;

  // Check if this is a partial update (disabled field present, no source = toggle request)
  // A full replacement always requires 'source', so its absence indicates toggle intent
  const bodyKeys = Object.keys(body);
  const hasDisabled = Object.prototype.hasOwnProperty.call(body, 'disabled');
  const hasSource = Object.prototype.hasOwnProperty.call(body, 'source');

  logger.info('PATCH pipeline request received', { userId, pipelineId, bodyKeys, body, hasDisabled, hasSource });

  if (hasDisabled && !hasSource) {
    // Toggle disabled state only
    await ctx.services.user.togglePipelineDisabled(userId, pipelineId, body.disabled as boolean);
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

  await ctx.services.user.replacePipeline(
    userId,
    {
      pipelineId,
      name: (body.name || '') as string,
      source: body.source as string,
      enrichers: (body.enrichers || []) as EnricherConfig[],
      destinations: body.destinations as string[]
    }
  );
  logger.info('Updated pipeline', { userId, pipelineId });
  return { message: 'Pipeline updated' };
}

async function handleDeletePipeline(userId: string, pipelineId: string, ctx: FrameworkContext) {
  const { logger } = ctx;
  await ctx.services.user.removePipeline(userId, pipelineId);
  logger.info('Deleted pipeline', { userId, pipelineId });
  return { message: 'Pipeline deleted' };
}

export const userPipelinesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
