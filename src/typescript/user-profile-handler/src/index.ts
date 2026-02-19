// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError, ForbiddenError } from '@fitglue/shared/errors';
import { INTEGRATIONS, IntegrationAuthType } from '@fitglue/shared/types';



/**
 * User Profile Handler
 *
 * Endpoints:
 * - GET /users/me: Get current user profile with integrations and pipelines
 * - PATCH /users/me: Update user profile (currently no updateable fields)
 * - DELETE /users/me: Cascade delete user and all associated data
 */

// Helper to mask sensitive tokens
function maskToken(token: string | undefined): string | undefined {
  if (!token) return undefined;
  if (token.length <= 8) return '****';
  return token.substring(0, 4) + '****' + token.substring(token.length - 4);
}

// Get integration status summary (with masked tokens)
function getIntegrationsSummary(user: { userId?: string; integrations?: unknown;[key: string]: unknown }) {
  const userIntegrations = (user.integrations as Record<string, Record<string, unknown>>) || {};
  const summary: Record<string, { connected: boolean; externalUserId?: string; lastUsedAt?: string }> = {};

  for (const [key, def] of Object.entries(INTEGRATIONS)) {
    const integration = (userIntegrations as Record<string, Record<string, unknown>>)[key];
    if (integration?.enabled) {
      let externalUserId = undefined;
      if (def.externalUserIdField) {
        const rawId = (integration as Record<string, unknown>)[def.externalUserIdField];
        if (rawId) {
          externalUserId = def.authType === IntegrationAuthType.INTEGRATION_AUTH_TYPE_API_KEY
            ? maskToken(String(rawId))
            : String(rawId);
        }
      }
      summary[key] = {
        connected: true,
        externalUserId,
        lastUsedAt: ((integration as { lastUsedAt?: { toISOString?: () => string } }).lastUsedAt?.toISOString?.() || (integration.lastUsedAt instanceof Date ? integration.lastUsedAt.toISOString() : undefined))
      };
    } else {
      summary[key] = { connected: false };
    }
  }

  return summary;
}

// Map pipeline to response format
function mapPipelineToResponse(pipeline: {
  id: string;
  source: string;
  enrichers?: { providerType: number; inputs?: Record<string, string> }[];
  destinations: number[];
}) {
  // Map destination enums to strings
  const destinationMap: Record<number, string> = {
    0: 'unspecified',
    1: 'strava',
    2: 'mock'
  };

  return {
    id: pipeline.id,
    source: pipeline.source,
    enrichers: (pipeline.enrichers || []).map(e => ({
      providerType: e.providerType,
      inputs: e.inputs
    })),
    destinations: pipeline.destinations.map(d => destinationMap[d] || 'unknown')
  };
}

