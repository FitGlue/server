import {
  createCloudFunction,
  db,
  FrameworkContext,
  FirebaseAuthStrategy,
  UserStore,
  UserService,
  ActivityStore
} from '@fitglue/shared';
import { Request, Response } from 'express';


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
function getIntegrationsSummary(user: {
  integrations?: {
    hevy?: { enabled?: boolean; apiKey?: string; lastUsedAt?: Date };
    strava?: { enabled?: boolean; athleteId?: number; lastUsedAt?: Date };
    fitbit?: { enabled?: boolean; fitbitUserId?: string; lastUsedAt?: Date };
  };
}) {
  const integrations = user.integrations || {};

  return {
    hevy: integrations.hevy?.enabled
      ? {
        connected: true,
        externalUserId: integrations.hevy.apiKey ? maskToken(integrations.hevy.apiKey) : undefined,
        lastUsedAt: integrations.hevy.lastUsedAt?.toISOString()
      }
      : { connected: false },
    strava: integrations.strava?.enabled
      ? {
        connected: true,
        externalUserId: integrations.strava.athleteId?.toString(),
        lastUsedAt: integrations.strava.lastUsedAt?.toISOString()
      }
      : { connected: false },
    fitbit: integrations.fitbit?.enabled
      ? {
        connected: true,
        externalUserId: integrations.fitbit.fitbitUserId,
        lastUsedAt: integrations.fitbit.lastUsedAt?.toISOString()
      }
      : { connected: false }
  };
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

  // --- GET /users/me ---
  if (req.method === 'GET') {
    try {
      const user = await userService.get(userId);

      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      const profile = {
        userId: user.userId,
        createdAt: user.createdAt?.toISOString(),
        integrations: getIntegrationsSummary(user),
        pipelines: (user.pipelines || []).map(mapPipelineToResponse)
      };

      res.status(200).json(profile);
    } catch (e) {
      logger.error('Failed to get user profile', { error: e, userId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // --- PATCH /users/me ---
  if (req.method === 'PATCH') {
    try {
      // Currently no updateable fields via this endpoint
      // Future: Could allow updating display name, preferences, etc.
      logger.info('Profile update request received', { userId });
      res.status(200).json({ success: true });
    } catch (e) {
      logger.error('Failed to update user profile', { error: e, userId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // --- DELETE /users/me (Cascade Delete) ---
  if (req.method === 'DELETE') {
    try {
      logger.warn('DELETE /users/me: Starting cascade delete', { userId });

      // 1. Delete all synchronized activities subcollection
      const syncActivityRef = db.collection('users').doc(userId).collection('synchronized_activities');
      const syncActivitySnapshot = await syncActivityRef.get();
      const syncDeleteBatch = db.batch();
      syncActivitySnapshot.forEach(doc => syncDeleteBatch.delete(doc.ref));
      if (!syncActivitySnapshot.empty) {
        await syncDeleteBatch.commit();
        logger.info('Deleted synchronized activities', { count: syncActivitySnapshot.size, userId });
      }

      // 2. Delete all raw activities subcollection
      const rawActivityRef = db.collection('users').doc(userId).collection('raw_activities');
      const rawActivitySnapshot = await rawActivityRef.get();
      const rawDeleteBatch = db.batch();
      rawActivitySnapshot.forEach(doc => rawDeleteBatch.delete(doc.ref));
      if (!rawActivitySnapshot.empty) {
        await rawDeleteBatch.commit();
        logger.info('Deleted raw activities', { count: rawActivitySnapshot.size, userId });
      }

      // 3. Delete all API keys for this user (using collection query)
      const apiKeyRef = db.collection('ingress_api_keys').where('user_id', '==', userId);
      const apiKeySnapshot = await apiKeyRef.get();
      const apiKeyDeleteBatch = db.batch();
      apiKeySnapshot.forEach(doc => apiKeyDeleteBatch.delete(doc.ref));
      if (!apiKeySnapshot.empty) {
        await apiKeyDeleteBatch.commit();
        logger.info('Deleted API keys', { count: apiKeySnapshot.size, userId });
      }

      // 4. Delete all execution records for this user (using collection query)
      const executionRef = db.collection('executions').where('user_id', '==', userId);
      const executionSnapshot = await executionRef.get();
      const executionDeleteBatch = db.batch();
      executionSnapshot.forEach(doc => executionDeleteBatch.delete(doc.ref));
      if (!executionSnapshot.empty) {
        await executionDeleteBatch.commit();
        logger.info('Deleted execution records', { count: executionSnapshot.size, userId });
      }

      // 5. Delete pending inputs (stored in top-level 'pending_inputs' collection)
      const pendingInputsRef = db.collection('pending_inputs').where('user_id', '==', userId);
      const pendingInputsSnapshot = await pendingInputsRef.get();
      const pendingDeleteBatch = db.batch();
      pendingInputsSnapshot.forEach(doc => pendingDeleteBatch.delete(doc.ref));
      if (!pendingInputsSnapshot.empty) {
        await pendingDeleteBatch.commit();
        logger.info('Deleted pending inputs', { count: pendingInputsSnapshot.size, userId });
      }

      // 6. Finally, delete the user document
      await userService.deleteUser(userId);
      logger.warn('User account deleted', { userId });

      res.status(200).json({ success: true });
    } catch (e) {
      logger.error('Failed to delete user account', { error: e, userId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }


  res.status(405).send('Method Not Allowed');
};

// Export the wrapped function
export const userProfileHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  }
});
