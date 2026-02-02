// Basic wiring test
import { mockSourceHandler } from './index';
import * as framework from '@fitglue/shared/framework';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: (handler: any) => handler,
  createWebhookProcessor: jest.fn(() => async () => ({ status: 'Mocked Processor Run' })),
  ApiKeyStrategy: jest.fn()
}));

// Mock connector
jest.mock('./connector', () => ({
  MockConnector: jest.fn()
}));

describe('mockSourceHandler', () => {
  it('should utilize createWebhookProcessor with MockConnector', () => {
    // Check if createWebhookProcessor was called
    expect(framework.createWebhookProcessor).toHaveBeenCalled();
  });

  it('should act as a handler', async () => {
    const res = await (mockSourceHandler as any)({}, {}, {});
    expect(res).toEqual({ status: 'Mocked Processor Run' });
  });
});