// eslint-disable-next-line complexity
export const handler: FrameworkHandler = async (req, ctx) => {
  const { logger, services } = ctx;
  const userId = ctx.userId;

  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  // Use ctx.services instead of creating new stores (already initialized by framework)
  const userService = services.user;

  // Extract subpath: /users/me, /admin/users, etc.
  const subPath = req.path.replace(/^\/api/, '') || '/';

  // --- GET /users/me/counters ---
  // Returns all user-defined counters for dynamic select in Auto Increment enricher
  if (subPath === '/users/me/counters' && req.method === 'GET') {
    const countersSnapshot = await db.collection('users').doc(userId).collection('counters').get();
    const counters = countersSnapshot.docs.map(doc => {
      const data = doc.data();
      return {
        id: doc.id,
        count: data.count || 0
      };
    });
    return counters;
  }

  // --- GET /users/me ---
  if (subPath === '/users/me' && req.method === 'GET') {
    const user = await services.user.get(userId);
    if (!user) {
      throw new HttpError(404, 'User not found');
    }

    // Query pipelines from sub-collection
    const pipelines = await services.user.pipelineStore.list(userId);

    const profile = {
      userId: user.userId,
      createdAt: user.createdAt?.toISOString(),
      tier: user.tier || 'hobbyist',
      trialEndsAt: user.trialEndsAt?.toISOString(),
      isAdmin: user.isAdmin || false,
      syncCountThisMonth: user.syncCountThisMonth || 0,
      accessEnabled: user.accessEnabled || false,
      integrations: getIntegrationsSummary(user as unknown as { userId?: string; integrations?: unknown;[key: string]: unknown }),
      pipelines: pipelines.map(mapPipelineToResponse)
    };

    return profile;
  }

  // --- GET /admin/users (Admin Only) ---
  if (subPath === '/admin/users' && req.method === 'GET') {
    try {
      await services.authorization.requireAdmin(userId);
    } catch (e) {
      if (e instanceof ForbiddenError) throw new HttpError(403, e.message);
      throw e;
    }

    // Use services.user.listUsers() which applies the converter for proper camelCase/snake_case mapping
    const users = await services.user.listUsers();
    return users.map(user => ({
      userId: user.userId,
      createdAt: user.createdAt?.toISOString?.() || user.createdAt,
      tier: user.tier || 'hobbyist',
      trialEndsAt: user.trialEndsAt?.toISOString?.() || user.trialEndsAt,
      isAdmin: user.isAdmin || false,
      syncCountThisMonth: user.syncCountThisMonth || 0,
      stripeCustomerId: user.stripeCustomerId || null,
    }));
  }

  // --- PATCH /admin/users/:targetUserId (Admin Only) ---
  const userUpdateMatch = subPath.match(/^\/admin\/users\/([^/]+)$/);
  if (userUpdateMatch && req.method === 'PATCH') {
    const targetUserId = userUpdateMatch[1];
    const { tier, isAdmin } = req.body;

    try {
      await services.authorization.requireAdmin(userId);
    } catch (e) {
      if (e instanceof ForbiddenError) throw new HttpError(403, e.message);
      throw e;
    }

    // Use store update which applies the converter for proper field mapping
    const updates: Partial<Pick<import('@fitglue/shared/types').UserRecord, 'tier' | 'isAdmin'>> = {};
    if (tier !== undefined) updates.tier = tier;
    if (isAdmin !== undefined) updates.isAdmin = isAdmin;

    if (Object.keys(updates).length > 0) {
      await ctx.stores.users.update(targetUserId, updates);
    }
    return { success: true };
  }

  // --- PATCH /users/me ---
  if (subPath === '/users/me' && req.method === 'PATCH') {
    // ... existing logic ...
    return { success: true };
  }

  // --- DELETE /users/me (Cascade Delete) ---
  if (subPath === '/users/me' && req.method === 'DELETE') {
    logger.warn('DELETE /users/me: Starting cascade delete', { userId });

    const userDocRef = db.collection('users').doc(userId);

    // Helper: delete all docs in a sub-collection using batched writes
    const deleteSubCollection = async (name: string) => {
      const ref = userDocRef.collection(name);
      const snapshot = await ref.get();
      if (snapshot.empty) return;
      const batch = db.batch();
      snapshot.forEach(doc => batch.delete(doc.ref));
      await batch.commit();
      logger.info(`Deleted ${name}`, { count: snapshot.size, userId });
    };

    // Helper: delete all docs in a top-level collection matching userId
    const deleteTopLevelByUserId = async (collectionName: string) => {
      const snapshot = await db.collection(collectionName).where('user_id', '==', userId).get();
      if (snapshot.empty) return;
      const batch = db.batch();
      snapshot.forEach(doc => batch.delete(doc.ref));
      await batch.commit();
      logger.info(`Deleted ${collectionName}`, { count: snapshot.size, userId });
    };

    // 1. Delete pipeline_runs (must delete nested destination_outcomes first)
    const pipelineRunsRef = userDocRef.collection('pipeline_runs');
    const pipelineRunsSnapshot = await pipelineRunsRef.get();
    if (!pipelineRunsSnapshot.empty) {
      for (const runDoc of pipelineRunsSnapshot.docs) {
        const outcomesSnapshot = await runDoc.ref.collection('destination_outcomes').get();
        if (!outcomesSnapshot.empty) {
          const outcomesBatch = db.batch();
          outcomesSnapshot.forEach(doc => outcomesBatch.delete(doc.ref));
          await outcomesBatch.commit();
        }
      }
      const runsBatch = db.batch();
      pipelineRunsSnapshot.forEach(doc => runsBatch.delete(doc.ref));
      await runsBatch.commit();
      logger.info('Deleted pipeline_runs', { count: pipelineRunsSnapshot.size, userId });
    }

    // 2-9. Delete all user sub-collections
    await deleteSubCollection('synchronized_activities');
    await deleteSubCollection('raw_activities');
    await deleteSubCollection('executions');
    await deleteSubCollection('pending_inputs');
    await deleteSubCollection('pipelines');
    await deleteSubCollection('counters');
    await deleteSubCollection('booster_data');
    await deleteSubCollection('personal_records');
    await deleteSubCollection('uploaded_activities');
    await deleteSubCollection('plugin_defaults');

    // 10. Delete API keys (top-level, queried by user_id)
    await deleteTopLevelByUserId('ingress_api_keys');

    // 11-12. Delete showcase data (top-level, queried by user_id)
    await deleteTopLevelByUserId('showcased_activities');
    await deleteTopLevelByUserId('showcase_profiles');

    // 13. Finally, delete the user document + Firebase Auth
    await userService.deleteUser(userId);
    logger.warn('User account deleted', { userId });

    return { success: true };
  }


  throw new HttpError(405, 'Method Not Allowed');
};

// Export the wrapped function
export const userProfileHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
