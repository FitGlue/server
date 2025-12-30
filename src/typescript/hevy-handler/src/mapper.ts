import { components } from '@fitglue/shared/dist/integrations/hevy/schema'; // Import from dist to access generated types
import { StandardizedActivity, Session, StrengthSet, MuscleGroup } from '@fitglue/shared/dist/types/pb/standardized_activity';

type HevyWorkout = components["schemas"]["Workout"];
type HevyExerciseTemplate = components["schemas"]["ExerciseTemplate"];

export function mapHevyWorkoutToStandardized(
  userId: string,
  workout: HevyWorkout,
  templateMap: Record<string, HevyExerciseTemplate>
): StandardizedActivity {
  const startTimeStr = workout.start_time || new Date().toISOString();
  const startTime = new Date(startTimeStr);

  // Calculate duration if not provided
  let durationSeconds = 0;
  if (workout.end_time && workout.start_time) {
    const endTime = new Date(workout.end_time);
    durationSeconds = (endTime.getTime() - startTime.getTime()) / 1000;
  }

  const exercises = workout.exercises || [];
  const strengthSets: StrengthSet[] = exercises.flatMap((ex) => {
    const exTitle = ex.title || 'Unknown Exercise';

    // Handle superset_id from payload (now correctly typed in schema)
    // Check for null/undefined specifically because 0 is a valid ID
    const rawSupersetId = ex.superset_id;

    const supersetId = (rawSupersetId !== undefined && rawSupersetId !== null)
      ? String(rawSupersetId)
      : undefined;

    // Lookup template from map if ID exists
    const templateId = ex.exercise_template_id;
    const template = templateId ? templateMap[templateId] : undefined;

    const primaryMuscle = mapToMuscleGroupEnum(template?.primary_muscle_group);
    const secondaryMuscles = (template?.secondary_muscle_groups || []).map(mapToMuscleGroupEnum);

    return (ex.sets || []).map((s) => {
      return {
        exerciseName: exTitle,
        reps: s.reps || 0,
        weightKg: s.weight_kg || 0,
        distanceMeters: s.distance_meters || 0,
        startTime: startTimeStr,
        durationSeconds: s.duration_seconds || 0,
        notes: ex.notes || '',
        supersetId: supersetId,
        primaryMuscleGroup: primaryMuscle,
        secondaryMuscleGroups: secondaryMuscles,
        setType: s.type || 'normal',
      } as StrengthSet;
    });
  });

  // Calculate total distance across all sets
  const totalDistance = strengthSets.reduce((sum, set) => sum + (set.distanceMeters || 0), 0);

  const session: Session = {
    startTime: startTimeStr,
    totalElapsedTime: durationSeconds,
    totalDistance: totalDistance,
    laps: [],
    strengthSets: strengthSets
  };

  return {
    source: 'HEVY',
    externalId: workout.id || 'unknown',
    userId: userId,
    startTime: startTimeStr,
    name: workout.title || 'Hevy Workout',
    type: 'WEIGHT_TRAINING',
    description: workout.description || '',
    sessions: [session],
    tags: [],
    notes: ''
  };
}

function mapToMuscleGroupEnum(muscle: string | undefined): MuscleGroup {
  if (!muscle) return MuscleGroup.MUSCLE_GROUP_UNSPECIFIED;
  const normalized = muscle.toLowerCase().trim();
  switch (normalized) {
    case 'abdominals': return MuscleGroup.MUSCLE_GROUP_ABDOMINALS;
    case 'shoulders': return MuscleGroup.MUSCLE_GROUP_SHOULDERS;
    case 'biceps': return MuscleGroup.MUSCLE_GROUP_BICEPS;
    case 'triceps': return MuscleGroup.MUSCLE_GROUP_TRICEPS;
    case 'forearms': return MuscleGroup.MUSCLE_GROUP_FOREARMS;
    case 'quadriceps': return MuscleGroup.MUSCLE_GROUP_QUADRICEPS;
    case 'hamstrings': return MuscleGroup.MUSCLE_GROUP_HAMSTRINGS;
    case 'calves': return MuscleGroup.MUSCLE_GROUP_CALVES;
    case 'glutes': return MuscleGroup.MUSCLE_GROUP_GLUTES;
    case 'abductors': return MuscleGroup.MUSCLE_GROUP_ABDUCTORS;
    case 'adductors': return MuscleGroup.MUSCLE_GROUP_ADDUCTORS;
    case 'lats': return MuscleGroup.MUSCLE_GROUP_LATS;
    case 'upper_back': return MuscleGroup.MUSCLE_GROUP_UPPER_BACK;
    case 'traps': return MuscleGroup.MUSCLE_GROUP_TRAPS;
    case 'lower_back': return MuscleGroup.MUSCLE_GROUP_LOWER_BACK;
    case 'chest': return MuscleGroup.MUSCLE_GROUP_CHEST;
    case 'cardio': return MuscleGroup.MUSCLE_GROUP_CARDIO;
    case 'neck': return MuscleGroup.MUSCLE_GROUP_NECK;
    case 'full_body': return MuscleGroup.MUSCLE_GROUP_FULL_BODY;
    case 'other': return MuscleGroup.MUSCLE_GROUP_OTHER;
    default: return MuscleGroup.MUSCLE_GROUP_OTHER;
  }
}
