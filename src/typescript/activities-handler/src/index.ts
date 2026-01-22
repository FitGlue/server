import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, HttpError, ExecutionStatus, formatExecutionStatus, formatActivityType, formatActivitySource } from '@fitglue/shared';
import { Request } from 'express';
import { SynchronizedActivity } from '@fitglue/shared/dist/types/pb/user';

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

export const handler = async (req: Request, ctx: FrameworkContext) => {
  const activityStore = ctx.stores.activities;

  // Auth check
  if (!ctx.userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  // GET /stats -> { synchronized_count: N }
  // Check if path ends with /stats (handling rewrites)
  if (req.path.endsWith('/stats') || req.query.mode === 'stats') {
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
      activityStore.countSynchronized(ctx.userId),
      activityStore.countSynchronized(ctx.userId, startOfMonth),
      activityStore.countSynchronized(ctx.userId, startOfWeek),
    ]);

    return {
      synchronizedCount: totalSynced, // Backward compatibility
      totalSynced,
      monthlySynced,
      weeklySynced,
    };
  }

  // GET /unsynchronized/:pipelineExecutionId -> Get full trace for an unsynchronized pipeline
  if (req.path.includes('/unsynchronized/') && !req.path.endsWith('/unsynchronized')) {
    const pathParts = req.path.split('/').filter(s => s !== '');
    const pipelineExecutionId = pathParts[pathParts.length - 1];

    if (pipelineExecutionId) {
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
  }

  // GET /unsynchronized -> List pipeline executions without matching synchronized activities
  if (req.path.endsWith('/unsynchronized')) {
    const limit = parseInt(req.query.limit as string || '20', 10);
    const offset = parseInt(req.query.offset as string || '0', 10);

    // OPTIMIZED: Use lightweight pipeline ID query (reduced from 3x to 2x multiplier)
    // and projection queries reduce data transfer by ~90%
    // Note: We fetch more than limit + offset to account for deduplication
    const allPipelines = await ctx.stores.executions.listDistinctPipelines(ctx.userId, (limit + offset) * 2);

    // Get synchronized pipeline IDs (already uses projection query)
    const syncedPipelineIds = await activityStore.getSynchronizedPipelineIds(ctx.userId);

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

  // GET /:id -> Single activity
  // Check if this is a list request (path ends with /activities or is just /)
  if (req.path === '' || req.path === '/' || req.path === '/activities' || req.path.endsWith('/activities')) {
    // GET / -> List
    const includeExecution = req.query.includeExecution === 'true';
    const offset = parseInt(req.query.offset as string || '0', 10);

    // Limit to 50 when fetching executions for performance (more Firestore reads)
    const limit = includeExecution
      ? Math.min(parseInt(req.query.limit as string || '20', 10), 50)
      : parseInt(req.query.limit as string || '20', 10);

    const activities = await activityStore.listSynchronized(ctx.userId, limit, offset);
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
          if (originalActivity?.pipelineExecutionId) {
            const executions = executionMap.get(originalActivity.pipelineExecutionId);
            if (executions) {
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
      }
    }

    return { activities: transformedActivities };
  }

  // Otherwise, extract ID from path
  // Path could be /activities/{id} or just /{id}
  const pathParts = req.path.split('/').filter(s => s !== '');
  const id = pathParts[pathParts.length - 1]; // Last segment is the ID

  if (id && id !== 'stats') {
    const activity = await activityStore.getSynchronized(ctx.userId, id);
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

  // Fallback to list if we somehow got here
  const limit = parseInt(req.query.limit as string || '20', 10);
  const activities = await activityStore.listSynchronized(ctx.userId, limit);
  const transformedActivities = activities.map(transformActivity);
  return { activities: transformedActivities };
};

export const activitiesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  },
  skipExecutionLogging: true
});
