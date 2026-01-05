import { ExecutionStore } from '../../storage/firestore';
import { ExecutionRecord } from '../../types/pb/execution';

import { ExecutionStatus } from '../../types/pb/execution';

// Helper to convert string status input to ExecutionStatus enum value (or undefined)
function resolveExecutionStatus(statusInput: string | undefined): number | undefined {
  if (statusInput === undefined || statusInput === null || statusInput === '') return undefined;

  // Check if input is "STATUS_FAILED" or just "FAILED"
  const normalized = statusInput.toUpperCase().startsWith('STATUS_')
    ? statusInput.toUpperCase()
    : `STATUS_${statusInput.toUpperCase()}`;

  // Check key existence in enum (ExecutionStatus is a numeric enum)
  if (normalized in ExecutionStatus) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return (ExecutionStatus as any)[normalized];
  }

  return undefined;
}

/**
 * ExecutionService provides business logic for execution tracking.
 */
export class ExecutionService {
  constructor(private executionStore: ExecutionStore) { }

  /**
   * Create a new execution record.
   * With proper required/optional fields, all required fields must be provided.
   */
  async create(executionId: string, data: ExecutionRecord): Promise<void> {
    const record = { ...data };
    if (!record.expireAt) {
      record.expireAt = new Date(Date.now() + (7 * 24 * 60 * 60 * 1000)); // Default 7 days retention
    }
    return this.executionStore.create(executionId, record);
  }

  /**
   * Update an execution record.
   */
  async update(executionId: string, data: Partial<ExecutionRecord>): Promise<void> {
    return this.executionStore.update(executionId, data);
  }

  /**
   * Get an execution by ID.
   */
  async get(executionId: string): Promise<ExecutionRecord | null> {
    return this.executionStore.get(executionId);
  }

  async listExecutions(filters: { service?: string, status?: string, userId?: string, limit?: number }): Promise<{ id: string, data: ExecutionRecord }[]> {
    return this.executionStore.list({
      ...filters,
      status: resolveExecutionStatus(filters.status)
    });
  }

  watchExecutions(filters: { service?: string, status?: string, userId?: string, limit?: number }, onNext: (executions: { id: string, data: ExecutionRecord }[]) => void, onError?: (error: Error) => void): () => void {
    return this.executionStore.watch({
      ...filters,
      status: resolveExecutionStatus(filters.status)
    }, onNext, onError);
  }

  async deleteAllExecutions(): Promise<number> {
    return this.executionStore.deleteAll();
  }
}


