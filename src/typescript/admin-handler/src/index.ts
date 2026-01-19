import {
  createCloudFunction,
  FrameworkContext,
  FirebaseAuthStrategy,
  ForbiddenError,
  db
} from '@fitglue/shared';
import { Request, Response } from 'express';
import { ExecutionStatus } from '@fitglue/shared/dist/types/pb/execution';

/**
 * Admin Handler - Consolidated admin operations
 *
 * Endpoints:
 * - GET  /api/admin/stats                              - Platform-wide statistics
 * - GET  /api/admin/users                              - Enhanced user list
 * - GET  /api/admin/users/:id                          - Full user details
 * - PATCH /api/admin/users/:id                         - Update tier/admin/trial
 * - DELETE /api/admin/users/:id/integrations/:provider - Remove integration
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
const executionStatusToString = (status: number | undefined): string => {
  if (status === undefined) return 'UNKNOWN';
  const name = ExecutionStatus[status];
  return name ? name.replace('STATUS_', '') : 'UNKNOWN';
};

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const { logger, services, stores } = ctx;
  const userId = ctx.userId;

  if (!userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  // All admin endpoints require admin access
  try {
    await services.authorization.requireAdmin(userId);
  } catch (e) {
    if (e instanceof ForbiddenError) {
      res.status(403).json({ error: 'Admin access required' });
      return;
    }
    throw e;
  }

  // Extract subpath: /admin/stats, /admin/users, etc.
  const subPath = req.path.replace(/^\/api\/admin/, '') || '/';

  // ========================================
  // GET /api/admin/stats - Platform statistics
  // ========================================
  if (subPath === '/stats' && req.method === 'GET') {
    try {
      const usersSnapshot = await db.collection('users').get();
      const users = usersSnapshot.docs.map(doc => doc.data());

      const totalUsers = users.length;
      const proUsers = users.filter(u => u.tier === 'pro' || u.tier === 'athlete').length;
      const adminUsers = users.filter(u => u.isAdmin === true).length;
      const totalSyncsThisMonth = users.reduce((sum, u) => sum + (u.syncCountThisMonth || 0), 0);

      // Get recent execution stats (last 100)
      const recentExecs = await stores.executions.listRecent(100);
      const execStats = {
        success: recentExecs.filter(e => e.data.status === ExecutionStatus.STATUS_SUCCESS).length,
        failed: recentExecs.filter(e => e.data.status === ExecutionStatus.STATUS_FAILED).length,
        started: recentExecs.filter(e => e.data.status === ExecutionStatus.STATUS_STARTED).length,
      };

      res.status(200).json({
        totalUsers,
        proUsers,
        adminUsers,
        totalSyncsThisMonth,
        recentExecutions: execStats
      });
    } catch (e) {
      logger.error('Failed to get admin stats', { error: e });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // GET /api/admin/users - Enhanced user list
  // ========================================
  if (subPath === '/users' && req.method === 'GET') {
    try {
      const snapshot = await db.collection('users').get();
      const users = snapshot.docs.map(doc => {
        const data = doc.data();

        // Count integrations
        const integrations: string[] = [];
        if (data.integrations?.hevy?.enabled || data.integrations?.hevy?.apiKey) integrations.push('hevy');
        if (data.integrations?.strava?.enabled) integrations.push('strava');
        if (data.integrations?.fitbit?.enabled) integrations.push('fitbit');

        return {
          userId: doc.id,
          createdAt: data.createdAt?.toDate?.()?.toISOString() || data.createdAt,
          tier: data.tier || 'free',
          trialEndsAt: data.trialEndsAt?.toDate?.()?.toISOString() || data.trialEndsAt,
          isAdmin: data.isAdmin || false,
          syncCountThisMonth: data.syncCountThisMonth || 0,
          stripeCustomerId: data.stripeCustomerId || null,
          integrations,
          pipelineCount: data.pipelines?.length || 0,
        };
      });
      res.status(200).json(users);
    } catch (e) {
      logger.error('Failed to list admin users', { error: e });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // GET /api/admin/users/:id - Full user details
  // ========================================
  const userDetailMatch = subPath.match(/^\/users\/([^/]+)$/);
  if (userDetailMatch && req.method === 'GET') {
    const targetUserId = userDetailMatch[1];
    try {
      const user = await services.user.get(targetUserId);
      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      // Get counts for activities and pending inputs
      const activitiesSnapshot = await db.collection('users').doc(targetUserId)
        .collection('synchronized_activities').count().get();
      const pendingInputsSnapshot = await db.collection('pending_inputs')
        .where('user_id', '==', targetUserId).count().get();

      // Build integrations with masked tokens
      const integrations: Record<string, unknown> = {};
      if (user.integrations) {
        if (user.integrations.hevy) {
          integrations.hevy = {
            enabled: user.integrations.hevy.enabled || !!user.integrations.hevy.apiKey,
            apiKey: maskToken(user.integrations.hevy.apiKey),
            lastUsedAt: user.integrations.hevy.lastUsedAt?.toISOString?.() || user.integrations.hevy.lastUsedAt
          };
        }
        if (user.integrations.strava) {
          integrations.strava = {
            enabled: user.integrations.strava.enabled,
            athleteId: user.integrations.strava.athleteId,
            lastUsedAt: user.integrations.strava.lastUsedAt?.toISOString?.() || user.integrations.strava.lastUsedAt
          };
        }
        if (user.integrations.fitbit) {
          integrations.fitbit = {
            enabled: user.integrations.fitbit.enabled,
            fitbitUserId: user.integrations.fitbit.fitbitUserId,
            lastUsedAt: user.integrations.fitbit.lastUsedAt?.toISOString?.() || user.integrations.fitbit.lastUsedAt
          };
        }
      }

      res.status(200).json({
        userId: user.userId,
        createdAt: user.createdAt?.toISOString?.() || user.createdAt,
        tier: user.tier || 'free',
        trialEndsAt: user.trialEndsAt?.toISOString?.() || user.trialEndsAt,
        isAdmin: user.isAdmin || false,
        syncCountThisMonth: user.syncCountThisMonth || 0,
        syncCountResetAt: user.syncCountResetAt?.toISOString?.() || user.syncCountResetAt,
        stripeCustomerId: user.stripeCustomerId || null,
        integrations,
        pipelines: user.pipelines || [],
        activityCount: activitiesSnapshot.data().count,
        pendingInputCount: pendingInputsSnapshot.data().count,
      });
    } catch (e) {
      logger.error('Failed to get user details', { error: e, targetUserId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // PATCH /api/admin/users/:id - Update user
  // ========================================
  const userUpdateMatch = subPath.match(/^\/users\/([^/]+)$/);
  if (userUpdateMatch && req.method === 'PATCH') {
    const targetUserId = userUpdateMatch[1];
    const { tier, isAdmin, trialEndsAt, syncCountThisMonth } = req.body;

    try {
      const updates: Record<string, unknown> = {};
      if (tier !== undefined) updates.tier = tier;
      if (isAdmin !== undefined) updates.isAdmin = isAdmin;
      if (trialEndsAt !== undefined) {
        updates.trialEndsAt = trialEndsAt ? new Date(trialEndsAt) : null;
      }
      if (syncCountThisMonth !== undefined) updates.syncCountThisMonth = syncCountThisMonth;

      if (Object.keys(updates).length > 0) {
        await db.collection('users').doc(targetUserId).update(updates);
        logger.info('Admin updated user', { adminUserId: userId, targetUserId, updates: Object.keys(updates) });
      }
      res.status(200).json({ success: true });
    } catch (e) {
      logger.error('Failed to update user', { error: e, targetUserId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // DELETE /api/admin/users/:id/integrations/:provider
  // ========================================
  const integrationDeleteMatch = subPath.match(/^\/users\/([^/]+)\/integrations\/([^/]+)$/);
  if (integrationDeleteMatch && req.method === 'DELETE') {
    const targetUserId = integrationDeleteMatch[1];
    const provider = integrationDeleteMatch[2];

    try {
      // Clear the integration data
      await db.collection('users').doc(targetUserId).update({
        [`integrations.${provider}`]: null
      });
      logger.info('Admin removed integration', { adminUserId: userId, targetUserId, provider });
      res.status(200).json({ success: true });
    } catch (e) {
      logger.error('Failed to remove integration', { error: e, targetUserId, provider });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // DELETE /api/admin/users/:id/pipelines/:pipelineId
  // ========================================
  const pipelineDeleteMatch = subPath.match(/^\/users\/([^/]+)\/pipelines\/([^/]+)$/);
  if (pipelineDeleteMatch && req.method === 'DELETE') {
    const targetUserId = pipelineDeleteMatch[1];
    const pipelineId = pipelineDeleteMatch[2];

    try {
      await services.user.removePipeline(targetUserId, pipelineId);
      logger.info('Admin removed pipeline', { adminUserId: userId, targetUserId, pipelineId });
      res.status(200).json({ success: true });
    } catch (e) {
      logger.error('Failed to remove pipeline', { error: e, targetUserId, pipelineId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // DELETE /api/admin/users/:id/activities
  // ========================================
  const activitiesDeleteMatch = subPath.match(/^\/users\/([^/]+)\/activities$/);
  if (activitiesDeleteMatch && req.method === 'DELETE') {
    const targetUserId = activitiesDeleteMatch[1];

    try {
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
      res.status(200).json({ success: true, deletedCount });
    } catch (e) {
      logger.error('Failed to delete activities', { error: e, targetUserId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // DELETE /api/admin/users/:id/pending-inputs
  // ========================================
  const pendingDeleteMatch = subPath.match(/^\/users\/([^/]+)\/pending-inputs$/);
  if (pendingDeleteMatch && req.method === 'DELETE') {
    const targetUserId = pendingDeleteMatch[1];

    try {
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
      res.status(200).json({ success: true, deletedCount });
    } catch (e) {
      logger.error('Failed to delete pending inputs', { error: e, targetUserId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // GET /api/admin/executions - Query executions
  // ========================================
  if (subPath === '/executions' && req.method === 'GET') {
    try {
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

      res.status(200).json({ executions: mapped, availableServices: services_list });
    } catch (e) {
      logger.error('Failed to list executions', { error: e });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // ========================================
  // GET /api/admin/executions/:id - Execution details
  // ========================================
  const executionDetailMatch = subPath.match(/^\/executions\/([^/]+)$/);
  if (executionDetailMatch && req.method === 'GET') {
    const executionId = executionDetailMatch[1];

    try {
      const execution = await stores.executions.get(executionId);
      if (!execution) {
        res.status(404).json({ error: 'Execution not found' });
        return;
      }

      res.status(200).json({
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
      });
    } catch (e) {
      logger.error('Failed to get execution details', { error: e, executionId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  res.status(404).json({ error: 'Not Found' });
};

// Export the wrapped function
export const adminHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
