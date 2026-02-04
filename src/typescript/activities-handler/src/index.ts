// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, FrameworkContext } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch, RouteHandler } from '@fitglue/shared/routing';
import { formatExecutionStatus, formatActivityType, formatActivitySource, PipelineRun, PipelineRunStatus, formatPipelineRunStatus } from '@fitglue/shared/types';

// Helpers are now using generated formatters
const activityTypeToString = (type: number | string | undefined | null) => formatActivityType(type);
const executionStatusToString = (status: number | string | undefined | null) => formatExecutionStatus(status);
const activitySourceToString = (source: number | string | undefined | null) => formatActivitySource(source);

// Activity response type for API
interface ActivityResponse {
  activityId: string;
  title: string;
  description: string;
  type: string;
  source: string;
  startTime?: Date;
  destinations: { [key: string]: string };
  syncedAt?: Date;
  pipelineId: string;
  pipelineExecutionId: string;
}

// Transform PipelineRun to activity response format
const transformPipelineRun = (run: PipelineRun): ActivityResponse => {
  // Convert destinations array to map format (dest enum -> externalId)
  const destinationsMap: { [key: string]: string } = {};
  for (const dest of run.destinations || []) {
    if (dest.externalId) {
      // Use the destination name as key (e.g., "DESTINATION_STRAVA")
      destinationsMap[String(dest.destination)] = dest.externalId;
    }
  }

  return {
    activityId: run.activityId,
    title: run.title,
    description: run.description,
    type: activityTypeToString(run.type),
    source: activitySourceToString(run.source),
    startTime: run.startTime,
    destinations: destinationsMap,
    syncedAt: run.updatedAt, // Use updatedAt as syncedAt
    pipelineId: run.pipelineId,
    pipelineExecutionId: run.id,
  };
};

