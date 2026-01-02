import { createWebhookProcessor } from './webhook-processor';
import { Connector, ConnectorConfig } from './connector';
import { CloudEventSource } from '../types/pb/events';
import { ActivitySource } from '../types/pb/activity';

// Mocks
const mockExtractId = jest.fn();
const mockFetchAndMap = jest.fn();
const mockValidateConfig = jest.fn();

const mockConnector: Connector<ConnectorConfig> = {
  name: 'test-connector',
  strategy: 'webhook',
  cloudEventSource: CloudEventSource.CLOUD_EVENT_SOURCE_HEVY,
  activitySource: ActivitySource.SOURCE_HEVY,
  validateConfig: mockValidateConfig,
  mapActivity: jest.fn(),
  extractId: mockExtractId,
  fetchAndMap: mockFetchAndMap,
  healthCheck: jest.fn().mockResolvedValue(true),
  verifyRequest: jest.fn().mockResolvedValue(undefined)
};

const mockHasProcessedActivity = jest.fn();
const mockMarkActivityAsProcessed = jest.fn();
jest.mock('../domain/services/user', () => ({
  UserService: jest.fn().mockImplementation(() => ({
    hasProcessedActivity: mockHasProcessedActivity,
    markActivityAsProcessed: mockMarkActivityAsProcessed
  }))
}));

const mockGet = jest.fn();
const mockDoc = jest.fn().mockReturnValue({ get: mockGet });
const mockGetUsersCollection = jest.fn().mockReturnValue({ doc: mockDoc });

jest.mock('../index', () => ({
  storage: {
    getUsersCollection: mockGetUsersCollection
  },
  // Mock types that might be imported
  FrameworkContext: jest.fn()
}));

const mockPublish = jest.fn().mockResolvedValue('msg-id-123');
jest.mock('../infrastructure/pubsub/cloud-event-publisher', () => ({
  CloudEventPublisher: jest.fn().mockImplementation(() => ({
    publish: mockPublish
  }))
}));

jest.mock('../types/events-helper', () => ({
  getCloudEventSource: jest.fn(),
  getCloudEventType: jest.fn()
}));

describe('createWebhookProcessor', () => {
  let handler: any;
  let req: any;
  let res: any;
  let ctx: any;

  beforeEach(() => {
    jest.clearAllMocks();
    handler = createWebhookProcessor(mockConnector);

    req = { body: { id: 'evt-123' } };
    res = {
      status: jest.fn().mockReturnThis(),
      send: jest.fn(),
      json: jest.fn()
    };
    ctx = {
      db: {},
      logger: { info: jest.fn(), error: jest.fn(), warn: jest.fn() },
      userId: 'user-1',
      pubsub: {}
    };

    mockExtractId.mockReturnValue('evt-123');
    mockGet.mockResolvedValue({
      exists: true,
      data: () => ({ integrations: { 'test-connector': { enabled: true } } })
    });
    mockHasProcessedActivity.mockResolvedValue(false);
    mockFetchAndMap.mockResolvedValue([{
      source: 'TEST',
      externalId: 'evt-123',
      userId: 'user-1'
    }]);
  });

  it('should process a valid webhook successfully', async () => {
    await handler(req, res, ctx);

    expect(mockExtractId).toHaveBeenCalledWith(req.body);
    expect(mockHasProcessedActivity).toHaveBeenCalledWith('user-1', 'test-connector', 'evt-123');
    expect(mockFetchAndMap).toHaveBeenCalledWith('evt-123', expect.objectContaining({ userId: 'user-1', enabled: true }));
    expect(mockPublish).toHaveBeenCalled();
    expect(mockMarkActivityAsProcessed).toHaveBeenCalledWith('user-1', 'test-connector', 'evt-123');
    expect(res.status).toHaveBeenCalledWith(200);
    expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ status: 'Processed' }));
  });

  it('should throw Unauthorized if userId is missing', async () => {
    ctx.userId = undefined;
    await expect(handler(req, res, ctx)).rejects.toThrow('Unauthorized');
    expect(res.status).toHaveBeenCalledWith(401);
  });

  it('should skip if deduplication finds existing', async () => {
    mockHasProcessedActivity.mockResolvedValue(true);
    const result = await handler(req, res, ctx);

    expect(result.status).toBe('Skipped');
    expect(mockFetchAndMap).not.toHaveBeenCalled();
    expect(mockPublish).not.toHaveBeenCalled();
  });

  it('should error if config is disabled', async () => {
    mockGet.mockResolvedValue({
      exists: true,
      data: () => ({ integrations: { 'test-connector': { enabled: false } } })
    });

    await handler(req, res, ctx);

    expect(res.status).toHaveBeenCalledWith(200);
    expect(res.send).toHaveBeenCalledWith(expect.stringContaining('Integration disabled'));
    // Note: my implementation returns 200 with message.
    // Checking specific message "Integration disabled or unconfigured"
  });

  it('should error if validateConfig fails', async () => {
    mockValidateConfig.mockImplementation(() => { throw new Error('Missing API Key'); });

    const result = await handler(req, res, ctx);

    expect(mockValidateConfig).toHaveBeenCalled();
    expect(res.status).toHaveBeenCalledWith(200);
    expect(res.send).toHaveBeenCalledWith(expect.stringContaining('Configuration Error'));
    expect(result.status).toBe('Failed');
  });
});
