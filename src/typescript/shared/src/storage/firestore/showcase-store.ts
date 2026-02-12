import * as admin from 'firebase-admin';
import { FirestoreToShowcasedActivity } from './converters';
import type { ShowcasedActivity } from '../../types/pb/user';

/**
 * ShowcaseStore provides typed access to showcased activity operations.
 * Showcased activities are public, shareable snapshots of activities.
 */
export class ShowcaseStore {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get the showcased activities collection.
   */
  private collection() {
    return this.db.collection('showcased_activities');
  }

  /**
   * Get a showcased activity by its human-readable ID.
   */
  async get(showcaseId: string): Promise<ShowcasedActivity | null> {
    const doc = await this.collection().doc(showcaseId).get();
    if (!doc.exists) {
      return null;
    }
    const rawData = doc.data() as Record<string, unknown>;
    return FirestoreToShowcasedActivity(rawData);
  }

  /**
   * Check if a showcase ID exists (for uniqueness checks during generation).
   */
  async exists(showcaseId: string): Promise<boolean> {
    const doc = await this.collection().doc(showcaseId).get();
    return doc.exists;
  }

  /**
   * List all showcased activities for a given user.
   */
  async listByUserId(userId: string): Promise<ShowcasedActivity[]> {
    const snap = await this.collection()
      .where('user_id', '==', userId)
      .orderBy('created_at', 'desc')
      .get();
    return snap.docs.map(doc => {
      const rawData = doc.data() as Record<string, unknown>;
      return FirestoreToShowcasedActivity(rawData);
    });
  }
}

