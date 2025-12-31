import { fitbitWebhookHandler } from './index';
import * as crypto from 'crypto';

// Mocks
const mockSecretsGet = jest.fn();
const mockPublishMessage = jest.fn();
const mockLogger = {
  info: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  child: jest.fn().mockReturnThis(),
  debug: jest.fn()
};

const mockCtx = {
  secrets: { get: mockSecretsGet },
  pubsub: { topic: jest.fn().mockReturnValue({ publishMessage: mockPublishMessage }) },
  logger: mockLogger,
  executionId: 'test-id'
};

// Mock Framework
jest.mock('@fitglue/shared', () => ({
  createCloudFunction: (handler: any) => (req: any, res: any) => handler(req, res, mockCtx),
  TOPICS: { RAW_ACTIVITY: 'raw-activity', FITBIT_UPDATES: 'topic-fitbit-updates' }
}));

describe('Fitbit Webhook Handler', () => {
  let req: any;
  let res: any;

  const OLD_ENV = process.env;

  beforeEach(() => {
    jest.clearAllMocks();
    process.env = { ...OLD_ENV }; // Copy env
    req = {
      method: 'GET',
      query: {},
      headers: {},
      body: {}
    };
    res = {
      status: jest.fn().mockReturnThis(),
      send: jest.fn()
    };
  });

  afterAll(() => {
    process.env = OLD_ENV; // Restore old env
  });

  describe('GET (Verification)', () => {
    it('should verify successfully with correct code', async () => {
      req.method = 'GET';
      req.query.verify = 'correct-code';
      process.env.FITBIT_VERIFICATION_CODE = 'correct-code';

      await (fitbitWebhookHandler as any)(req, res);

      expect(res.status).toHaveBeenCalledWith(204);
      expect(res.send).toHaveBeenCalled();
    });

    it('should fail verification with incorrect code', async () => {
      req.method = 'GET';
      req.query.verify = 'wrong-code';
      process.env.FITBIT_VERIFICATION_CODE = 'correct-code';

      await expect(async () => {
        await (fitbitWebhookHandler as any)(req, res);
      }).rejects.toThrow('Invalid verification code');

      expect(res.status).toHaveBeenCalledWith(404);
    });

    it('should fail if verify code is missing', async () => {
      req.method = 'GET';
      req.query = {};
      await (fitbitWebhookHandler as any)(req, res);
      expect(res.status).toHaveBeenCalledWith(404);
    });
  });

  describe('POST (Notifications)', () => {
    const clientSecret = 'my-secret';

    beforeEach(() => {
      req.method = 'POST';
      process.env.FITBIT_CLIENT_SECRET = clientSecret;
    });

    it('should process valid signature notifications', async () => {
      const body: any[] = [{ collectionType: 'activities', ownerId: 'user1', date: '2023-01-01' }];
      const rawBody = Buffer.from(JSON.stringify(body));
      req.body = body;
      (req as any).rawBody = rawBody;

      const hmac = crypto.createHmac('sha1', `${clientSecret}&`);
      hmac.update(rawBody);
      req.headers['x-fitbit-signature'] = hmac.digest('base64');

      await (fitbitWebhookHandler as any)(req, res);

      expect(res.status).toHaveBeenCalledWith(204);
      expect(mockPublishMessage).toHaveBeenCalledWith({ json: body[0] });
    });

    it('should reject invalid signature', async () => {
      const body: any[] = [];
      const rawBody = Buffer.from(JSON.stringify(body));
      req.body = body;
      (req as any).rawBody = rawBody;
      req.headers['x-fitbit-signature'] = 'invalid-sig';

      await expect(async () => {
        await (fitbitWebhookHandler as any)(req, res);
      }).rejects.toThrow('Invalid Signature');

      expect(res.status).toHaveBeenCalledWith(404);
      expect(mockPublishMessage).not.toHaveBeenCalled();
    });

    it('should fail if rawBody is missing', async () => {
      req.headers['x-fitbit-signature'] = 'sig';
      // req.rawBody undefined

      await (fitbitWebhookHandler as any)(req, res);
      expect(mockLogger.error).toHaveBeenCalledWith(expect.stringContaining('Raw body not available'));
      expect(res.status).toHaveBeenCalledWith(500);
    })
  });
});
