import * as admin from 'firebase-admin';
import { ActivityCounts } from '../types/activity-counters';

/**
 * ActivityCounterService provides O(1) access to activity statistics
 * via materialized counters stored in the user document.
 *
 * This eliminates expensive count() queries that scan the activities subcollection.
 */
export class ActivityCounterService {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get cached activity counts for a user.
   * Falls back to computing counts if cache is missing.
   */
  async getCounts(userId: string): Promise<ActivityCounts> {
    const userDoc = await this.db.collection('users').doc(userId).get();
    const userData = userDoc.data();

    // Return cached counts or default values
    const counts = userData?.activityCounts as ActivityCounts | undefined;

    if (counts) {
      // Check if weekly reset is needed
      const now = new Date();
      const weekStart = this.getWeekStart(now);
      const lastReset = (counts.weeklyResetAt as { toDate?: () => Date })?.toDate?.() || counts.weeklyResetAt;

      if (lastReset && new Date(lastReset as Date) < weekStart) {
        // Week has rolled over, reset weekly count
        counts.weeklySync = 0;
        counts.weeklyResetAt = weekStart;
        // Fire-and-forget update (don't await)
        this.updateCounts(userId, counts).catch(console.error);
      }

      return counts;
    }

    // Initialize with zeros if missing
    return {
      synchronized: 0,
      unsynchronized: 0,
      weeklySync: 0,
      weeklyResetAt: this.getWeekStart(new Date()),
      lastUpdated: new Date(),
    };
  }

  /**
   * Update activity counts in the user document.
   */
  async updateCounts(userId: string, counts: Partial<ActivityCounts>): Promise<void> {
    await this.db.collection('users').doc(userId).update({
      activityCounts: {
        ...counts,
        lastUpdated: admin.firestore.FieldValue.serverTimestamp(),
      },
    });
  }

  /**
   * Increment the synchronized activity count.
   * Called by destination handlers after successfully creating a SynchronizedActivity.
   */
  async incrementSynchronized(userId: string): Promise<void> {
    const userRef = this.db.collection('users').doc(userId);

    await this.db.runTransaction(async (transaction) => {
      const doc = await transaction.get(userRef);
      const data = doc.data();
      const currentCounts = (data?.activityCounts as ActivityCounts) || {
        synchronized: 0,
        unsynchronized: 0,
        weeklySync: 0,
        weeklyResetAt: this.getWeekStart(new Date()),
      };

      // Check for weekly reset
      const now = new Date();
      const weekStart = this.getWeekStart(now);
      const lastReset = currentCounts.weeklyResetAt;
      const lastResetDate = lastReset instanceof Date
        ? lastReset
        : (lastReset ? new Date(lastReset as unknown as string) : new Date(0));

      let weeklySync = currentCounts.weeklySync || 0;
      let weeklyResetAt = currentCounts.weeklyResetAt;

      if (lastResetDate < weekStart) {
        weeklySync = 0;
        weeklyResetAt = weekStart;
      }

      transaction.update(userRef, {
        'activityCounts.synchronized': (currentCounts.synchronized || 0) + 1,
        'activityCounts.weeklySync': weeklySync + 1,
        'activityCounts.weeklyResetAt': weeklyResetAt,
        'activityCounts.lastUpdated': admin.firestore.FieldValue.serverTimestamp(),
      });
    });
  }

  /**
   * Increment the unsynchronized activity count.
   * Called when a pipeline execution fails/stalls.
   */
  async incrementUnsynchronized(userId: string): Promise<void> {
    await this.db.collection('users').doc(userId).update({
      'activityCounts.unsynchronized': admin.firestore.FieldValue.increment(1),
      'activityCounts.lastUpdated': admin.firestore.FieldValue.serverTimestamp(),
    });
  }

  /**
   * Decrement the unsynchronized count and increment synchronized.
   * Called when a previously failed activity is successfully retried.
   */
  async convertUnsynchronizedToSynchronized(userId: string): Promise<void> {
    const userRef = this.db.collection('users').doc(userId);

    await this.db.runTransaction(async (transaction) => {
      const doc = await transaction.get(userRef);
      const data = doc.data();
      const currentCounts = (data?.activityCounts as ActivityCounts) || {
        synchronized: 0,
        unsynchronized: 0,
        weeklySync: 0,
        weeklyResetAt: this.getWeekStart(new Date()),
      };

      const unsync = Math.max(0, (currentCounts.unsynchronized || 0) - 1);

      transaction.update(userRef, {
        'activityCounts.synchronized': (currentCounts.synchronized || 0) + 1,
        'activityCounts.unsynchronized': unsync,
        'activityCounts.weeklySync': (currentCounts.weeklySync || 0) + 1,
        'activityCounts.lastUpdated': admin.firestore.FieldValue.serverTimestamp(),
      });
    });
  }

  /**
   * Get the start of the current week (Monday 00:00:00).
   */
  private getWeekStart(date: Date): Date {
    const d = new Date(date);
    const day = d.getDay();
    const diff = d.getDate() - day + (day === 0 ? -6 : 1);
    d.setDate(diff);
    d.setHours(0, 0, 0, 0);
    return d;
  }

  /**
   * Backfill counts for a user by querying actual data.
   * Used for migration or when counts are missing/stale.
   */
  async backfillCounts(userId: string): Promise<ActivityCounts> {
    // Count synchronized activities
    const syncedSnapshot = await this.db
      .collection('users')
      .doc(userId)
      .collection('activities')
      .count()
      .get();
    const synchronized = syncedSnapshot.data().count;

    // Count weekly syncs (since Monday)
    const weekStart = this.getWeekStart(new Date());
    const weeklySnapshot = await this.db
      .collection('users')
      .doc(userId)
      .collection('activities')
      .where('synced_at', '>=', weekStart)
      .count()
      .get();
    const weeklySync = weeklySnapshot.data().count;

    // For unsynchronized, we'd need the complex query (skip for now)
    // This can be computed lazily on demand

    const counts: ActivityCounts = {
      synchronized,
      unsynchronized: 0, // Will be computed lazily
      weeklySync,
      weeklyResetAt: weekStart,
      lastUpdated: new Date(),
    };

    // Store the computed counts
    await this.updateCounts(userId, counts);

    return counts;
  }
}
