import { ExecutionStore } from '../../storage/firestore';
import { ExecutionRecord } from '../../types/pb/execution';

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
    return this.executionStore.create(executionId, data);
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
    return this.executionStore.list(filters);
  }

  async deleteAllExecutions(): Promise<number> {
    return this.executionStore.deleteAll();
  }
}
