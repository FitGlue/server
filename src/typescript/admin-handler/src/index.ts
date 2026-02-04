// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkContext, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError, ForbiddenError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch } from '@fitglue/shared/routing';
import { userConverter } from '@fitglue/shared/storage';
import {
  UserTier,
  UserRecord,
  ExecutionStatus,
  formatExecutionStatus,
  formatDestination,
  formatActivitySource,
  formatPipelineRunStatus,
  PendingInput_Status,
} from '@fitglue/shared/types';
import * as admin from 'firebase-admin';

/**
 * Admin Handler - Consolidated admin operations
 *
 * Endpoints:
 * - GET  /api/admin/stats                              - Platform-wide statistics
 * - GET  /api/admin/users                              - Enhanced user list
 * - GET  /api/admin/users/:id                          - Full user details
 * - PATCH /api/admin/users/:id                         - Update tier/admin/trial
 * - DELETE /api/admin/users/:id/integrations/:provider - Remove integration
 * - PATCH /api/admin/users/:id/pipelines/:pipelineId   - Toggle pipeline disabled state
 * - DELETE /api/admin/users/:id/pipelines/:pipelineId  - Remove pipeline
 * - DELETE /api/admin/users/:id/activities             - Delete synchronized activities
 * - DELETE /api/admin/users/:id/pending-inputs         - Delete pending inputs
 * - GET  /api/admin/executions                         - Query executions with filters
 * - GET  /api/admin/executions/:id                     - Get execution details
 */

// Helper to mask sensitive tokens
function maskToken(token: string | undefined): string | undefined {
  if (!token) return undefined;
  if (token.length <= 8) return '****';
  return token.substring(0, 4) + '****' + token.substring(token.length - 4);
}

// Helper to convert ExecutionStatus enum to readable string
const executionStatusToString = (status: number | string | undefined | null): string => formatExecutionStatus(status);

// ========================================
// Route Handlers
// ========================================

async function handleGetStats(ctx: FrameworkContext) {
  const { stores } = ctx;

  const usersSnapshot = await db.collection('users').withConverter(userConverter).get();
  const users = usersSnapshot.docs.map(doc => doc.data());

  const totalUsers = users.length;
  const proUsersCount = users.filter((u: UserRecord) => u.tier === UserTier.USER_TIER_ATHLETE).length;
  const adminUsersCount = users.filter((u: UserRecord) => u.isAdmin === true).length;
  const totalSyncsThisMonth = users.reduce((sum: number, u: UserRecord) => sum + (u.syncCountThisMonth || 0), 0);

  // Get recent execution stats (last 100)
  const recentExecs = await stores.executions.listRecent(100);
  const execStats = {
    success: recentExecs.filter(e => e.data.status === ExecutionStatus.STATUS_SUCCESS).length,
    failed: recentExecs.filter(e => e.data.status === ExecutionStatus.STATUS_FAILED).length,
    started: recentExecs.filter(e => e.data.status === ExecutionStatus.STATUS_STARTED).length,
  };

  return {
    totalUsers,
    athleteUsers: proUsersCount,
    adminUsers: adminUsersCount,
    totalSyncsThisMonth,
    recentExecutions: execStats
  };
}

async function handleListUsers(match: RouteMatch) {
  const page = parseInt(match.query.page as string || '1', 10);
  const limit = Math.min(parseInt(match.query.limit as string || '25', 10), 100);
  const offset = (page - 1) * limit;

  // Get total count
  const totalSnapshot = await db.collection('users').count().get();
  const total = totalSnapshot.data().count;

  // Get paginated users (simple query without orderBy to avoid index)
  const snapshot = await db.collection('users')
    .withConverter(userConverter)
    .limit(limit)
    .offset(offset)
    .get();

  const users = snapshot.docs.map(doc => {
    const data = doc.data();

    // Iterate through all integrations dynamically
    const integrations: string[] = [];
    if (data.integrations && typeof data.integrations === 'object') {
      for (const [provider, config] of Object.entries(data.integrations)) {
        const cfg = config as Record<string, unknown>;
        if (cfg?.enabled || cfg?.apiKey) {
          integrations.push(provider);
        }
      }
    }

    return {
      userId: doc.id,
      createdAt: data.createdAt?.toISOString() || data.createdAt,
      tier: data.tier || 'hobbyist',
      trialEndsAt: data.trialEndsAt?.toISOString() || data.trialEndsAt,
      isAdmin: data.isAdmin || false,
      accessEnabled: data.accessEnabled || false,
      syncCountThisMonth: data.syncCountThisMonth || 0,
      stripeCustomerId: data.stripeCustomerId || null,
      preventedSyncCount: data.preventedSyncCount || 0,
      integrations,
    };
  });

  return {
    data: users,
    pagination: {
      page,
      limit,
      total,
      hasMore: offset + users.length < total
    }
  };
}

