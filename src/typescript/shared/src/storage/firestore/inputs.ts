import { Firestore } from 'firebase-admin/firestore';
import { PendingInput } from '../../types/pb/pending_input';
import { FirestoreToPendingInput } from './converters';

export class InputStore {
  constructor(private db: Firestore) { }

  /**
   * Get the pending_inputs sub-collection for a specific user.
   */
  private userPendingInputs(userId: string) {
    return this.db.collection('users').doc(userId).collection('pending_inputs');
  }

  async getPending(userId: string, activityId: string): Promise<PendingInput | null> {
    const doc = await this.userPendingInputs(userId).doc(activityId).get();
    if (!doc.exists) return null;
    return FirestoreToPendingInput(doc.data() as Record<string, unknown>);
  }

  /**
   * List pending inputs for a user.
   * Returns ALL pending inputs - the UI is responsible for styling based on state:
   * - autoPopulated: false → "User Input Required" (editable, action required)
   * - autoPopulated: true, deadline NOT passed → "Waiting for Results" (read-only, styled as pending)
   * - autoPopulated: true, deadline passed → "Action Required" (editable, action required)
   */
  async listPending(userId: string): Promise<PendingInput[]> {
    const snapshot = await this.userPendingInputs(userId)
      .where('status', '==', 1) // STATUS_WAITING
      .orderBy('created_at', 'desc')
      .get();

    return snapshot.docs
      .map(doc => FirestoreToPendingInput(doc.data() as Record<string, unknown>));
  }

  async resolve(userId: string, activityId: string, inputData: Record<string, string>): Promise<void> {
    await this.userPendingInputs(userId).doc(activityId).update({
      status: 2, // STATUS_COMPLETED
      input_data: inputData,
      updated_at: new Date()
    });
  }

  async delete(userId: string, activityId: string): Promise<void> {
    await this.userPendingInputs(userId).doc(activityId).delete();
  }
}
