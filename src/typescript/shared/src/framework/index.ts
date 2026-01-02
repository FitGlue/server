import * as admin from 'firebase-admin';
import * as winston from 'winston';
import { logExecutionStart, logExecutionSuccess, logExecutionFailure } from '../execution/logger';
import { AuthStrategy } from './auth';
export * from './connector';
export * from './base-connector';
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
const db = admin.firestore();

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
  pubsub: PubSub;
  secrets: SecretsHelper;
  logger: winston.Logger;
  executionId: string;
  userId?: string;
  authScopes?: string[];
}

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

    // Extract metadata from request (handles both HTTP and Pub/Sub)
    let { userId } = extractMetadata(req);
    const { testRunId, triggerType } = extractMetadata(req);

    // Initial Logger (Pre-Auth)
    const preambleLogger = logger.child({});

    // DEBUG: Log incoming request details
    if (isHttp) {
      preambleLogger.debug('Incoming HTTP Request', {
        method: req.method,
        path: req.path,
        query: req.query,
        headers: req.headers,
        body: req.body, // Log full body for debugging (only happens at debug log level)
        testRunId
      });
    } else {
      preambleLogger.debug('Incoming Event', {
        triggerType,
        body: req.body,
        testRunId
      });
    }

    // --- AUTHENTICATION MIDDLEWARE ---
    // (Only run Auth for HTTP triggers usually, unless payload carries auth)
    let authScopes: string[] = [];

    // Initialize stores once (singleton pattern)
    const stores = {
      users: new UserStore(db),
      executions: new ExecutionStore(db),
      apiKeys: new ApiKeyStore(db),
      integrationIdentities: new IntegrationIdentityStore(db),
      activities: new ActivityStore(db)
    };

    // Initialize services (singleton pattern) - services use stores
    const services = {
      user: new UserService(stores.users, stores.activities),
      apiKey: new ApiKeyService(stores.apiKeys),
      execution: new ExecutionService(stores.executions)
    };

    // Auth Loop
    if (options?.auth?.strategies) {
      let authenticated = false;

      // Prepare context for Auth Strategy
      const tempCtx: FrameworkContext = {
        services,
        pubsub,
        secrets: new SecretManagerHelper(process.env.GOOGLE_CLOUD_PROJECT || process.env.GCP_PROJECT || ''),
        logger: preambleLogger,
        executionId: 'pre-auth'
      };

      for (const strategy of options.auth.strategies) {
        try {
          const authResult = await strategy.authenticate(req, tempCtx);
          if (authResult) {
            userId = authResult.userId; // Auth overrides extracted user ID
            authScopes = authResult.scopes || [];
            authenticated = true;
            preambleLogger.info(`Authenticated via ${strategy.name}`, { userId, scopes: authScopes });
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
          preambleLogger.warn(`Authenticated user ${userId} missing required scopes`);
          res.status(403).send('Forbidden: Insufficient Scopes');
          return;
        }
      }
    }
    // --- END AUTH ---

    // Generate execution ID
    const executionId = `${serviceName}-${Date.now()}`;

    // Create context with enriched logger
    const contextLogger = logger.child({
      executionId,
      ...(userId && { user_id: userId })
    });

    // Build the full context
    const ctx: FrameworkContext = {
      services,
      pubsub,
      secrets: new SecretManagerHelper(process.env.GOOGLE_CLOUD_PROJECT || process.env.GCP_PROJECT || ''),
      logger: contextLogger,
      executionId,
      userId,
      authScopes
    };

    // Log execution start (now that we have services)
    await logExecutionStart(ctx, executionId, serviceName, triggerType);

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
