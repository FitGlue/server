import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, db } from '@fitglue/shared';
import { Request, Response } from 'express';
import { SynchronizedActivity } from '@fitglue/shared/dist/types/pb/user';
import { ActivityType } from '@fitglue/shared/dist/types/pb/standardized_activity';
import { ActivitySource } from '@fitglue/shared/dist/types/pb/activity';
import { ExecutionStatus } from '@fitglue/shared/dist/types/pb/execution';

// Helper to convert ActivityType enum to readable string
const activityTypeToString = (type: number | undefined): string => {
  if (type === undefined) return 'Unknown';

  const typeMap: Record<number, string> = {
    [ActivityType.ACTIVITY_TYPE_UNSPECIFIED]: 'Unspecified',
    [ActivityType.ACTIVITY_TYPE_ALPINE_SKI]: 'Alpine Ski',
    [ActivityType.ACTIVITY_TYPE_BACKCOUNTRY_SKI]: 'Backcountry Ski',
    [ActivityType.ACTIVITY_TYPE_BADMINTON]: 'Badminton',
    [ActivityType.ACTIVITY_TYPE_CANOEING]: 'Canoeing',
    [ActivityType.ACTIVITY_TYPE_CROSSFIT]: 'Crossfit',
    [ActivityType.ACTIVITY_TYPE_EBIKE_RIDE]: 'E-Bike Ride',
    [ActivityType.ACTIVITY_TYPE_ELLIPTICAL]: 'Elliptical',
    [ActivityType.ACTIVITY_TYPE_EMOUNTAIN_BIKE_RIDE]: 'E-Mountain Bike Ride',
    [ActivityType.ACTIVITY_TYPE_GOLF]: 'Golf',
    [ActivityType.ACTIVITY_TYPE_GRAVEL_RIDE]: 'Gravel Ride',
    [ActivityType.ACTIVITY_TYPE_HANDCYCLE]: 'Handcycle',
    [ActivityType.ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING]: 'HIIT',
    [ActivityType.ACTIVITY_TYPE_HIKE]: 'Hike',
    [ActivityType.ACTIVITY_TYPE_ICE_SKATE]: 'Ice Skate',
    [ActivityType.ACTIVITY_TYPE_INLINE_SKATE]: 'Inline Skate',
    [ActivityType.ACTIVITY_TYPE_KAYAKING]: 'Kayaking',
    [ActivityType.ACTIVITY_TYPE_KITESURF]: 'Kitesurf',
    [ActivityType.ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE]: 'Mountain Bike Ride',
    [ActivityType.ACTIVITY_TYPE_NORDIC_SKI]: 'Nordic Ski',
    [ActivityType.ACTIVITY_TYPE_PICKLEBALL]: 'Pickleball',
    [ActivityType.ACTIVITY_TYPE_PILATES]: 'Pilates',
    [ActivityType.ACTIVITY_TYPE_RACQUETBALL]: 'Racquetball',
    [ActivityType.ACTIVITY_TYPE_RIDE]: 'Ride',
    [ActivityType.ACTIVITY_TYPE_ROCK_CLIMBING]: 'Rock Climbing',
    [ActivityType.ACTIVITY_TYPE_ROLLER_SKI]: 'Roller Ski',
    [ActivityType.ACTIVITY_TYPE_ROWING]: 'Rowing',
    [ActivityType.ACTIVITY_TYPE_RUN]: 'Run',
    [ActivityType.ACTIVITY_TYPE_SAIL]: 'Sail',
    [ActivityType.ACTIVITY_TYPE_SKATEBOARD]: 'Skateboard',
    [ActivityType.ACTIVITY_TYPE_SNOWBOARD]: 'Snowboard',
    [ActivityType.ACTIVITY_TYPE_SNOWSHOE]: 'Snowshoe',
    [ActivityType.ACTIVITY_TYPE_SOCCER]: 'Soccer',
    [ActivityType.ACTIVITY_TYPE_SQUASH]: 'Squash',
    [ActivityType.ACTIVITY_TYPE_STAIR_STEPPER]: 'Stair Stepper',
    [ActivityType.ACTIVITY_TYPE_STAND_UP_PADDLING]: 'Stand Up Paddling',
    [ActivityType.ACTIVITY_TYPE_SURFING]: 'Surfing',
    [ActivityType.ACTIVITY_TYPE_SWIM]: 'Swim',
    [ActivityType.ACTIVITY_TYPE_TABLE_TENNIS]: 'Table Tennis',
    [ActivityType.ACTIVITY_TYPE_TENNIS]: 'Tennis',
    [ActivityType.ACTIVITY_TYPE_TRAIL_RUN]: 'Trail Run',
    [ActivityType.ACTIVITY_TYPE_VELOMOBILE]: 'Velomobile',
    [ActivityType.ACTIVITY_TYPE_VIRTUAL_RIDE]: 'Virtual Ride',
    [ActivityType.ACTIVITY_TYPE_VIRTUAL_ROW]: 'Virtual Row',
    [ActivityType.ACTIVITY_TYPE_VIRTUAL_RUN]: 'Virtual Run',
    [ActivityType.ACTIVITY_TYPE_WALK]: 'Walk',
    [ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING]: 'Weight Training',
    [ActivityType.ACTIVITY_TYPE_WHEELCHAIR]: 'Wheelchair',
    [ActivityType.ACTIVITY_TYPE_WINDSURF]: 'Windsurf',
    [ActivityType.ACTIVITY_TYPE_WORKOUT]: 'Workout',
    [ActivityType.ACTIVITY_TYPE_YOGA]: 'Yoga',
  };

  return typeMap[type] || `Unknown (${type})`;
};

