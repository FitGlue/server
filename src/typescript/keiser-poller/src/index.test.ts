
// Mocks
jest.mock('@fitglue/shared', () => ({
    getSecret: jest.fn(),
    createCloudFunction: (handler: any) => handler,
    FrameworkContext: jest.fn(),
    TOPICS: { RAW_ACTIVITY: 'test-topic' },
    ActivitySource: { SOURCE_KEISER: 'KEISER' }
}));

jest.mock('@google-cloud/pubsub', () => {
    const publishMessage = jest.fn().mockResolvedValue('msg-id-123');
    const topic = jest.fn().mockReturnValue({ publishMessage });
    return {
        PubSub: jest.fn().mockImplementation(() => ({ topic }))
    };
});

jest.mock('firebase-admin', () => {
    return {
        initializeApp: jest.fn(),
        firestore: jest.fn() // We'll mock the instance in beforeEach
    };
});

import { keiserPoller } from './index';



describe('Keiser Poller', () => {
    let req: any; let res: any;
    let mockStatus: jest.Mock; let mockSend: jest.Mock; let mockJson: jest.Mock;
    let mockDb: any;
    let mockLogger: any;

    beforeEach(() => {
        jest.clearAllMocks();
        mockStatus = jest.fn().mockReturnThis();
        mockSend = jest.fn();
        mockJson = jest.fn();
        res = { status: mockStatus, send: mockSend, json: mockJson };
        req = {};

        // Mock Firestore Chain
        // db.collection('users').limit(50).get()
        const mockGetUsers = jest.fn();
        mockDb = {
            collection: jest.fn().mockReturnValue({
                limit: jest.fn().mockReturnValue({
                    get: mockGetUsers
                }),
                doc: jest.fn()
            })
        };
        mockLogger = { info: jest.fn(), error: jest.fn(), warn: jest.fn() };

        // Attach to context
        // We call the function as (req, res, ctx) because we mocked createCloudFunction to return handler strictly
    });

    it('should return NO_USERS if no users found', async () => {
        // Mock empty users
        const mockGetUsers = mockDb.collection().limit().get;
        mockGetUsers.mockResolvedValue({ empty: true, size: 0, docs: [] });

        const result = await (keiserPoller as any)(req, res, { db: mockDb, logger: mockLogger });

        expect(mockStatus).toHaveBeenCalledWith(200);
        expect(mockJson).toHaveBeenCalledWith({ status: 'NO_USERS' });
        expect(result).toEqual({ status: 'NO_USERS' });
    });

    it('should skip user if keiser disabled', async () => {
        const mockGetUsers = mockDb.collection().limit().get;
        mockGetUsers.mockResolvedValue({
            empty: false,
            size: 1,
            docs: [{
                id: 'user-1',
                data: () => ({ integrations: { keiser: { enabled: false } } })
            }]
        });

        await (keiserPoller as any)(req, res, { db: mockDb, logger: mockLogger });
        expect(mockStatus).toHaveBeenCalledWith(200);
        // Expect 0 sessions
    });
});
