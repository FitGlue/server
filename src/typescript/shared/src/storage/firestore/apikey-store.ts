import * as admin from 'firebase-admin';
import * as converters from './converters';
import { ApiKeyRecord } from '../../types/pb/auth';

/**
 * ApiKeyStore provides typed access to ingress API key operations.
 */
export class ApiKeyStore {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get the API keys collection reference.
   */
  private collection() {
    return this.db.collection('ingress_api_keys').withConverter(converters.apiKeyConverter);
  }

  /**
   * Get an API key by ID.
   */
  async getByHash(keyId: string): Promise<ApiKeyRecord | null> {
    const doc = await this.collection().doc(keyId).get();
    if (!doc.exists) {
      return null;
    }
    return doc.data() || null;
  }

  /**
   * Create an API key.
   */
  async create(record: ApiKeyRecord & { id?: string; hash?: string; enabled?: boolean }): Promise<void> {
    const { id, ...data } = record;
    if (id) {
      await this.collection().doc(id).set(data as any);
    } else {
      await this.collection().add(data as any);
    }
  }
}
