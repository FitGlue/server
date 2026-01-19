// Basic wiring test
import { hevyHandler } from './index';
import * as shared from '@fitglue/shared';

// Mock shared
jest.mock('@fitglue/shared', () => ({
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
        expect(shared.createWebhookProcessor).toHaveBeenCalled();
    });

    it('should act as a handler', async () => {
        const res = await (hevyHandler as any)({}, {}, {});
        expect(res).toEqual({ status: 'Mocked Processor Run' });
    });
});
