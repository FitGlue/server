// Module-level imports for smart pruning
import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, FrameworkHandler } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { routeRequest, RouteMatch } from '@fitglue/shared/routing';
import { getRegistry } from '@fitglue/shared/plugin';
import { IntegrationAuthType } from '@fitglue/shared/types';
import { generateOAuthState } from '@fitglue/shared/infrastructure/oauth';
import { getSecret } from '@fitglue/shared/infrastructure';
import { canAddConnection, countActiveConnections } from '@fitglue/shared/domain';
import { createHevyClient } from '@fitglue/shared/integrations/hevy';


export const handler: FrameworkHandler = async (req, ctx) => {
  const userId = ctx.userId;
  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  return await routeRequest(req, ctx, [
    {
      method: 'GET',
      pattern: '/api/users/me/integrations',
      handler: async () => await handleListIntegrations(userId, ctx)
    },
    {
      method: 'POST',
      pattern: '/api/users/me/integrations/:provider/connect',
      handler: async (match: RouteMatch) => await handleConnect(userId, match.params.provider, ctx)
    },
    {
      method: 'DELETE',
      pattern: '/api/users/me/integrations/:provider',
      handler: async (match: RouteMatch) => await handleDisconnect(userId, match.params.provider, ctx)
    },
    {
      method: 'PUT',
      pattern: '/api/users/me/integrations/:provider',
      handler: async (match: RouteMatch) => await handleConfigure(userId, match.params.provider, req.body, ctx)
    }
  ]);
};

async function handleListIntegrations(userId: string, ctx: FrameworkContext) {
  const user = await ctx.services.user.get(userId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }

  const integrations = user.integrations || {};

  // Build masked summary
  const summary: Record<string, { connected: boolean; externalUserId?: string; lastUsedAt?: Date }> = {};

  if (integrations.hevy) {
    summary.hevy = {
      connected: !!integrations.hevy.enabled,
      externalUserId: integrations.hevy.userId ? `***${integrations.hevy.userId.slice(-4)}` : undefined,
      lastUsedAt: integrations.hevy.lastUsedAt
    };
  }

  if (integrations.strava) {
    summary.strava = {
      connected: !!integrations.strava.enabled,
      externalUserId: integrations.strava.athleteId?.toString(),
      lastUsedAt: integrations.strava.lastUsedAt
    };
  }

  if (integrations.fitbit) {
    summary.fitbit = {
      connected: !!integrations.fitbit.enabled,
      externalUserId: integrations.fitbit.fitbitUserId,
      lastUsedAt: integrations.fitbit.lastUsedAt
    };
  }

  if (integrations.parkrun) {
    summary.parkrun = {
      connected: !!integrations.parkrun.enabled,
      externalUserId: integrations.parkrun.athleteId,
      lastUsedAt: integrations.parkrun.lastUsedAt
    };
  }

  return summary;
}

async function handleConnect(userId: string, provider: string, ctx: FrameworkContext) {
  const { logger } = ctx;

  // Validate provider
  if (!['strava', 'fitbit'].includes(provider)) {
    throw new HttpError(400, `Invalid OAuth provider: ${provider}. Hevy uses API key configuration.`);
  }

  // Check connection limit for free tier users
  const user = await ctx.services.user.get(userId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }
  const currentCount = countActiveConnections(user);
  const { allowed, reason } = canAddConnection(user, currentCount);
  if (!allowed) {
    throw new HttpError(403, reason || 'Connection limit reached');
  }

  const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
  const env = projectId.includes('-prod') ? 'prod' : projectId.includes('-test') ? 'test' : 'dev';
  const baseUrl = env === 'prod' ? 'https://fitglue.tech' : `https://${env}.fitglue.tech`;

  // Get client ID from secrets (environment variable)
  const clientId = getSecret(`${provider.toUpperCase()}_CLIENT_ID`);

  // Generate state token
  const state = await generateOAuthState(userId);

  let authUrl: string;
  if (provider === 'strava') {
    authUrl = 'https://www.strava.com/oauth/authorize?' +
      `client_id=${clientId}&` +
      `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/strava/callback`)}&` +
      'response_type=code&' +
      'scope=read,activity:read_all,activity:write&' +
      `state=${state}`;
  } else {
    authUrl = 'https://www.fitbit.com/oauth2/authorize?' +
      `client_id=${clientId}&` +
      `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/fitbit/callback`)}&` +
      'response_type=code&' +
      `scope=${encodeURIComponent('activity heartrate profile location')}&` +
      `state=${state}`;
  }

  logger.info('Generated OAuth URL', { userId, provider });
  return { url: authUrl };
}