// Helper: Fetch pending inputs for a user
type PendingInputInfo = { activityId: string; status: string; enricherProviderId?: string; createdAt?: string };

async function fetchPendingInputs(targetUserId: string, logger: FrameworkContext['logger']): Promise<PendingInputInfo[]> {
  try {
    // Use user sub-collection for pending inputs
    const snapshot = await db.collection('users').doc(targetUserId).collection('pending_inputs')
      .get();

    return snapshot.docs
      .map(doc => {
        const data = doc.data();
        return {
          activityId: data.activity_id || doc.id,
          status: data.status,
          enricherProviderId: data.enricher_provider_id,
          createdAt: data.created_at?.toDate?.()?.toISOString() || data.created_at,
        };
      })
      .filter(pi => pi.status !== PendingInput_Status.STATUS_COMPLETED)
      .map(pi => ({
        ...pi,
        status: pi.status === PendingInput_Status.STATUS_WAITING ? 'waiting' : 'unspecified',
      }));
  } catch (pendingErr) {
    logger.warn('Failed to fetch pending inputs', { targetUserId, error: pendingErr });
    return [];
  }
}

// Helper: Fetch Firebase Auth user info
async function fetchAuthUserInfo(targetUserId: string, logger: FrameworkContext['logger']): Promise<{ email?: string; displayName?: string }> {
  try {
    const authUser = await admin.auth().getUser(targetUserId);
    return { email: authUser.email, displayName: authUser.displayName };
  } catch (authErr) {
    logger.debug('Could not fetch Firebase Auth user', { targetUserId, error: authErr });
    return {};
  }
}

// Helper: Build integrations with masked tokens
function buildMaskedIntegrations(integrations: Record<string, unknown> | undefined): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  if (!integrations || typeof integrations !== 'object') return result;

  for (const [provider, config] of Object.entries(integrations)) {
    const cfg = config as Record<string, unknown>;
    if (!cfg?.enabled && !cfg?.apiKey) continue;

    const info: Record<string, unknown> = { enabled: cfg.enabled || !!cfg.apiKey };
    if (cfg.apiKey) info.apiKey = maskToken(cfg.apiKey as string);
    if (cfg.athleteId) info.athleteId = cfg.athleteId;
    if (cfg.fitbitUserId) info.fitbitUserId = cfg.fitbitUserId;
    if (cfg.lastUsedAt) {
      const lastUsed = cfg.lastUsedAt as Date | string;
      info.lastUsedAt = typeof lastUsed === 'object' && 'toISOString' in lastUsed
        ? lastUsed.toISOString()
        : lastUsed;
    }
    result[provider] = info;
  }
  return result;
}

async function handleGetUserDetails(match: RouteMatch, ctx: FrameworkContext) {
  const { logger, services, stores } = ctx;
  const targetUserId = match.params.id;

  const user = await services.user.get(targetUserId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }

  const [pipelineRunCount, pendingInputs, authInfo] = await Promise.all([
    stores.pipelineRuns.list(targetUserId, { limit: 0 }).then(runs => runs.length),
    fetchPendingInputs(targetUserId, logger),
    fetchAuthUserInfo(targetUserId, logger),
  ]);

  const integrations = buildMaskedIntegrations(user.integrations as Record<string, unknown>);

  // Query pipelines from sub-collection
  const pipelines = (await services.user.pipelineStore.list(targetUserId)).map(p => ({
    id: p.id,
    name: p.name || 'Unnamed Pipeline',
    source: p.source,
    destinations: (p.destinations || []).map(d => formatDestination(d)),
    disabled: p.disabled || false,
  }));

  return {
    userId: user.userId,
    email: authInfo.email,
    displayName: authInfo.displayName,
    createdAt: user.createdAt?.toISOString?.() || user.createdAt,
    tier: user.tier || 'hobbyist',
    trialEndsAt: user.trialEndsAt?.toISOString?.() || user.trialEndsAt,
    isAdmin: user.isAdmin || false,
    accessEnabled: user.accessEnabled || false,
    syncCountThisMonth: user.syncCountThisMonth || 0,
    preventedSyncCount: user.preventedSyncCount || 0,
    syncCountResetAt: user.syncCountResetAt?.toISOString?.() || user.syncCountResetAt,
    stripeCustomerId: user.stripeCustomerId || null,
    integrations,
    pipelines,
    activityCount: pipelineRunCount,
    pendingInputCount: pendingInputs.length,
    pendingInputs,
  };
}

