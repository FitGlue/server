import { AuthorizationService, ForbiddenError } from './authorization';
import { UserStore } from '../../storage/firestore';

describe('AuthorizationService', () => {
  let authService: AuthorizationService;
  let mockUserStore: jest.Mocked<UserStore>;

  beforeEach(() => {
    mockUserStore = {
      get: jest.fn(),
    } as unknown as jest.Mocked<UserStore>;
    authService = new AuthorizationService(mockUserStore);
  });

  describe('isAdmin', () => {
    it('returns true for admin users', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: true } as any);
      expect(await authService.isAdmin('user-1')).toBe(true);
    });

    it('returns false for non-admin users', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: false } as any);
      expect(await authService.isAdmin('user-1')).toBe(false);
    });

    it('returns false if user not found', async () => {
      mockUserStore.get.mockResolvedValue(null);
      expect(await authService.isAdmin('user-1')).toBe(false);
    });

    it('caches admin status', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: true } as any);
      await authService.isAdmin('user-1');
      await authService.isAdmin('user-1');
      expect(mockUserStore.get).toHaveBeenCalledTimes(1);
    });
  });

  describe('canAccessUser', () => {
    it('returns true when accessing own resources', async () => {
      expect(await authService.canAccessUser('user-1', 'user-1')).toBe(true);
      expect(mockUserStore.get).not.toHaveBeenCalled();
    });

    it('returns true when admin accesses other user', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: true } as any);
      expect(await authService.canAccessUser('admin-1', 'user-1')).toBe(true);
    });

    it('returns false when non-admin accesses other user', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: false } as any);
      expect(await authService.canAccessUser('user-1', 'user-2')).toBe(false);
    });
  });

  describe('requireAccess', () => {
    it('does not throw when accessing own resources', async () => {
      await expect(authService.requireAccess('user-1', 'user-1')).resolves.not.toThrow();
    });

    it('does not throw when admin accesses other user', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: true } as any);
      await expect(authService.requireAccess('admin-1', 'user-1')).resolves.not.toThrow();
    });

    it('throws ForbiddenError when non-admin accesses other user', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: false } as any);
      await expect(authService.requireAccess('user-1', 'user-2')).rejects.toThrow(ForbiddenError);
    });
  });

  describe('requireAdmin', () => {
    it('does not throw for admin users', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: true } as any);
      await expect(authService.requireAdmin('admin-1')).resolves.not.toThrow();
    });

    it('throws ForbiddenError for non-admin users', async () => {
      mockUserStore.get.mockResolvedValue({ isAdmin: false } as any);
      await expect(authService.requireAdmin('user-1')).rejects.toThrow(ForbiddenError);
    });
  });
});