async function handleDisconnect(userId: string, provider: string, ctx: FrameworkContext) {
  const { logger } = ctx;

  // Validate provider using registry
  const registry = getRegistry();
  const integrationManifest = registry.integrations.find((i: { id: string }) => i.id === provider);

  if (!integrationManifest) {
    throw new HttpError(400, `Invalid provider: ${provider}`);
  }

  // Verify user exists
  const user = await ctx.services.user.get(userId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }

  // Delete the integration data entirely (not just disable)
  await ctx.stores.users.deleteIntegration(userId, provider);

  // Try to delete any associated ingress keys (matches label pattern from handleConfigure)
  const label = `${integrationManifest.name} Webhook`;
  const deletedKeys = await ctx.stores.apiKeys.deleteByUserAndLabel(userId, label);
  if (deletedKeys > 0) {
    logger.info('Deleted ingress key', { userId, provider, label, count: deletedKeys });
  }

  logger.info('Deleted integration', { userId, provider });
  return { message: `Disconnected ${provider}` };
}

async function handleConfigure(
  userId: string,
  provider: string,
  body: { apiKey?: string },
  ctx: FrameworkContext
) {
  const { logger } = ctx;

  // Get registry and auth types
  const registry = getRegistry();

  // Look up integration from registry
  const integrationManifest = registry.integrations.find((i: { id: string }) => i.id === provider);
  if (!integrationManifest) {
    throw new HttpError(400, `Unknown integration: ${provider}`);
  }

  // Only API_KEY and PUBLIC_ID can be configured via PUT
  if (integrationManifest.authType === IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH) {
    throw new HttpError(400, `${provider} uses OAuth authentication. Use the Connect flow instead.`);
  }
  if (integrationManifest.authType === IntegrationAuthType.INTEGRATION_AUTH_TYPE_APP_SYNC) {
    throw new HttpError(400, `${provider} requires the mobile app. Download FitGlue from your app store.`);
  }

  const inputValue = body?.apiKey?.trim();
  if (!inputValue) {
    const fieldName = integrationManifest.apiKeyLabel || 'Value';
    throw new HttpError(400, `${fieldName} is required`);
  }

  // Check connection limit
  const user = await ctx.services.user.get(userId);
  if (!user) {
    throw new HttpError(404, 'User not found');
  }

  // Check if this provider is already connected
  const existingIntegration = user.integrations?.[provider as keyof typeof user.integrations];
  const alreadyConnected = existingIntegration && 'enabled' in existingIntegration && existingIntegration.enabled;

  if (!alreadyConnected) {
    const currentCount = countActiveConnections(user);
    const { allowed, reason } = canAddConnection(user, currentCount);
    if (!allowed) {
      throw new HttpError(403, reason || 'Connection limit reached');
    }
  }

  // Handle based on provider
  if (provider === 'hevy') {
    // Hevy: Validate API key, save integration, generate ingress key for webhook
    const isValid = await validateHevyApiKey(inputValue);
    if (!isValid) {
      throw new HttpError(400, 'Invalid API key. Please check and try again.');
    }

    await ctx.stores.users.setIntegration(userId, 'hevy', {
      enabled: true,
      apiKey: inputValue,
      userId: '', // Will be populated on first webhook
      createdAt: new Date(),
    });

    // Generate ingress key for webhook configuration
    const crypto = await import('crypto');
    const rawIngressKey = crypto.randomBytes(32).toString('hex');
    const keyHash = crypto.createHash('sha256').update(rawIngressKey).digest('hex');
    const label = `${integrationManifest.name} Webhook`;

    await ctx.stores.apiKeys.create(keyHash, {
      userId,
      label,
      scopes: ['ingress'],
      createdAt: new Date(),
    });

    logger.info('Configured Hevy integration and generated ingress key', { userId, provider, label });
    return {
      message: `${integrationManifest.name} connected successfully`,
      ingressApiKey: rawIngressKey,
      ingressKeyLabel: label,
    };

  } else if (provider === 'parkrun') {
    // Parkrun: Just save the barcode number, no validation or ingress key needed
    await ctx.stores.users.setIntegration(userId, 'parkrun', {
      enabled: true,
      athleteId: inputValue,
      countryUrl: 'www.parkrun.org.uk', // Default, could be made configurable later
      consentGiven: true,
      createdAt: new Date(),
    });

    logger.info('Configured Parkrun integration', { userId, athleteId: inputValue });
    return {
      message: `${integrationManifest.name} connected successfully`,
      // No ingress key for Parkrun - it uses pull-based results fetching
    };

  } else {
    // Unsupported provider for PUT configuration
    throw new HttpError(400, `${provider} cannot be configured this way.`);
  }
}

async function validateHevyApiKey(apiKey: string): Promise<boolean> {
  try {
    // Use typed Hevy client to validate the API key
    const client = createHevyClient({ apiKey });
    const { response } = await client.GET('/v1/workouts', {
      params: { query: { page: 1, pageSize: 1 } }
    });
    return response.ok;
  } catch {
    return false;
  }
}

export const userIntegrationsHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()]
  },
  skipExecutionLogging: true
});
