import { mapHevyWorkoutToStandardized } from './mapper';
import { components } from '@fitglue/shared/dist/integrations/hevy/schema';

type HevyWorkout = components["schemas"]["Workout"];

describe('mapHevyWorkoutToStandardized', () => {
  const mockWorkout: HevyWorkout = {
    id: 'w123',
    title: 'Morning Lift',
    start_time: '2023-01-01T10:00:00Z',
    end_time: '2023-01-01T11:00:00Z', // 1 hour duration
    description: 'Heavy chest day',
    exercises: [
      {
        title: 'Bench Press',
        superset_id: 0, // Test 0 ID mapping
        notes: 'Felt strong',
        exercise_template_id: 'tmpl_bench',
        sets: [
          { reps: 10, weight_kg: 60, index: 0 },
          { reps: 8, weight_kg: 80, index: 1 }
        ]
      },
      {
        title: 'Push ups',
        superset_id: 123,
        exercise_template_id: 'tmpl_pushup',
        sets: [
          { reps: 20, weight_kg: 0, index: 0 }
        ]
      }
    ]
  };

  const mockTemplateMap: any = {
    'tmpl_bench': {
      primary_muscle_group: 'Chest',
      secondary_muscle_groups: ['Triceps']
    },
    'tmpl_pushup': {
      primary_muscle_group: 'Chest'
    }
  };

  it('should map flat activity fields correctly', () => {
    const result = mapHevyWorkoutToStandardized('user1', mockWorkout, {});

    expect(result.source).toBe('HEVY');
    expect(result.userId).toBe('user1');
    expect(result.name).toBe('Morning Lift');
    expect(result.type).toBe('WEIGHT_TRAINING');
    expect(result.description).toBe('Heavy chest day');
    expect(result.startTime).toBe('2023-01-01T10:00:00Z');
  });

  it('should calculate duration from start and end time', () => {
    const result = mapHevyWorkoutToStandardized('user1', mockWorkout, {});
    // 1 hour = 3600 seconds
    expect(result.sessions[0].totalElapsedTime).toBe(3600);
  });

  it('should map strength sets including muscle groups and supersets', () => {
    const result = mapHevyWorkoutToStandardized('user1', mockWorkout, mockTemplateMap);
    const sets = result.sessions[0].strengthSets;

    expect(sets).toHaveLength(3); // 2 Bench, 1 Push up

    // Check Bench Set 1 (Expect 0 -> "0")
    expect(sets[0]).toMatchObject({
      exerciseName: 'Bench Press',
      reps: 10,
      weightKg: 60,
      primaryMuscleGroup: 'Chest',
      secondaryMuscleGroups: ['Triceps'],
      supersetId: '0'
    });

    // Check Push up (Superset)
    expect(sets[2]).toMatchObject({
      exerciseName: 'Push ups',
      reps: 20,
      supersetId: '123'
    });
  });

  it('should handle missing exercises', () => {
    const emptyWorkout = { ...mockWorkout, exercises: undefined };
    const result = mapHevyWorkoutToStandardized('user1', emptyWorkout, {});
    expect(result.sessions[0].strengthSets).toEqual([]);
  });
});
