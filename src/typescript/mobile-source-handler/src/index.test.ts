import { handler } from './index';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
    createCloudFunction: jest.fn((handler: any) => handler),
    db: {
        collection: jest.fn(),
    },
}));

// Mock @fitglue/shared/infrastructure/pubsub
const mockPublish = jest.fn().mockResolvedValue('msg-id-456');
jest.mock('@fitglue/shared/infrastructure/pubsub', () => ({
    CloudEventPublisher: jest.fn().mockImplementation(() => ({
        publish: mockPublish,
    })),
}));

// Mock @fitglue/shared/types
jest.mock('@fitglue/shared/types', () => ({
    ActivitySource: {
        SOURCE_APPLE_HEALTH: 8,
        SOURCE_HEALTH_CONNECT: 9,
        SOURCE_UNSPECIFIED: 0,
        // Reverse mappings (TypeScript enum behavior)
        8: 'SOURCE_APPLE_HEALTH',
        9: 'SOURCE_HEALTH_CONNECT',
        0: 'SOURCE_UNSPECIFIED',
    },
    ActivityType: {
        ACTIVITY_TYPE_RUN: 1,
        ACTIVITY_TYPE_WORKOUT: 10,
        ACTIVITY_TYPE_WEIGHT_TRAINING: 13,
    },
    CloudEventType: { CLOUD_EVENT_TYPE_ACTIVITY_CREATED: 1 },
    CloudEventSource: {
        CLOUD_EVENT_SOURCE_APPLE_HEALTH: 15,
        CLOUD_EVENT_SOURCE_HEALTH_CONNECT: 16,
    },
    getCloudEventSource: jest.fn((s: number) => `/integrations/${s}`),
    getCloudEventType: jest.fn((t: number) => `com.fitglue.activity.${t}`),
}));

// Mock firebase-admin/storage
jest.mock('firebase-admin/storage', () => {
    const mockDownload = jest.fn().mockResolvedValue([Buffer.from('{}')]);
    const mockExists = jest.fn().mockResolvedValue([true]);
    const mockFile = jest.fn().mockReturnValue({
        exists: mockExists,
        download: mockDownload,
    });
    const mockBucket = jest.fn().mockReturnValue({ file: mockFile });
    return {
        getStorage: jest.fn().mockReturnValue({ bucket: mockBucket }),
        __mocks: { mockDownload, mockExists, mockFile, mockBucket },
    };
});

import { db } from '@fitglue/shared/framework';

// Access GCS mocks
const gcsMocks = () => require('firebase-admin/storage').__mocks as {
    mockDownload: jest.Mock;
    mockExists: jest.Mock;
    mockFile: jest.Mock;
    mockBucket: jest.Mock;
};

