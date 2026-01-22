import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, HttpError } from '@fitglue/shared';
import { Request } from 'express';
import { v4 as uuidv4 } from 'uuid';

// ... imports

export const handler = async (req: Request, ctx: FrameworkContext) => {
  const userId = ctx.userId;
  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  const { logger } = ctx;
  const path = req.path;

  // Extract path segments for sub-routes
  // Path may be /api/users/me/pipelines or /pipelines or just /
  // Get the last meaningful segments after 'pipelines'
  const pipelinesIndex = path.indexOf('/pipelines');
  const subPath = pipelinesIndex >= 0
    ? path.substring(pipelinesIndex + '/pipelines'.length)
    : path;
  const pathParts = subPath.split('/').filter(p => p !== '');

  logger.info('Routing request', { path, subPath, pathParts, method: req.method });

  // GET /users/me/pipelines - List all pipelines
  if (req.method === 'GET' && pathParts.length === 0) {
    return await handleListPipelines(userId, ctx);
  }

  // GET /users/me/pipelines/{pipelineId} - Get single pipeline
  if (req.method === 'GET' && pathParts.length >= 1) {
    const pipelineId = pathParts[0];
    return await handleGetPipeline(userId, pipelineId, ctx);
  }

  // POST /users/me/pipelines - Create new pipeline
  if (req.method === 'POST' && pathParts.length === 0) {
    return await handleCreatePipeline(userId, req, ctx);
  }

  // PATCH /users/me/pipelines/{pipelineId} - Update pipeline
  if (req.method === 'PATCH' && pathParts.length >= 1) {
    const pipelineId = pathParts[0];
    return await handleUpdatePipeline(userId, pipelineId, req, ctx);
  }

  // DELETE /users/me/pipelines/{pipelineId} - Delete pipeline
  if (req.method === 'DELETE' && pathParts.length >= 1) {
    const pipelineId = pathParts[0];
    return await handleDeletePipeline(userId, pipelineId, ctx);
  }

  throw new HttpError(404, 'Not found');
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

async function handleCreatePipeline(userId: string, req: Request, ctx: FrameworkContext) {
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
    pipeline.name,
    pipeline.source,
    pipeline.enrichers,
    pipeline.destinations
  );
  logger.info('Created pipeline', { userId, pipelineId: generatedId });
  return { id: generatedId };
}

async function handleUpdatePipeline(userId: string, pipelineId: string, req: Request, ctx: FrameworkContext) {
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
    await ctx.services.user.togglePipelineDisabled(userId, pipelineId, body.disabled);
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
    pipelineId,
    body.name || '',
    body.source,
    body.enrichers || [],
    body.destinations
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
