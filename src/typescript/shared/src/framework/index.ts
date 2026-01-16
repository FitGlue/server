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
import { AuthorizationService } from '../domain/services/authorization';

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
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const gcpInfo: any = {
            timestamp: info.timestamp,
            ...info,
            severity: info.level.toUpperCase(),
            message: info.component ? `[${info.component}] ${info.message}` : info.message
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
    authorization: AuthorizationService;
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
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  req: any,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  res: any,
  ctx: FrameworkContext
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
) => Promise<any>;

export interface CloudFunctionOptions {
  auth?: {
    strategies: AuthStrategy[]; // Only accept strategy instances
    requiredScopes?: string[];
  };
  /**
   * Set to true for public endpoints that don't require authentication.
   * If false/undefined and no auth.strategies, createCloudFunction throws.
   * Cannot have auth strategies *and* allowUnauthenticated.
   */
  allowUnauthenticated?: boolean;
}

/**
 * Extract metadata from HTTP request
 * Handles both HTTP requests and Pub/Sub messages
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function extractMetadata(req: any): { userId?: string; testRunId?: string; pipelineExecutionId?: string; triggerType: string } {
  let userId: string | undefined;
  let testRunId: string | undefined;
  let pipelineExecutionId: string | undefined;
  let triggerType = 'http';

  // Check if this is a Pub/Sub message (has message.data structure)
  if (req.body && req.body.message && req.body.message.data) {
    triggerType = 'pubsub';

    // Decode base64 Pub/Sub message data
    try {
      const messageData = Buffer.from(req.body.message.data, 'base64').toString('utf-8');
      const payload = JSON.parse(messageData);
      userId = payload.user_id || payload.userId;
      // Extract from payload
      pipelineExecutionId = payload.pipeline_execution_id || payload.pipelineExecutionId;
    } catch (e) {
      // If parsing fails, continue without user_id
    }

    // Check Pub/Sub message attributes for test_run_id
    if (req.body.message.attributes) {
      testRunId = req.body.message.attributes.test_run_id || req.body.message.attributes.testRunId;
      // Also check attributes for pipeline_execution_id if not in payload
      if (!pipelineExecutionId) {
        pipelineExecutionId = req.body.message.attributes.pipeline_execution_id || req.body.message.attributes.pipelineExecutionId;
      }
    }
  } else {
    // HTTP request
    // Try to extract user_id from request body
    if (req.body) {
      userId = req.body.user_id || req.body.userId;
      pipelineExecutionId = req.body.pipeline_execution_id || req.body.pipelineExecutionId;
    }

    // Try to extract metadata from headers (check both formats)
    if (req.headers) {
      testRunId = req.headers['x-test-run-id'] || req.headers['x-testrunid'];
      if (!pipelineExecutionId) {
        pipelineExecutionId = req.headers['x-pipeline-execution-id'];
      }
    }
  }

  return { userId, testRunId, pipelineExecutionId, triggerType };
}

export const createCloudFunction = (handler: FrameworkHandler, options?: CloudFunctionOptions) => {
  // SECURITY: Require auth by default - handlers must explicitly opt out
  const hasAuth = options?.auth?.strategies && options.auth.strategies.length > 0;
  const isPublic = options?.allowUnauthenticated === true;

  if (!hasAuth && !isPublic) {
    throw new Error(
      'Security: Auth required. Add auth.strategies or set allowUnauthenticated: true'
    );
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
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

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let capturedResponse: any = undefined;

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
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        send: (body: any) => { capturedResponse = body; },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        json: (body: any) => { capturedResponse = body; },
        set: () => { }, // Safe no-op for headers
        headersSent: false
      };
    } else {
      // HTTP Trigger: Wrap res.send and res.json to capture output
      const originalSend = res.send.bind(res);
      const originalJson = res.json.bind(res);

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      res.send = (body: any) => {
        capturedResponse = body;
        return originalSend(body);
      };

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      res.json = (body: any) => {
        capturedResponse = body;
        return originalJson(body);
      };
    }

    // Generate execution ID immediately
    const executionId = `${serviceName}-${Date.now()}`;

    // Extract basic metadata for logging
    const metadata = extractMetadata(req);
    const { userId, testRunId, triggerType, pipelineExecutionId } = metadata;

    // Use current executionId as pipelineExecutionId if not provided (Root Execution)
    const currentPipelineExecutionId = pipelineExecutionId || executionId;

    // Initial Logger
    const preambleLogger = logger.child({
      executionId,
      ...(userId && { user_id: userId }),
      component: 'framework'
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
      execution: executionService,
      authorization: new AuthorizationService(stores.users)
    };

    // Full context reference (mutable so we can assign it as we build it)
    let ctx: FrameworkContext | undefined;

    try {
      // Auth Loop
      if (options?.auth?.strategies && options.auth.strategies.length > 0) {
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
          const msg = 'Request failed authentication filters';
          preambleLogger.warn(msg);

          // Log execution failure for Auth rejection
          await logExecutionFailure(loggingCtx, executionId, new Error(msg));

          res.status(401).send('Unauthorized');
          return;
        }

        // Scope Validation (Optional)
        if (options.auth.requiredScopes) {
          const hasScopes = options.auth.requiredScopes.every(scope => authScopes.includes(scope));
          if (!hasScopes) {
            const msg = `Authenticated user ${authenticatedUserId} missing required scopes`;
            preambleLogger.warn(msg);

            // Log execution failure for Scope rejection
            await logExecutionFailure(loggingCtx, executionId, new Error(msg));

            res.status(403).send('Forbidden: Insufficient Scopes');
            return;
          }
        }
      }
      // --- END AUTH ---

      // Build the full context
      ctx = {
        services,
        stores,
        pubsub,
        secrets: new SecretManagerHelper(process.env.GOOGLE_CLOUD_PROJECT || process.env.GCP_PROJECT || ''),
        logger: logger.child({
          executionId,
          ...(authenticatedUserId && { user_id: authenticatedUserId }),
          component: 'context'
        }),
        executionId,
        userId: authenticatedUserId,
        authScopes
      };

      // Capture original payload for logging
      const originalPayload = isHttp ? req.body : (req.body?.message?.data ? JSON.parse(Buffer.from(req.body.message.data, 'base64').toString()) : req.body);

      // Log execution start (update to running + payload)
      await logExecutionStart(loggingCtx, executionId, triggerType, originalPayload, currentPipelineExecutionId);

      // Attach execution ID to response header early (so it's present even if handler sends response)
      if (isHttp) {
        res.set('x-execution-id', executionId);
      }

      // Execute Handler
      const result = await handler(req, res, ctx);

      // Check HTTP Status for failure
      if (isHttp && res.statusCode && res.statusCode >= 400) {
        // It was a handled error (e.g. 400 Bad Request, 404 Not Found), but still an execution failure
        // from the perspective of "did the task succeed?"
        const errorMsg = `HTTP Error ${res.statusCode}`;

        // Merge result/capturedResponse for failure context as well
        let finalResult = capturedResponse || result;
        if (capturedResponse && typeof capturedResponse === 'object' && result && typeof result === 'object') {
          finalResult = { ...result, ...capturedResponse };
        }

        await logExecutionFailure(loggingCtx, executionId, new Error(errorMsg), finalResult);
      } else {
        // Log execution success
        // Combine capturedResponse and result if both exist and are objects, to maximize visibility
        let finalResult = capturedResponse || result || {};
        if (capturedResponse && typeof capturedResponse === 'object' && result && typeof result === 'object') {
          finalResult = { ...result, ...capturedResponse };
        }

        await logExecutionSuccess(ctx, executionId, finalResult);
      }

      preambleLogger.info('Function completed successfully');

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } catch (err: any) {
      // Log execution failure
      preambleLogger.error('Function failed', { error: err.message, stack: err.stack });

      if (capturedResponse && typeof capturedResponse === 'object') {
        await logExecutionFailure(loggingCtx, executionId, err, capturedResponse);
      } else {
        await logExecutionFailure(loggingCtx, executionId, err);
      }

      // Attach execution ID to response header (safety check)
      if (isHttp && !res.headersSent) {
        res.set('x-execution-id', executionId);
        res.status(500).send('Internal Server Error');
      }
    }
  };
}
