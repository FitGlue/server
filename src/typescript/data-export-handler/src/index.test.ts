/* eslint-disable @typescript-eslint/no-explicit-any */

// Mock framework and dependencies before any imports
const mockGet = jest.fn();
const mockSet = jest.fn();
const mockUpdate = jest.fn();
const mockWhere = jest.fn();
const mockLimit = jest.fn();
const mockOrderBy = jest.fn();

const mockDocRef = {
    get: mockGet,
    set: mockSet,
    update: mockUpdate,
    collection: jest.fn(),
};
const mockCollectionRef = {
    doc: jest.fn().mockReturnValue(mockDocRef),
    where: mockWhere,
    orderBy: mockOrderBy,
    get: mockGet,
    limit: mockLimit,
};

const mockDb = {
    collection: jest.fn().mockReturnValue(mockCollectionRef),
};

mockCollectionRef.doc.mockReturnValue(mockDocRef);
mockDocRef.collection.mockReturnValue(mockCollectionRef);
mockWhere.mockReturnValue(mockCollectionRef);
mockLimit.mockReturnValue(mockCollectionRef);
mockOrderBy.mockReturnValue(mockCollectionRef);

const mockLogger = {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
};

jest.mock('@fitglue/shared/framework', () => ({
    createCloudFunction: jest.fn((h: any) => h),
    FirebaseAuthStrategy: jest.fn(),
    PayloadUserStrategy: jest.fn(),
    db: mockDb,
}));

jest.mock('@fitglue/shared/errors', () => ({
    HttpError: class HttpError extends Error {
        statusCode: number;
        constructor(statusCode: number, message: string) {
            super(message);
            this.statusCode = statusCode;
        }
    },
}));

jest.mock('@google-cloud/tasks', () => ({
    CloudTasksClient: jest.fn().mockImplementation(() => ({
        queuePath: jest.fn().mockReturnValue('projects/test/locations/us-central1/queues/data-export'),
        createTask: jest.fn().mockResolvedValue({}),
    })),
}));

jest.mock('@google-cloud/secret-manager', () => ({
    SecretManagerServiceClient: jest.fn().mockImplementation(() => ({
        accessSecretVersion: jest.fn().mockResolvedValue([{
            payload: { data: 'test-password' },
        }]),
    })),
}));

jest.mock('firebase-admin/messaging', () => ({
    getMessaging: jest.fn().mockReturnValue({
        send: jest.fn().mockResolvedValue('message-id'),
    }),
}));

jest.mock('firebase-admin/auth', () => ({
    getAuth: jest.fn().mockReturnValue({
        getUser: jest.fn().mockResolvedValue({ email: 'test@example.com' }),
    }),
}));

jest.mock('firebase-admin/storage', () => ({
    getStorage: jest.fn().mockReturnValue({
        bucket: jest.fn().mockReturnValue({
            file: jest.fn().mockReturnValue({
                exists: jest.fn().mockResolvedValue([false]),
                download: jest.fn().mockResolvedValue([Buffer.from('{}')]),
                save: jest.fn().mockResolvedValue(undefined),
                getSignedUrl: jest.fn().mockResolvedValue(['https://storage.example.com/signed-url']),
            }),
        }),
    }),
}));

jest.mock('firebase-admin/firestore', () => ({
    Timestamp: { now: jest.fn().mockReturnValue({ seconds: 1234567890 }) },
    FieldValue: { arrayRemove: jest.fn() },
}));

jest.mock('nodemailer', () => ({
    createTransport: jest.fn().mockReturnValue({
        sendMail: jest.fn().mockResolvedValue({}),
    }),
}));

jest.mock('jszip', () => {
    const mockFolder = {
        file: jest.fn().mockReturnThis(),
        folder: jest.fn().mockReturnThis(),
    };
    return jest.fn().mockImplementation(() => ({
        file: jest.fn().mockReturnThis(),
        folder: jest.fn().mockReturnValue(mockFolder),
        generateAsync: jest.fn().mockResolvedValue(Buffer.from('zip-content')),
    }));
});

import { handler } from './index';

