// Mocks must be defined before imports
jest.mock('./shared/secrets/secrets', () => ({
  getSecret: jest.fn()
}));

// Mock the shared framework
jest.mock('./shared/framework/index', () => ({
  createCloudFunction: (handler: any) => handler,
  FrameworkContext: jest.fn()
}));

jest.mock('@google-cloud/pubsub', () => {
    const publishMessage = jest.fn().mockResolvedValue('msg-id-123');
    const topic = jest.fn().mockReturnValue({ publishMessage });
    return {
        PubSub: jest.fn().mockImplementation(() => ({ topic }))
    };
});

jest.mock('firebase-admin', () => {
    const collection = jest.fn();
    return {
        initializeApp: jest.fn(),
        firestore: Object.assign(
            jest.fn(() => ({ collection })),
            { collection }
        )
    };
});

import { hevyWebhookHandler } from './index';
import { getSecret } from './shared/secrets/secrets';
import * as crypto from 'crypto';
const admin = require('firebase-admin');

const mockGetSecret = getSecret as jest.MockedFunction<typeof getSecret>;

describe('hevyWebhookHandler', () => {
    let req: any; let res: any;
    let mockStatus: jest.Mock; let mockSend: jest.Mock;
    let mockGet: jest.Mock;

    beforeEach(() => {
        jest.clearAllMocks();
        mockStatus = jest.fn().mockReturnThis();
        mockSend = jest.fn();
        res = { status: mockStatus, send: mockSend };
        req = { headers: {}, body: {} };
        mockGetSecret.mockResolvedValue('test-secret');

        mockGet = jest.fn();
        const mockCollection = admin.firestore().collection;
        mockCollection.mockImplementation((name: string) => {
             if (name === 'users') return { where: jest.fn().mockReturnThis(), limit: jest.fn().mockReturnThis(), get: mockGet };
             // For executions logging
             return { doc: jest.fn().mockReturnValue({ set: jest.fn(), update: jest.fn() }) };
        });
    });

    it('should skip signature check if secret is missing', async () => {
        mockGetSecret.mockRejectedValue(new Error('Secret missing'));
        process.env.HEVY_SIGNING_SECRET = '';

        mockGet.mockResolvedValue({ empty: false, docs: [{ id: 'user-1' }] });
        req.body = { user_id: '1', workout: { title: 'T' } };

        const mockCtx: any = { db: admin.firestore(), logger: { info: jest.fn(), warn: jest.fn(), error: jest.fn() } };
        await (hevyWebhookHandler as any)(req, res, mockCtx);

        expect(mockStatus).toHaveBeenCalledWith(200);
        expect(mockSend).toHaveBeenCalledWith('Processed');
    });

    it('should 401 on invalid signature', async () => {
        mockGetSecret.mockResolvedValue('secret');
        req.headers['x-hevy-signature'] = crypto.randomBytes(32).toString('hex');
        req.body = { a: 1 };
        const mockCtx: any = { db: admin.firestore(), logger: { info: jest.fn(), warn: jest.fn(), error: jest.fn() } };
        await expect(async () => {
            await (hevyWebhookHandler as any)(req, res, mockCtx);
        }).rejects.toThrow('Invalid X-Hevy-Signature');
        expect(mockStatus).toHaveBeenCalledWith(401);
    });

    it('should 200 on valid signature', async () => {
        const secret = 'secret';
        mockGetSecret.mockResolvedValue(secret);
        const payload = { user_id: '1', workout: {} };
        const sig = crypto.createHmac('sha256', secret).update(JSON.stringify(payload)).digest('hex');

        req.headers['x-hevy-signature'] = sig;
        req.body = payload;

        mockGet.mockResolvedValue({ empty: false, docs: [{ id: 'user-1' }] });

        const mockCtx: any = { db: admin.firestore(), logger: { info: jest.fn(), warn: jest.fn(), error: jest.fn() } };
        await (hevyWebhookHandler as any)(req, res, mockCtx);
        expect(mockStatus).toHaveBeenCalledWith(200);
    });
});
