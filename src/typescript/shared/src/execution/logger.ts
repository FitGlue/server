import * as winston from 'winston';
import { ExecutionService } from '../domain/services';
import { ExecutionStatus } from '../types/pb/execution';

/**
 * Logs the pending state of a function execution.
 */
export async function logExecutionPending(
  ctx: { services: { execution: ExecutionService }; logger: winston.Logger },
  executionId: string,
  functionName: string,
  trigger: string
): Promise<void> {
  ctx.logger.info(`Execution pending`, { executionId, trigger });

  await ctx.services.execution.create(executionId, {
    executionId,
    service: functionName,
    triggerType: trigger,
    timestamp: new Date(),
    status: ExecutionStatus.STATUS_PENDING
  });
}

/**
 * Logs the start of a function execution.
 */
export async function logExecutionStart(
  ctx: { services: { execution: ExecutionService }; logger: winston.Logger },
  executionId: string,
  trigger: string,
  originalPayload?: any
): Promise<void> {
  ctx.logger.info(`Execution started`, { executionId, trigger });

  // Update existing record to running
  await ctx.services.execution.update(executionId, {
    startTime: new Date(),
    status: ExecutionStatus.STATUS_STARTED,
    inputsJson: originalPayload ? JSON.stringify(originalPayload) : undefined
  });
}

/**
 * Logs successful completion of a function execution.
 */
export async function logExecutionSuccess(
  ctx: { services: { execution: ExecutionService }; logger: winston.Logger },
  executionId: string,
  result?: any
): Promise<void> {
  ctx.logger.info(`Execution completed successfully`, { executionId });

  await ctx.services.execution.update(executionId, {
    endTime: new Date(),
    status: ExecutionStatus.STATUS_SUCCESS,
    outputsJson: result ? JSON.stringify(result) : undefined
  });
}

/**
 * Logs failed execution.
 */
export async function logExecutionFailure(
  ctx: { services: { execution: ExecutionService }; logger: winston.Logger },
  executionId: string,
  error: Error,
  result?: any
): Promise<void> {
  ctx.logger.error(`Execution failed`, { executionId, error: error.message, stack: error.stack, result });

  await ctx.services.execution.update(executionId, {
    endTime: new Date(),
    status: ExecutionStatus.STATUS_FAILED,
    errorMessage: error.message,
    outputsJson: result ? JSON.stringify(result) : undefined
  });
}
