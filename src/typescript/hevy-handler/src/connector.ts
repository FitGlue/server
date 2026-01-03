import { BaseConnector, ConnectorConfig, IngestStrategy, StandardizedActivity, Session, StrengthSet, MuscleGroup, CloudEventSource, ActivitySource, createHevyClient, FrameworkContext } from '@fitglue/shared';
import type { components } from "@fitglue/shared/dist/integrations/hevy/schema";

// Define Hevy-specific types
type HevyWorkout = components["schemas"]["Workout"];
type HevyExerciseTemplate = components["schemas"]["ExerciseTemplate"];

export interface HevyConnectorConfig extends ConnectorConfig {
  apiKey: string;
}

export class HevyConnector extends BaseConnector<HevyConnectorConfig, HevyWorkout> {
  readonly name = 'hevy';
  readonly strategy: IngestStrategy = 'webhook';
  readonly cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_HEVY;
  readonly activitySource = ActivitySource.SOURCE_HEVY;

  constructor(context: FrameworkContext) {
    super(context);
    context.logger.debug(`HevyConnector: initialized`);
  }

  extractId(body: any): string | null {
    // Return workoutId from payload (support various payload shapes)
    if (!body) return null;
    return body.payload?.workoutId || null;
  }

  async fetchAndMap(activityId: string, config: HevyConnectorConfig): Promise<StandardizedActivity[]> {
    this.context.logger.debug(`HevyConnector: fetching and mapping workout ${activityId}`, { config });
    const client = createHevyClient({
      apiKey: config.apiKey,
      usageTracking: {
        userStore: this.context.stores.users,
        userId: (config as any).userId
      }
    });
    const { data: fullWorkout, error, response } = await client.GET("/v1/workouts/{workoutId}", {
      params: { path: { workoutId: activityId } }
    });

    if (error || !fullWorkout) {
      throw new Error(`Hevy API error: ${response.status} ${response.statusText}`);
    }

    // Identify unique exercise template IDs
    const templateIds = new Set<string>();
    (fullWorkout.exercises || []).forEach((ex: any) => {
      if (ex.exercise_template_id) {
        templateIds.add(ex.exercise_template_id);
      }
    });

    // Fetch all templates concurrently
    const templatePromises = Array.from(templateIds).map(async (tmplId) => {
      const { data: tmplData, error: tmplError } = await client.GET("/v1/exercise_templates/{exerciseTemplateId}", {
        params: { path: { exerciseTemplateId: tmplId } }
      });

      if (tmplError || !tmplData) {
        console.warn(`Failed to fetch template ${tmplId}`);
        return { id: tmplId, data: undefined };
      }

      return { id: tmplId, data: tmplData };
    });

    const templates = await Promise.all(templatePromises);
    const templateMap: Record<string, any> = {};
    templates.forEach((res) => {
      if (res.data) {
        templateMap[res.id] = res.data;
      }
    });

    // I'll assume config has userId for now, provided by the caller who loads the config.
    const userId = (config as any).userId;
    if (!userId) {
      throw new Error("userId missing in connector config");
    }

    const standardized = await this.mapActivity(fullWorkout, { userId, templateMap });
    return [standardized]; // Wrap in array for consistent interface
  }

  /**
   * Maps a Hevy workout to a StandardizedActivity.
   * Expects `context` to contain `templateMap` (Record<string, HevyExerciseTemplate>).
   */
  async mapActivity(workout: HevyWorkout, context?: { userId: string, templateMap: Record<string, HevyExerciseTemplate> }): Promise<StandardizedActivity> {
    this.context.logger.debug(`HevyConnector: mapping workout ${workout.id}, ${workout.title || 'Unknown Title'}`, { workout });
    if (!context?.userId) throw new Error("HevyMapping requires userId in context");
    const templateMap = context.templateMap || {};
    const userId = context.userId;

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

      // Handle superset_id from payload
      const rawSupersetId = ex.superset_id;
      const supersetId = (rawSupersetId !== undefined && rawSupersetId !== null)
        ? String(rawSupersetId)
        : undefined;

      // Lookup template from map if ID exists
      const templateId = ex.exercise_template_id;
      const template = templateId ? templateMap[templateId] : undefined;

      const primaryMuscle = this.mapToMuscleGroupEnum(template?.primary_muscle_group);
      const secondaryMuscles = (template?.secondary_muscle_groups || []).map(this.mapToMuscleGroupEnum);

      return (ex.sets || []).map((s) => {
        return {
          exerciseName: exTitle,
          reps: s.reps || 0,
          weightKg: s.weight_kg || 0,
          distanceMeters: s.distance_meters || 0,
          startTime: startTime, // Use Date object
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
      startTime: startTime,
      totalElapsedTime: durationSeconds,
      totalDistance: totalDistance,
      laps: [],
      strengthSets: strengthSets
    };

    return {
      source: 'HEVY',
      externalId: workout.id || 'unknown',
      userId: userId,
      startTime: startTime,
      name: workout.title || 'Hevy Workout',
      type: 'WEIGHT_TRAINING',
      description: workout.description || '',
      sessions: [session],
      tags: [],
      notes: ''
    };
  }

  private mapToMuscleGroupEnum(muscle: string | undefined): MuscleGroup {
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
}