describe('mobile-source-handler', () => {
    let ctx: any;
    let mockActivityDocRef: any;
    let mockPipelinesQuery: any;

    const firestoreMetadata = {
        userId: 'user-1',
        activityId: 'act-1',
        source: 'SOURCE_APPLE_HEALTH',
        activityType: 'Run',
        name: 'Running',
        startTime: { toDate: () => new Date('2026-01-01T12:00:00Z') },
        endTime: { toDate: () => new Date('2026-01-01T12:30:00Z') },
        durationSeconds: 1800,
        calories: 300,
        distanceMeters: 5000,
        heartRateSampleCount: 2,
        routePointCount: 0,
        telemetryUri: 'gs://test-bucket/mobile_activities/user-1/act-1.json',
        status: 'stored',
    };

    beforeEach(() => {
        jest.clearAllMocks();

        mockActivityDocRef = {
            get: jest.fn().mockResolvedValue({
                exists: true,
                data: () => ({ ...firestoreMetadata }),
            }),
            update: jest.fn().mockResolvedValue(undefined),
        };

        // Pipeline query mock — default: has a matching pipeline
        mockPipelinesQuery = {
            where: jest.fn().mockReturnThis(),
            limit: jest.fn().mockReturnThis(),
            get: jest.fn().mockResolvedValue({ empty: false }),
        };

        // Mock: db.collection('users').doc(userId).collection('pipelines' | 'mobile_activities')
        const mockActivitiesCollection = {
            doc: jest.fn().mockReturnValue(mockActivityDocRef),
        };
        const mockUserDoc = {
            collection: jest.fn((name: string) => {
                if (name === 'pipelines') return mockPipelinesQuery;
                return mockActivitiesCollection;
            }),
        };
        const mockUsersCollection = {
            doc: jest.fn().mockReturnValue(mockUserDoc),
        };
        (db.collection as jest.Mock).mockReturnValue(mockUsersCollection);

        ctx = {
            logger: {
                info: jest.fn(),
                warn: jest.fn(),
                error: jest.fn(),
            },
            pubsub: {},
        };

        // Setup GCS telemetry response
        const { mockDownload } = gcsMocks();
        const telemetry = {
            heartRateSamples: [
                { timestamp: '2026-01-01T12:00:00Z', bpm: 120 },
                { timestamp: '2026-01-01T12:15:00Z', bpm: 150 },
            ],
            route: [],
        };
        mockDownload.mockResolvedValue([Buffer.from(JSON.stringify(telemetry))]);
    });

    function makeReq(message: object) {
        return {
            body: {
                message: {
                    data: Buffer.from(JSON.stringify(message)).toString('base64'),
                },
            },
        };
    }

    it('processes a mobile activity and publishes to topic-raw-activity', async () => {
        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'healthkit',
            telemetryUri: 'gs://test-bucket/mobile_activities/user-1/act-1.json',
        });

        const result: any = await handler(req as any, ctx);

        expect(result.status).toBe('published');
        expect(result.activityId).toBe('act-1');
        expect(result.messageId).toBe('msg-id-456');

        // Verify published to topic-raw-activity
        const { CloudEventPublisher } = require('@fitglue/shared/infrastructure/pubsub');
        expect(CloudEventPublisher).toHaveBeenCalledWith(
            ctx.pubsub,
            'topic-raw-activity',
            expect.any(String),
            expect.any(String),
            ctx.logger
        );

        // Verify ActivityPayload was published with correct structure
        expect(mockPublish).toHaveBeenCalledWith(
            expect.objectContaining({
                userId: 'user-1',
                source: 8, // SOURCE_APPLE_HEALTH
                originalPayloadJson: expect.any(String),
                standardizedActivity: expect.objectContaining({
                    externalId: 'act-1',
                    type: 1, // ACTIVITY_TYPE_RUN
                }),
            }),
            'act-1'
        );
    });

    it('updates Firestore status to published', async () => {
        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'healthkit',
        });

        await handler(req as any, ctx);

        expect(mockActivityDocRef.update).toHaveBeenCalledWith(
            expect.objectContaining({
                status: 'published',
                publishedAt: expect.any(Date),
                pipelineExecutionId: expect.stringContaining('mobile-act-1-'),
            })
        );
    });

    it('fetches telemetry from GCS and includes HR stats', async () => {
        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'healthkit',
            telemetryUri: 'gs://test-bucket/mobile_activities/user-1/act-1.json',
        });

        await handler(req as any, ctx);

        // Verify GCS was accessed
        const { mockBucket, mockFile } = gcsMocks();
        expect(mockBucket).toHaveBeenCalledWith('test-bucket');
        expect(mockFile).toHaveBeenCalledWith('mobile_activities/user-1/act-1.json');

        // Verify HR data flows through to StandardizedActivity.sessions
        expect(mockPublish).toHaveBeenCalledWith(
            expect.objectContaining({
                standardizedActivity: expect.objectContaining({
                    sessions: expect.arrayContaining([
                        expect.objectContaining({
                            avgHeartRate: 135,
                            maxHeartRate: 150,
                        }),
                    ]),
                }),
            }),
            'act-1'
        );
    });

    it('skips gracefully if activity not found in Firestore', async () => {
        mockActivityDocRef.get.mockResolvedValue({ exists: false });

        const req = makeReq({
            userId: 'user-1',
            activityId: 'missing-act',
            source: 'healthkit',
        });

        const result: any = await handler(req as any, ctx);

        expect(result.status).toBe('skipped');
        expect(result.reason).toBe('activity_not_found');
        expect(mockPublish).not.toHaveBeenCalled();
    });

    it('skips if no pipeline is configured for the source', async () => {
        mockPipelinesQuery.get.mockResolvedValue({ empty: true });

        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'healthkit',
        });

        const result: any = await handler(req as any, ctx);

        expect(result.status).toBe('skipped');
        expect(result.reason).toBe('no_pipeline_for_source');
        expect(mockPublish).not.toHaveBeenCalled();
        // Should not even fetch the activity from Firestore
        expect(mockActivityDocRef.get).not.toHaveBeenCalled();
    });

    it('proceeds without telemetry if GCS file does not exist', async () => {
        const { mockExists } = gcsMocks();
        mockExists.mockResolvedValue([false]);

        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'healthkit',
            telemetryUri: 'gs://test-bucket/mobile_activities/user-1/act-1.json',
        });

        const result: any = await handler(req as any, ctx);

        expect(result.status).toBe('published');
        // Should still publish but Session won't have HR stats
        expect(mockPublish).toHaveBeenCalledWith(
            expect.objectContaining({
                standardizedActivity: expect.objectContaining({
                    sessions: expect.arrayContaining([
                        expect.objectContaining({
                            avgHeartRate: undefined,
                            maxHeartRate: undefined,
                        }),
                    ]),
                }),
            }),
            'act-1'
        );
    });

    it('handles Health Connect source correctly', async () => {
        mockActivityDocRef.get.mockResolvedValue({
            exists: true,
            data: () => ({
                ...firestoreMetadata,
                source: 'SOURCE_HEALTH_CONNECT',
                telemetryUri: null,
            }),
        });

        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'health_connect',
        });

        await handler(req as any, ctx);

        expect(mockPublish).toHaveBeenCalledWith(
            expect.objectContaining({
                source: 9, // SOURCE_HEALTH_CONNECT
            }),
            'act-1'
        );
    });

    it('falls back to Firestore telemetryUri if not in message', async () => {
        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'healthkit',
            // No telemetryUri in message — should use metadata.telemetryUri
        });

        await handler(req as any, ctx);

        // Should still fetch from GCS using metadata.telemetryUri
        const { mockBucket } = gcsMocks();
        expect(mockBucket).toHaveBeenCalled();
    });

    it('serializes originalPayload as JSON string', async () => {
        const req = makeReq({
            userId: 'user-1',
            activityId: 'act-1',
            source: 'healthkit',
        });

        await handler(req as any, ctx);

        const publishedPayload = mockPublish.mock.calls[0][0];
        expect(typeof publishedPayload.originalPayloadJson).toBe('string');
        const parsed = JSON.parse(publishedPayload.originalPayloadJson);
        expect(parsed.activityId).toBe('act-1');
    });

    describe('parseMessage CloudEvent unwrapping', () => {
        it('handles framework-unwrapped CloudEvent payload for Health Connect', async () => {
            // After the framework fix, req.body arrives as the direct payload
            mockActivityDocRef.get.mockResolvedValue({
                exists: true,
                data: () => ({
                    ...firestoreMetadata,
                    source: 'SOURCE_HEALTH_CONNECT',
                    telemetryUri: null,
                }),
            });

            const req = {
                body: {
                    userId: 'user-1',
                    activityId: 'act-1',
                    source: 'health_connect',
                },
            };

            const result: any = await handler(req as any, ctx);

            expect(result.status).toBe('published');
            expect(result.activityId).toBe('act-1');
            expect(mockPublish).toHaveBeenCalledWith(
                expect.objectContaining({
                    source: 9, // SOURCE_HEALTH_CONNECT
                    userId: 'user-1',
                }),
                'act-1'
            );
        });

        it('handles framework-unwrapped CloudEvent payload for Apple Health', async () => {
            const req = {
                body: {
                    userId: 'user-1',
                    activityId: 'act-1',
                    source: 'healthkit',
                },
            };

            const result: any = await handler(req as any, ctx);

            expect(result.status).toBe('published');
            expect(result.activityId).toBe('act-1');
        });

        it('falls back to Pub/Sub decoding with inner CloudEvent', async () => {
            // Tests the parseMessage fallback: single-level Pub/Sub wrapping
            // with a CloudEvent JSON inside (e.g. if framework unwrapping fails)
            const cloudEventJson = JSON.stringify({
                specversion: '1.0',
                type: 'com.fitglue.activity.created',
                source: '/integrations/health_connect',
                id: 'test-ce-id',
                data: {
                    userId: 'user-1',
                    activityId: 'act-1',
                    source: 'healthkit',
                },
                datacontenttype: 'application/json',
            });

            const req = {
                body: {
                    message: {
                        data: Buffer.from(cloudEventJson).toString('base64'),
                    },
                },
            };

            const result: any = await handler(req as any, ctx);

            expect(result.status).toBe('published');
            expect(result.activityId).toBe('act-1');
        });
    });
});
