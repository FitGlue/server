import { UserStore } from '../../storage/firestore';

/**
 * Error thrown when authorization fails.
 */
export class ForbiddenError extends Error {
  constructor(message: string = 'Access denied') {
    super(message);
    this.name = 'ForbiddenError';
  }
}

/**
 * AuthorizationService provides centralized ownership and admin checks.
 *
 * Key principle: Users can only access their own resources, unless they are admins.
 */
export class AuthorizationService {
  private adminCache: Map<string, boolean> = new Map();

  constructor(private userStore: UserStore) { }

  /**
   * Check if a user is an admin.
   * Results are cached per request lifecycle.
   */
  async isAdmin(userId: string): Promise<boolean> {
    if (this.adminCache.has(userId)) {
      return this.adminCache.get(userId)!;
    }

    const user = await this.userStore.get(userId);
    const isAdmin = user?.isAdmin === true;
    this.adminCache.set(userId, isAdmin);
    return isAdmin;
  }

  /**
   * Check if the authenticated user can access the target user's resources.
   * Returns true if:
   * - authUserId === targetUserId (owner)
   * - authUserId is an admin
   */
  async canAccessUser(authUserId: string, targetUserId: string): Promise<boolean> {
    // Owner check (most common case)
    if (authUserId === targetUserId) {
      return true;
    }

    // Admin check
    return this.isAdmin(authUserId);
  }

  /**
   * Require that the authenticated user can access the target user's resources.
   * Throws ForbiddenError if access is denied.
   */
  async requireAccess(authUserId: string, targetUserId: string): Promise<void> {
    const allowed = await this.canAccessUser(authUserId, targetUserId);
    if (!allowed) {
      throw new ForbiddenError(`User ${authUserId} cannot access resources of user ${targetUserId}`);
    }
  }

  /**
   * Require that the authenticated user is an admin.
   * Throws ForbiddenError if not an admin.
   */
  async requireAdmin(userId: string): Promise<void> {
    const isAdmin = await this.isAdmin(userId);
    if (!isAdmin) {
      throw new ForbiddenError('Admin access required');
    }
  }

  /**
   * Clear the admin cache (useful for testing or long-running processes).
   */
  clearCache(): void {
    this.adminCache.clear();
  }
}
