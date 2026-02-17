import { mapToStandardizedActivity, MobileActivityMetadata, TelemetryData } from './mapper';

// Mock @fitglue/shared/types — provide required enums matching actual protobuf values
jest.mock('@fitglue/shared/types', () => ({
    ActivityType: {
        ACTIVITY_TYPE_UNSPECIFIED: 0,
        ACTIVITY_TYPE_CROSSFIT: 5,
        ACTIVITY_TYPE_ELLIPTICAL: 7,
        ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING: 12,
        ACTIVITY_TYPE_HIKE: 13,
        ACTIVITY_TYPE_RIDE: 23,
        ACTIVITY_TYPE_ROWING: 26,
        ACTIVITY_TYPE_RUN: 27,
        ACTIVITY_TYPE_STAIR_STEPPER: 34,
        ACTIVITY_TYPE_SWIM: 37,
        ACTIVITY_TYPE_TRAIL_RUN: 40,
        ACTIVITY_TYPE_WALK: 45,
        ACTIVITY_TYPE_WEIGHT_TRAINING: 46,
        ACTIVITY_TYPE_WORKOUT: 49,
        ACTIVITY_TYPE_YOGA: 50,
    },
}));

describe('mapper', () => {
    const baseMetadata: MobileActivityMetadata = {
        userId: 'user-1',
        activityId: 'act-1',
        source: 'SOURCE_APPLE_HEALTH',
        activityType: 'Run',
        name: 'Running',
        startTime: new Date('2026-01-01T12:00:00Z'),
        endTime: new Date('2026-01-01T12:30:00Z'),
        durationSeconds: 1800,
        calories: 300,
        distanceMeters: 5000,
        heartRateSampleCount: 0,
        routePointCount: 0,
    };

    it('maps summary-only activity (no telemetry)', () => {
        const result = mapToStandardizedActivity(baseMetadata);

        expect(result.externalId).toBe('act-1');
        expect(result.source).toBe('SOURCE_APPLE_HEALTH');
        expect(result.name).toBe('Running');
        expect(result.type).toBe(27); // ACTIVITY_TYPE_RUN
        expect(result.sessions).toHaveLength(1);

        const session = result.sessions[0];
        expect(session.totalElapsedTime).toBe(1800);
        expect(session.totalDistance).toBe(5000);
        expect(session.totalCalories).toBe(300);
        expect(session.laps).toHaveLength(1);
        expect(session.laps[0].records).toHaveLength(0);
    });

    it('maps Health Connect source correctly', () => {
        const metadata = { ...baseMetadata, source: 'SOURCE_HEALTH_CONNECT' };
        const result = mapToStandardizedActivity(metadata);
        expect(result.source).toBe('SOURCE_HEALTH_CONNECT');
    });

    it('maps activity with HR samples only', () => {
        const telemetry: TelemetryData = {
            heartRateSamples: [
                { timestamp: '2026-01-01T12:00:00Z', bpm: 120 },
                { timestamp: '2026-01-01T12:05:00Z', bpm: 140 },
                { timestamp: '2026-01-01T12:10:00Z', bpm: 160 },
            ],
            route: [],
        };

        const result = mapToStandardizedActivity(baseMetadata, telemetry);

        const session = result.sessions[0];
        expect(session.avgHeartRate).toBe(140);
        expect(session.maxHeartRate).toBe(160);

        const records = session.laps[0].records;
        expect(records).toHaveLength(3);
        expect(records[0].heartRate).toBe(120);
        expect(records[0].positionLat).toBe(0); // No GPS, defaults to 0
    });

    it('maps activity with GPS route only', () => {
        const telemetry: TelemetryData = {
            heartRateSamples: [],
            route: [
                { timestamp: '2026-01-01T12:00:00Z', latitude: 52.95, longitude: -1.15, altitude: 50 },
                { timestamp: '2026-01-01T12:05:00Z', latitude: 52.96, longitude: -1.16, altitude: 55 },
            ],
        };

        const result = mapToStandardizedActivity(baseMetadata, telemetry);

        const records = result.sessions[0].laps[0].records;
        expect(records).toHaveLength(2);
        expect(records[0].positionLat).toBe(52.95);
        expect(records[0].positionLong).toBe(-1.15);
        expect(records[0].altitude).toBe(50);
        expect(records[0].heartRate).toBe(0); // No HR, defaults to 0
        expect(result.sessions[0].avgHeartRate).toBeUndefined();
    });

    it('merges HR + GPS into time-aligned records', () => {
        const telemetry: TelemetryData = {
            heartRateSamples: [
                { timestamp: '2026-01-01T12:00:00Z', bpm: 120 },
                { timestamp: '2026-01-01T12:05:00Z', bpm: 140 },
                { timestamp: '2026-01-01T12:10:00Z', bpm: 155 }, // HR-only timestamp
            ],
            route: [
                { timestamp: '2026-01-01T12:00:00Z', latitude: 52.95, longitude: -1.15 },
                { timestamp: '2026-01-01T12:05:00Z', latitude: 52.96, longitude: -1.16 },
                { timestamp: '2026-01-01T12:07:00Z', latitude: 52.97, longitude: -1.17 }, // GPS-only timestamp
            ],
        };

        const result = mapToStandardizedActivity(baseMetadata, telemetry);

        const records = result.sessions[0].laps[0].records;
        // 4 unique timestamps: 12:00, 12:05, 12:07, 12:10
        expect(records).toHaveLength(4);

        // 12:00 — both HR and GPS
        expect(records[0].heartRate).toBe(120);
        expect(records[0].positionLat).toBe(52.95);

        // 12:05 — both HR and GPS
        expect(records[1].heartRate).toBe(140);
        expect(records[1].positionLat).toBe(52.96);

        // 12:07 — GPS only
        expect(records[2].heartRate).toBe(0);
        expect(records[2].positionLat).toBe(52.97);

        // 12:10 — HR only
        expect(records[3].heartRate).toBe(155);
        expect(records[3].positionLat).toBe(0);
    });

    it('handles Firestore Timestamp objects', () => {
        const metadata = {
            ...baseMetadata,
            startTime: { toDate: () => new Date('2026-01-01T12:00:00Z') },
            endTime: { toDate: () => new Date('2026-01-01T12:30:00Z') },
        };

        const result = mapToStandardizedActivity(metadata);
        expect(result.startTime).toEqual(new Date('2026-01-01T12:00:00Z'));
    });

    it('handles null calories and distance', () => {
        const metadata = { ...baseMetadata, calories: null, distanceMeters: null };
        const result = mapToStandardizedActivity(metadata);
        expect(result.sessions[0].totalDistance).toBe(0);
        expect(result.sessions[0].totalCalories).toBeUndefined();
    });

    it('maps WeightTraining activity type', () => {
        const metadata = { ...baseMetadata, name: 'WeightTraining' };
        const result = mapToStandardizedActivity(metadata);
        expect(result.type).toBe(46); // ACTIVITY_TYPE_WEIGHT_TRAINING
    });

    it('defaults unknown activity types to WORKOUT', () => {
        const metadata = { ...baseMetadata, name: 'Parkour' };
        const result = mapToStandardizedActivity(metadata);
        expect(result.type).toBe(49); // ACTIVITY_TYPE_WORKOUT
    });
});
