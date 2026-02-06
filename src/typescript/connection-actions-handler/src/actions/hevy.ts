/**
 * Hevy Action: Import Strength PRs
 *
 * Fetches 12 months of workout history from Hevy and calculates strength PRs
 * (1RM, volume records, max reps) for each exercise.
 */

import { db } from '@fitglue/shared/framework';
import { Timestamp } from 'firebase-admin/firestore';

interface ActionResult {
    recordsImported: number;
    recordsSkipped: number;
    details: string[];
}

interface Logger {
    info: (msg: string, data?: Record<string, unknown>) => void;
}

interface HevyWorkout {
    id: string;
    title: string;
    start_time: string;
    end_time: string;
    exercises: HevyExercise[];
}

interface HevyExercise {
    index: number;
    title: string;
    exercise_template_id: string;
    sets: HevySet[];
}

interface HevySet {
    index: number;
    set_type: 'normal' | 'warmup' | 'dropset' | 'failure';
    reps?: number;
    weight_kg?: number;
    duration_seconds?: number;
}

interface ExerciseRecord {
    oneRM: number | null;
    maxVolume: number | null;
    maxReps: number | null;
    workoutId: string;
    date: string;
    exerciseTitle: string;
}

interface SaveOptions {
    collection: FirebaseFirestore.CollectionReference;
    recordType: string;
    newValue: number;
    unit: string;
    record: ExerciseRecord;
    result: ActionResult;
    lowerIsBetter: boolean;
}

export async function importHevyStrengthPRs(userId: string, logger: Logger): Promise<ActionResult> {
    const result: ActionResult = {
        recordsImported: 0,
        recordsSkipped: 0,
        details: [],
    };

    const apiKey = await getHevyApiKey(userId);
    const workouts = await fetchHevyWorkouts(apiKey, getDateTwelveMonthsAgo(), logger);
    logger.info('Fetched Hevy workouts', { count: workouts.length });

    const exerciseRecords = calculateExerciseRecords(workouts);
    await saveRecordsToFirestore(userId, exerciseRecords, result);

    logger.info('Hevy PR import complete', {
        imported: result.recordsImported,
        skipped: result.recordsSkipped,
    });

    return result;
}

async function getHevyApiKey(userId: string): Promise<string> {
    const userDoc = await db.collection('users').doc(userId).get();
    const userData = userDoc.data();
    const hevyIntegration = userData?.integrations?.hevy;

    if (!hevyIntegration?.api_key) {
        throw new Error('Hevy not connected. Please connect your Hevy account first.');
    }

    return hevyIntegration.api_key;
}

function getDateTwelveMonthsAgo(): Date {
    const date = new Date();
    date.setMonth(date.getMonth() - 12);
    return date;
}

function calculateExerciseRecords(workouts: HevyWorkout[]): Map<string, ExerciseRecord> {
    const exerciseRecords = new Map<string, ExerciseRecord>();

    for (const workout of workouts) {
        processWorkout(workout, exerciseRecords);
    }

    return exerciseRecords;
}

function processWorkout(workout: HevyWorkout, records: Map<string, ExerciseRecord>): void {
    for (const exercise of workout.exercises) {
        processExercise(workout, exercise, records);
    }
}

function processExercise(
    workout: HevyWorkout,
    exercise: HevyExercise,
    records: Map<string, ExerciseRecord>
): void {
    const exerciseKey = normalizeExerciseName(exercise.title);

    for (const set of exercise.sets) {
        if (set.set_type === 'warmup' || !set.reps || !set.weight_kg) {
            continue;
        }

        updateRecordIfBetter(records, exerciseKey, {
            oneRM: calculate1RM(set.weight_kg, set.reps),
            volume: set.weight_kg * set.reps,
            reps: set.reps,
            workoutId: workout.id,
            date: workout.start_time,
            exerciseTitle: exercise.title,
        });
    }
}

