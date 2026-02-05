import { registrationSummaryHandler } from './index';

describe('registrationSummaryHandler', () => {
    it('should export the handler function', () => {
        expect(typeof registrationSummaryHandler).toBe('function');
    });
});