describe('data-export-handler', () => {
    const mockCtx = {
        userId: 'test-user-123',
        logger: mockLogger,
    };

    beforeEach(() => {
        jest.clearAllMocks();
        process.env.GOOGLE_CLOUD_PROJECT = 'test-project';
        process.env.FUNCTION_REGION = 'us-central1';

        mockCollectionRef.doc.mockReturnValue(mockDocRef);
        mockDocRef.collection.mockReturnValue(mockCollectionRef);
        mockWhere.mockReturnValue(mockCollectionRef);
        mockLimit.mockReturnValue(mockCollectionRef);
        mockOrderBy.mockReturnValue(mockCollectionRef);
    });

    describe('POST /export/full', () => {
        it('should return 401 when unauthenticated', async () => {
            await expect(handler(
                { path: '/export/full', method: 'POST', body: {} } as any,
                { userId: null, logger: mockLogger } as any
            )).rejects.toThrow('Unauthorized');
        });

        it('should enqueue an export job', async () => {
            mockGet.mockResolvedValueOnce({ empty: true });
            mockSet.mockResolvedValueOnce(undefined);

            const result = await handler(
                { path: '/export/full', method: 'POST', body: {} } as any,
                mockCtx as any
            );

            expect(result).toHaveProperty('success', true);
            expect(result).toHaveProperty('jobId');
            expect(result).toHaveProperty('message', 'Export queued for processing');
        });

        it('should return existing job if one is in progress', async () => {
            mockGet.mockResolvedValueOnce({
                empty: false,
                docs: [{
                    data: () => ({
                        jobId: 'existing-job',
                        status: 'RUNNING',
                    }),
                }],
            });

            const result = await handler(
                { path: '/export/full', method: 'POST', body: {} } as any,
                mockCtx as any
            );

            expect(result).toHaveProperty('jobId', 'existing-job');
            expect(result).toHaveProperty('status', 'RUNNING');
        });
    });

    describe('GET /export/status/:jobId', () => {
        it('should return 404 for non-existent job', async () => {
            mockGet.mockResolvedValueOnce({ exists: false });

            await expect(handler(
                { path: '/export/status/fake-job', method: 'GET', body: {} } as any,
                mockCtx as any
            )).rejects.toThrow('Export job not found');
        });

        it('should return job status', async () => {
            mockGet.mockResolvedValueOnce({
                exists: true,
                data: () => ({
                    jobId: 'job-123',
                    userId: 'test-user-123',
                    status: 'COMPLETED',
                    downloadUrl: 'https://example.com/download',
                    createdAt: '2026-02-11T14:00:00Z',
                    completedAt: '2026-02-11T14:05:00Z',
                }),
            });

            const result = await handler(
                { path: '/export/status/job-123', method: 'GET', body: {} } as any,
                mockCtx as any
            );

            expect(result).toHaveProperty('status', 'COMPLETED');
            expect(result).toHaveProperty('downloadUrl', 'https://example.com/download');
        });
    });

    describe('GET /export/run/:runId', () => {
        it('should return 404 for non-existent run', async () => {
            mockGet.mockResolvedValueOnce({ exists: false });

            await expect(handler(
                { path: '/export/run/fake-run', method: 'GET', body: {} } as any,
                mockCtx as any
            )).rejects.toThrow('Pipeline run not found');
        });

        it('should return download URL for run data', async () => {
            mockGet.mockResolvedValueOnce({
                exists: true,
                data: () => ({
                    status: 'COMPLETED',
                    access_token: 'secret-token-value',
                    enriched_event_uri: '',
                    original_payload_uri: '',
                }),
            });

            const result = await handler(
                { path: '/export/run/run-123', method: 'GET', body: {} } as any,
                mockCtx as any
            );

            expect(result).toHaveProperty('downloadUrl');
            expect(result).toHaveProperty('fitFileAvailable', false);
        });
    });

    describe('unknown routes', () => {
        it('should return 404 for unknown paths', async () => {
            await expect(handler(
                { path: '/unknown', method: 'GET', body: {} } as any,
                mockCtx as any
            )).rejects.toThrow('Not found');
        });
    });
});