function updateRecordIfBetter(
    records: Map<string, ExerciseRecord>,
    key: string,
    candidate: { oneRM: number | null; volume: number; reps: number; workoutId: string; date: string; exerciseTitle: string }
): void {
    const existing = records.get(key);
    const { oneRM, volume, reps, workoutId, date, exerciseTitle } = candidate;

    const newOneRM = (!existing || (oneRM && (!existing.oneRM || oneRM > existing.oneRM)))
        ? oneRM
        : existing?.oneRM ?? null;

    const newVolume = (!existing || volume > (existing.maxVolume ?? 0))
        ? volume
        : existing?.maxVolume ?? null;

    const newReps = (!existing || reps > (existing.maxReps ?? 0))
        ? reps
        : existing?.maxReps ?? null;

    const improved = newOneRM !== existing?.oneRM || newVolume !== existing?.maxVolume || newReps !== existing?.maxReps;

    if (improved || !existing) {
        records.set(key, {
            oneRM: newOneRM,
            maxVolume: newVolume,
            maxReps: newReps,
            workoutId,
            date,
            exerciseTitle,
        });
    }
}

async function saveRecordsToFirestore(
    userId: string,
    exerciseRecords: Map<string, ExerciseRecord>,
    result: ActionResult
): Promise<void> {
    const recordsCollection = db.collection('users').doc(userId).collection('personal_records');

    for (const [exerciseKey, record] of exerciseRecords.entries()) {
        if (record.oneRM) {
            const saved = await saveIfBetter({
                collection: recordsCollection,
                recordType: `${exerciseKey}_1rm`,
                newValue: record.oneRM,
                unit: 'kg',
                record,
                result,
                lowerIsBetter: false,
            });
            if (saved) {
                result.details.push(`${record.exerciseTitle} 1RM: ${record.oneRM.toFixed(1)}kg`);
            }
        }

        if (record.maxVolume) {
            const saved = await saveIfBetter({
                collection: recordsCollection,
                recordType: `${exerciseKey}_volume`,
                newValue: record.maxVolume,
                unit: 'kg',
                record,
                result,
                lowerIsBetter: false,
            });
            if (saved) {
                result.details.push(`${record.exerciseTitle} Volume: ${record.maxVolume.toFixed(0)}kg`);
            }
        }
    }
}

async function fetchHevyWorkouts(apiKey: string, since: Date, logger: Logger): Promise<HevyWorkout[]> {
    const workouts: HevyWorkout[] = [];
    const pageSize = 10;
    const maxPages = 100;

    for (let page = 1; page <= maxPages; page++) {
        const response = await fetch(
            `https://api.hevyapp.com/v1/workouts?page=${page}&pageSize=${pageSize}`,
            { headers: { 'api-key': apiKey, 'Content-Type': 'application/json' } }
        );

        if (!response.ok) {
            if (response.status === 429) {
                logger.info('Rate limited by Hevy API, stopping pagination', { page });
                break;
            }
            throw new Error(`Hevy API error: ${response.status}`);
        }

        const data = await response.json() as { workouts: HevyWorkout[]; page_count: number };
        const pageWorkouts = data.workouts.filter(w => new Date(w.start_time) >= since);
        workouts.push(...pageWorkouts);

        if (pageWorkouts.length < data.workouts.length || page >= data.page_count) {
            break;
        }
    }

    return workouts;
}

function calculate1RM(weight: number, reps: number): number | null {
    if (reps <= 0 || reps > 36 || weight <= 0) return null;
    if (reps === 1) return weight;
    return weight * (36 / (37 - reps));
}

function normalizeExerciseName(name: string): string {
    return name.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
}

async function saveIfBetter(options: SaveOptions): Promise<boolean> {
    const { collection, recordType, newValue, unit, record, result, lowerIsBetter } = options;
    const existingDoc = await collection.doc(recordType).get();
    const existing = existingDoc.data();

    if (existing) {
        const existingIsBetter = lowerIsBetter ? existing.value <= newValue : existing.value >= newValue;
        if (existingIsBetter) {
            result.recordsSkipped++;
            return false;
        }
    }

    const improvement = existing
        ? (lowerIsBetter
            ? ((existing.value - newValue) / existing.value) * 100
            : ((newValue - existing.value) / existing.value) * 100)
        : undefined;

    await collection.doc(recordType).set({
        record_type: recordType,
        value: newValue,
        unit,
        activity_id: record.workoutId,
        achieved_at: Timestamp.fromDate(new Date(record.date)),
        previous_value: existing?.value,
        improvement,
        source: 'hevy_import',
    });

    result.recordsImported++;
    return true;
}
