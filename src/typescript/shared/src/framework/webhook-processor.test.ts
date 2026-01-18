import { createWebhookProcessor } from './webhook-processor';
import { Connector, ConnectorConfig } from './connector';
import { CloudEventSource } from '../types/pb/events';
import { ActivitySource } from '../types/pb/activity';

// Mocks
const mockExtractId = jest.fn();
const mockFetchAndMap = jest.fn();
const mockValidateConfig = jest.fn();


class MockConnectorClass implements Connector<ConnectorConfig> {
  name = 'test-connector';
  strategy = 'webhook' as const;
  cloudEventSource = CloudEventSource.CLOUD_EVENT_SOURCE_HEVY;
  activitySource = ActivitySource.SOURCE_HEVY;

  constructor(public context: any) { }

  validateConfig = mockValidateConfig;
  mapActivity = jest.fn();
  extractId = mockExtractId;
  fetchAndMap = mockFetchAndMap;
  healthCheck = jest.fn().mockResolvedValue(true);
  verifyRequest = jest.fn().mockResolvedValue(undefined);
  resolveUser = jest.fn();
}

const mockHasProcessedActivity = jest.fn();
const mockCheckDestinationExists = jest.fn();
const mockMarkActivityAsProcessed = jest.fn();
const mockGet = jest.fn();

const mockUserService = {
  hasProcessedActivity: mockHasProcessedActivity,
  checkDestinationExists: mockCheckDestinationExists,
  markActivityAsProcessed: mockMarkActivityAsProcessed,
  get: mockGet
};

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
    handler = createWebhookProcessor(MockConnectorClass);

    req = { body: { id: 'evt-123' } };
    res = {
      status: jest.fn().mockReturnThis(),
      send: jest.fn(),
      json: jest.fn()
    };

    // Proper context mocking with services
    ctx = {
      services: {
        user: mockUserService,
        execution: {
          create: jest.fn().mockResolvedValue(true)
        }
      },
      logger: { info: jest.fn(), error: jest.fn(), warn: jest.fn() },
      userId: 'user-1',
      pubsub: {}
    };

    mockExtractId.mockReturnValue('evt-123');
    mockGet.mockResolvedValue({
      integrations: { 'test-connector': { enabled: true } },
      pipelines: [{ id: 'pipe-1', source: 'SOURCE_HEVY', enrichers: [], destinations: [1] }]
    });
    mockHasProcessedActivity.mockResolvedValue(false);
    mockCheckDestinationExists.mockResolvedValue(false);
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
    expect(mockCheckDestinationExists).toHaveBeenCalledWith('user-1', 'test-connector', 'evt-123');
    expect(mockFetchAndMap).toHaveBeenCalledWith('evt-123', expect.objectContaining({ enabled: true }));
    expect(mockPublish).toHaveBeenCalled();
    expect(mockMarkActivityAsProcessed).toHaveBeenCalledWith('user-1', 'test-connector', 'evt-123', expect.anything());
    expect(res.status).toHaveBeenCalledWith(200);
    expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ status: 'Success' }));
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

  it('should skip if loop detected', async () => {
    mockCheckDestinationExists.mockResolvedValue(true);
    const result = await handler(req, res, ctx);

    expect(result.status).toBe('Skipped');
    expect(res.send).toHaveBeenCalledWith(expect.stringContaining('Loop prevention'));
    expect(mockHasProcessedActivity).not.toHaveBeenCalled(); // Should skip dedup cheek entirely? Actually it returns early.
    expect(mockFetchAndMap).not.toHaveBeenCalled();
    expect(mockPublish).not.toHaveBeenCalled();

  });

  it('should error if config is disabled', async () => {
    mockGet.mockResolvedValue({
      integrations: { 'test-connector': { enabled: false } },
      pipelines: [{ id: 'pipe-1', source: 'SOURCE_HEVY', enrichers: [], destinations: [1] }]
    });

    await handler(req, res, ctx);

    expect(res.status).toHaveBeenCalledWith(200);
    expect(res.send).toHaveBeenCalledWith(expect.stringContaining('Integration disabled'));
  });

  it('should skip if no pipeline configured for source', async () => {
    mockGet.mockResolvedValue({
      integrations: { 'test-connector': { enabled: true } },
      pipelines: [{ id: 'pipe-1', source: 'SOURCE_FITBIT', enrichers: [], destinations: [1] }] // Different source!
    });

    const result = await handler(req, res, ctx);

    expect(result.status).toBe('Skipped');
    expect(res.send).toHaveBeenCalledWith(expect.stringContaining('No pipeline configured'));
    expect(mockFetchAndMap).not.toHaveBeenCalled();
    expect(mockPublish).not.toHaveBeenCalled();
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