async function handleUpdateUser(match: RouteMatch, req: { body: Record<string, unknown> }, adminUserId: string, ctx: FrameworkContext) {
  const { logger } = ctx;
  const targetUserId = match.params.id;
  const { tier, isAdmin, trialEndsAt, syncCountThisMonth, accessEnabled } = req.body;

  const updates: Record<string, unknown> = {};
  if (tier !== undefined) updates.tier = tier;
  if (isAdmin !== undefined) updates.is_admin = isAdmin;
  if (accessEnabled !== undefined) updates.access_enabled = accessEnabled;
  if (trialEndsAt !== undefined) {
    updates.trial_ends_at = trialEndsAt ? new Date(trialEndsAt as string) : null;
  }
  if (syncCountThisMonth !== undefined) updates.sync_count_this_month = syncCountThisMonth;

  if (Object.keys(updates).length > 0) {
    await db.collection('users').doc(targetUserId).update(updates);
    logger.info('Admin updated user', { adminUserId, targetUserId, updates: Object.keys(updates) });
  }
  return { success: true };
}

async function handleDeleteIntegration(match: RouteMatch, adminUserId: string, ctx: FrameworkContext) {
  const { logger } = ctx;
  const targetUserId = match.params.id;
  const provider = match.params.provider;

  // Clear the integration data
  await db.collection('users').doc(targetUserId).update({
    [`integrations.${provider}`]: null
  });
  logger.info('Admin removed integration', { adminUserId, targetUserId, provider });
  return { success: true };
}

async function handleUpdatePipeline(match: RouteMatch, req: { body: { disabled?: boolean } }, adminUserId: string, ctx: FrameworkContext) {
  const { logger, services } = ctx;
  const targetUserId = match.params.id;
  const pipelineId = match.params.pipelineId;
  const { disabled } = req.body;

  if (disabled === undefined) {
    throw new HttpError(400, 'Missing disabled field in request body');
  }

  // Use PipelineStore to toggle disabled state
  await services.user.pipelineStore.toggleDisabled(targetUserId, pipelineId, disabled);

  logger.info('Admin toggled pipeline disabled state', {
    adminUserId,
    targetUserId,
    pipelineId,
    disabled
  });
  return { success: true };
}

async function handleDeletePipeline(match: RouteMatch, adminUserId: string, ctx: FrameworkContext) {
  const { logger, services } = ctx;
  const targetUserId = match.params.id;
  const pipelineId = match.params.pipelineId;

  await services.user.pipelineStore.delete(targetUserId, pipelineId);
  logger.info('Admin removed pipeline', { adminUserId, targetUserId, pipelineId });
  return { success: true };
}

async function handleDeleteActivities(match: RouteMatch, adminUserId: string, ctx: FrameworkContext) {
  const { logger } = ctx;
  const targetUserId = match.params.id;

  // Delete pipeline runs
  const batchSize = 50;
  let deletedCount = 0;
  const collectionRef = db.collection('users').doc(targetUserId).collection('pipeline_runs');

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const snapshot = await collectionRef.limit(batchSize).get();
    if (snapshot.empty) break;

    const batch = db.batch();
    snapshot.docs.forEach(doc => batch.delete(doc.ref));
    await batch.commit();
    deletedCount += snapshot.size;
  }

  logger.info('Admin deleted activities', { adminUserId, targetUserId, deletedCount });
  return { success: true, deletedCount };
}

