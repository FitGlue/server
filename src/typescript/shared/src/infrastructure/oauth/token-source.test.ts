import { FirestoreTokenSource } from './token-source';
import { UserStore } from '../../storage/firestore';

// Mock UserStore
const mockUserStore = {
  get: jest.fn(),
  setIntegration: jest.fn()
} as unknown as UserStore;

// Mock fetch
global.fetch = jest.fn() as unknown as jest.Mock;

describe('FirestoreTokenSource', () => {
  const userId = 'test-user';
  const provider = 'fitbit';
  const now = new Date('2026-01-01T12:00:00Z');

  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers().setSystemTime(now);
    // Set env vars for OAuth credentials
    process.env.FITBIT_CLIENT_ID = 'test-client-id';
    process.env.FITBIT_CLIENT_SECRET = 'test-client-secret';
    process.env.STRAVA_CLIENT_ID = 'test-client-id';
    process.env.STRAVA_CLIENT_SECRET = 'test-client-secret';
  });

  afterEach(() => {
    jest.useRealTimers();
    delete process.env.FITBIT_CLIENT_ID;
    delete process.env.FITBIT_CLIENT_SECRET;
    delete process.env.STRAVA_CLIENT_ID;
    delete process.env.STRAVA_CLIENT_SECRET;
  });

  it('should return valid existing token', async () => {
    (mockUserStore.get as jest.Mock).mockResolvedValue({
      userId,
      integrations: {
        fitbit: {
          enabled: true,
          accessToken: 'valid-token',
          refreshToken: 'valid-refresh',
          expiresAt: new Date(now.getTime() + 3600 * 1000) // 1 hour future
        }
      }
    });

    const source = new FirestoreTokenSource(mockUserStore, userId, provider);
    const token = await source.getToken();

    expect(token.accessToken).toBe('valid-token');
    expect(mockUserStore.setIntegration).not.toHaveBeenCalled();
  });

  it('should refresh when expired', async () => {
    // Setup UserStore with expired token
    (mockUserStore.get as jest.Mock).mockResolvedValue({
      userId,
      integrations: {
        fitbit: {
          enabled: true,
          accessToken: 'old-token',
          refreshToken: 'old-refresh',
          expiresAt: new Date(now.getTime() - 60000) // 1 min past
        }
      }
    });

    // Mock Refresh Response
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: 'new-token',
        refresh_token: 'new-refresh',
        expires_in: 3600
      })
    });

    const source = new FirestoreTokenSource(mockUserStore, userId, provider);
    const token = await source.getToken();

    expect(token.accessToken).toBe('new-token');
    expect(global.fetch).toHaveBeenCalledWith(
      'https://api.fitbit.com/oauth2/token',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          'Authorization': expect.stringContaining('Basic')
        })
      })
    );
    expect(mockUserStore.setIntegration).toHaveBeenCalledWith(
      userId,
      provider,
      expect.objectContaining({
        accessToken: 'new-token',
        refreshToken: 'new-refresh'
      })
    );
  });

  it('should refresh when PROACTIVELY expiring soon (within 1 min)', async () => {
    // Setup UserStore with token expiring in 30s
    (mockUserStore.get as jest.Mock).mockResolvedValue({
      userId,
      integrations: {
        fitbit: {
          enabled: true,
          accessToken: 'soon-expiring-token',
          refreshToken: 'old-refresh',
          expiresAt: new Date(now.getTime() + 30 * 1000)
        }
      }
    });

    // Mock Refresh Response
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: 'new-valid-token',
        refresh_token: 'new-valid-refresh',
        expires_in: 3600
      })
    });

    const source = new FirestoreTokenSource(mockUserStore, userId, provider);
    const token = await source.getToken();

    expect(token.accessToken).toBe('new-valid-token');
    expect(global.fetch).toHaveBeenCalled(); // Should have refreshed
  });

  it('should handle Strava refresh (Body params)', async () => {
    const stravaProvider = 'strava';
    // Setup UserStore with expired token
    (mockUserStore.get as jest.Mock).mockResolvedValue({
      userId,
      integrations: {
        strava: {
          enabled: true,
          accessToken: 'old-strava-token',
          refreshToken: 'old-strava-refresh',
          expiresAt: new Date(now.getTime() - 1000)
        }
      }
    });

    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: 'new-strava-token',
        refresh_token: 'new-strava-refresh',
        expires_in: 3600
      })
    });

    const source = new FirestoreTokenSource(mockUserStore, userId, stravaProvider);
    const token = await source.getToken();

    expect(token.accessToken).toBe('new-strava-token');
    expect(global.fetch).toHaveBeenCalledWith(
      'https://www.strava.com/oauth/token',
      expect.objectContaining({
        body: expect.any(URLSearchParams)
      })
    );
    // Verify body contains client_id for Strava
    const call = (global.fetch as jest.Mock).mock.calls[0];
    const body = call[1].body as URLSearchParams;
    expect(body.get('client_id')).toBe('test-client-id');
  });

  it('should return GitHub token without requiring refreshToken', async () => {
    (mockUserStore.get as jest.Mock).mockResolvedValue({
      userId,
      integrations: {
        github: {
          enabled: true,
          accessToken: 'ghp_test_token',
          refreshToken: '', // GitHub doesn't use refresh tokens
          expiresAt: new Date('2099-12-31'),
        }
      }
    });

    const source = new FirestoreTokenSource(mockUserStore, userId, 'github');
    const token = await source.getToken();

    expect(token.accessToken).toBe('ghp_test_token');
    expect(token.refreshToken).toBe('');
    expect(global.fetch).not.toHaveBeenCalled();
    expect(mockUserStore.setIntegration).not.toHaveBeenCalled();
  });

  it('should not attempt refresh for GitHub even with past expiry', async () => {
    (mockUserStore.get as jest.Mock).mockResolvedValue({
      userId,
      integrations: {
        github: {
          enabled: true,
          accessToken: 'ghp_test_token',
          refreshToken: '',
          expiresAt: new Date(now.getTime() - 60000), // 1 min in the past
        }
      }
    });

    const source = new FirestoreTokenSource(mockUserStore, userId, 'github');
    const token = await source.getToken();

    expect(token.accessToken).toBe('ghp_test_token');
    expect(global.fetch).not.toHaveBeenCalled(); // No refresh attempt
  });
});
