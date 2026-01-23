import {
  createCloudFunction,
  FirebaseAuthStrategy,
  FrameworkContext,
  FrameworkHandler,
  ForbiddenError,
  db,
  userConverter,
  UserTier,
  UserRecord,
  HttpError,
  ExecutionStatus,
  formatExecutionStatus,
  formatDestination,
  PendingInput_Status,
  routeRequest,
  RouteMatch,
} from '@fitglue/shared';
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
    const snapshot = await db.collection('pending_inputs')
      .where('user_id', '==', targetUserId)
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

  const [activityCount, pendingInputs, authInfo] = await Promise.all([
    stores.activities.countSynchronized(targetUserId),
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
    activityCount,
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

  // Batched deletion
  const batchSize = 50;
  let deletedCount = 0;
  const collectionRef = db.collection('users').doc(targetUserId).collection('synchronized_activities');

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

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const snapshot = await db.collection('pending_inputs')
      .where('user_id', '==', targetUserId)
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
  ]);
};

// Export the wrapped function
export const adminHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