async function handleDeletePendingInputs(match: RouteMatch, adminUserId: string, ctx: FrameworkContext) {
  const { logger } = ctx;
  const targetUserId = match.params.id;

  const batchSize = 50;
  let deletedCount = 0;
  // Use user sub-collection for pending inputs
  const collectionRef = db.collection('users').doc(targetUserId).collection('pending_inputs');

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const snapshot = await collectionRef
      .limit(batchSize)
      .get();
    if (snapshot.empty) break;

    const batch = db.batch();
    snapshot.docs.forEach(doc => batch.delete(doc.ref));
    await batch.commit();
    deletedCount += snapshot.size;
  }

  logger.info('Admin deleted pending inputs', { adminUserId, targetUserId, deletedCount });
  return { success: true, deletedCount };
}

async function handleListExecutions(match: RouteMatch, ctx: FrameworkContext) {
  const { services, stores } = ctx;

  const service = match.query.service as string | undefined;
  const status = match.query.status as string | undefined;
  const targetUser = match.query.userId as string | undefined;
  const limit = parseInt(match.query.limit as string || '50', 10);

  const executions = await services.execution.listExecutions({
    service,
    status,
    userId: targetUser,
    limit: Math.min(limit, 200)
  });

  // Get distinct services for dropdown population
  const servicesList = await stores.executions.listDistinctServices?.() || [];

  const mapped = executions.map(e => ({
    id: e.id,
    service: e.data.service,
    status: executionStatusToString(e.data.status),
    userId: e.data.userId,
    pipelineExecutionId: e.data.pipelineExecutionId,
    timestamp: e.data.timestamp ? new Date(e.data.timestamp as unknown as string).toISOString() : null,
    errorMessage: e.data.errorMessage,
    triggerType: e.data.triggerType,
  }));

  return { executions: mapped, availableServices: servicesList };
}

async function handleGetExecutionDetails(match: RouteMatch, ctx: FrameworkContext) {
  const { stores } = ctx;
  const executionId = match.params.id;

  const execution = await stores.executions.get(executionId);
  if (!execution) {
    throw new HttpError(404, 'Execution not found');
  }

  return {
    id: executionId,
    service: execution.service,
    status: executionStatusToString(execution.status),
    userId: execution.userId,
    pipelineExecutionId: execution.pipelineExecutionId,
    timestamp: execution.timestamp ? new Date(execution.timestamp as unknown as string).toISOString() : null,
    startTime: execution.startTime ? new Date(execution.startTime as unknown as string).toISOString() : null,
    endTime: execution.endTime ? new Date(execution.endTime as unknown as string).toISOString() : null,
    errorMessage: execution.errorMessage,
    triggerType: execution.triggerType,
    inputsJson: execution.inputsJson,
    outputsJson: execution.outputsJson,
  };
}

// ========================================
// Pipeline Runs Handlers
// ========================================

interface PipelineRunDoc {
  id: string;
  pipeline_id?: string;
  activity_id?: string;
  source?: number;
  source_activity_id?: string;
  title?: string;
  description?: string;
  type?: number;
  start_time?: admin.firestore.Timestamp;
  status?: number;
  status_message?: string;
  boosters?: Array<{
    provider_name?: string;
    status?: string;
    duration_ms?: number;
    metadata?: Record<string, string>;
    error?: string;
  }>;
  destinations?: Array<{
    destination?: number;
    status?: number;
    external_id?: string;
    error?: string;
    completed_at?: admin.firestore.Timestamp;
  }>;
  created_at?: admin.firestore.Timestamp;
  updated_at?: admin.firestore.Timestamp;
}

