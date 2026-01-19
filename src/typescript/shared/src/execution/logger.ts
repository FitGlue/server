import * as winston from 'winston';
import { ExecutionService } from '../domain/services';
import { ExecutionStatus } from '../types/pb/execution';

// Maximum size for outputsJson (Firestore field limit is ~1MB, use 512KB for safety margin)
const MAX_OUTPUTS_JSON_SIZE = 512 * 1024; // 512KB

/**
 * Safely stringify result with size limit.
 * If result is too large, returns a truncated summary with metadata.
 */
function safeStringifyOutput(result: unknown): string | undefined {
  if (!result) return undefined;

  const json = JSON.stringify(result);
  if (json.length > MAX_OUTPUTS_JSON_SIZE) {
    // Return a truncated summary with metadata about what was omitted
    const summary = {
      _truncated: true,
      _originalSize: json.length,
      _limit: MAX_OUTPUTS_JSON_SIZE,
      _message: 'Output too large for logging, truncated for storage'
    };
    return JSON.stringify(summary);
  }
  return json;
}

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
  originalPayload?: unknown,
  pipelineExecutionId?: string
): Promise<void> {
  ctx.logger.info(`Execution started`, { executionId, trigger, pipelineExecutionId });

  // Update existing record to running
  await ctx.services.execution.update(executionId, {
    startTime: new Date(),
    status: ExecutionStatus.STATUS_STARTED,
    inputsJson: originalPayload ? JSON.stringify(originalPayload) : undefined,
    pipelineExecutionId
  });
}

/**
 * Logs successful completion of a function execution.
 */
export async function logExecutionSuccess(
  ctx: { services: { execution: ExecutionService }; logger: winston.Logger },
  executionId: string,
  result?: unknown
): Promise<void> {
  ctx.logger.info(`Execution completed successfully`, { executionId });

  await ctx.services.execution.update(executionId, {
    endTime: new Date(),
    status: ExecutionStatus.STATUS_SUCCESS,
    outputsJson: safeStringifyOutput(result)
  });
}

/**
 * Logs failed execution.
 */
export async function logExecutionFailure(
  ctx: { services: { execution: ExecutionService }; logger: winston.Logger },
  executionId: string,
  error: Error,
  result?: unknown
): Promise<void> {
  ctx.logger.error(`Execution failed`, { executionId, error: error.message, stack: error.stack, result });

  await ctx.services.execution.update(executionId, {
    endTime: new Date(),
    status: ExecutionStatus.STATUS_FAILED,
    errorMessage: error.message,
    outputsJson: safeStringifyOutput(result)
  });
}
