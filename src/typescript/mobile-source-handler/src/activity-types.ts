/**
 * Activity Type Re-export
 *
 * Re-exports mapMobileActivityType from the shared types module
 * (originally defined in mobile-sync-handler/types.ts).
 * This avoids a cross-package dependency while keeping the mapping
 * logic in a single canonical location.
 *
 * TODO: Consider moving mapMobileActivityType to @fitglue/shared if
 * used by more than 2 handlers.
 */

/**
 * Map mobile activity type name to FitGlue standard sport string.
 * Mirrors the mapping in mobile-sync-handler/src/types.ts.
 */
export function mapMobileActivityType(activityName: string): string {
    const lowerName = activityName.toLowerCase();

    const typeMap: Record<string, string> = {
        'running': 'Run', 'run': 'Run',
        'trailrun': 'TrailRun', 'trail run': 'TrailRun',
        'walking': 'Walk', 'walk': 'Walk',
        'cycling': 'Ride', 'biking': 'Ride', 'bike': 'Ride', 'ride': 'Ride',
        'swimming': 'Swim', 'swim': 'Swim',
        'weighttraining': 'WeightTraining', 'weight_training': 'WeightTraining',
        'weight training': 'WeightTraining', 'strength': 'WeightTraining',
        'strength_training': 'WeightTraining', 'strength training': 'WeightTraining',
        'gym': 'WeightTraining',
        'elliptical': 'Elliptical', 'rowing': 'Rowing',
        'stairclimbing': 'StairStepper', 'stair_climbing': 'StairStepper',
        'yoga': 'Yoga', 'pilates': 'Pilates',
        'hiit': 'HIIT', 'crossfit': 'Crossfit',
        'hiking': 'Hike', 'hike': 'Hike',
        'alpineski': 'AlpineSki', 'snowboarding': 'Snowboard',
        'tennis': 'Tennis', 'soccer': 'Soccer', 'basketball': 'Basketball',
        'golf': 'Golf', 'kayaking': 'Kayaking', 'surfing': 'Surfing',
        'rockclimbing': 'RockClimbing', 'rock climbing': 'RockClimbing',
        'workout': 'Workout', 'exercise': 'Workout',
    };

    if (typeMap[lowerName]) return typeMap[lowerName];

    for (const [key, value] of Object.entries(typeMap)) {
        if (lowerName.includes(key)) return value;
    }

    // Default: split PascalCase into words (e.g. "WeightTraining" → "Weight Training")
    const split = activityName.replace(/([a-z])([A-Z])/g, '$1 $2')
        .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2');
    return split.charAt(0).toUpperCase() + split.slice(1);
}
