import * as admin from 'firebase-admin';
import * as converters from './converters';

/**
 * ActivityStore provides typed access to user activity operations.
 */
export class ActivityStore {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get the raw activities collection for a specific user.
   */
  private collection(userId: string) {
    return this.db.collection('users').doc(userId).collection('raw_activities').withConverter(converters.processedActivityConverter);
  }

  /**
   * Check if an activity has been processed.
   */
  async isProcessed(userId: string, activityId: string): Promise<boolean> {
    const doc = await this.collection(userId).doc(activityId).get();
    return doc.exists;
  }

  /**
   * Mark an activity as processed.
   */
  async markProcessed(userId: string, activityId: string, data: import('../../types/pb/user').ProcessedActivityRecord): Promise<void> {
    await this.collection(userId).doc(activityId).set(data);
  }

  /**
   * List processed activities for a user.
   */
  async list(userId: string, limit: number = 20): Promise<import('../../types/pb/user').ProcessedActivityRecord[]> {
    const snapshot = await this.collection(userId)
      .orderBy('processed_at', 'desc')
      .limit(limit)
      .get();

    return snapshot.docs.map(doc => doc.data());
  }

  /**
   * Delete a processed activity record.
   */
  async delete(userId: string, activityId: string): Promise<void> {
    await this.collection(userId).doc(activityId).delete();
  }

  /**
   * Get the synchronized activities collection for a specific user.
   */
  private synchronizedCollection(userId: string) {
    return this.db.collection('users').doc(userId).collection('activities').withConverter(converters.synchronizedActivityConverter);
  }

  async createSynchronized(userId: string, activity: import('../../types/pb/user').SynchronizedActivity): Promise<void> {
    await this.synchronizedCollection(userId).doc(activity.activityId).set(activity);
  }

  async countSynchronized(userId: string, since?: Date): Promise<number> {

    let q: FirebaseFirestore.Query = this.synchronizedCollection(userId);
    if (since) {
      q = q.where('synced_at', '>=', since);
    }
    const snapshot = await q.count().get();
    return snapshot.data().count;
  }

  async listSynchronized(userId: string, limit: number = 20, startAfter?: unknown): Promise<import('../../types/pb/user').SynchronizedActivity[]> {
    let q = this.synchronizedCollection(userId).orderBy('synced_at', 'desc').limit(limit);
    if (startAfter) {
      q = q.startAfter(startAfter);
    }
    const snapshot = await q.get();
    return snapshot.docs.map(doc => doc.data());
  }

  async getSynchronized(userId: string, activityId: string): Promise<import('../../types/pb/user').SynchronizedActivity | null> {
    const doc = await this.synchronizedCollection(userId).doc(activityId).get();
    if (!doc.exists) {
      return null;
    }
    return doc.data() || null;
  }

  /**
   * Check if an external ID exists as a destination in any synchronized activity.
   * Used for loop prevention - if incoming trigger ID was already posted as a destination,
   * it means we created this activity and should skip to prevent infinite loops.
   *
   * @param userId - User to check
   * @param destinationKey - e.g., 'strava', 'hevy'
   * @param externalId - The external ID to check
   * @returns true if this external ID was already used as a destination
   */
  async checkDestinationExists(userId: string, destinationKey: string, externalId: string): Promise<boolean> {
    // Query for any synchronized activity where destinations.{destinationKey} == externalId
    // Note: Firestore requires composite index for this query
    const fieldPath = `destinations.${destinationKey}`;
    const snapshot = await this.synchronizedCollection(userId)
      .where(fieldPath, '==', externalId)
      .limit(1)
      .get();

    return !snapshot.empty;
  }

  /**
   * Get all pipeline_execution_ids that have synchronized activities for a user.
   * Used for absence-based unsynchronized detection.
   */
  async getSynchronizedPipelineIds(userId: string, limit: number = 200): Promise<Set<string>> {
    const snapshot = await this.synchronizedCollection(userId)
      .orderBy('synced_at', 'desc')
      .limit(limit)
      .select('pipeline_execution_id')
      .get();

    const ids = new Set<string>();
    for (const doc of snapshot.docs) {
      const data = doc.data();
      if (data.pipelineExecutionId) {
        ids.add(data.pipelineExecutionId);
      }
    }
    return ids;
  }

  /**
   * Update a destination in a synchronized activity.
   * Used after a successful re-post to add or update the external ID for a destination.
   *
   * @param userId - User ID
   * @param activityId - Activity ID
   * @param destination - Destination key (e.g., 'strava', 'showcase')
   * @param externalId - External ID from the destination platform
   */
  async updateDestination(
    userId: string,
    activityId: string,
    destination: string,
    externalId: string
  ): Promise<void> {
    await this.synchronizedCollection(userId)
      .doc(activityId)
      .update({ [`destinations.${destination}`]: externalId });
  }
}
