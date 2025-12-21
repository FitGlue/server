import { HttpFunction } from '@google-cloud/functions-framework';
import * as admin from 'firebase-admin';
import * as winston from 'winston';

// Initialize Firebase (Idempotent)
if (admin.apps.length === 0) {
  admin.initializeApp();
}
const db = admin.firestore();

// Configure Structured Logging
const logger = winston.createLogger({
  level: 'info',
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
  logger: winston.Logger;
  executionId: string;
}

export type FrameworkHandler = (
  req: any,
  res: any,
  ctx: FrameworkContext
) => Promise<any>;

export function createCloudFunction(handler: FrameworkHandler): HttpFunction {
  return async (req, res) => {
    const timestamp = new Date().toISOString();

    // Auto-create Execution ID
    const executionRef = db.collection('executions').doc();
    const executionId = executionRef.id;

    const ctx: FrameworkContext = {
      db,
      logger: logger.child({ executionId }),
      executionId
    };

    // 1. Audit Log Start
    try {
        // Try to identify service name from env or default
        const serviceName = process.env.K_SERVICE || 'unknown-function';

        await executionRef.set({
            service: serviceName,
            status: 'STARTED',
            startTime: timestamp,
            trigger: 'http'
        });
    } catch (e) {
        ctx.logger.warn('Failed to write audit log start', { error: e });
    }

    try {
      // 2. Execute Handler
      const result = await handler(req, res, ctx);

      // 3. Audit Log Success
      if (!res.headersSent) {
          // If handler didn't send response, we send a default one?
          // Use Case: Handler might just return data and we send it.
          // For now, assume handler manages response OR returns data.
          // Let's assume handler sends response for flexibility, but returns output for logging.
      }

      // Update execution with result if provided (optional)
      const outputs = result || {};

      await executionRef.update({
          status: 'SUCCESS',
          outputs: outputs,
          endTime: new Date().toISOString()
      });

    } catch (err: any) {
      // 4. Audit Log Failure
      ctx.logger.error('Function failed', { error: err.message, stack: err.stack });

      await executionRef.update({
          status: 'FAILED',
          error: err.message || 'Unknown error',
          endTime: new Date().toISOString()
      });

      if (!res.headersSent) {
        res.status(500).send('Internal Server Error');
      }
    }
  };
}
