import { components } from '@fitglue/shared/dist/integrations/hevy/schema'; // Import from dist to access generated types
import { StandardizedActivity, Session, StrengthSet } from '@fitglue/shared/dist/types/pb/standardized_activity';

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

    const primaryMuscle = template?.primary_muscle_group;
    const secondaryMuscles = template?.secondary_muscle_groups || [];

    return (ex.sets || []).map((s) => {
      return {
        exerciseName: exTitle,
        reps: s.reps || 0,
        weightKg: s.weight_kg || 0,
        startTime: startTimeStr,
        durationSeconds: 0,
        notes: ex.notes || '',
        supersetId: supersetId,
        primaryMuscleGroup: primaryMuscle,
        secondaryMuscleGroups: secondaryMuscles,
      } as StrengthSet;
    });
  });

  const session: Session = {
    startTime: startTimeStr,
    totalElapsedTime: durationSeconds,
    totalDistance: 0,
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
