import * as winston from 'winston';

/**
 * Logs the pending state of a function execution.
 */
export async function logExecutionPending(
  ctx: { services: { execution: any }; logger: winston.Logger },
  executionId: string,
  functionName: string,
  trigger: string
): Promise<void> {
  ctx.logger.info(`[${functionName}] Execution pending`, { executionId, trigger });

  await ctx.services.execution.create(executionId, {
    functionName,
    trigger,
    startedAt: new Date(),
    status: 'pending'
  });
}

/**
 * Logs the start of a function execution.
 */
export async function logExecutionStart(
  ctx: { services: { execution: any }; logger: winston.Logger },
  executionId: string,
  functionName: string,
  trigger: string,
  originalPayload?: any
): Promise<void> {
  ctx.logger.info(`[${functionName}] Execution started`, { executionId, trigger });

  // Update existing record to running
  await ctx.services.execution.update(executionId, {
    startedAt: new Date(),
    status: 'running',
    inputsJson: originalPayload ? JSON.stringify(originalPayload) : undefined
  });
}

/**
 * Logs successful completion of a function execution.
 */
export async function logExecutionSuccess(
  ctx: { services: { execution: any }; logger: winston.Logger },
  executionId: string,
  result?: any
): Promise<void> {
  ctx.logger.info(`Execution completed successfully`, { executionId });

  await ctx.services.execution.update(executionId, {
    completedAt: new Date(),
    status: 'success',
    result
  });
}

/**
 * Logs failed execution.
 */
export async function logExecutionFailure(
  ctx: { services: { execution: any }; logger: winston.Logger },
  executionId: string,
  error: Error
): Promise<void> {
  ctx.logger.error(`Execution failed`, { executionId, error: error.message, stack: error.stack });

  await ctx.services.execution.update(executionId, {
    completedAt: new Date(),
    status: 'failed',
    error: {
      message: error.message,
      stack: error.stack
    }
  });
}
