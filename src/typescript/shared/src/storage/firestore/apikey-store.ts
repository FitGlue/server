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
  async create(hash: string, record: ApiKeyRecord): Promise<void> {
    await this.collection().doc(hash).set(record);
  }

  /**
   * Delete all API keys for a user matching a specific label.
   * Returns the number of keys deleted.
   */
  async deleteByUserAndLabel(userId: string, label: string): Promise<number> {
    const snapshot = await this.collection()
      .where('user_id', '==', userId)
      .where('label', '==', label)
      .get();

    if (snapshot.empty) return 0;

    const batch = this.db.batch();
    snapshot.docs.forEach((doc) => batch.delete(doc.ref));
    await batch.commit();
    return snapshot.size;
  }
}