async function handleListPipelineRuns(match: RouteMatch, ctx: FrameworkContext) {
  const { logger } = ctx;

  const status = match.query.status as string | undefined;
  const source = match.query.source as string | undefined;
  const userIdFilter = match.query.userId as string | undefined;
  const limit = Math.min(parseInt(match.query.limit as string || '50', 10), 200);
  const cursor = match.query.cursor as string | undefined;

  // Use collection group query to get pipeline_runs across all users
  let q: admin.firestore.Query = db.collectionGroup('pipeline_runs');

  // Apply status filter
  if (status) {
    const statusNum = parseInt(status, 10);
    if (!isNaN(statusNum)) {
      q = q.where('status', '==', statusNum);
    }
  }

  // Apply source filter (only if no status filter - avoid needing complex composite indexes)
  if (source && !status) {
    const sourceNum = parseInt(source, 10);
    if (!isNaN(sourceNum)) {
      q = q.where('source', '==', sourceNum);
    }
  }

  // Order by created_at desc
  q = q.orderBy('created_at', 'desc');

  // Apply cursor for pagination
  if (cursor) {
    try {
      const cursorDate = new Date(cursor);
      q = q.startAfter(cursorDate);
    } catch (e) {
      logger.warn('Invalid cursor', { cursor, error: e });
    }
  }

  q = q.limit(limit + 1); // Fetch one extra to check if there are more

  let docs: admin.firestore.QueryDocumentSnapshot[];
  let hasMore = false;

  try {
    const snapshot = await q.get();
    docs = snapshot.docs;
    hasMore = docs.length > limit;
  } catch (error: unknown) {
    // Check for FAILED_PRECONDITION (missing index)
    const errorMessage = error instanceof Error ? error.message : String(error);
    if (errorMessage.includes('FAILED_PRECONDITION') || errorMessage.includes('requires an index')) {
      logger.warn('Pipeline runs query requires index, falling back to simple query', { error: errorMessage });

      // Fallback: simple query without filters
      const fallbackQuery = db.collectionGroup('pipeline_runs')
        .orderBy('created_at', 'desc')
        .limit(limit + 1);

      const fallbackSnapshot = await fallbackQuery.get();
      docs = fallbackSnapshot.docs;
      hasMore = docs.length > limit;
    } else {
      throw error;
    }
  }

  const resultDocs = hasMore ? docs.slice(0, limit) : docs;

  // Extract user IDs from document paths
  const runs = resultDocs.map(doc => {
    const data = doc.data() as PipelineRunDoc;
    // Path is: users/{userId}/pipeline_runs/{runId}
    const pathParts = doc.ref.path.split('/');
    const docUserId = pathParts[1]; // users/{userId}/...

    return {
      id: doc.id,
      userId: docUserId,
      pipelineId: data.pipeline_id || '',
      activityId: data.activity_id || '',
      source: formatActivitySource(data.source),
      sourceActivityId: data.source_activity_id || '',
      title: data.title || 'Untitled Activity',
      description: data.description || '',
      type: data.type?.toString() || 'unknown',
      startTime: data.start_time?.toDate?.()?.toISOString() || null,
      status: formatPipelineRunStatus(data.status),
      statusMessage: data.status_message || null,
      boosters: (data.boosters || []).map(b => ({
        providerName: b.provider_name || '',
        status: b.status || 'UNKNOWN',
        durationMs: b.duration_ms || 0,
        metadata: b.metadata || {},
        error: b.error || null,
      })),
      destinations: (data.destinations || []).map(d => ({
        destination: formatDestination(d.destination),
        status: d.status?.toString() || 'UNKNOWN',
        externalId: d.external_id || null,
        error: d.error || null,
        completedAt: d.completed_at?.toDate?.()?.toISOString() || null,
      })),
      createdAt: data.created_at?.toDate?.()?.toISOString() || null,
      updatedAt: data.updated_at?.toDate?.()?.toISOString() || null,
    };
  });

  // Filter by userId if provided (post-query filter since collectionGroup can't filter by path)
  const filteredRuns = userIdFilter
    ? runs.filter(run => run.userId === userIdFilter)
    : runs;

  // Compute stats from recent runs (just from this batch for performance)
  const stats = {
    total: filteredRuns.length,
    byStatus: {} as Record<string, number>,
    bySource: {} as Record<string, number>,
  };

  for (const run of filteredRuns) {
    stats.byStatus[run.status] = (stats.byStatus[run.status] || 0) + 1;
    stats.bySource[run.source] = (stats.bySource[run.source] || 0) + 1;
  }

  // Get last doc's created_at for next cursor
  const lastDoc = resultDocs[resultDocs.length - 1];
  const nextCursor = hasMore && lastDoc
    ? (lastDoc.data() as PipelineRunDoc).created_at?.toDate?.()?.toISOString()
    : undefined;

  return {
    runs: filteredRuns,
    stats,
    hasMore,
    nextCursor,
  };
}

