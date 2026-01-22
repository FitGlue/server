import {
  createCloudFunction,
  FrameworkContext,
  FirebaseAuthStrategy,
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
} from '@fitglue/shared';
import { Request } from 'express';
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

export const handler = async (req: Request, ctx: FrameworkContext) => {
  const { logger, services, stores } = ctx;
  const userId = ctx.userId;

  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  // All admin endpoints require admin access
  try {
    await services.authorization.requireAdmin(userId);
  } catch (e) {
    if (e instanceof ForbiddenError) {
      throw new HttpError(403, 'Admin access required');
    }
    throw e;
  }

  // Extract subpath: /admin/stats, /admin/users, etc.
  const subPath = req.path.replace(/^\/api\/admin/, '') || '/';

  // ========================================
  // GET /api/admin/stats - Platform statistics
  // ========================================
  if (subPath === '/stats' && req.method === 'GET') {
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

  // ========================================
  // GET /api/admin/users - Enhanced user list with pagination
  // ========================================
  if (subPath === '/users' && req.method === 'GET') {
    const page = parseInt(req.query.page as string || '1', 10);
    const limit = Math.min(parseInt(req.query.limit as string || '25', 10), 100);
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
        pipelineCount: data.pipelines?.length || 0,
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

  // ========================================
  // GET /api/admin/users/:id - Full user details
  // ========================================
  const userDetailMatch = subPath.match(/^\/users\/([^/]+)$/);
  if (userDetailMatch && req.method === 'GET') {
    const targetUserId = userDetailMatch[1];
    const user = await services.user.get(targetUserId);
    if (!user) {
      throw new HttpError(404, 'User not found');
    }

    // Get activity count using store (correct 'activities' subcollection)
    const activityCount = await stores.activities.countSynchronized(targetUserId);

    // Get pending inputs that are NOT completed - filter in memory to avoid composite index
    let pendingInputs: { activityId: string; status: string; enricherProviderId?: string; createdAt?: string }[] = [];
    try {
      const pendingInputsSnapshot = await db.collection('pending_inputs')
        .where('user_id', '==', targetUserId)
        .get();

      pendingInputs = pendingInputsSnapshot.docs
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
    }

    // Fetch email/displayName from Firebase Auth (only for detail view)
    let email: string | undefined;
    let displayName: string | undefined;
    try {
      const authUser = await admin.auth().getUser(targetUserId);
      email = authUser.email;
      displayName = authUser.displayName;
    } catch (authErr) {
      // User may not have Firebase Auth record (e.g., API-only users)
      logger.debug('Could not fetch Firebase Auth user', { targetUserId, error: authErr });
    }

    // Build integrations dynamically with masked tokens
    const integrations: Record<string, unknown> = {};
    if (user.integrations && typeof user.integrations === 'object') {
      for (const [provider, config] of Object.entries(user.integrations)) {
        const cfg = config as Record<string, unknown>;
        if (cfg?.enabled || cfg?.apiKey) {
          const integrationInfo: Record<string, unknown> = {
            enabled: cfg.enabled || !!cfg.apiKey,
          };
          if (cfg.apiKey) integrationInfo.apiKey = maskToken(cfg.apiKey as string);
          if (cfg.athleteId) integrationInfo.athleteId = cfg.athleteId;
          if (cfg.fitbitUserId) integrationInfo.fitbitUserId = cfg.fitbitUserId;
          if (cfg.lastUsedAt) {
            const lastUsed = cfg.lastUsedAt as Date | string;
            integrationInfo.lastUsedAt = typeof lastUsed === 'object' && 'toISOString' in lastUsed
              ? lastUsed.toISOString()
              : lastUsed;
          }
          integrations[provider] = integrationInfo;
        }
      }
    }

    // Build pipelines with names and destination names
    const pipelines = (user.pipelines || []).map(p => ({
      id: p.id,
      name: p.name || 'Unnamed Pipeline',
      source: p.source,
      destinations: (p.destinations || []).map(d => formatDestination(d)),
      disabled: p.disabled || false,
    }));

    return {
      userId: user.userId,
      email,
      displayName,
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

  // ========================================
  // PATCH /api/admin/users/:id - Update user
  // ========================================
  const userUpdateMatch = subPath.match(/^\/users\/([^/]+)$/);
  if (userUpdateMatch && req.method === 'PATCH') {
    const targetUserId = userUpdateMatch[1];
    const { tier, isAdmin, trialEndsAt, syncCountThisMonth, accessEnabled } = req.body;

    const updates: Record<string, unknown> = {};
    if (tier !== undefined) updates.tier = tier;
    if (isAdmin !== undefined) updates.is_admin = isAdmin;
    if (accessEnabled !== undefined) updates.access_enabled = accessEnabled;
    if (trialEndsAt !== undefined) {
      updates.trial_ends_at = trialEndsAt ? new Date(trialEndsAt) : null;
    }
    if (syncCountThisMonth !== undefined) updates.sync_count_this_month = syncCountThisMonth;

    if (Object.keys(updates).length > 0) {
      await db.collection('users').doc(targetUserId).update(updates);
      logger.info('Admin updated user', { adminUserId: userId, targetUserId, updates: Object.keys(updates) });
    }
    return { success: true };
  }

  // ========================================
  // DELETE /api/admin/users/:id/integrations/:provider
  // ========================================
  const integrationDeleteMatch = subPath.match(/^\/users\/([^/]+)\/integrations\/([^/]+)$/);
  if (integrationDeleteMatch && req.method === 'DELETE') {
    const targetUserId = integrationDeleteMatch[1];
    const provider = integrationDeleteMatch[2];

    // Clear the integration data
    await db.collection('users').doc(targetUserId).update({
      [`integrations.${provider}`]: null
    });
    logger.info('Admin removed integration', { adminUserId: userId, targetUserId, provider });
    return { success: true };
  }

  // ========================================
  // PATCH /api/admin/users/:id/pipelines/:pipelineId - Toggle disabled state
  // ========================================
  const pipelineUpdateMatch = subPath.match(/^\/users\/([^/]+)\/pipelines\/([^/]+)$/);
  if (pipelineUpdateMatch && req.method === 'PATCH') {
    const targetUserId = pipelineUpdateMatch[1];
    const pipelineId = pipelineUpdateMatch[2];
    const { disabled } = req.body;

    if (disabled === undefined) {
      throw new HttpError(400, 'Missing disabled field in request body');
    }

    const user = await services.user.get(targetUserId);
    if (!user) {
      throw new HttpError(404, 'User not found');
    }

    const pipelineIndex = user.pipelines?.findIndex(p => p.id === pipelineId);
    if (pipelineIndex === undefined || pipelineIndex === -1) {
      throw new HttpError(404, 'Pipeline not found');
    }

    // Update the disabled field using Firestore field path notation
    await db.collection('users').doc(targetUserId).update({
      [`pipelines.${pipelineIndex}.disabled`]: disabled
    });

    logger.info('Admin toggled pipeline disabled state', {
      adminUserId: userId,
      targetUserId,
      pipelineId,
      disabled
    });
    return { success: true };
  }

  // ========================================
  // DELETE /api/admin/users/:id/pipelines/:pipelineId
  // ========================================
  const pipelineDeleteMatch = subPath.match(/^\/users\/([^/]+)\/pipelines\/([^/]+)$/);
  if (pipelineDeleteMatch && req.method === 'DELETE') {
    const targetUserId = pipelineDeleteMatch[1];
    const pipelineId = pipelineDeleteMatch[2];

    await services.user.removePipeline(targetUserId, pipelineId);
    logger.info('Admin removed pipeline', { adminUserId: userId, targetUserId, pipelineId });
    return { success: true };
  }

  // ========================================
  // DELETE /api/admin/users/:id/activities
  // ========================================
  const activitiesDeleteMatch = subPath.match(/^\/users\/([^/]+)\/activities$/);
  if (activitiesDeleteMatch && req.method === 'DELETE') {
    const targetUserId = activitiesDeleteMatch[1];

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

    logger.info('Admin deleted activities', { adminUserId: userId, targetUserId, deletedCount });
    return { success: true, deletedCount };
  }

  // ========================================
  // DELETE /api/admin/users/:id/pending-inputs
  // ========================================
  const pendingDeleteMatch = subPath.match(/^\/users\/([^/]+)\/pending-inputs$/);
  if (pendingDeleteMatch && req.method === 'DELETE') {
    const targetUserId = pendingDeleteMatch[1];

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

    logger.info('Admin deleted pending inputs', { adminUserId: userId, targetUserId, deletedCount });
    return { success: true, deletedCount };
  }

  // ========================================
  // GET /api/admin/executions - Query executions
  // ========================================
  if (subPath === '/executions' && req.method === 'GET') {
    const service = req.query.service as string | undefined;
    const status = req.query.status as string | undefined;
    const targetUser = req.query.userId as string | undefined;
    const limit = parseInt(req.query.limit as string || '50', 10);

    const executions = await services.execution.listExecutions({
      service,
      status,
      userId: targetUser,
      limit: Math.min(limit, 200)
    });

    // Get distinct services for dropdown population
    const services_list = await stores.executions.listDistinctServices?.() || [];

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

    return { executions: mapped, availableServices: services_list };
  }

  // ========================================
  // GET /api/admin/executions/:id - Execution details
  // ========================================
  const executionDetailMatch = subPath.match(/^\/executions\/([^/]+)$/);
  if (executionDetailMatch && req.method === 'GET') {
    const executionId = executionDetailMatch[1];

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

  throw new HttpError(404, 'Not Found');
};

// Export the wrapped function
export const adminHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
