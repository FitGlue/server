import * as admin from 'firebase-admin';
import * as converters from './converters';
import { UserRecord } from '../../types/pb/user';

/**
 * UserStore provides typed access to user-related Firestore operations.
 */
export class UserStore {
  constructor(private db: admin.firestore.Firestore) { }

  /**
   * Get the users collection reference.
   */
  private collection() {
    return this.db.collection('users').withConverter(converters.userConverter);
  }

  /**
   * Find a user by a specific field value.
   */
  async findByField(field: string, value: any): Promise<UserRecord | null> {
    const snapshot = await this.collection()
      .where(field, '==', value)
      .limit(1)
      .get();

    if (snapshot.empty) {
      return null;
    }

    return snapshot.docs[0].data();
  }

  /**
   * Find a user by their Fitbit ID.
   */
  async findByFitbitId(fitbitUserId: string): Promise<{ id: string; data: UserRecord } | null> {
    const snapshot = await this.collection()
      .where('integrations.fitbit.fitbit_user_id', '==', fitbitUserId)
      .limit(1)
      .get();

    if (snapshot.empty) {
      return null;
    }

    const doc = snapshot.docs[0];
    return { id: doc.id, data: doc.data() };
  }

  /**
   * Get a user by ID.
   */
  async get(userId: string): Promise<UserRecord | null> {
    const doc = await this.collection().doc(userId).get();
    return doc.exists ? doc.data() || null : null;
  }

  /**
   * Update a user document.
   */
  async update(userId: string, data: any): Promise<void> {
    await this.collection().doc(userId).update(data);
  }
}