// Helper to convert ExecutionStatus enum to readable string
const executionStatusToString = (status: number | undefined): string => {
  if (status === undefined) return 'UNKNOWN';
  // ExecutionStatus is a numeric enum
  const name = ExecutionStatus[status];
  return name ? name.replace('STATUS_', '') : 'UNKNOWN';
};

// Helper to convert ActivitySource to readable string (dynamic lookup)
const activitySourceToString = (source: string | undefined): string => {
  if (!source) return 'Unknown';

  const sourceStr = source.toString().toUpperCase();

  // Handle SOURCE_X format: SOURCE_FILE_UPLOAD -> "File Upload"
  if (sourceStr.startsWith('SOURCE_')) {
    const name = sourceStr.replace('SOURCE_', '').replace(/_/g, ' ');
    // Title case: "FILE UPLOAD" -> "File Upload"
    return name.split(' ').map(w => w.charAt(0) + w.slice(1).toLowerCase()).join(' ');
  }

  // Fallback for numeric values - lookup in ActivitySource enum
  const numericValue = parseInt(sourceStr, 10);
  if (!isNaN(numericValue)) {
    const enumName = ActivitySource[numericValue as ActivitySource];
    if (enumName && enumName.startsWith('SOURCE_')) {
      const name = enumName.replace('SOURCE_', '').replace(/_/g, ' ');
      return name.split(' ').map(w => w.charAt(0) + w.slice(1).toLowerCase()).join(' ');
    }
  }

  return source;
};

