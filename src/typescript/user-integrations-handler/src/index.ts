import {
  createCloudFunction,
  db,
  FrameworkContext,
  FirebaseAuthStrategy,
  UserStore,
  UserService,
  ActivityStore,
  generateOAuthState,
  getSecret
} from '@fitglue/shared';
import { Request, Response } from 'express';

/**
 * User Integrations Handler
 *
 * Endpoints:
 * - GET /users/me/integrations: List integrations with masked tokens
 * - POST /users/me/integrations/{provider}/connect: Generate OAuth URL
 * - DELETE /users/me/integrations/{provider}: Disconnect integration
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

// Extract provider from path
function extractProvider(path: string): string | null {
  // Path patterns:
  // /integrations/{provider}/connect
  // /integrations/{provider}
  // Also handle Firebase hosting rewrite paths like /api/users/me/integrations/strava/connect
  const segments = path.split('/').filter(s => s.length > 0);

  // Find 'integrations' in path and get next segment
  const integrationIdx = segments.findIndex(s => s === 'integrations');
  if (integrationIdx >= 0 && segments.length > integrationIdx + 1) {
    return segments[integrationIdx + 1];
  }

  return null;
}

// Check if path is a connect request
function isConnectPath(path: string): boolean {
  return path.includes('/connect');
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
  const path = req.path;

  // --- GET /users/me/integrations ---
  if (req.method === 'GET') {
    try {
      const user = await userService.get(userId);

      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      res.status(200).json(getIntegrationsSummary(user));
    } catch (e) {
      logger.error('Failed to get integrations', { error: e, userId });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // --- POST /users/me/integrations/{provider}/connect ---
  if (req.method === 'POST' && isConnectPath(path)) {
    const provider = extractProvider(path);

    if (!provider || !['strava', 'fitbit'].includes(provider)) {
      res.status(400).json({ error: 'Invalid or unsupported provider' });
      return;
    }

    try {
      const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
      const baseUrl = process.env.BASE_URL || 'https://fitglue.app';

      // Generate state token for CSRF protection
      const state = await generateOAuthState(userId);

      let authUrl: string;

      if (provider === 'strava') {
        const clientId = await getSecret(projectId, 'strava-client-id');
        const redirectUri = `${baseUrl}/api/strava-oauth-handler`;

        authUrl = `https://www.strava.com/oauth/authorize?` +
          `client_id=${clientId}&` +
          `redirect_uri=${encodeURIComponent(redirectUri)}&` +
          `response_type=code&` +
          `scope=activity:read_all,activity:write&` +
          `state=${state}`;
      } else if (provider === 'fitbit') {
        const clientId = await getSecret(projectId, 'fitbit-client-id');
        const redirectUri = `${baseUrl}/api/fitbit-oauth-handler`;

        authUrl = `https://www.fitbit.com/oauth2/authorize?` +
          `client_id=${clientId}&` +
          `redirect_uri=${encodeURIComponent(redirectUri)}&` +
          `response_type=code&` +
          `scope=${encodeURIComponent('activity heartrate location nutrition profile settings sleep social weight')}&` +
          `state=${state}`;
      } else {
        res.status(400).json({ error: 'Invalid provider' });
        return;
      }

      logger.info('Generated OAuth URL', { userId, provider });
      res.status(200).json({ authUrl });
    } catch (e) {
      logger.error('Failed to generate OAuth URL', { error: e, userId, provider });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  // --- DELETE /users/me/integrations/{provider} ---
  if (req.method === 'DELETE') {
    const provider = extractProvider(path);

    if (!provider || !['strava', 'fitbit', 'hevy'].includes(provider)) {
      res.status(400).json({ error: 'Invalid provider' });
      return;
    }

    try {
      const user = await userService.get(userId);
      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      // Check if integration exists
      const integrations = user.integrations as Record<string, { enabled?: boolean }> || {};
      if (!integrations[provider]?.enabled) {
        res.status(404).json({ error: 'Integration not found' });
        return;
      }

      // Clear the integration by setting enabled to false and removing tokens
      if (provider === 'hevy') {
        await userStore.setIntegration(userId, 'hevy', {
          enabled: false,
          apiKey: '',
          userId: '',
          createdAt: new Date(),
          lastUsedAt: new Date()
        });
      } else if (provider === 'strava') {
        await userStore.setIntegration(userId, 'strava', {
          enabled: false,
          accessToken: '',
          refreshToken: '',
          expiresAt: new Date(),
          athleteId: 0,
          createdAt: new Date(),
          lastUsedAt: new Date()
        });
      } else if (provider === 'fitbit') {
        await userStore.setIntegration(userId, 'fitbit', {
          enabled: false,
          accessToken: '',
          refreshToken: '',
          expiresAt: new Date(),
          fitbitUserId: '',
          createdAt: new Date(),
          lastUsedAt: new Date()
        });
      }

      logger.info('Disconnected integration', { userId, provider });
      res.status(200).json({ success: true });
    } catch (e) {
      logger.error('Failed to disconnect integration', { error: e, userId, provider });
      res.status(500).json({ error: 'Internal Server Error' });
    }
    return;
  }

  res.status(405).send('Method Not Allowed');
};

// Export the wrapped function
export const userIntegrationsHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  }
});
