import * as admin from 'firebase-admin';
import * as converters from './converters';
import { ExecutionRecord } from '../../types/pb/execution';

/**
 * ExecutionStore provides typed access to execution-related Firestore operations.
 */
export class ExecutionStore {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get the executions collection reference.
   */
  private collection() {
    return this.db.collection('executions').withConverter(converters.executionConverter);
  }

  /**
   * Create a new execution record.
   */
  async create(executionId: string, data: ExecutionRecord): Promise<void> {
    await this.collection().doc(executionId).set(data);
  }

  /**
   * Update an execution record.
   */
  async update(executionId: string, data: Partial<ExecutionRecord>): Promise<void> {
    await this.collection().doc(executionId).update(data);
  }

  /**
   * Get an execution by ID.
   */
  async get(executionId: string): Promise<ExecutionRecord | null> {
    const doc = await this.collection().doc(executionId).get();
    if (!doc.exists) {
      return null;
    }
    return doc.data() || null;
  }

  /**
   * List executions with optional filters.
   */
  async list(filters: { service?: string, status?: string, userId?: string, limit?: number }): Promise<{ id: string, data: ExecutionRecord }[]> {
    let query: admin.firestore.Query = this.collection().orderBy('timestamp', 'desc');

    if (filters.service) {
      query = query.where('service', '==', filters.service);
    }
    if (filters.status) {
      query = query.where('status', '==', filters.status);
    }
    if (filters.userId) {
      query = query.where('user_id', '==', filters.userId);
    }

    if (filters.limit) {
      query = query.limit(filters.limit);
    }

    const snapshot = await query.get();
    return snapshot.docs.map(doc => ({ id: doc.id, data: doc.data() as ExecutionRecord }));
  }

  /**
   * Delete all executions (batched).
   */
  async deleteAll(): Promise<number> {
    let deletedCount = 0;
    const batchSize = 500;

    // eslint-disable-next-line no-constant-condition
    while (true) {
      const snapshot = await this.collection().limit(batchSize).get();
      if (snapshot.empty) {
        break;
      }

      const batch = this.db.batch();
      snapshot.docs.forEach(doc => {
        batch.delete(doc.ref);
      });

      await batch.commit();
      deletedCount += snapshot.size;
    }
    return deletedCount;
  }
}
