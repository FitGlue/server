// Mocks must be defined before imports
// Mock the shared package
jest.mock('@fitglue/shared', () => ({
    createCloudFunction: (handler: any) => handler,
    FrameworkContext: jest.fn(),
    TOPICS: { RAW_ACTIVITY: 'test-topic' },
    ActivitySource: { SOURCE_HEVY: 'HEVY' }, // Mock enum
    createHevyClient: jest.fn()
}));

import { hevyWebhookHandler } from './index';

import * as path from 'path';
import * as fs from 'fs';

const mockWorkout = JSON.parse(fs.readFileSync(path.join(__dirname, '../test-data/mock_workout.json'), 'utf-8'));

describe('hevyWebhookHandler', () => {
    let req: any; let res: any;
    let mockStatus: jest.Mock; let mockSend: jest.Mock; let mockJson: jest.Mock;
    let mockCtx: any;
    let mockUserGet: jest.Mock;
    let mockDb: any;
    let mockLogger: any;
    let mockPubSub: any;
    let mockPublishMessage: jest.Mock;

    let mockClientGet: jest.Mock;

    beforeEach(() => {
        jest.clearAllMocks();


        mockStatus = jest.fn().mockReturnThis();
        mockSend = jest.fn();
        mockJson = jest.fn();
        res = { status: mockStatus, send: mockSend, json: mockJson, set: jest.fn() };
        req = { headers: {}, body: {} };

        mockLogger = { info: jest.fn(), warn: jest.fn(), error: jest.fn() };

        mockUserGet = jest.fn();
        mockDb = {
            collection: jest.fn((name) => {
                if (name === 'users') {
                    return { doc: jest.fn(() => ({ get: mockUserGet })) };
                }
                return { doc: jest.fn() };
            })
        };

        mockPublishMessage = jest.fn().mockResolvedValue('msg-id-123');
        mockPubSub = {
            topic: jest.fn().mockReturnValue({ publishMessage: mockPublishMessage })
        };

        mockCtx = {
            db: mockDb,
            logger: mockLogger,
            pubsub: mockPubSub, // Injected PubSub Mock
            userId: 'test-user',
            authScopes: ['read:activity']
        };

        mockClientGet = jest.fn().mockResolvedValue({
            data: mockWorkout,
            error: null,
            response: { status: 200 }
        });

        const { createHevyClient } = require('@fitglue/shared');
        createHevyClient.mockReturnValue({
            GET: mockClientGet
        });
    });

    it('should throw Unauthorized if userId is missing', async () => {
        mockCtx.userId = undefined;
        await expect(async () => {
            await (hevyWebhookHandler as any)(req, res, mockCtx);
        }).rejects.toThrow('Unauthorized');
        expect(mockStatus).toHaveBeenCalledWith(401);
    });

    it('should throw if workout_id is missing', async () => {
        req.body = {};
        await expect(async () => {
            await (hevyWebhookHandler as any)(req, res, mockCtx);
        }).rejects.toThrow('Invalid payload: Missing workout_id');
    });

    it('should perform Active Fetch and Publish', async () => {
        req.body = {
            id: 'webhook-event-id',
            payload: {
                workoutId: mockWorkout.id
            }
        };

        // Mock User with Hevy Key
        mockUserGet.mockResolvedValue({
            exists: true,
            data: () => ({ integrations: { hevy: { apiKey: 'hevy-key' } } })
        });

        await (hevyWebhookHandler as any)(req, res, mockCtx);

        // Verify Hevy Client was mocked and called
        const { createHevyClient } = require('@fitglue/shared');
        expect(createHevyClient).toHaveBeenCalledWith({ apiKey: 'hevy-key' });
        expect(mockClientGet).toHaveBeenCalledWith("/v1/workouts/{workoutId}", {
            params: { path: { workoutId: mockWorkout.id } }
        });

        // Verify PubSub Injection Usage
        expect(mockPubSub.topic).toHaveBeenCalledWith('test-topic');
        expect(mockPublishMessage).toHaveBeenCalledWith(
            expect.objectContaining({
                json: expect.objectContaining({
                    source: 'HEVY',
                    userId: 'test-user',
                    originalPayloadJson: JSON.stringify(mockWorkout),
                    metadata: expect.objectContaining({ fetch_method: 'active_fetch' }),
                    standardizedActivity: expect.objectContaining({
                        source: 'HEVY',
                        externalId: mockWorkout.id
                    })
                })
            })
        );
        expect(mockStatus).toHaveBeenCalledWith(200);
        expect(mockJson).toHaveBeenCalledWith(expect.objectContaining({ status: 'Processed' }));
        expect(mockJson).toHaveBeenCalledWith(expect.objectContaining({ status: 'Processed' }));
    });


});
