import { ApiKeyStore } from '../../storage/firestore';
import { ApiKeyRecord } from '../../types/pb/auth';

/**
 * ApiKeyService provides business logic for API key operations.
 */
export class ApiKeyService {
  constructor(private apiKeyStore: ApiKeyStore) { }

  /**
   * Find an API key by its hash.
   */
  async findByHash(hash: string): Promise<ApiKeyRecord | null> {
    return this.apiKeyStore.getByHash(hash);
  }

  /**
   * Validate an API key (checks if exists and enabled).
   */
  async validate(hash: string): Promise<{ valid: boolean; userId?: string; scopes?: string[] }> {
    const apiKey = await this.findByHash(hash);

    if (!apiKey) {
      return { valid: false };
    }

    return {
      valid: true,
      userId: apiKey.userId,
      scopes: apiKey.scopes || []
    };
  }

  /**
   * Create a new API key.
   */
  async create(record: ApiKeyRecord): Promise<void> {
    return this.apiKeyStore.create(record);
  }
}
