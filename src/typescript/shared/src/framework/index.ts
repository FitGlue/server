import * as admin from 'firebase-admin';
import * as winston from 'winston';
import { logExecutionStart, logExecutionSuccess, logExecutionFailure, logExecutionPending } from '../execution/logger';
import { AuthStrategy } from './auth';
export * from './connector';
export * from './base-connector';
export * from './webhook-processor';
import { PubSub } from '@google-cloud/pubsub';
import { SecretManagerServiceClient } from '@google-cloud/secret-manager';
import { UserStore, ExecutionStore, ApiKeyStore, IntegrationIdentityStore, ActivityStore } from '../storage/firestore';
import { UserService, ApiKeyService, ExecutionService } from '../domain/services';

// Initialize Secret Manager
const secretClient = new SecretManagerServiceClient();

export interface SecretsHelper {
  get(name: string): Promise<string>;
}

class SecretManagerHelper implements SecretsHelper {
  private projectId: string;

  constructor(projectId: string) {
    this.projectId = projectId;
  }

  async get(name: string): Promise<string> {
    if (!this.projectId) {
      // Fallback logic could go here, or we enforce project ID availability
      throw new Error('Project ID not configured for SecretsHelper');
    }
    // Access latest version
    const [version] = await secretClient.accessSecretVersion({
      name: `projects/${this.projectId}/secrets/${name}/versions/latest`,
    });
    return version.payload?.data?.toString() || '';
  }
}

// Initialize Firebase (Idempotent)
if (admin.apps.length === 0) {
  admin.initializeApp();
}
export const db = admin.firestore();

// Initialize PubSub
const pubsub = new PubSub();

// Configure Structured Logging
const logLevel = (process.env.LOG_LEVEL || 'info').toLowerCase();
const logger = winston.createLogger({
  level: logLevel, // Use configured level
  format: winston.format.json(),
  defaultMeta: { service: process.env.K_SERVICE || 'unknown-service' },
  transports: [
    new winston.transports.Console({
      format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.printf(info => {
          // Map to GCP keys
          const gcpInfo: any = {
            timestamp: info.timestamp,
            ...info,
            severity: info.level.toUpperCase(),
            message: info.message
          };
          // Remove default keys to avoid duplication/conflict
          delete gcpInfo.level;
          return JSON.stringify(gcpInfo);
        })
      )
    })
  ]
});

export interface FrameworkContext {
  services: {
    user: import('../domain/services/user').UserService;
    apiKey: import('../domain/services/apikey').ApiKeyService;
    execution: import('../domain/services/execution').ExecutionService;
  };
  stores: {
    users: import('../storage/firestore').UserStore;
    executions: import('../storage/firestore').ExecutionStore;
    apiKeys: import('../storage/firestore').ApiKeyStore;
    integrationIdentities: import('../storage/firestore').IntegrationIdentityStore;
    activities: import('../storage/firestore').ActivityStore;
  };
  pubsub: PubSub;
  secrets: SecretsHelper;
  logger: winston.Logger;
  executionId: string;
  userId?: string;
  authScopes?: string[];
}
// ...
// Build the full context


export type FrameworkHandler = (
  req: any,
  res: any,
  ctx: FrameworkContext
) => Promise<any>;

export interface CloudFunctionOptions {
  auth?: {
    strategies: AuthStrategy[]; // Only accept strategy instances
    requiredScopes?: string[];
  };
}

/**
 * Extract metadata from HTTP request
 * Handles both HTTP requests and Pub/Sub messages
 */
function extractMetadata(req: any): { userId?: string; testRunId?: string; triggerType: string } {
  let userId: string | undefined;
  let testRunId: string | undefined;
  let triggerType = 'http';

  // Check if this is a Pub/Sub message (has message.data structure)
  if (req.body && req.body.message && req.body.message.data) {
    triggerType = 'pubsub';

    // Decode base64 Pub/Sub message data
    try {
      const messageData = Buffer.from(req.body.message.data, 'base64').toString('utf-8');
      const payload = JSON.parse(messageData);
      userId = payload.user_id || payload.userId;
    } catch (e) {
      // If parsing fails, continue without user_id
    }

    // Check Pub/Sub message attributes for test_run_id
    if (req.body.message.attributes) {
      testRunId = req.body.message.attributes.test_run_id || req.body.message.attributes.testRunId;
    }
  } else {
    // HTTP request
    // Try to extract user_id from request body
    if (req.body) {
      userId = req.body.user_id || req.body.userId;
    }

    // Try to extract test_run_id from headers (check both formats)
    if (req.headers) {
      testRunId = req.headers['x-test-run-id'] || req.headers['x-testrunid'];
    }
  }

  return { userId, testRunId, triggerType };
}