export const handler: FrameworkHandler = async (req, ctx) => {
  // Auth check
  if (!ctx.userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  // Store userId after auth check for type narrowing
  const userId = ctx.userId;

  return await routeRequest(req, ctx, [
    {
      method: 'GET',
      pattern: '/api/activities/stats',
      handler: async () => await handleStats(userId, ctx)
    },
    {
      method: 'GET',
      pattern: '/api/activities/unsynchronized/:pipelineExecutionId',
      handler: async (match: RouteMatch) => await handleUnsynchronizedTrace(match.params.pipelineExecutionId, ctx)
    },
    {
      method: 'GET',
      pattern: '/api/activities/unsynchronized',
      handler: async () => await handleListUnsynchronized(userId, req.query, ctx)
    },
    {
      method: 'GET',
      pattern: '/api/activities/:id',
      handler: async (match: RouteMatch) => await handleGetActivity(userId, match.params.id, ctx)
    },
    {
      method: 'GET',
      pattern: '/api/activities',
      handler: async () => await handleListActivities(userId, req.query, ctx)
    }
  ] as RouteHandler[]);
};

async function handleStats(userId: string, ctx: FrameworkContext) {
  const pipelineRuns = ctx.stores.pipelineRuns;
  const now = new Date();
  // Start of month
  const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
  startOfMonth.setHours(0, 0, 0, 0);

  // Start of week (Monday)
  const day = now.getDay();
  const diff = now.getDate() - day + (day === 0 ? -6 : 1);
  const startOfWeek = new Date(now);
  startOfWeek.setDate(diff);
  startOfWeek.setHours(0, 0, 0, 0);

  // Fetch all stats in parallel using pipeline_runs
  const [totalSynced, monthlySynced, weeklySynced] = await Promise.all([
    pipelineRuns.countSynced(userId),
    pipelineRuns.countSynced(userId, startOfMonth),
    pipelineRuns.countSynced(userId, startOfWeek),
  ]);

  return {
    synchronizedCount: totalSynced, // Backward compatibility
    totalSynced,
    monthlySynced,
    weeklySynced,
  };
}

async function handleUnsynchronizedTrace(pipelineExecutionId: string, ctx: FrameworkContext) {
  const executions = await ctx.services.execution.listByPipeline(pipelineExecutionId);
  if (executions.length === 0) {
    throw new HttpError(404, 'Pipeline execution not found');
  }

  // Map to swagger schema
  const pipelineExecution = executions.map(e => ({
    executionId: e.id,
    service: e.data.service,
    status: executionStatusToString(e.data.status),
    timestamp: e.data.timestamp ? new Date(e.data.timestamp as unknown as string).toISOString() : null,
    startTime: e.data.startTime ? new Date(e.data.startTime as unknown as string).toISOString() : null,
    endTime: e.data.endTime ? new Date(e.data.endTime as unknown as string).toISOString() : null,
    errorMessage: e.data.errorMessage,
    triggerType: e.data.triggerType,
    inputsJson: e.data.inputsJson,
    outputsJson: e.data.outputsJson
  }));

  return { pipelineExecution, pipelineExecutionId };
}

async function handleListUnsynchronized(userId: string, query: Record<string, unknown>, ctx: FrameworkContext) {
  const pipelineRuns = ctx.stores.pipelineRuns;
  const limit = parseInt(query.limit as string || '20', 10);
  const offset = parseInt(query.offset as string || '0', 10);

  // Query pipeline runs that are failed or have failed destinations
  // These are "unsynchronized" - they didn't complete successfully
  const allRuns = await pipelineRuns.list(userId, { limit: (limit + offset) * 2 });

  // Filter to failed runs (status indicates failure or has failed destinations)
  // DESTINATION_STATUS_FAILED = 4, PIPELINE_RUN_STATUS_FAILED = 4
  const failedRuns = allRuns.filter(run =>
    run.status === PipelineRunStatus.PIPELINE_RUN_STATUS_FAILED ||
    run.destinations?.some(d => (d.status as number) === 4) // 4 = DESTINATION_STATUS_FAILED
  ).slice(offset, offset + limit);

  // Map to execution format for backwards compatibility with web UI
  const executions = failedRuns.map(run => ({
    pipelineExecutionId: run.id,
    title: run.title || 'Unknown Activity',
    activityType: activityTypeToString(run.type || 0),
    source: run.source || 'Unknown',
    status: formatPipelineRunStatus(run.status),
    errorMessage: run.statusMessage,
    timestamp: run.createdAt ? new Date(run.createdAt as unknown as string).toISOString() : null
  }));

  return { executions };
}

async function handleGetActivity(userId: string, id: string, ctx: FrameworkContext) {
  const pipelineRuns = ctx.stores.pipelineRuns;

  // Try to find by activity ID first (most common case)
  const run = await pipelineRuns.findByActivityId(userId, id);
  if (!run) {
    throw new HttpError(404, 'Not found');
  }

  const transformed = transformPipelineRun(run);

  // Fetch execution trace if we have a run ID
  try {
    const executions = await ctx.services.execution.listByPipeline(run.id);
    // Map to swagger schema
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (transformed as any).pipelineExecution = executions.map(e => ({
      executionId: e.id,
      service: e.data.service,
      status: executionStatusToString(e.data.status),
      timestamp: e.data.timestamp ? new Date(e.data.timestamp as unknown as string).toISOString() : null,
      startTime: e.data.startTime ? new Date(e.data.startTime as unknown as string).toISOString() : null,
      endTime: e.data.endTime ? new Date(e.data.endTime as unknown as string).toISOString() : null,
      errorMessage: e.data.errorMessage,
      triggerType: e.data.triggerType,
      inputsJson: e.data.inputsJson,
      outputsJson: e.data.outputsJson
    }));
  } catch (err) {
    ctx.logger.error('Failed to fetch pipeline executions', { error: err });
    // Don't fail the request, just omit the trace
  }

  return { activity: transformed };
}

async function handleListActivities(userId: string, query: Record<string, unknown>, ctx: FrameworkContext) {
  const pipelineRuns = ctx.stores.pipelineRuns;
  const includeExecution = query.includeExecution === 'true';
  const offset = parseInt(query.offset as string || '0', 10);

  // Limit to 50 when fetching executions for performance (more Firestore reads)
  const limit = includeExecution
    ? Math.min(parseInt(query.limit as string || '20', 10), 50)
    : parseInt(query.limit as string || '20', 10);

  const runs = await pipelineRuns.listSynced(userId, limit, offset);
  const transformedActivities = runs.map(transformPipelineRun);

  // OPTIMIZED: Batch load all pipeline executions in ONE query instead of N+1 queries
  // This eliminates the biggest performance bottleneck in the dashboard
  if (includeExecution && ctx.stores.executions) {
    // Collect all pipeline IDs (run.id is the pipelineExecutionId)
    const pipelineIds = runs.map(r => r.id);

    if (pipelineIds.length > 0) {
      // Single batch query instead of N individual queries
      const executionMap = await ctx.stores.executions.batchListByPipelines(pipelineIds);

      // Attach execution traces to each activity
      for (let i = 0; i < transformedActivities.length; i++) {
        const run = runs[i];
        const activity = transformedActivities[i];

        const executions = executionMap.get(run.id);
        if (!executions) continue;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (activity as any).pipelineExecution = executions.map(e => ({
          executionId: e.id,
          service: e.data.service,
          status: executionStatusToString(e.data.status),
          timestamp: e.data.timestamp ? new Date(e.data.timestamp as unknown as string).toISOString() : null,
          startTime: e.data.startTime ? new Date(e.data.startTime as unknown as string).toISOString() : null,
          endTime: e.data.endTime ? new Date(e.data.endTime as unknown as string).toISOString() : null,
          errorMessage: e.data.errorMessage,
          triggerType: e.data.triggerType,
          inputsJson: e.data.inputsJson,
          outputsJson: e.data.outputsJson
        }));
      }
    }
  }

  return { activities: transformedActivities };
}

export const activitiesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  },
  skipExecutionLogging: true
});
