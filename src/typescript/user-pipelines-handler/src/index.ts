import {
  createCloudFunction,
  db,
  FrameworkContext,
  FirebaseAuthStrategy,
  UserStore,
  UserService,
  ActivityStore,
  EnricherConfig
} from '@fitglue/shared';
import { Request, Response } from 'express';

/**
 * User Pipelines Handler
 *
 * Endpoints:
 * - GET /users/me/pipelines: List user's pipelines
 * - POST /users/me/pipelines: Create a new pipeline
 * - PATCH /users/me/pipelines/{pipelineId}: Update a pipeline
 * - DELETE /users/me/pipelines/{pipelineId}: Delete a pipeline
 */

// Valid sources and destinations
const VALID_SOURCES = ['hevy', 'fitbit'];
const VALID_DESTINATIONS = ['strava', 'mock'];

// Map destination enums to strings
const DESTINATION_ENUM_MAP: Record<number, string> = {
  0: 'unspecified',
  1: 'strava',
  2: 'mock'
};


// Extract pipeline ID from path
function extractPipelineId(path: string): string | null {
  // Path patterns:
  // /pipelines/{pipelineId}
  // Also handle Firebase hosting rewrite paths like /api/users/me/pipelines/pipe_123456
  const segments = path.split('/').filter(s => s.length > 0);

  // Find 'pipelines' in path and get next segment
  const pipelinesIdx = segments.findIndex(s => s === 'pipelines');
  if (pipelinesIdx >= 0 && segments.length > pipelinesIdx + 1) {
    return segments[pipelinesIdx + 1];
  }

  return null;
}

// Map pipeline to response format
function mapPipelineToResponse(pipeline: {
  id: string;
  source: string;
  enrichers?: EnricherConfig[];
  destinations: number[];
}) {
  return {
    id: pipeline.id,
    source: pipeline.source,
    enrichers: (pipeline.enrichers || []).map(e => ({
      providerType: e.providerType,
      inputs: e.inputs
    })),
    destinations: pipeline.destinations.map(d => DESTINATION_ENUM_MAP[d] || 'unknown')
  };
}

// Validate pipeline config
function validatePipelineConfig(
  source: string,
  destinations: string[],
  enrichers?: { providerType: number; inputs?: Record<string, string> }[]
): string | null {
  if (!source || !VALID_SOURCES.includes(source)) {
    return `Invalid source. Must be one of: ${VALID_SOURCES.join(', ')}`;
  }

  if (!destinations || !Array.isArray(destinations) || destinations.length === 0) {
    return 'At least one destination is required';
  }

  for (const dest of destinations) {
    if (!VALID_DESTINATIONS.includes(dest)) {
      return `Invalid destination '${dest}'. Must be one of: ${VALID_DESTINATIONS.join(', ')}`;
    }
  }

  if (enrichers) {
    for (const enricher of enrichers) {
      if (typeof enricher.providerType !== 'number') {
        return 'Invalid enricher: providerType must be a number';
      }
    }
  }

  return null;
}

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const { logger } = ctx;
  const userId = ctx.userId;

  if (!userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  const userStore = new UserStore(db);
  const activityStore = new ActivityStore(db);
  const userService = new UserService(userStore, activityStore);
  const path = req.path;
  const pipelineId = extractPipelineId(path);

  // --- GET /users/me/pipelines ---
  if (req.method === 'GET') {
    try {
      const user = await userService.get(userId);

      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      const pipelines = (user.pipelines || []).map(mapPipelineToResponse);
      res.status(200).json({ pipelines });
    } catch (e) {
      logger.error('Failed to get pipelines', { error: e, userId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // --- POST /users/me/pipelines ---
  if (req.method === 'POST' && !pipelineId) {
    try {
      const { source, destinations, enrichers } = req.body as {
        source: string;
        destinations: string[];
        enrichers?: { providerType: number; inputs?: Record<string, string> }[];
      };

      const validationError = validatePipelineConfig(source, destinations, enrichers);
      if (validationError) {
        res.status(400).json({ error: validationError });
        return;
      }

      // Convert enrichers to the expected format
      const enricherConfigs: EnricherConfig[] = (enrichers || []).map(e => ({
        providerType: e.providerType,
        inputs: e.inputs || {}
      }));

      // Use the service to add the pipeline
      const newPipelineId = await userService.addPipeline(userId, source, enricherConfigs, destinations);

      logger.info('Created pipeline', { userId, pipelineId: newPipelineId, source, destinations });

      // Return the created pipeline
      const newPipeline = {
        id: newPipelineId,
        source,
        enrichers: enricherConfigs.map(e => ({
          providerType: e.providerType,
          inputs: e.inputs
        })),
        destinations
      };

      res.status(201).json(newPipeline);
    } catch (e) {
      logger.error('Failed to create pipeline', { error: e, userId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // --- PATCH /users/me/pipelines/{pipelineId} ---
  if (req.method === 'PATCH' && pipelineId) {
    try {
      const user = await userService.get(userId);
      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      // Find existing pipeline
      const existingPipeline = (user.pipelines || []).find(p => p.id === pipelineId);
      if (!existingPipeline) {
        res.status(404).json({ error: 'Pipeline not found' });
        return;
      }

      const { destinations, enrichers } = req.body as {
        destinations?: string[];
        enrichers?: { providerType: number; inputs?: Record<string, string> }[];
      };

      // Merge with existing values
      const newDestinations = destinations || existingPipeline.destinations.map(d => DESTINATION_ENUM_MAP[d]);
      const newEnrichers = enrichers !== undefined ? enrichers : existingPipeline.enrichers;

      // Validate if destinations provided
      if (destinations) {
        for (const dest of destinations) {
          if (!VALID_DESTINATIONS.includes(dest)) {
            res.status(400).json({ error: `Invalid destination '${dest}'` });
            return;
          }
        }
      }

      // Convert enrichers
      const enricherConfigs: EnricherConfig[] = (newEnrichers || []).map(e => ({
        providerType: e.providerType,
        inputs: e.inputs || {}
      }));

      // Replace the pipeline
      await userService.replacePipeline(
        userId,
        pipelineId,
        existingPipeline.source,
        enricherConfigs,
        newDestinations
      );

      logger.info('Updated pipeline', { userId, pipelineId });

      // Return updated pipeline
      const updatedPipeline = {
        id: pipelineId,
        source: existingPipeline.source,
        enrichers: enricherConfigs.map(e => ({
          providerType: e.providerType,
          inputs: e.inputs
        })),
        destinations: newDestinations
      };

      res.status(200).json(updatedPipeline);
    } catch (e) {
      logger.error('Failed to update pipeline', { error: e, userId, pipelineId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // --- DELETE /users/me/pipelines/{pipelineId} ---
  if (req.method === 'DELETE' && pipelineId) {
    try {
      const user = await userService.get(userId);
      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      // Check if pipeline exists
      const existingPipeline = (user.pipelines || []).find(p => p.id === pipelineId);
      if (!existingPipeline) {
        res.status(404).json({ error: 'Pipeline not found' });
        return;
      }

      await userService.removePipeline(userId, pipelineId);
      logger.info('Deleted pipeline', { userId, pipelineId });

      res.status(200).json({ success: true });
    } catch (e) {
      logger.error('Failed to delete pipeline', { error: e, userId, pipelineId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  res.status(405).send('Method Not Allowed');
};

// Export the wrapped function
export const userPipelinesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  }
});
