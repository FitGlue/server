// Basic wiring test
import { hevyHandler } from './index';
import * as framework from '@fitglue/shared/framework';

// Mock shared/framework
jest.mock('@fitglue/shared/framework', () => ({
    createCloudFunction: (handler: any) => handler,
    createWebhookProcessor: jest.fn(() => async () => ({ status: 'Mocked Processor Run' })),
    ApiKeyStrategy: jest.fn()
}));

// Mock connector
jest.mock('./connector', () => ({
    HevyConnector: jest.fn()
}));

describe('hevyHandler', () => {
    it('should utilize createWebhookProcessor with HevyConnector', () => {
        // Check if createWebhookProcessor was called
        expect(framework.createWebhookProcessor).toHaveBeenCalled();
    });

    it('should act as a handler', async () => {
        const res = await (hevyHandler as any)({}, {}, {});
        expect(res).toEqual({ status: 'Mocked Processor Run' });
    });
});
