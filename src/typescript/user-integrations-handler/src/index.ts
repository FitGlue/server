import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, generateOAuthState, getSecret, canAddConnection, countActiveConnections, HttpError } from '@fitglue/shared';
import { Request } from 'express';


export const handler = async (req: Request, ctx: FrameworkContext) => {
  const userId = ctx.userId;
  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  const { logger } = ctx;
  const path = req.path;

  // Extract path segments for sub-routes
  // Path may be /api/users/me/integrations or /integrations or just /
  // Get the last meaningful segments after 'integrations'
  const integrationsIndex = path.indexOf('/integrations');
  const subPath = integrationsIndex >= 0
    ? path.substring(integrationsIndex + '/integrations'.length)
    : path;
  const pathParts = subPath.split('/').filter(p => p !== '');

  logger.info('Routing request', { path, subPath, pathParts, method: req.method });

  // GET /users/me/integrations - List all integrations
  if (req.method === 'GET' && pathParts.length === 0) {
    return await handleListIntegrations(userId, ctx);
  }

  // POST /users/me/integrations/{provider}/connect - Generate OAuth URL
  if (req.method === 'POST' && pathParts.length >= 2 && pathParts[1] === 'connect') {
    const provider = pathParts[0];
    return await handleConnect(userId, provider, ctx);
  }

  // DELETE /users/me/integrations/{provider} - Disconnect integration
  if (req.method === 'DELETE' && pathParts.length >= 1) {
    const provider = pathParts[0];
    return await handleDisconnect(userId, provider, ctx);
  }

  // PUT /users/me/integrations/{provider} - Configure API key integration
  if (req.method === 'PUT' && pathParts.length >= 1) {
    const provider = pathParts[0];
    return await handleConfigure(userId, provider, req.body, ctx);
  }

  throw new HttpError(404, 'Not found');
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

  // Get client ID from secrets
  const clientId = await getSecret(projectId, `${provider}-client-id`);

  // Generate state token
  const state = await generateOAuthState(userId);

  let authUrl: string;
  if (provider === 'strava') {
    authUrl = `https://www.strava.com/oauth/authorize?` +
      `client_id=${clientId}&` +
      `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/strava/callback`)}&` +
      `response_type=code&` +
      `scope=read,activity:read_all,activity:write&` +
      `state=${state}`;
  } else {
    authUrl = `https://www.fitbit.com/oauth2/authorize?` +
      `client_id=${clientId}&` +
      `redirect_uri=${encodeURIComponent(`${baseUrl}/auth/fitbit/callback`)}&` +
      `response_type=code&` +
      `scope=${encodeURIComponent('activity heartrate profile location')}&` +
      `state=${state}`;
  }

  logger.info('Generated OAuth URL', { userId, provider });
  return { url: authUrl };
}

async function handleDisconnect(userId: string, provider: string, ctx: FrameworkContext) {
  const { logger } = ctx;

  // Import registry dynamically to validate provider
  const { getRegistry } = await import('@fitglue/shared');
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

  // Import registry and auth types
  const { getRegistry, IntegrationAuthType } = await import('@fitglue/shared');
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
    // Make a simple API call to validate the key
    // Use /v1/workouts with page_count=1 for efficiency (the /v1/user endpoint doesn't exist)
    const response = await fetch('https://api.hevyapp.com/v1/workouts?page_count=1', {
      headers: {
        'api-key': apiKey,
        'Accept': 'application/json',
      },
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