export const createCloudFunction = (handler: FrameworkHandler, options?: CloudFunctionOptions) => {
  return async (reqOrEvent: any, resOrContext?: any) => {
    const serviceName = process.env.K_SERVICE || 'unknown-function';

    // DETECT TRIGGER TYPE
    // HTTP: (req, res)
    // CloudEvent: (event)
    // Background (Legacy): (data, context)

    let isHttp = false;
    let req = reqOrEvent;
    let res = resOrContext;

    // If 'res' has 'status' and 'send' methods, it's HTTP
    if (res && typeof res.status === 'function' && typeof res.send === 'function') {
      isHttp = true;
    }

    // ADAPT CLOUDEVENT TO REQUEST-LIKE OBJECT
    if (!isHttp) {
      // It's a CloudEvent or Background Function
      // We construct a synthetic "req" object to normalize downstream logic
      const event = reqOrEvent;
      req = {
        body: event, // CloudEvents usually have data in body or are the body
        headers: {},
        method: 'POST', // Synthetic method
        query: {}
      };
      // CloudEvents (v2) often come with data property directly
      if (event.data && typeof event.data === 'string') {
        // Handle base64 encoded data if raw
        req.body = { message: { data: event.data } };
      } else if (event.data) {
        // Direct object
        req.body = { message: { data: Buffer.from(JSON.stringify(event.data)).toString('base64') } };
      }

      // Mock Response object for the handler to use without crashing
      res = {
        status: () => res, // Chainable
        send: () => { },
        json: () => { },
        set: () => { }, // Safe no-op for headers
        headersSent: false
      };
    }

    // Generate execution ID immediately
    const executionId = `${serviceName}-${Date.now()}`;

    // Extract basic metadata for logging
    const metadata = extractMetadata(req);
    const { userId, testRunId, triggerType } = metadata;

    // Initial Logger
    const preambleLogger = logger.child({
      executionId,
      ...(userId && { user_id: userId }),
      service: serviceName
    });

    // DEBUG: Log incoming request details
    preambleLogger.debug('Incoming Request', {
      method: req.method,
      path: req.path,
      query: req.query,
      body: req.body,
      userId,
      testRunId,
      triggerType
    });

    // EARLY EXECUTION LOGGING
    // Instantiate just enough to log pending state
    // const db = admin.firestore(); // Use module-level db
    const executionStore = new ExecutionStore(db);
    const executionService = new ExecutionService(executionStore);

    // Minimal context for logger
    const loggingCtx = {
      services: { execution: executionService },
      logger: preambleLogger
    };

    try {
      await logExecutionPending(loggingCtx, executionId, serviceName, triggerType);
    } catch (e) {
      preambleLogger.error('Failed to log execution pending', { error: e });
      // Proceeding anyway, though visibility is compromised
    }

    // --- AUTHENTICATION MIDDLEWARE ---
    // (Only run Auth for HTTP triggers usually, unless payload carries auth)
    let authScopes: string[] = [];
    let authenticatedUserId = userId; // Can be overridden by auth

    // Initialize remaining stores once (singleton pattern)
    // We reuse executionStore
    const stores = {
      users: new UserStore(db),
      executions: executionStore,
      apiKeys: new ApiKeyStore(db),
      integrationIdentities: new IntegrationIdentityStore(db),
      activities: new ActivityStore(db)
    };

    // Initialize services
    const services = {
      user: new UserService(stores.users, stores.activities),
      apiKey: new ApiKeyService(stores.apiKeys),
      execution: executionService
    };

    // Auth Loop
    if (options?.auth?.strategies) {
      let authenticated = false;

      // Prepare minimal context for Auth Strategy
      const tempCtx: FrameworkContext = {
        services,
        stores,
        pubsub,
        secrets: new SecretManagerHelper(process.env.GOOGLE_CLOUD_PROJECT || process.env.GCP_PROJECT || ''),
        logger: preambleLogger,
        executionId,
        userId: authenticatedUserId
      };

      for (const strategy of options.auth.strategies) {
        try {
          const authResult = await strategy.authenticate(req, tempCtx);
          if (authResult) {
            authenticatedUserId = authResult.userId; // Auth overrides extracted user ID
            authScopes = authResult.scopes || [];
            authenticated = true;
            preambleLogger.info(`Authenticated via ${strategy.name}`, { userId: authenticatedUserId, scopes: authScopes });
            break;
          }
        } catch (e) {
          preambleLogger.warn(`Auth strategy ${strategy.name} failed`, { error: e });
        }
      }

      if (!authenticated) {
        preambleLogger.warn('Request failed authentication filters');
        res.status(401).send('Unauthorized');
        return;
      }

      // Scope Validation (Optional)
      if (options.auth.requiredScopes) {
        const hasScopes = options.auth.requiredScopes.every(scope => authScopes.includes(scope));
        if (!hasScopes) {
          preambleLogger.warn(`Authenticated user ${authenticatedUserId} missing required scopes`);
          res.status(403).send('Forbidden: Insufficient Scopes');
          return;
        }
      }
    }
    // --- END AUTH ---

    // Create context with enriched logger (if user ID changed)
    const contextLogger = logger.child({
      executionId,
      ...(authenticatedUserId && { user_id: authenticatedUserId }),
      service: serviceName
    });

    // Build the full context
    const ctx: FrameworkContext = {
      services,
      stores,
      pubsub,
      secrets: new SecretManagerHelper(process.env.GOOGLE_CLOUD_PROJECT || process.env.GCP_PROJECT || ''),
      logger: contextLogger,
      executionId,
      userId: authenticatedUserId,
      authScopes
    };

    // Capture original payload for logging
    const originalPayload = isHttp ? req.body : (req.body?.message?.data ? JSON.parse(Buffer.from(req.body.message.data, 'base64').toString()) : req.body);

    // Log execution start (update to running + payload)
    await logExecutionStart(ctx, executionId, serviceName, triggerType, originalPayload);

    try {
      // Attach execution ID to response header early (so it's present even if handler sends response)
      if (isHttp) {
        res.set('x-execution-id', executionId);
      }

      // Execute Handler
      const result = await handler(req, res, ctx);

      // Log execution success
      await logExecutionSuccess(ctx, executionId, result || {});

      contextLogger.info('Function completed successfully');

    } catch (err: any) {
      // Log execution failure
      ctx.logger.error('Function failed', { error: err.message, stack: err.stack });

      await logExecutionFailure(ctx, executionId, err);

      // Attach execution ID to response header (safety check)
      if (isHttp && !res.headersSent) {
        res.set('x-execution-id', executionId);
        res.status(500).send('Internal Server Error');
      }
    }
  };
}
