import { HttpFunction } from '@google-cloud/functions-framework';
import * as admin from 'firebase-admin';
import * as winston from 'winston';
import { logExecutionStart, logExecutionSuccess, logExecutionFailure } from '../execution/logger';
import { AuthStrategy, ApiKeyStrategy } from './auth';

import { PubSub } from '@google-cloud/pubsub';

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
  db: admin.firestore.Firestore;
  pubsub: PubSub;
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
    strategies: ('api_key')[]; // Extensible list of strategy names
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

const AUTHORIZED_STRATEGIES: Record<string, AuthStrategy> = {
  'api_key': new ApiKeyStrategy()
};

export function createCloudFunction(handler: FrameworkHandler, options?: CloudFunctionOptions): HttpFunction {
  return async (req, res) => {
    const serviceName = process.env.K_SERVICE || 'unknown-function';

    // Extract metadata from request (handles both HTTP and Pub/Sub)
    let { userId } = extractMetadata(req);
    const { testRunId, triggerType } = extractMetadata(req);

    // Initial Logger (Pre-Auth)
    const preambleLogger = logger.child({});

    // DEBUG: Log incoming request details
    preambleLogger.debug('Incoming Request', {
      method: req.method,
      path: req.path,
      query: req.query,
      headers: req.headers,
      body: req.body // Log full body for debugging (only happens at debug log level)
    });

    // --- AUTHENTICATION MIDDLEWARE ---
    let authScopes: string[] = [];
    if (options?.auth?.strategies && options.auth.strategies.length > 0) {
      let authenticated = false;

      // Prepare context for Auth Strategy
      const tempCtx: FrameworkContext = {
        db,
        pubsub,
        logger: preambleLogger,
        executionId: 'pre-auth'
      };

      for (const strategyName of options.auth.strategies) {
        const strategy = AUTHORIZED_STRATEGIES[strategyName];
        if (strategy) {
          try {
            const result = await strategy.authenticate(req, tempCtx);
            if (result) {
              userId = result.userId; // Auth overrides extracted user ID
              authScopes = result.scopes;
              authenticated = true;
              break;
            }
          } catch (e) {
            preambleLogger.warn(`Auth strategy ${strategyName} failed`, { error: e });
          }
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

    // Log execution start
    let executionId: string;
    try {
      executionId = await logExecutionStart(db, serviceName, {
        userId,
        testRunId,
        triggerType,
      });
    } catch (e) {
      logger.warn('Failed to log execution start', { error: e });
      executionId = 'unknown';
    }

    // Create context with enriched logger
    const contextLogger = logger.child({
      executionId,
      ...(userId && { user_id: userId })
    });

    const ctx: FrameworkContext = {
      db,
      pubsub,
      logger: contextLogger,
      executionId,
      userId,
      authScopes
    };

    contextLogger.info('Function started');

    try {
      // Attach execution ID to response header early (so it's present even if handler sends response)
      res.set('x-execution-id', executionId);

      // Execute Handler
      const result = await handler(req, res, ctx);

      // Log execution success
      await logExecutionSuccess(db, executionId, result || {});

      contextLogger.info('Function completed successfully');

    } catch (err: any) {
      // Log execution failure
      ctx.logger.error('Function failed', { error: err.message, stack: err.stack });

      await logExecutionFailure(db, executionId, err);

      // Attach execution ID to response header (safety check)
      if (!res.headersSent) {
        res.set('x-execution-id', executionId);
        res.status(500).send('Internal Server Error');
      }
    }
  };
}
