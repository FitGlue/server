// Basic wiring test
import { hevyWebhookHandler } from './index';
import * as shared from '@fitglue/shared';

// Mock shared
jest.mock('@fitglue/shared', () => ({
    createCloudFunction: (handler: any) => handler,
    HevyConnector: jest.fn(),
    createWebhookProcessor: jest.fn(() => async () => ({ status: 'Mocked Processor Run' })),
    ApiKeyStrategy: jest.fn()
}));

describe('hevyWebhookHandler', () => {
    it('should utilize createWebhookProcessor with HevyConnector', () => {
        // Check if createWebhookProcessor was called
        expect(shared.createWebhookProcessor).toHaveBeenCalled();
    });

    it('should act as a handler', async () => {
        const res = await (hevyWebhookHandler as any)({}, {}, {});
        expect(res).toEqual({ status: 'Mocked Processor Run' });
    });
});