// Transform activity to include readable enum strings
const transformActivity = (activity: SynchronizedActivity): Omit<SynchronizedActivity, 'type' | 'source'> & { type: string; source: string } => {
  return {
    ...activity,
    type: activityTypeToString(activity.type),
    source: activitySourceToString(activity.source),
  };
};

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const activityStore = ctx.stores.activities;

  // Auth check
  if (!ctx.userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  try {
    // GET /stats -> { synchronized_count: N }
    // Check if path ends with /stats (handling rewrites)
    if (req.path.endsWith('/stats') || req.query.mode === 'stats') {
      // PHASE 2 OPTIMIZATION: Try to use cached counters first (O(1))
      // Fallback to count() query if counters not available
      const userDoc = await db.collection('users').doc(ctx.userId).get();
      const userData = userDoc.data();
      const cachedCounts = userData?.activityCounts;

      if (cachedCounts?.weeklySync !== undefined) {
        // Check if weekly reset is needed
        const now = new Date();
        const day = now.getDay();
        const diff = now.getDate() - day + (day === 0 ? -6 : 1);
        const weekStart = new Date(now);
        weekStart.setDate(diff);
        weekStart.setHours(0, 0, 0, 0);

        const lastReset = cachedCounts.weeklyResetAt?.toDate?.() || cachedCounts.weeklyResetAt;
        if (lastReset && new Date(lastReset) < weekStart) {
          // Week has rolled over, reset weekly count
          // Return 0 and fire-and-forget update
          db.collection('users').doc(ctx.userId).update({
            'activityCounts.weeklySync': 0,
            'activityCounts.weeklyResetAt': weekStart,
          }).catch(() => { /* ignore */ });
          res.status(200).json({ synchronizedCount: 0 });
          return;
        }

        res.status(200).json({ synchronizedCount: cachedCounts.weeklySync });
        return;
      }

      // Fallback: Legacy count() query for users without cached counters
      const now = new Date();
      const day = now.getDay();
      const diff = now.getDate() - day + (day === 0 ? -6 : 1);
      const monday = new Date(now);
      monday.setDate(diff);
      monday.setHours(0, 0, 0, 0);

      const count = await activityStore.countSynchronized(ctx.userId, monday);
      res.status(200).json({ synchronizedCount: count });
      return;
    }

    // GET /unsynchronized/:pipelineExecutionId -> Get full trace for an unsynchronized pipeline
    if (req.path.includes('/unsynchronized/') && !req.path.endsWith('/unsynchronized')) {
      const pathParts = req.path.split('/').filter(s => s !== '');
      const pipelineExecutionId = pathParts[pathParts.length - 1];

      if (pipelineExecutionId) {
        try {
          const executions = await ctx.services.execution.listByPipeline(pipelineExecutionId);
          if (executions.length === 0) {
            res.status(404).json({ error: 'Pipeline execution not found' });
            return;
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

          res.status(200).json({ pipelineExecution, pipelineExecutionId });
          return;
        } catch (err) {
          ctx.logger.error('Failed to fetch pipeline trace', { error: err });
          res.status(500).json({ error: 'Internal Server Error' });
          return;
        }
      }
    }

    // GET /unsynchronized -> List pipeline executions without matching synchronized activities
    if (req.path.endsWith('/unsynchronized')) {
      const limit = parseInt(req.query.limit as string || '20', 10);

      // OPTIMIZED: Use lightweight pipeline ID query (reduced from 3x to 2x multiplier)
      // and projection queries reduce data transfer by ~90%
      const allPipelines = await ctx.stores.executions.listDistinctPipelines(ctx.userId, limit * 2);

      // Get synchronized pipeline IDs (already uses projection query)
      const syncedPipelineIds = await activityStore.getSynchronizedPipelineIds(ctx.userId);

      // Filter to unsynchronized (absence-based) AND not successful (to avoid historical false positives)
      const unsyncedPipelines = allPipelines.filter(
        p => p.data.pipelineExecutionId &&
          !syncedPipelineIds.has(p.data.pipelineExecutionId) &&
          p.data.status !== ExecutionStatus.STATUS_SUCCESS
      ).slice(0, limit);

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

      res.status(200).json({ executions });
      return;
    }

    // GET /:id -> Single activity
    // Check if this is a list request (path ends with /activities or is just /)
    if (req.path === '' || req.path === '/' || req.path === '/activities' || req.path.endsWith('/activities')) {
      // GET / -> List
      const includeExecution = req.query.includeExecution === 'true';
      // Limit to 10 when fetching executions for performance (more Firestore reads)
      const limit = includeExecution
        ? Math.min(parseInt(req.query.limit as string || '10', 10), 10)
        : parseInt(req.query.limit as string || '20', 10);

      const activities = await activityStore.listSynchronized(ctx.userId, limit);
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

      res.status(200).json({ activities: transformedActivities });
      return;
    }

    // Otherwise, extract ID from path
    // Path could be /activities/{id} or just /{id}
    const pathParts = req.path.split('/').filter(s => s !== '');
    const id = pathParts[pathParts.length - 1]; // Last segment is the ID

    if (id && id !== 'stats') {
      const activity = await activityStore.getSynchronized(ctx.userId, id);
      if (!activity) {
        res.status(404).json({ error: 'Not found' });
        return;
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

      res.status(200).json({ activity: transformed });
      return;
    }

    // Fallback to list if we somehow got here
    const limit = parseInt(req.query.limit as string || '20', 10);
    const activities = await activityStore.listSynchronized(ctx.userId, limit);
    const transformedActivities = activities.map(transformActivity);
    res.status(200).json({ activities: transformedActivities });

  } catch (e) {
    ctx.logger.error('Failed to handle activities request', { error: e });
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

export const activitiesHandler = createCloudFunction(handler, {
  auth: {
    strategies: [
      new FirebaseAuthStrategy()
    ]
  },
  skipExecutionLogging: true
});
