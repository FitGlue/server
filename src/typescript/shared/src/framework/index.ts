import * as admin from 'firebase-admin';
import * as winston from 'winston';
import { Request } from 'express';
import { logExecutionStart, logExecutionSuccess, logExecutionFailure, logExecutionPending } from '../execution/logger';
import { AuthStrategy } from './auth';

// Re-export framework components
export * from './connector';
export * from './base-connector';
export * from './webhook-processor';
export * from './errors';
export * from './auth';
export * from './auth-strategies';

import { PubSub } from '@google-cloud/pubsub';
import { SecretManagerServiceClient } from '@google-cloud/secret-manager';
import { UserStore, ExecutionStore, ApiKeyStore, IntegrationIdentityStore, ActivityStore, PipelineStore, PipelineRunStore, PluginDefaultsStore } from '../storage/firestore';
import { UserService, ApiKeyService, ExecutionService } from '../domain/services';
import { AuthorizationService } from '../domain/services/authorization';
import * as SentryModule from '../infrastructure/sentry';

// Re-export Sentry utilities for handlers to use directly
export const captureException = SentryModule.captureException;
export const flushSentry = SentryModule.flushSentry;

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

// Helper to serialize Error objects for logging
// eslint-disable-next-line @typescript-eslint/no-explicit-any
// Helper to serialize Error objects for logging
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function serializeErrors(obj: any, visited = new WeakSet<any>()): any {
  // Primitives
  if (!obj || typeof obj !== 'object') {
    return obj;
  }

  // Check for cycles
  if (visited.has(obj)) {
    return '[Circular]';
  }
  visited.add(obj);

  if (obj instanceof Error) {
    return {
      message: obj.message,
      name: obj.name,
      stack: obj.stack,
      // Include any custom properties
      ...Object.getOwnPropertyNames(obj).reduce((acc, key) => {
        if (!['message', 'name', 'stack'].includes(key)) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          acc[key] = serializeErrors((obj as any)[key], visited);
        }
        return acc;
      }, {} as Record<string, unknown>)
    };
  }

  if (Array.isArray(obj)) {
    return obj.map(item => serializeErrors(item, visited));
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const result: any = {};
  for (const key of Object.keys(obj)) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    result[key] = serializeErrors(obj[key], visited);
  }
  // Preserve Symbol properties (Winston uses Symbol.for('level'), Symbol.for('message'),
  // Symbol.for('splat') internally — dropping these causes silent log loss)
  for (const sym of Object.getOwnPropertySymbols(obj)) {
    result[sym] = obj[sym];
  }
  return result;
}

// Custom format to properly serialize Error objects in metadata
const errorSerializer = winston.format((info) => {
  return serializeErrors(info);
});

// Configure Structured Logging
const logLevel = (process.env.LOG_LEVEL || 'info').toLowerCase();
const logger = winston.createLogger({
  level: logLevel, // Use configured level
  format: winston.format.combine(
    errorSerializer(),
    winston.format.json()
  ),
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
            // Map Winston levels to GCP Cloud Logging severity strings
            // (Winston uses 'warn' but GCP expects 'WARNING')
            severity: info.level === 'warn' ? 'WARNING' : info.level.toUpperCase(),
            message: info.component ? `[${info.component}] ${info.message}` : info.message
          };
          // Remove default keys to avoid duplication/conflict
          delete gcpInfo.level;

          // Use safer stringify here too just in case
          try {
            return JSON.stringify(gcpInfo);
          } catch {
            return JSON.stringify({
              severity: 'ERROR',
              message: 'Failed to serialize log message',
              originalMessage: info.message
            });
          }
        })
      )
    })
  ]
});

// Initialize Sentry
const sentryDsn = process.env.SENTRY_DSN;
const environment = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-server-dev';
const release = process.env.SENTRY_RELEASE || process.env.K_REVISION || 'unknown';

SentryModule.initSentry({
  dsn: sentryDsn,
  environment,
  release,
  serverName: process.env.K_SERVICE,
  tracesSampleRate: environment.includes('prod') ? 0.1 : 1.0,
  profilesSampleRate: environment.includes('prod') ? 0.1 : 1.0,
}, logger);

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
    pipelines: import('../storage/firestore').PipelineStore;
    pipelineRuns: import('../storage/firestore').PipelineRunStore;
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

