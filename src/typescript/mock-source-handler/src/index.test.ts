// Basic wiring test
import { mockSourceHandler } from './index';
import * as shared from '@fitglue/shared';

// Mock shared
jest.mock('@fitglue/shared', () => ({
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
    expect(shared.createWebhookProcessor).toHaveBeenCalled();
  });

  it('should act as a handler', async () => {
    const res = await (mockSourceHandler as any)({}, {}, {});
    expect(res).toEqual({ status: 'Mocked Processor Run' });
  });
});
