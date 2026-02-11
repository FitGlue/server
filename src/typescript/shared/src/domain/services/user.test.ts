
import { UserService } from './user';
import { UserStore, ActivityStore } from '../../storage/firestore';
import { FirestoreTokenSource } from '../../infrastructure/oauth/token-source';

// Mock Stores
const mockUserStore = {
  get: jest.fn(),
  update: jest.fn(),
  findByFitbitId: jest.fn(),
  collection: jest.fn()
} as unknown as UserStore;

const mockActivityStore = {
  isProcessed: jest.fn(),
  markProcessed: jest.fn()
} as unknown as ActivityStore;

// Mock TokenSource
jest.mock('../../infrastructure/oauth/token-source', () => {
  return {
    FirestoreTokenSource: jest.fn().mockImplementation(() => {
      return {
        getToken: jest.fn()
      };
    })
  };
});

describe('UserService', () => {
  let userService: UserService;

  beforeEach(() => {
    jest.clearAllMocks();
    userService = new UserService(mockUserStore, mockActivityStore, null as any, null as any);
  });

  describe('getValidToken', () => {
    it('should delegate to FirestoreTokenSource', async () => {
      const mockGetToken = jest.fn().mockResolvedValue({ accessToken: 'mock-token' });
      (FirestoreTokenSource as unknown as jest.Mock).mockImplementation(() => ({
        getToken: mockGetToken
      }));

      const token = await userService.getValidToken('u1', 'fitbit');

      expect(FirestoreTokenSource).toHaveBeenCalledWith(mockUserStore, 'u1', 'fitbit');
      expect(mockGetToken).toHaveBeenCalledWith(false); // Default forceRefresh
      expect(token).toBe('mock-token');
    });

    it('should pass forceRefresh to FirestoreTokenSource', async () => {
      const mockGetToken = jest.fn().mockResolvedValue({ accessToken: 'mock-token' });
      (FirestoreTokenSource as unknown as jest.Mock).mockImplementation(() => ({
        getToken: mockGetToken
      }));

      await userService.getValidToken('u1', 'fitbit', true);
      expect(mockGetToken).toHaveBeenCalledWith(true);
    });

    it('should propagate errors from TokenSource', async () => {
      const mockGetToken = jest.fn().mockRejectedValue(new Error('Refresh failed'));
      (FirestoreTokenSource as unknown as jest.Mock).mockImplementation(() => ({
        getToken: mockGetToken
      }));

      await expect(userService.getValidToken('u1', 'fitbit'))
        .rejects.toThrow('Refresh failed');
    });
  });

  describe('Activity Processing', () => {
    it('should check if activity is processed', async () => {
      await userService.hasProcessedActivity('u1', 'fitbit', 'a1');
      expect(mockActivityStore.isProcessed).toHaveBeenCalledWith('u1', 'fitbit_a1');
    });

    it('should mark activity as processed', async () => {
      const now = new Date();

      await userService.markActivityAsProcessed('u1', 'fitbit', 'a1', { processedAt: now, source: 'fitbit', externalId: 'ext-1' });
      expect(mockActivityStore.markProcessed).toHaveBeenCalledWith('u1', 'fitbit_a1', { processedAt: now, source: 'fitbit', externalId: 'ext-1' });
    });
  });
});
