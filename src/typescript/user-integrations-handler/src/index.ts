import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, generateOAuthState, getSecret, canAddConnection, countActiveConnections } from '@fitglue/shared';
import { Request, Response } from 'express';


export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const userId = ctx.userId;
  if (!userId) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  const { logger } = ctx;

  try {
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
      return await handleListIntegrations(userId, res, ctx);
    }

    // POST /users/me/integrations/{provider}/connect - Generate OAuth URL
    if (req.method === 'POST' && pathParts.length >= 2 && pathParts[1] === 'connect') {
      const provider = pathParts[0];
      return await handleConnect(userId, provider, res, ctx);
    }

    // DELETE /users/me/integrations/{provider} - Disconnect integration
    if (req.method === 'DELETE' && pathParts.length >= 1) {
      const provider = pathParts[0];
      return await handleDisconnect(userId, provider, res, ctx);
    }

    // PUT /users/me/integrations/{provider} - Configure API key integration
    if (req.method === 'PUT' && pathParts.length >= 1) {
      const provider = pathParts[0];
      return await handleConfigure(userId, provider, req.body, res, ctx);
    }

    res.status(404).json({ error: 'Not found' });
  } catch (err) {
    logger.error('Failed to handle integrations request', { error: err });
    res.status(500).json({ error: 'Internal Server Error' });
  }
};

async function handleListIntegrations(userId: string, res: Response, ctx: FrameworkContext) {
  const user = await ctx.services.user.get(userId);
  if (!user) {
    res.status(404).json({ error: 'User not found' });
    return;
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

  res.status(200).json(summary);
}

async function handleConnect(userId: string, provider: string, res: Response, ctx: FrameworkContext) {
  const { logger } = ctx;

  // Validate provider
  if (!['strava', 'fitbit'].includes(provider)) {
    res.status(400).json({ error: `Invalid OAuth provider: ${provider}. Hevy uses API key configuration.` });
    return;
  }

  // Check connection limit for free tier users
  const user = await ctx.services.user.get(userId);
  if (!user) {
    res.status(404).json({ error: 'User not found' });
    return;
  }
  const currentCount = countActiveConnections(user);
  const { allowed, reason } = canAddConnection(user, currentCount);
  if (!allowed) {
    res.status(403).json({ error: reason });
    return;
  }

  try {

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
    res.status(200).json({ url: authUrl });

  } catch (err) {
    logger.error('Failed to generate OAuth URL', { error: err, provider });
    res.status(500).json({ error: 'Failed to generate authorization URL' });
  }
}

async function handleDisconnect(userId: string, provider: string, res: Response, ctx: FrameworkContext) {
  const { logger } = ctx;

  // Import registry dynamically to validate provider
  const { getRegistry } = await import('@fitglue/shared');
  const registry = getRegistry();
  const integrationManifest = registry.integrations.find((i: { id: string }) => i.id === provider);

  if (!integrationManifest) {
    res.status(400).json({ error: `Invalid provider: ${provider}` });
    return;
  }

  try {
    // Verify user exists
    const user = await ctx.services.user.get(userId);
    if (!user) {
      res.status(404).json({ error: 'User not found' });
      return;
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
    res.status(200).json({ message: `Disconnected ${provider}` });

  } catch (err) {
    logger.error('Failed to disconnect integration', { error: err, provider });
    res.status(500).json({ error: 'Failed to disconnect integration' });
  }
}

async function handleConfigure(
  userId: string,
  provider: string,
  body: { apiKey?: string },
  res: Response,
  ctx: FrameworkContext
) {
  const { logger } = ctx;

  // Import registry and auth types
  const { getRegistry, IntegrationAuthType } = await import('@fitglue/shared');
  const registry = getRegistry();

  // Look up integration from registry
  const integrationManifest = registry.integrations.find((i: { id: string }) => i.id === provider);
  if (!integrationManifest) {
    res.status(400).json({ error: `Unknown integration: ${provider}` });
    return;
  }

  // Only API_KEY and PUBLIC_ID can be configured via PUT
  if (integrationManifest.authType === IntegrationAuthType.INTEGRATION_AUTH_TYPE_OAUTH) {
    res.status(400).json({
      error: `${provider} uses OAuth authentication. Use the Connect flow instead.`
    });
    return;
  }
  if (integrationManifest.authType === IntegrationAuthType.INTEGRATION_AUTH_TYPE_APP_SYNC) {
    res.status(400).json({
      error: `${provider} requires the mobile app. Download FitGlue from your app store.`
    });
    return;
  }

  const inputValue = body?.apiKey?.trim();
  if (!inputValue) {
    const fieldName = integrationManifest.apiKeyLabel || 'Value';
    res.status(400).json({ error: `${fieldName} is required` });
    return;
  }

  // Check connection limit
  const user = await ctx.services.user.get(userId);
  if (!user) {
    res.status(404).json({ error: 'User not found' });
    return;
  }

  // Check if this provider is already connected
  const existingIntegration = user.integrations?.[provider as keyof typeof user.integrations];
  const alreadyConnected = existingIntegration && 'enabled' in existingIntegration && existingIntegration.enabled;

  if (!alreadyConnected) {
    const currentCount = countActiveConnections(user);
    const { allowed, reason } = canAddConnection(user, currentCount);
    if (!allowed) {
      res.status(403).json({ error: reason });
      return;
    }
  }

  try {
    // Handle based on provider
    if (provider === 'hevy') {
      // Hevy: Validate API key, save integration, generate ingress key for webhook
      const isValid = await validateHevyApiKey(inputValue);
      if (!isValid) {
        res.status(400).json({ error: 'Invalid API key. Please check and try again.' });
        return;
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
      res.status(200).json({
        message: `${integrationManifest.name} connected successfully`,
        ingressApiKey: rawIngressKey,
        ingressKeyLabel: label,
      });

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
      res.status(200).json({
        message: `${integrationManifest.name} connected successfully`,
        // No ingress key for Parkrun - it uses pull-based results fetching
      });

    } else {
      // Unsupported provider for PUT configuration
      res.status(400).json({ error: `${provider} cannot be configured this way.` });
    }

  } catch (err) {
    logger.error('Failed to configure integration', { error: err, provider });
    res.status(500).json({ error: 'Failed to configure integration' });
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
  }
});
