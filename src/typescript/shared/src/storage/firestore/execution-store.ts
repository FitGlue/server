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
    const firestoreData = converters.mapExecutionPartialToFirestore(data);
    await this.collection().doc(executionId).update(firestoreData);
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
  async list(filters: { service?: string, status?: number, userId?: string, limit?: number }): Promise<{ id: string, data: ExecutionRecord }[]> {
    let query: admin.firestore.Query = this.collection().orderBy('timestamp', 'desc');

    if (filters.service) {
      query = query.where('service', '==', filters.service);
    }
    if (filters.status !== undefined) {
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
   * List executions belonging to a specific pipeline run.
   */
  async listByPipeline(pipelineExecutionId: string): Promise<{ id: string, data: ExecutionRecord }[]> {
    const query = this.collection()
      .where('pipeline_execution_id', '==', pipelineExecutionId)
      .orderBy('timestamp', 'asc');

    const snapshot = await query.get();
    return snapshot.docs.map(doc => ({ id: doc.id, data: doc.data() as ExecutionRecord }));
  }

  /**
   * List distinct pipeline executions for a user.
   * Returns the most recent execution record per unique pipeline_execution_id.
   * Used for finding unsynchronized executions (those that don't have a matching synced activity).
   *
   * Note: We fetch more than limit to account for deduplication, then trim.
   */
  async listDistinctPipelines(userId: string, limit: number = 50): Promise<{ id: string, data: ExecutionRecord }[]> {
    // Fetch a larger set to account for multiple executions per pipeline
    const fetchLimit = limit * 10;

    const query = this.collection()
      .where('user_id', '==', userId)
      .orderBy('timestamp', 'desc')
      .limit(fetchLimit);

    const snapshot = await query.get();

    // Deduplicate by pipeline_execution_id, keeping the most recent (first seen since ordered desc)
    const seenPipelines = new Set<string>();
    const deduped: { id: string, data: ExecutionRecord }[] = [];

    for (const doc of snapshot.docs) {
      const data = doc.data() as ExecutionRecord;
      const pipelineId = data.pipelineExecutionId;

      if (pipelineId && !seenPipelines.has(pipelineId)) {
        seenPipelines.add(pipelineId);
        deduped.push({ id: doc.id, data });
      }

      if (deduped.length >= limit) break;
    }

    return deduped;
  }

  /**
   * Watch executions with real-time updates.
   */
  watch(filters: { service?: string, status?: number, userId?: string, limit?: number }, onNext: (executions: { id: string, data: ExecutionRecord }[]) => void, onError?: (error: Error) => void): () => void {
    let query: admin.firestore.Query = this.collection().orderBy('timestamp', 'desc');

    if (filters.service) {
      query = query.where('service', '==', filters.service);
    }
    if (filters.status !== undefined) {
      query = query.where('status', '==', filters.status);
    }
    if (filters.userId) {
      query = query.where('user_id', '==', filters.userId);
    }

    if (filters.limit) {
      query = query.limit(filters.limit);
    }

    return query.onSnapshot(snapshot => {
      const executions = snapshot.docs.map(doc => ({ id: doc.id, data: doc.data() as ExecutionRecord }));
      onNext(executions);
    }, error => {
      if (onError) {
        onError(error);
      } else {
        console.error('Error watching executions:', error);
      }
    });
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
