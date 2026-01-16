import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy } from '@fitglue/shared';
import { Request, Response } from 'express';
import { v4 as uuidv4 } from 'uuid';

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const userId = ctx.userId;
  if (!userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  const { logger } = ctx;

  try {
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
      return await handleListPipelines(userId, res, ctx);
    }

    // GET /users/me/pipelines/{pipelineId} - Get single pipeline
    if (req.method === 'GET' && pathParts.length >= 1) {
      const pipelineId = pathParts[0];
      return await handleGetPipeline(userId, pipelineId, res, ctx);
    }

    // POST /users/me/pipelines - Create new pipeline
    if (req.method === 'POST' && pathParts.length === 0) {
      return await handleCreatePipeline(userId, req, res, ctx);
    }

    // PATCH /users/me/pipelines/{pipelineId} - Update pipeline
    if (req.method === 'PATCH' && pathParts.length >= 1) {
      const pipelineId = pathParts[0];
      return await handleUpdatePipeline(userId, pipelineId, req, res, ctx);
    }

    // DELETE /users/me/pipelines/{pipelineId} - Delete pipeline
    if (req.method === 'DELETE' && pathParts.length >= 1) {
      const pipelineId = pathParts[0];
      return await handleDeletePipeline(userId, pipelineId, res, ctx);
    }

    res.status(404).json({ error: 'Not found' });
  } catch (err) {
    logger.error('Failed to handle pipelines request', { error: err });
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

async function handleListPipelines(userId: string, res: Response, ctx: FrameworkContext) {
  const user = await ctx.services.user.get(userId);
  if (!user) {
    res.status(404).json({ error: 'User not found' });
    return;
  }

  res.status(200).json({ pipelines: user.pipelines || [] });
}

async function handleGetPipeline(userId: string, pipelineId: string, res: Response, ctx: FrameworkContext) {
  const user = await ctx.services.user.get(userId);
  if (!user) {
    res.status(404).json({ error: 'User not found' });
    return;
  }

  const pipeline = (user.pipelines || []).find((p: { id: string }) => p.id === pipelineId);
  if (!pipeline) {
    res.status(404).json({ error: 'Pipeline not found' });
    return;
  }

  res.status(200).json(pipeline);
}

async function handleCreatePipeline(userId: string, req: Request, res: Response, ctx: FrameworkContext) {
  const { logger } = ctx;
  const body = req.body;

  // Validate required fields
  if (!body.source) {
    res.status(400).json({ error: 'Missing required field: source' });
    return;
  }

  if (!body.destinations || !Array.isArray(body.destinations) || body.destinations.length === 0) {
    res.status(400).json({ error: 'Missing required field: destinations (must be non-empty array)' });
    return;
  }

  // Generate pipeline ID
  const pipelineId = body.id || uuidv4();

  const pipeline = {
    id: pipelineId,
    source: body.source,
    enrichers: body.enrichers || [],
    destinations: body.destinations
  };

  try {
    // addPipeline(userId, source, enrichers, destinations) returns generated ID
    const generatedId = await ctx.services.user.addPipeline(
      userId,
      pipeline.source,
      pipeline.enrichers,
      pipeline.destinations
    );
    logger.info('Created pipeline', { userId, pipelineId: generatedId });
    res.status(200).json({ id: generatedId });
  } catch (err) {
    logger.error('Failed to create pipeline', { error: err });
    res.status(500).json({ error: 'Failed to create pipeline' });
  }
}

async function handleUpdatePipeline(userId: string, pipelineId: string, req: Request, res: Response, ctx: FrameworkContext) {
  const { logger } = ctx;
  const body = req.body;

  try {
    // replacePipeline(userId, pipelineId, source, enrichers, destinations)
    await ctx.services.user.replacePipeline(
      userId,
      pipelineId,
      body.source,
      body.enrichers || [],
      body.destinations || []
    );
    logger.info('Updated pipeline', { userId, pipelineId });
    res.status(200).json({ message: 'Pipeline updated' });
  } catch (err) {
    logger.error('Failed to update pipeline', { error: err });
    res.status(500).json({ error: 'Failed to update pipeline' });
  }
}

async function handleDeletePipeline(userId: string, pipelineId: string, res: Response, ctx: FrameworkContext) {
  const { logger } = ctx;

  try {
    await ctx.services.user.removePipeline(userId, pipelineId);
    logger.info('Deleted pipeline', { userId, pipelineId });
    res.status(200).json({ message: 'Pipeline deleted' });
  } catch (err) {
    logger.error('Failed to delete pipeline', { error: err });
    res.status(500).json({ error: 'Failed to delete pipeline' });
  }
}

export const userPipelinesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  }
});
