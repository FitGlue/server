import { fitbitWebhookHandler } from './index';
import * as framework from '@fitglue/shared/framework';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: jest.fn().mockImplementation((handler) => handler),
  createWebhookProcessor: jest.fn().mockReturnValue((_req: any, _res: any, _ctx: any) => Promise.resolve()),
  PayloadUserStrategy: jest.fn(),
}));

// Mock connector
jest.mock('./connector', () => ({
  FitbitConnector: jest.fn(),
  FitbitBody: jest.fn(),
}));

// Mock auth
jest.mock('./auth', () => ({
  FitbitVerificationStrategy: jest.fn(),
}));

describe('fitbitWebhookHandler', () => {
  it('should be defined', () => {
    expect(fitbitWebhookHandler).toBeDefined();
  });

  it('should be created via createCloudFunction', () => {
    expect(framework.createCloudFunction).toHaveBeenCalled();
  });
});
