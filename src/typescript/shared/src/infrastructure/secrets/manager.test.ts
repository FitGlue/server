import { getSecret } from './manager';

describe('getSecret', () => {
    const originalEnv = process.env;

    beforeEach(() => {
        jest.resetModules();
        process.env = { ...originalEnv };
    });

    afterAll(() => {
        process.env = originalEnv;
    });

    it('should return env var if set', () => {
        process.env['MY_SECRET'] = 'test-secret-value';
        const val = getSecret('MY_SECRET');
        expect(val).toBe('test-secret-value');
    });

    it('should throw error if env var not set', () => {
        delete process.env['MY_SECRET'];
        expect(() => getSecret('MY_SECRET')).toThrow('Secret MY_SECRET not found in environment variables');
    });
});
