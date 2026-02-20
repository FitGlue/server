/**
 * Mobile Sync Types
 *
 * Defines the payload structure for mobile health data sync.
 * Used by both iOS HealthKit and Android Health Connect bridges.
 */

/**
 * Heart rate sample from mobile health platform
 */
export interface MobileHeartRateSample {
  timestamp: string; // ISO 8601
  bpm: number;
}

/**
 * GPS route point from mobile health platform
 */
export interface MobileRoutePoint {
  timestamp: string; // ISO 8601
  latitude: number;
  longitude: number;
  altitude?: number;
}

/**
 * Standardized activity from mobile health platforms
 */
export interface MobileActivity {
  /** Unique identifier from the source platform */
  externalId?: string;

  /** Activity type name (e.g., "Running", "WeightTraining", "Cycling") */
  activityName: string;

  /** Activity start time (ISO 8601) */
  startTime: string;

  /** Activity end time (ISO 8601) */
  endTime: string;

  /** Duration in seconds */
  duration: number;

  /** Calories burned (optional) */
  calories?: number;

  /** Distance in meters (optional) */
  distance?: number;

  /** Heart rate samples (optional) */
  heartRateSamples?: MobileHeartRateSample[];

  /** GPS route points (optional) */
  route?: MobileRoutePoint[];

  /** Source platform indicator */
  source: 'healthkit' | 'health_connect';
}

/**
 * Mobile sync request payload
 */
export interface MobileSyncRequest {
  /** Array of activities to sync */
  activities: MobileActivity[];

  /** Device information */
  device?: {
    platform: 'ios' | 'android';
    osVersion?: string;
    appVersion?: string;
  };

  /** Sync metadata */
  sync?: {
    /** Last successful sync timestamp (ISO 8601) */
    lastSyncDate?: string;
    /** Unique sync batch ID */
    batchId?: string;
  };
}

/**
 * Mobile sync response payload
 */
export interface MobileSyncResponse {
  /** Whether the sync was successful */
  success: boolean;

  /** Number of activities processed */
  processedCount: number;

  /** Number of activities skipped (duplicates, etc.) */
  skippedCount: number;

  /** Pipeline execution IDs for tracing */
  executionIds: string[];

  /** Error message if sync failed */
  error?: string;

  /** Timestamp of this sync */
  syncedAt: string;
}

/**
 * Map mobile activity type to FitGlue activity type.
 *
 * Must cover all names sent by:
 * - Android Health Connect (via EXERCISE_TYPE_MAP in AndroidHealthService.ts)
 * - iOS HealthKit (via WORKOUT_ACTIVITY_TYPES in AppleHealthService.ts)
 *
 * Values here correspond to FitGlue's Strava-centric ActivityType display names
 * (e.g., 'Run', 'Ride', 'WeightTraining').
 */
