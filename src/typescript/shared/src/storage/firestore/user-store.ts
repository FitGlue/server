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
  async findByField(field: string, value: unknown): Promise<UserRecord | null> {
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
   * Find a user by their Strava Athlete ID.
   */
  async findByStravaId(athleteId: number): Promise<{ id: string; data: UserRecord } | null> {
    const snapshot = await this.collection()
      .where('integrations.strava.athlete_id', '==', athleteId)
      .limit(1)
      .get();

    if (snapshot.empty) {
      return null;
    }

    const doc = snapshot.docs[0];
    return { id: doc.id, data: doc.data() };
  }

  /**
   * Find a user by their Polar User ID.
   */
  async findByPolarId(polarUserId: string): Promise<{ id: string; data: UserRecord } | null> {
    const snapshot = await this.collection()
      .where('integrations.polar.polar_user_id', '==', polarUserId)
      .limit(1)
      .get();

    if (snapshot.empty) {
      return null;
    }

    const doc = snapshot.docs[0];
    return { id: doc.id, data: doc.data() };
  }

  /**
   * Find a user by their Oura User ID.
   */
  async findByOuraId(ouraUserId: string): Promise<{ id: string; data: UserRecord } | null> {
    const snapshot = await this.collection()
      .where('integrations.oura.oura_user_id', '==', ouraUserId)
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
   * Delete an integration from a user (removes the field entirely, not just disables).
   */
  async deleteIntegration(userId: string, provider: string): Promise<void> {
    await this.collection().doc(userId).update({
      [`integrations.${provider}`]: admin.firestore.FieldValue.delete()
    });
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

    // Use generic converter to ensure snake_case mapping based on logic
    // We cast data to Record<string, unknown> because we know it matches the generic integration structure
    // enforced by the K generic, and the converter handles the dynamic key lookup.
    const firestoreData = converters.mapGenericIntegrationToFirestore(data as unknown as Record<string, unknown>, provider);

    await this.collection().doc(userId).update({
      [fieldPath]: firestoreData
    });
  }

  /**
   * Update pipelines for a user.
   */
  async updatePipelines(userId: string, pipelines: import('../../types/pb/user').PipelineConfig[]): Promise<void> {
    const firestorePipelines = pipelines.map(converters.mapPipelineToFirestore);
    await this.collection().doc(userId).update({
      pipelines: firestorePipelines
    });
  }

  /**
   * Add a pipeline to the user's list.
   */
  async addPipeline(userId: string, pipeline: import('../../types/pb/user').PipelineConfig): Promise<void> {
    const firestorePipeline = converters.mapPipelineToFirestore(pipeline);
    await this.collection().doc(userId).update({
      pipelines: admin.firestore.FieldValue.arrayUnion(firestorePipeline)
    });
  }

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

  /**
   * Add a new FCM token to the user's list.
   */
  async addFcmToken(userId: string, token: string): Promise<void> {
    // Use raw collection to bypass converter types for arrayUnion on specific field
    await this.db.collection('users').doc(userId).update({
      fcm_tokens: admin.firestore.FieldValue.arrayUnion(token)
    });
  }

  /**
   * Toggle the disabled state of a specific pipeline.
   */
  async togglePipelineDisabled(userId: string, pipelineId: string, disabled: boolean): Promise<void> {
    const user = await this.get(userId);
    if (!user) {
      throw new Error('User not found');
    }

    const pipelineIndex = user.pipelines?.findIndex(p => p.id === pipelineId);
    if (pipelineIndex === undefined || pipelineIndex === -1) {
      throw new Error('Pipeline not found');
    }

    // Update the disabled field using Firestore field path notation
    await this.db.collection('users').doc(userId).update({
      [`pipelines.${pipelineIndex}.disabled`]: disabled
    });
  }
}
