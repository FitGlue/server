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
    let q: admin.firestore.Query = this.synchronizedCollection(userId);
    if (since) {
      q = q.where('synced_at', '>=', since);
    }
    const snapshot = await q.count().get();
    return snapshot.data().count;
  }

  async listSynchronized(userId: string, limit: number = 20, offset: number = 0, startAfter?: unknown): Promise<import('../../types/pb/user').SynchronizedActivity[]> {
    let q = this.synchronizedCollection(userId).orderBy('synced_at', 'desc').limit(limit);
    if (startAfter) {
      q = q.startAfter(startAfter);
    } else if (offset > 0) {
      q = q.offset(offset);
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

  /**
   * Check if an incoming activity is a "bounceback" from our own upload.
   * Used for source-level loop prevention - when a destination sends a webhook,
   * we check if we recently uploaded an activity with the same destination ID.
   *
   * Uses exponential backoff to handle race conditions where the webhook
   * arrives before we've finished writing the upload record to Firestore.
   *
   * @param userId - User to check
   * @param destination - The destination enum value (e.g., Destination.DESTINATION_HEVY)
   * @param destinationId - The external ID from the webhook (which IS the destination's ID)
   * @returns true if this activity was uploaded by us (should skip processing)
   */
  async isBounceback(userId: string, destination: number, destinationId: string): Promise<boolean> {
    const maxRetries = 3;
    const baseDelayMs = 100; // 100ms, 200ms, 400ms

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      // Query by enum fields - exactly like Go does
      const snapshot = await this.uploadedActivitiesCollection(userId)
        .where('destination', '==', destination)
        .where('destination_id', '==', destinationId)
        .limit(1)
        .get();

      if (!snapshot.empty) {
        return true;
      }

      // If not found and we have retries left, wait with exponential backoff
      if (attempt < maxRetries) {
        const delayMs = baseDelayMs * Math.pow(2, attempt);
        await this.sleep(delayMs);
      }
    }

    return false;
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  /**
   * Get the uploaded_activities collection for a specific user.
   * Used for loop prevention tracking.
   */
  private uploadedActivitiesCollection(userId: string) {
    return this.db.collection('users').doc(userId).collection('uploaded_activities');
  }
}