async function handleGetPipelineRunDetails(match: RouteMatch, ctx: FrameworkContext) {
  const userId = match.query.userId as string;
  const runId = match.params.id;

  if (!userId) {
    throw new HttpError(400, 'userId query parameter is required');
  }

  const doc = await db.collection('users').doc(userId).collection('pipeline_runs').doc(runId).get();

  if (!doc.exists) {
    throw new HttpError(404, 'Pipeline run not found');
  }

  const data = doc.data() as PipelineRunDoc;

  return {
    id: doc.id,
    userId,
    pipelineId: data.pipeline_id || '',
    activityId: data.activity_id || '',
    source: formatActivitySource(data.source),
    sourceActivityId: data.source_activity_id || '',
    title: data.title || 'Untitled Activity',
    description: data.description || '',
    type: data.type?.toString() || 'unknown',
    startTime: data.start_time?.toDate?.()?.toISOString() || null,
    status: formatPipelineRunStatus(data.status),
    statusMessage: data.status_message || null,
    boosters: (data.boosters || []).map(b => ({
      providerName: b.provider_name || '',
      status: b.status || 'UNKNOWN',
      durationMs: b.duration_ms || 0,
      metadata: b.metadata || {},
      error: b.error || null,
    })),
    destinations: (data.destinations || []).map(d => ({
      destination: formatDestination(d.destination),
      status: d.status?.toString() || 'UNKNOWN',
      externalId: d.external_id || null,
      error: d.error || null,
      completedAt: d.completed_at?.toDate?.()?.toISOString() || null,
    })),
    createdAt: data.created_at?.toDate?.()?.toISOString() || null,
    updatedAt: data.updated_at?.toDate?.()?.toISOString() || null,
  };
}

// ========================================
// Main Handler
// ========================================

export const handler: FrameworkHandler = async (req, ctx) => {
  const userId = ctx.userId;

  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  // All admin endpoints require admin access
  try {
    await ctx.services.authorization.requireAdmin(userId);
  } catch (e) {
    if (e instanceof ForbiddenError) {
      throw new HttpError(403, 'Admin access required');
    }
    throw e;
  }

  return await routeRequest(req, ctx, [
    // Stats
    {
      method: 'GET',
      pattern: '/api/admin/stats',
      handler: async () => await handleGetStats(ctx)
    },
    // Users list
    {
      method: 'GET',
      pattern: '/api/admin/users',
      handler: async (match: RouteMatch) => await handleListUsers(match)
    },
    // User details
    {
      method: 'GET',
      pattern: '/api/admin/users/:id',
      handler: async (match: RouteMatch) => await handleGetUserDetails(match, ctx)
    },
    // Update user
    {
      method: 'PATCH',
      pattern: '/api/admin/users/:id',
      handler: async (match: RouteMatch) => await handleUpdateUser(match, req, userId, ctx)
    },
    // Delete integration
    {
      method: 'DELETE',
      pattern: '/api/admin/users/:id/integrations/:provider',
      handler: async (match: RouteMatch) => await handleDeleteIntegration(match, userId, ctx)
    },
    // Update pipeline (toggle disabled)
    {
      method: 'PATCH',
      pattern: '/api/admin/users/:id/pipelines/:pipelineId',
      handler: async (match: RouteMatch) => await handleUpdatePipeline(match, req, userId, ctx)
    },
    // Delete pipeline
    {
      method: 'DELETE',
      pattern: '/api/admin/users/:id/pipelines/:pipelineId',
      handler: async (match: RouteMatch) => await handleDeletePipeline(match, userId, ctx)
    },
    // Delete activities
    {
      method: 'DELETE',
      pattern: '/api/admin/users/:id/activities',
      handler: async (match: RouteMatch) => await handleDeleteActivities(match, userId, ctx)
    },
    // Delete pending inputs
    {
      method: 'DELETE',
      pattern: '/api/admin/users/:id/pending-inputs',
      handler: async (match: RouteMatch) => await handleDeletePendingInputs(match, userId, ctx)
    },
    // Executions list
    {
      method: 'GET',
      pattern: '/api/admin/executions',
      handler: async (match: RouteMatch) => await handleListExecutions(match, ctx)
    },
    // Execution details
    {
      method: 'GET',
      pattern: '/api/admin/executions/:id',
      handler: async (match: RouteMatch) => await handleGetExecutionDetails(match, ctx)
    },
    // Pipeline runs list (cross-user)
    {
      method: 'GET',
      pattern: '/api/admin/pipeline-runs',
      handler: async (match: RouteMatch) => await handleListPipelineRuns(match, ctx)
    },
    // Pipeline run details
    {
      method: 'GET',
      pattern: '/api/admin/pipeline-runs/:id',
      handler: async (match: RouteMatch) => await handleGetPipelineRunDetails(match, ctx)
    },
  ]);
};

// Export the wrapped function
export const adminHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
