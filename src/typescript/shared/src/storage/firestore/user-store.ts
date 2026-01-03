import * as admin from 'firebase-admin';
import * as converters from './converters';
import { UserRecord } from '../../types/pb/user';

/**
 * UserStore provides typed access to user-related Firestore operations.
 */
export class UserStore {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get the users collection reference.
   */
  private collection() {
    return this.db.collection('users').withConverter(converters.userConverter);
  }

  /**
   * Find a user by a specific field value.
   */
  async findByField(field: string, value: any): Promise<UserRecord | null> {
    const snapshot = await this.collection()
      .where(field, '==', value)
      .limit(1)
      .get();

    if (snapshot.empty) {
      return null;
    }

    return snapshot.docs[0].data();
  }

  /**
   * Find a user by their Fitbit ID.
   */
  async findByFitbitId(fitbitUserId: string): Promise<{ id: string; data: UserRecord } | null> {
    const snapshot = await this.collection()
      .where('integrations.fitbit.fitbit_user_id', '==', fitbitUserId)
      .limit(1)
      .get();

    if (snapshot.empty) {
      return null;
    }

    const doc = snapshot.docs[0];
    return { id: doc.id, data: doc.data() };
  }

  /**
   * Get a user by ID.
   */
  async get(userId: string): Promise<UserRecord | null> {
    const doc = await this.collection().doc(userId).get();
    return doc.exists ? doc.data() || null : null;
  }

  /**
   * List all users.
   */
  async list(): Promise<UserRecord[]> {
    const snapshot = await this.collection().get();
    return snapshot.docs.map(doc => doc.data());
  }

  /**
   * Delete a user by ID.
   */
  async delete(userId: string): Promise<void> {
    await this.collection().doc(userId).delete();
  }

  /**
   * Update a user document (root level fields only).
   */
  async update(userId: string, data: Partial<UserRecord>): Promise<void> {
    await this.collection().doc(userId).update(data);
  }

  /**
   * Delete all users.
   */
  async deleteAll(): Promise<number> {
    const snapshot = await this.collection().get();
    if (snapshot.empty) return 0;

    const batch = this.db.batch();
    snapshot.docs.forEach(doc => {
      batch.delete(doc.ref);
    });

    await batch.commit();
    return snapshot.size;
  }

  /**
   * Set an integration configuration for a user.
   * This handles the nested update path strictly.
   */
  async setIntegration<K extends keyof import('../../types/pb/user').UserIntegrations>(
    userId: string,
    provider: K,
    data: import('../../types/pb/user').UserIntegrations[K]
  ): Promise<void> {
    // Construct the dot-notation key for updating nested field
    const fieldPath = `integrations.${provider}`;
    await this.collection().doc(userId).update({
      [fieldPath]: data
    });
  }

  /**
   * Update pipelines for a user.
   */
  async updatePipelines(userId: string, pipelines: import('../../types/pb/user').PipelineConfig[]): Promise<void> {
    await this.collection().doc(userId).update({
      pipelines: pipelines
    });
  }

  /**
   * Add a pipeline to the user's list.
   */
  async addPipeline(userId: string, pipeline: import('../../types/pb/user').PipelineConfig): Promise<void> {
    await this.collection().doc(userId).update({
      pipelines: admin.firestore.FieldValue.arrayUnion(pipeline)
    });
  }

  /**
   * Create or overwrite a user document.
   */
  /**
   * Create or overwrite a user document.
   */
  async create(userId: string, data: UserRecord): Promise<void> {
    await this.collection().doc(userId).set(data);
  }

  /**
   * Update the last_used_at timestamp for a specific integration.
   */
  async updateLastUsed(userId: string, provider: string): Promise<void> {
    const fieldPath = `integrations.${provider}.last_used_at`;
    await this.collection().doc(userId).update({
      [fieldPath]: new Date()
    });
  }
}
