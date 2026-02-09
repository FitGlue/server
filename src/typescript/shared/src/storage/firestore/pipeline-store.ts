import * as admin from 'firebase-admin';
import { PipelineConfig } from '../../types/pb/user';

/**
 * Firestore converter for pipeline documents.
 * Maps between PipelineConfig (camelCase) and Firestore (snake_case).
 */
export const pipelineConverter: admin.firestore.FirestoreDataConverter<PipelineConfig> = {
  toFirestore(model: PipelineConfig): admin.firestore.DocumentData {
    const data: admin.firestore.DocumentData = {
      id: model.id,
      name: model.name || '',
      source: model.source,
      destinations: model.destinations,
      enrichers: model.enrichers?.map(e => ({
        provider_type: e.providerType,
        typed_config: e.typedConfig || {}
      })) || [],
      disabled: model.disabled || false
    };
    if (model.sourceConfig && Object.keys(model.sourceConfig).length > 0) {
      data.source_config = model.sourceConfig;
    }
    if (model.destinationConfigs && Object.keys(model.destinationConfigs).length > 0) {
      const destConfigs: Record<string, { config: Record<string, string> }> = {};
      for (const [k, v] of Object.entries(model.destinationConfigs)) {
        destConfigs[k] = { config: v.config || {} };
      }
      data.destination_configs = destConfigs;
    }
    return data;
  },

  fromFirestore(snapshot: admin.firestore.QueryDocumentSnapshot): PipelineConfig {
    const data = snapshot.data();
    const destConfigsRaw = (data.destination_configs || {}) as Record<string, { config?: Record<string, string> }>;
    const destinationConfigs: Record<string, { config: Record<string, string> }> = {};
    for (const [k, v] of Object.entries(destConfigsRaw)) {
      destinationConfigs[k] = { config: (v?.config || {}) as Record<string, string> };
    }
    return {
      id: data.id as string,
      name: (data.name as string) || '',
      source: data.source as string,
      destinations: (data.destinations as number[]) || [],
      enrichers: ((data.enrichers as Record<string, unknown>[]) || []).map((e: Record<string, unknown>) => ({
        providerType: (e.provider_type || e.providerType) as number,
        typedConfig: (e.typed_config || e.typedConfig || {}) as Record<string, string>
      })),
      disabled: (data.disabled as boolean) || false,
      sourceConfig: (data.source_config || {}) as Record<string, string>,
      destinationConfigs,
    };
  }
};

/**
 * PipelineStore provides typed access to pipeline sub-collection operations.
 * Pipelines are stored at: users/{userId}/pipelines/{pipelineId}
 */
export class PipelineStore {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get the pipelines sub-collection reference for a user.
   */
  private collection(userId: string) {
    return this.db
      .collection('users')
      .doc(userId)
      .collection('pipelines')
      .withConverter(pipelineConverter);
  }

  /**
   * Get a single pipeline by ID.
   */
  async get(userId: string, pipelineId: string): Promise<PipelineConfig | null> {
    const doc = await this.collection(userId).doc(pipelineId).get();
    return doc.exists ? doc.data() || null : null;
  }

  /**
   * List all pipelines for a user.
   */
  async list(userId: string): Promise<PipelineConfig[]> {
    const snapshot = await this.collection(userId).get();
    return snapshot.docs.map(doc => doc.data());
  }

  /**
   * Create a new pipeline.
   */
  async create(userId: string, pipeline: PipelineConfig): Promise<void> {
    await this.collection(userId).doc(pipeline.id).set(pipeline);
  }

  /**
   * Update a pipeline (partial update).
   */
  async update(userId: string, pipelineId: string, updates: Partial<PipelineConfig>): Promise<void> {
    await this.collection(userId).doc(pipelineId).update(updates);
  }

  /**
   * Delete a pipeline.
   */
  async delete(userId: string, pipelineId: string): Promise<void> {
    await this.collection(userId).doc(pipelineId).delete();
  }

  /**
   * Toggle the disabled state of a pipeline.
   */
  async toggleDisabled(userId: string, pipelineId: string, disabled: boolean): Promise<void> {
    await this.collection(userId).doc(pipelineId).update({ disabled });
  }
}
