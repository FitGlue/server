/**
 * Mobile Activity Mapper
 *
 * Maps mobile health data (Firestore metadata + GCS telemetry) into
 * StandardizedActivity format for the FitGlue pipeline.
 *
 * Handles:
 * - Activity summary fields (name, duration, calories, distance)
 * - Heart rate samples → Record[] entries
 * - GPS route points → Record[] entries
 * - Time-aligned merging when both HR + GPS present
 */

import {
    StandardizedActivity,
    Session,
    Lap,
    Record as FitRecord,
    ActivityType,
} from '@fitglue/shared/types';

import { mapMobileActivityType } from './activity-types';

/**
 * Firestore metadata stored by mobile-sync-handler
 */
export interface MobileActivityMetadata {
    userId: string;
    activityId: string;
    source: string; // 'SOURCE_APPLE_HEALTH' | 'SOURCE_HEALTH_CONNECT'
    activityType: string;
    name: string;
    startTime: { toDate(): Date } | Date;
    endTime: { toDate(): Date } | Date;
    durationSeconds: number;
    calories?: number | null;
    distanceMeters?: number | null;
    heartRateSampleCount: number;
    routePointCount: number;
    telemetryUri?: string | null;
}

/**
 * GCS telemetry blob structure
 */
export interface TelemetryData {
    heartRateSamples: Array<{ timestamp: string; bpm: number }>;
    route: Array<{ timestamp: string; latitude: number; longitude: number; altitude?: number }>;
}

/**
 * Convert Firestore Timestamp-like or Date to Date
 */
function toDate(value: { toDate(): Date } | Date): Date {
    if (value instanceof Date) return value;
    return value.toDate();
}

/**
 * Build Record[] from heart rate samples and route points.
 * When both are present, merge by aligning on nearest timestamp.
 * Note: FitRecord fields default to 0 when not available.
 */
function buildRecords(telemetry: TelemetryData): FitRecord[] {
    const records: FitRecord[] = [];

    const hrSamples = telemetry.heartRateSamples || [];
    const routePoints = telemetry.route || [];

    if (hrSamples.length === 0 && routePoints.length === 0) {
        return records;
    }

    // If only HR samples
    if (routePoints.length === 0) {
        for (const sample of hrSamples) {
            records.push({
                timestamp: new Date(sample.timestamp),
                heartRate: sample.bpm,
                power: 0,
                cadence: 0,
                speed: 0,
                altitude: 0,
                positionLat: 0,
                positionLong: 0,
            });
        }
        return records;
    }

    // If only route points
    if (hrSamples.length === 0) {
        for (const point of routePoints) {
            records.push({
                timestamp: new Date(point.timestamp),
                heartRate: 0,
                power: 0,
                cadence: 0,
                speed: 0,
                altitude: point.altitude || 0,
                positionLat: point.latitude,
                positionLong: point.longitude,
            });
        }
        return records;
    }

    // Both present — merge by timestamp alignment
    const hrMap = new Map<string, number>();
    for (const s of hrSamples) {
        hrMap.set(s.timestamp, s.bpm);
    }

    const routeMap = new Map<string, typeof routePoints[0]>();
    for (const p of routePoints) {
        routeMap.set(p.timestamp, p);
    }

    // Combine all unique timestamps
    const allTimestamps = new Set([...hrMap.keys(), ...routeMap.keys()]);
    const sorted = Array.from(allTimestamps).sort();

    for (const ts of sorted) {
        const hr = hrMap.get(ts);
        const point = routeMap.get(ts);

        records.push({
            timestamp: new Date(ts),
            heartRate: hr || 0,
            power: 0,
            cadence: 0,
            speed: 0,
            altitude: point?.altitude || 0,
            positionLat: point?.latitude || 0,
            positionLong: point?.longitude || 0,
        });
    }

    return records;
}

/**
 * Map mobile activity type name to ActivityType enum
 */
function mapActivityTypeEnum(name: string): ActivityType {
    const sportStr = mapMobileActivityType(name);

    // Map to ActivityType enum — these align with Strava activity types
    const mapping: Record<string, ActivityType> = {
        'Run': ActivityType.ACTIVITY_TYPE_RUN,
        'TrailRun': ActivityType.ACTIVITY_TYPE_TRAIL_RUN,
        'Walk': ActivityType.ACTIVITY_TYPE_WALK,
        'Ride': ActivityType.ACTIVITY_TYPE_RIDE,
        'Swim': ActivityType.ACTIVITY_TYPE_SWIM,
        'WeightTraining': ActivityType.ACTIVITY_TYPE_WEIGHT_TRAINING,
        'Hike': ActivityType.ACTIVITY_TYPE_HIKE,
        'Yoga': ActivityType.ACTIVITY_TYPE_YOGA,
        'Workout': ActivityType.ACTIVITY_TYPE_WORKOUT,
        'Elliptical': ActivityType.ACTIVITY_TYPE_ELLIPTICAL,
        'Rowing': ActivityType.ACTIVITY_TYPE_ROWING,
        'StairStepper': ActivityType.ACTIVITY_TYPE_STAIR_STEPPER,
        'Crossfit': ActivityType.ACTIVITY_TYPE_CROSSFIT,
        'HIIT': ActivityType.ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING,
    };

    return mapping[sportStr] || ActivityType.ACTIVITY_TYPE_WORKOUT;
}

/**
 * Map mobile activity metadata + telemetry to StandardizedActivity
 */
export function mapToStandardizedActivity(
    metadata: MobileActivityMetadata,
    telemetry?: TelemetryData
): StandardizedActivity {
    const startTime = toDate(metadata.startTime);

    const records = telemetry ? buildRecords(telemetry) : [];

    // Compute HR stats from telemetry if available
    let avgHr: number | undefined;
    let maxHr: number | undefined;
    if (telemetry?.heartRateSamples && telemetry.heartRateSamples.length > 0) {
        const hrs = telemetry.heartRateSamples.map(s => s.bpm);
        avgHr = Math.round(hrs.reduce((a, b) => a + b, 0) / hrs.length);
        maxHr = Math.max(...hrs);
    }

    const lap: Lap = {
        startTime,
        totalElapsedTime: metadata.durationSeconds,
        totalDistance: metadata.distanceMeters || 0,
        records,
        exerciseName: '',
        intensity: '',
    };

    const session: Session = {
        startTime,
        totalElapsedTime: metadata.durationSeconds,
        totalDistance: metadata.distanceMeters || 0,
        totalCalories: metadata.calories || undefined,
        avgHeartRate: avgHr,
        maxHeartRate: maxHr,
        laps: [lap],
        strengthSets: [],
    };

    return {
        source: metadata.source,
        externalId: metadata.activityId,
        userId: metadata.userId,
        startTime,
        name: metadata.name,
        type: mapActivityTypeEnum(metadata.name),
        sessions: [session],
        description: '',
        tags: [],
        notes: '',
        timeMarkers: [],
    };
}
