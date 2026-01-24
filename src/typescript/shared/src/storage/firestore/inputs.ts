import { Firestore } from 'firebase-admin/firestore';
import { PendingInput } from '../../types/pb/pending_input';
import { FirestoreToPendingInput } from './converters';

export class InputStore {
  constructor(private db: Firestore) { }

  async getPending(activityId: string): Promise<PendingInput | null> {
    const doc = await this.db.collection('pending_inputs').doc(activityId).get();
    if (!doc.exists) return null;
    return FirestoreToPendingInput(doc.data() as Record<string, unknown>);
  }

  /**
   * List pending inputs for a user.
   * Filters out auto-populated inputs that are still within their grace period
   * (auto_deadline hasn't passed yet), giving the automated system time to resolve them.
   */
  async listPending(userId: string): Promise<PendingInput[]> {
    const snapshot = await this.db.collection('pending_inputs')
      .where('user_id', '==', userId)
      .where('status', '==', 1) // STATUS_WAITING
      .orderBy('created_at', 'desc')
      .get();

    const now = new Date();
    return snapshot.docs
      .map(doc => FirestoreToPendingInput(doc.data() as Record<string, unknown>))
      .filter(input => {
        // If not auto-populated, always show to user
        if (!input.autoPopulated) {
          return true;
        }
        // If auto-populated but no deadline, show to user (shouldn't happen, but be safe)
        if (!input.autoDeadline) {
          return true;
        }
        // Only show auto-populated inputs if the deadline has passed
        // (automated system has had its chance to resolve)
        return new Date(input.autoDeadline) <= now;
      });
  }

  async resolve(activityId: string, inputData: Record<string, string>): Promise<void> {
    await this.db.collection('pending_inputs').doc(activityId).update({
      status: 2, // STATUS_COMPLETED
      input_data: inputData,
      updated_at: new Date()
    });
  }

  async delete(activityId: string): Promise<void> {
    await this.db.collection('pending_inputs').doc(activityId).delete();
  }
}
