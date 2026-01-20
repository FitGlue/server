/**
 * ActivityCounters - Materialized counters for activity statistics.
 *
 * This interface extends the base UserRecord to add cached activity counts,
 * eliminating the need for expensive count() queries on every dashboard load.
 *
 * These fields are stored in the user document at the root level under 'activityCounts'.
 */
export interface ActivityCounts {
  /** Total synchronized activities for this user */
  synchronized: number;

  /** Total unsynchronized (failed/stalled) activities */
  unsynchronized: number;

  /** Syncs this week (for the "boosted this week" stat) */
  weeklySync: number;

  /** When the weekly count was last reset (start of week) */
  weeklyResetAt?: Date;

  /** Last time counters were updated */
  lastUpdated?: Date;
}

/**
 * Extended execution record with sync status tracking.
 *
 * When a destination handler successfully creates a SynchronizedActivity,
 * it should also set hasSynchronizedActivity = true on the execution record.
 * This enables efficient unsynchronized queries without cross-collection joins.
 */
export interface ExecutionRecordExtended {
  /** Whether this execution has a corresponding SynchronizedActivity */
  hasSynchronizedActivity?: boolean;
}
