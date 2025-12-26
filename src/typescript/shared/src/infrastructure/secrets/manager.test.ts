import { getSecret } from './manager';

// Mock dependencies if needed, but getSecret logic is simple fallback
// We want to test env var fallback and mocked GSM call

jest.mock('@google-cloud/secret-manager', () => {
    return {
        SecretManagerServiceClient: jest.fn().mockImplementation(() => ({
            accessSecretVersion: jest.fn().mockResolvedValue([{
                payload: {
                    data: Buffer.from('gsm-secret')
                }
            }])
        }))
    };
});

describe('getSecret', () => {
    const originalEnv = process.env;

    beforeEach(() => {
        jest.resetModules();
        process.env = { ...originalEnv };
    });

    afterAll(() => {
        process.env = originalEnv;
    });

    it('should return env var if set', async () => {
        process.env['MY_SECRET'] = 'local-value';
        const val = await getSecret('proj', 'MY_SECRET');
        expect(val).toBe('local-value');
    });

    it('should return GSM value if env var not set', async () => {
        // GSM mock returns 'gsm-secret'
        const val = await getSecret('proj', 'MY_SECRET');
        expect(val).toBe('gsm-secret');
    });
});