// ... (previous code)

// ... (previous code)

export class FrameworkResponse {
  constructor(
    public readonly options: {
      status?: number;
      body?: unknown;
      headers?: Record<string, string>;
    }
  ) { }
}

export type SafeHandler = (
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  req: any,
  ctx: FrameworkContext
) => Promise<unknown | FrameworkResponse>;

/**
 * FrameworkHandler is the standard signature for Cloud Function handlers.
 * Use this type to avoid manually specifying parameter types.
 *
 * @example
 * export const handler: FrameworkHandler = async (req, ctx) => {
 *   // TypeScript will infer req: Request and ctx: FrameworkContext
 * };
 */
export type FrameworkHandler = (
  req: Request,
  ctx: FrameworkContext
) => Promise<unknown | FrameworkResponse>;

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
  /**
   * Set to true to skip writing execution records to Firestore.
   * Use for user-facing API handlers where execution traces aren't needed.
   * Pipeline handlers (sources, enricher, router, destinations) should NOT set this.
   */
  skipExecutionLogging?: boolean;
}

/**
 * Extract metadata from HTTP request
 * Handles both HTTP requests and Pub/Sub messages
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any, complexity
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
    } catch {
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

// eslint-disable-next-line max-lines-per-function, complexity
export const createCloudFunction = (handler: SafeHandler, options?: CloudFunctionOptions) => {
  // SECURITY: Require auth by default - handlers must explicitly opt out
  const hasAuth = options?.auth?.strategies && options.auth.strategies.length > 0;
  const isPublic = options?.allowUnauthenticated === true;
  const shouldLogExecution = options?.skipExecutionLogging !== true;

  if (!hasAuth && !isPublic) {
    throw new Error(
      'Security: Auth required. Add auth.strategies or set allowUnauthenticated: true'
    );
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any, max-lines-per-function, complexity
  return async (reqOrEvent: any, resOrContext?: any) => {
    const serviceName = process.env.K_SERVICE || 'unknown-function';
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'unknown';
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const isProd = projectId.includes('-prod'); // Determine environment

    // DETECT TRIGGER TYPE
    // HTTP: (req, res)
    // CloudEvent: (event)
    // Background (Legacy): (data, context)

    let isHttp = false;
    let req = reqOrEvent;
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
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
      // HTTP Trigger: We STILL wrap res methods to capture what's being sent,
      // ALTHOUGH SafeHandler shouldn't use them directly.
      // This is a safety net if legacy code persists or specialized headers are needed.
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
      // body: req.body, // Omitted for brevity/privacy in logs
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

    if (shouldLogExecution) {
      try {
        await logExecutionPending(loggingCtx, executionId, serviceName, triggerType);
      } catch (e) {
        preambleLogger.error('Failed to log execution pending', { error: e });
        // Proceeding anyway, though visibility is compromised
      }
    }

    // --- AUTHENTICATION MIDDLEWARE ---
    let authScopes: string[] = [];
    let authenticatedUserId = userId; // Can be overridden by auth

    // Initialize remaining stores once (singleton pattern)
    // We reuse executionStore
    const stores = {
      users: new UserStore(db),
      executions: executionStore,
      apiKeys: new ApiKeyStore(db),
      integrationIdentities: new IntegrationIdentityStore(db),
      activities: new ActivityStore(db),
      pipelines: new PipelineStore(db),
      pipelineRuns: new PipelineRunStore(db),
      pluginDefaults: new PluginDefaultsStore(db)
    };

    // Initialize services
    const services = {
      user: new UserService(stores.users, stores.activities, stores.pipelines, stores.pluginDefaults),
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
          // Throw specific 401
          // We'll catch this in the specialized catch block below
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const err: any = new Error(msg);
          err.statusCode = 401;
          throw err;
        }

        // Scope Validation (Optional)
        if (options.auth.requiredScopes) {
          const hasScopes = options.auth.requiredScopes.every(scope => authScopes.includes(scope));
          if (!hasScopes) {
            const msg = `Authenticated user ${authenticatedUserId} missing required scopes`;
            // Throw specific 403
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const err: any = new Error(msg);
            err.statusCode = 403;
            throw err;
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
      if (shouldLogExecution) {
        await logExecutionStart(loggingCtx, executionId, triggerType, originalPayload, currentPipelineExecutionId);
      }

      // Attach execution ID to response header early (so it's present even if handler sends response)
      if (isHttp) {
        res.set('x-execution-id', executionId);
      }

      // Execute Handler (SafeHandler expects return value)
      // IMPORTANT: call handler(req, ctx) - NO res, NO next
      let result = await handler(req, ctx);

      // Handle FrameworkResponse wrapper
      if (result instanceof FrameworkResponse) {
        const fwRes = result as FrameworkResponse;

        // Apply headers
        if (fwRes.options.headers) {
          for (const [key, value] of Object.entries(fwRes.options.headers)) {
            res.set(key, value);
          }
        }

        // Set status
        if (fwRes.options.status) {
          res.status(fwRes.options.status);
        } else {
          res.status(200);
        }

        // Send body
        // If body is undefined but status is something like 204 or 302, send() empty
        if (fwRes.options.body !== undefined) {
          // If body is object/array, use json(), else send()
          if (typeof fwRes.options.body === 'object') {
            res.json(fwRes.options.body);
          } else {
            res.send(fwRes.options.body);
          }
        } else {
          res.send();
        }

        // Update capturedResponse for logs
        capturedResponse = fwRes.options.body;
        // Unwrap result for logs (so we log the body, not the wrapper)
        result = fwRes.options.body;
      } else {
        // STANDARD RETURN PATTERN
        // If result is returned and headers not sent, send 200 OK
        if (isHttp && !res.headersSent && result !== undefined) {
          res.status(200).json(result);
          capturedResponse = result;
        } else if (isHttp && !res.headersSent) {
          // If undefined returned, assume 204 No Content or similar?
          // Or user forgot to return?
          // For safety, if no response sent, send 200 OK empty
          res.status(200).send();
        }
      }

      // Log success
      if (shouldLogExecution) {
        // Construct final result for logging
        let finalResult = capturedResponse || result || {};
        // Merge if both exist and are objects
        if (capturedResponse && typeof capturedResponse === 'object' && result && typeof result === 'object') {
          finalResult = { ...result, ...capturedResponse };
        }
        await logExecutionSuccess(ctx, executionId, finalResult);
      }

      preambleLogger.info('Function completed successfully');

    } catch (err: unknown) {
      // CENTRALIZED ERROR HANDLING
      const error = err as Error & { statusCode?: number; stack?: string };
      const statusCode = error.statusCode || 500;
      const isSystemError = statusCode >= 500;

      // Log execution failure
      preambleLogger.error('Function failed', { error: error.message, stack: error.stack, statusCode });

      // Capture error in Sentry (Only for 500s or explicit system errors)
      if (isSystemError) {
        const { captureException, flushSentry } = await import('../infrastructure/sentry');
        captureException(error, {
          service: serviceName,
          execution_id: executionId,
          user_id: authenticatedUserId,
          trigger_type: triggerType,
          status_code: statusCode
        }, preambleLogger);

        // Critical: Wait for Sentry to send the event before function terminates
        await flushSentry(2000);
      }

      // Log Failure to Execution Store
      if (shouldLogExecution) {
        // Pass captured response if any (e.g. partial writes)
        await logExecutionFailure(loggingCtx, executionId, error, capturedResponse);
      }

      // Send Error Response (if not already sent)
      if (isHttp && !res.headersSent) {
        res.set('x-execution-id', executionId);

        // Response Body Logic:
        // Prod + 500 = Generic Message
        // Dev/Test OR < 500 = Specific Message
        const showDetails = !isProd; // Show details if NOT prod

        // If it's a 4xx error, we always show the message (client error)
        // If it's a 5xx error, we hide it in prod
        let message = error.message || 'Internal Server Error';
        if (isProd && isSystemError) {
          message = 'Internal Server Error';
        }

        const responseBody: Record<string, unknown> = { error: message };
        if (showDetails && error.stack) {
          responseBody.stack = error.stack;
        }

        res.status(statusCode).json(responseBody);
      }
    }
  };
};