export function mapMobileActivityType(activityName: string): string {
  const lowerName = activityName.toLowerCase();

  const typeMap: Record<string, string> = {
    // === Cardio / Running ===
    'running': 'Run',
    'run': 'Run',
    'trailrun': 'TrailRun',
    'trail run': 'TrailRun',

    // === Walking ===
    'walking': 'Walk',
    'walk': 'Walk',

    // === Cycling ===
    'cycling': 'Ride',
    'biking': 'Ride',
    'bike': 'Ride',
    'ride': 'Ride',

    // === Swimming ===
    'swimming': 'Swim',
    'swim': 'Swim',
    'waterpolo': 'Swim',

    // === Strength ===
    'weighttraining': 'WeightTraining',
    'weight_training': 'WeightTraining',
    'weight training': 'WeightTraining',
    'strength': 'WeightTraining',
    'strength_training': 'WeightTraining',
    'strength training': 'WeightTraining',
    'gym': 'WeightTraining',

    // === Cardio Equipment ===
    'elliptical': 'Elliptical',
    'rowing': 'Rowing',
    'stairclimbing': 'StairStepper',
    'stair_climbing': 'StairStepper',
    'stairstepper': 'StairStepper',

    // === Mind & Body ===
    'yoga': 'Yoga',
    'pilates': 'Pilates',
    'meditation': 'Yoga',
    'stretching': 'Yoga',

    // === HIIT / CrossFit ===
    'hiit': 'HIIT',
    'crossfit': 'Crossfit',
    'crosstraining': 'Crossfit',

    // === Hiking ===
    'hiking': 'Hike',
    'hike': 'Hike',

    // === Winter Sports ===
    'alpineski': 'AlpineSki',
    'alpine ski': 'AlpineSki',
    'nordicski': 'NordicSki',
    'nordic ski': 'NordicSki',
    'snowboarding': 'Snowboard',
    'snowshoeing': 'Snowshoe',
    'iceskating': 'IceSkate',
    'ice skating': 'IceSkate',

    // === Racket Sports ===
    'tennis': 'Tennis',
    'tabletennis': 'TableTennis',
    'table tennis': 'TableTennis',
    'badminton': 'Badminton',
    'squash': 'Squash',
    'racquetball': 'Racquetball',
    'pickleball': 'Pickleball',

    // === Team Sports ===
    'soccer': 'Soccer',
    'basketball': 'Basketball',
    'volleyball': 'Volleyball',
    'handball': 'Handball',
    'rugby': 'Rugby',
    'football': 'Workout',
    'hockey': 'Workout',
    'baseball': 'Workout',
    'softball': 'Workout',
    'cricket': 'Cricket',

    // === Water Sports ===
    'kayaking': 'Kayaking',
    'sailing': 'Sail',
    'surfing': 'Surfing',
    'standuppaddling': 'StandUpPaddling',
    'stand up paddling': 'StandUpPaddling',
    'scubadiving': 'Workout',

    // === Combat ===
    'boxing': 'Workout',
    'martialarts': 'Workout',
    'martial arts': 'Workout',
    'fencing': 'Workout',

    // === Climbing ===
    'rockclimbing': 'RockClimbing',
    'rock climbing': 'RockClimbing',

    // === Other ===
    'golf': 'Golf',
    'gymnastics': 'Workout',
    'dancing': 'Workout',
    'coretraining': 'Workout',
    'core training': 'Workout',
    'flexibility': 'Yoga',
    'cardio': 'Workout',
    'jumprope': 'Workout',
    'jump rope': 'Workout',
    'wheelchair': 'Wheelchair',
    'horsebackriding': 'Workout',
    'horseback riding': 'Workout',
    'paragliding': 'Workout',
    'frisbee': 'Workout',
    'housework': 'Workout',

    // === Generic ===
    'workout': 'Workout',
    'exercise': 'Workout',
  };

  // Try exact match first
  if (typeMap[lowerName]) {
    return typeMap[lowerName];
  }

  // Try partial match
  for (const [key, value] of Object.entries(typeMap)) {
    if (lowerName.includes(key)) {
      return value;
    }
  }

  // Default: split PascalCase into words (e.g. "WeightTraining" → "Weight Training")
  const split = activityName.replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2');
  return split.charAt(0).toUpperCase() + split.slice(1);
}

/**
 * Format a PascalCase or camelCase activity name into a human-readable display name.
 * e.g. "WeightTraining" → "Weight Training", "RockClimbing" → "Rock Climbing"
 * Preserves consecutive uppercase runs (e.g. "HIIT" stays "HIIT").
 * Names that already contain spaces are returned as-is with title casing.
 */
export function formatActivityDisplayName(name: string): string {
  if (!name) return name;

  // If it already has spaces, just title-case it
  if (name.includes(' ')) {
    return name.replace(/\b\w/g, c => c.toUpperCase());
  }

  // Split on transitions: lowercase→uppercase, or end of uppercase run before lowercase
  // e.g. "WeightTraining" → "Weight Training", "HIIT" → "HIIT"
  const split = name.replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2');
  return split.charAt(0).toUpperCase() + split.slice(1);
}

/**
 * Get the source identifier for the mobile platform
 */
export function getMobileSourceId(source: 'healthkit' | 'health_connect'): string {
  return source === 'healthkit' ? 'SOURCE_APPLE_HEALTH' : 'SOURCE_HEALTH_CONNECT';
}
