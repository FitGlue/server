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
