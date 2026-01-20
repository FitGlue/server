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
   * Get the router execution record for a pipeline.
   * Used by re-post handler to retrieve the EnrichedActivityEvent from inputsJson.
   */
  async getRouterExecution(pipelineExecutionId: string): Promise<{ id: string, data: ExecutionRecord } | null> {
    const executions = await this.listByPipeline(pipelineExecutionId);
    const routerExec = executions.find(e => e.data.service === 'router');
    return routerExec || null;
  }

  /**
   * Get the enricher execution record for a pipeline.
   * Used by re-post handler for full pipeline re-execution.
   */
  async getEnricherExecution(pipelineExecutionId: string): Promise<{ id: string, data: ExecutionRecord } | null> {
    const executions = await this.listByPipeline(pipelineExecutionId);
    const enricherExec = executions.find(e => e.data.service === 'enricher');
    return enricherExec || null;
  }

  /**
   * List distinct pipeline executions for a user.
   * Returns the most recent execution record per unique pipeline_execution_id.
   * Used for finding unsynchronized executions (those that don't have a matching synced activity).
   *
   * OPTIMIZED: Uses projection queries to reduce data transfer by ~90%.
   * Note: We fetch more than limit to account for deduplication, then trim.
   */
  async listDistinctPipelines(userId: string, limit: number = 50): Promise<{ id: string, data: ExecutionRecord }[]> {
    // Fetch a larger set to account for multiple executions per pipeline
    // Reduced from 10x to 5x as an optimization (most pipelines have 2-4 executions)
    const fetchLimit = limit * 5;

    // Use projection to only fetch fields needed for display and filtering
    // This dramatically reduces data transfer since inputsJson/outputsJson can be MB
    const query = this.db.collection('executions')
      .where('user_id', '==', userId)
      .orderBy('timestamp', 'desc')
      .select(
        'pipeline_execution_id',
        'timestamp',
        'status',
        'service',
        'error_message',
        'inputs_json' // Needed to extract activity info for display
      )
      .limit(fetchLimit);

    const snapshot = await query.get();

    // Deduplicate by pipeline_execution_id, keeping the most recent (first seen since ordered desc)
    const seenPipelines = new Set<string>();
    const deduped: { id: string, data: ExecutionRecord }[] = [];

    for (const doc of snapshot.docs) {
      const rawData = doc.data();
      // Manually map snake_case fields since we bypassed the converter
      // Include required fields: executionId from doc.id, triggerType defaults to 'projection'
      const data: ExecutionRecord = {
        executionId: doc.id,
        service: rawData.service || '',
        status: rawData.status,
        triggerType: rawData.trigger_type || 'projection', // Placeholder for projection queries
        pipelineExecutionId: rawData.pipeline_execution_id,
        timestamp: rawData.timestamp?.toDate?.() || rawData.timestamp,
        errorMessage: rawData.error_message,
        inputsJson: rawData.inputs_json,
      };
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
   * Lightweight version that only returns pipeline IDs and status.
   * Used for quick unsynchronized detection without needing display data.
   */
  async listDistinctPipelineIds(userId: string, limit: number = 100): Promise<{ pipelineId: string, status: number }[]> {
    const fetchLimit = limit * 5;

    const query = this.db.collection('executions')
      .where('user_id', '==', userId)
      .orderBy('timestamp', 'desc')
      .select('pipeline_execution_id', 'status')
      .limit(fetchLimit);

    const snapshot = await query.get();

    const seenPipelines = new Set<string>();
    const results: { pipelineId: string, status: number }[] = [];

    for (const doc of snapshot.docs) {
      const data = doc.data();
      const pipelineId = data.pipeline_execution_id;

      if (pipelineId && !seenPipelines.has(pipelineId)) {
        seenPipelines.add(pipelineId);
        results.push({ pipelineId, status: data.status });
      }

      if (results.length >= limit) break;
    }

    return results;
  }

  /**
   * Batch load execution traces for multiple pipeline IDs in a single query.
   * Solves the N+1 query problem when loading activity list with execution traces.
   *
   * @param pipelineIds Array of pipeline execution IDs to fetch
   * @returns Map of pipelineId -> array of executions
   */
  async batchListByPipelines(pipelineIds: string[]): Promise<Map<string, { id: string, data: ExecutionRecord }[]>> {
    if (pipelineIds.length === 0) {
      return new Map();
    }

    // Firestore 'in' queries support up to 30 values
    // For larger sets, we'd need to batch, but 10 activities * ~4 executions each fits
    const chunkSize = 30;
    const results = new Map<string, { id: string, data: ExecutionRecord }[]>();

    for (let i = 0; i < pipelineIds.length; i += chunkSize) {
      const chunk = pipelineIds.slice(i, i + chunkSize);

      const query = this.collection()
        .where('pipeline_execution_id', 'in', chunk)
        .orderBy('timestamp', 'asc');

      const snapshot = await query.get();

      for (const doc of snapshot.docs) {
        const data = doc.data() as ExecutionRecord;
        const pipelineId = data.pipelineExecutionId;

        if (pipelineId) {
          if (!results.has(pipelineId)) {
            results.set(pipelineId, []);
          }
          results.get(pipelineId)!.push({ id: doc.id, data });
        }
      }
    }

    return results;
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

  /**
   * Delete all executions for a specific service (batched).
   */
  async deleteByService(service: string): Promise<number> {
    let deletedCount = 0;
    // Small batch size because execution docs can be very large (MB of outputsJson)
    const batchSize = 50;

    // eslint-disable-next-line no-constant-condition
    while (true) {
      const snapshot = await this.collection()
        .where('service', '==', service)
        .limit(batchSize)
        .get();

      if (snapshot.empty) {
        break;
      }

      const batch = this.db.batch();
      snapshot.docs.forEach(doc => {
        batch.delete(doc.ref);
      });

      await batch.commit();
      deletedCount += snapshot.size;
      console.log(`  Deleted batch of ${snapshot.size} executions (total: ${deletedCount})...`);
    }
    return deletedCount;
  }

  /**
   * List recent executions (for admin stats).
   */
  async listRecent(limit: number = 100): Promise<{ id: string, data: ExecutionRecord }[]> {
    return this.list({ limit });
  }

  /**
   * Get distinct service names from recent executions (for admin filtering).
   */
  async listDistinctServices(): Promise<string[]> {
    // Fetch recent executions and extract unique services
    const executions = await this.list({ limit: 500 });
    const services = new Set<string>();
    for (const exec of executions) {
      if (exec.data.service) {
        services.add(exec.data.service);
      }
    }
    return Array.from(services).sort();
  }
}
