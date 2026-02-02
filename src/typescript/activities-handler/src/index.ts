// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, FrameworkContext } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch, RouteHandler } from '@fitglue/shared/routing';
import { ExecutionStatus, formatExecutionStatus, formatActivityType, formatActivitySource, SynchronizedActivity } from '@fitglue/shared/types';

// Helpers are now using generated formatters
const activityTypeToString = (type: number | string | undefined | null) => formatActivityType(type);
const executionStatusToString = (status: number | string | undefined | null) => formatExecutionStatus(status);
const activitySourceToString = (source: number | string | undefined | null) => formatActivitySource(source);

// Transform activity to include readable enum strings
const transformActivity = (activity: SynchronizedActivity): Omit<SynchronizedActivity, 'type' | 'source'> & { type: string; source: string } => {
  return {
    ...activity,
    type: activityTypeToString(activity.type),
    source: activitySourceToString(activity.source),
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
  const activityStore = ctx.stores.activities;
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

  // Fetch all stats in parallel
  const [totalSynced, monthlySynced, weeklySynced] = await Promise.all([
    activityStore.countSynchronized(userId),
    activityStore.countSynchronized(userId, startOfMonth),
    activityStore.countSynchronized(userId, startOfWeek),
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
  const activityStore = ctx.stores.activities;
  const limit = parseInt(query.limit as string || '20', 10);
  const offset = parseInt(query.offset as string || '0', 10);

  // OPTIMIZED: Use lightweight pipeline ID query (reduced from 3x to 2x multiplier)
  // and projection queries reduce data transfer by ~90%
  // Note: We fetch more than limit + offset to account for deduplication
  const allPipelines = await ctx.stores.executions.listDistinctPipelines(userId, (limit + offset) * 2);

  // Get synchronized pipeline IDs (already uses projection query)
  const syncedPipelineIds = await activityStore.getSynchronizedPipelineIds(userId);

  // Filter to unsynchronized (absence-based) AND not successful (to avoid historical false positives)
  const unsyncedPipelines = allPipelines.filter(
    p => p.data.pipelineExecutionId &&
      !syncedPipelineIds.has(p.data.pipelineExecutionId) &&
      p.data.status !== ExecutionStatus.STATUS_SUCCESS
  ).slice(offset, offset + limit);

  // Synthesize meaningful entries from inputsJson
  const executions = unsyncedPipelines.map(e => {
    let title: string | undefined;
    let activityType: string | undefined;
    let source: string | undefined;

    // Try to extract activity info from inputsJson
    if (e.data.inputsJson) {
      try {
        const inputs = JSON.parse(e.data.inputsJson);
        // Activity data could be nested in various ways
        const activity = inputs.activity || inputs.standardizedActivity || inputs;
        title = activity.title || activity.name;
        if (activity.type !== undefined) {
          activityType = activityTypeToString(activity.type);
        }
        source = activity.source ? activitySourceToString(activity.source) : undefined;
      } catch {
        // Ignore parse errors
      }
    }

    return {
      pipelineExecutionId: e.data.pipelineExecutionId,
      title: title || 'Unknown Activity',
      activityType: activityType || 'Unknown',
      source: source || 'Unknown',
      status: executionStatusToString(e.data.status),
      errorMessage: e.data.errorMessage,
      timestamp: e.data.timestamp ? new Date(e.data.timestamp as unknown as string).toISOString() : null
    };
  });

  return { executions };
}

async function handleGetActivity(userId: string, id: string, ctx: FrameworkContext) {
  const activityStore = ctx.stores.activities;
  const activity = await activityStore.getSynchronized(userId, id);
  if (!activity) {
    throw new HttpError(404, 'Not found');
  }

  const transformed = transformActivity(activity);

  // Fetch execution trace if pipelineExecutionId is present
  if (activity.pipelineExecutionId) {
    try {
      const executions = await ctx.services.execution.listByPipeline(activity.pipelineExecutionId);
      // Map to swagger schema
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (transformed as any).pipelineExecution = executions.map(e => ({
        executionId: e.id,
        service: e.data.service,
        status: executionStatusToString(e.data.status),
        timestamp: e.data.timestamp ? new Date(e.data.timestamp as unknown as string).toISOString() : null, // Handle Firestore/Proto timestamp quirks if needed, usually Date object in JS SDK
        startTime: e.data.startTime ? new Date(e.data.startTime as unknown as string).toISOString() : null,
        endTime: e.data.endTime ? new Date(e.data.endTime as unknown as string).toISOString() : null,
        errorMessage: e.data.errorMessage,
        triggerType: e.data.triggerType,
        inputsJson: e.data.inputsJson,
        outputsJson: e.data.outputsJson
      }));
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (transformed as any).pipelineExecutionId = activity.pipelineExecutionId;
    } catch (err) {
      ctx.logger.error('Failed to fetch pipeline executions', { error: err });
      // Don't fail the request, just omit the trace
    }
  }

  return { activity: transformed };
}

async function handleListActivities(userId: string, query: Record<string, unknown>, ctx: FrameworkContext) {
  const activityStore = ctx.stores.activities;
  const includeExecution = query.includeExecution === 'true';
  const offset = parseInt(query.offset as string || '0', 10);

  // Limit to 50 when fetching executions for performance (more Firestore reads)
  const limit = includeExecution
    ? Math.min(parseInt(query.limit as string || '20', 10), 50)
    : parseInt(query.limit as string || '20', 10);

  const activities = await activityStore.listSynchronized(userId, limit, offset);
  const transformedActivities = activities.map(transformActivity);

  // OPTIMIZED: Batch load all pipeline executions in ONE query instead of N+1 queries
  // This eliminates the biggest performance bottleneck in the dashboard
  if (includeExecution && ctx.stores.executions) {
    // Collect all pipeline IDs that have execution data
    const pipelineIds = activities
      .filter(a => a.pipelineExecutionId)
      .map(a => a.pipelineExecutionId as string);

    if (pipelineIds.length > 0) {
      // Single batch query instead of N individual queries
      const executionMap = await ctx.stores.executions.batchListByPipelines(pipelineIds);

      // Attach execution traces to each activity
      for (const activity of transformedActivities) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const originalActivity = activities.find(a => a.activityId === (activity as any).activityId);
        if (!originalActivity?.pipelineExecutionId) continue;

        const executions = executionMap.get(originalActivity.pipelineExecutionId);
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
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (activity as any).pipelineExecutionId = originalActivity.pipelineExecutionId;
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
