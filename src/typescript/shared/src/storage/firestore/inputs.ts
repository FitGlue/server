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

  async listPending(userId: string): Promise<PendingInput[]> {
    const snapshot = await this.db.collection('pending_inputs')
      .where('user_id', '==', userId)
      .where('status', '==', 1) // STATUS_WAITING
      .orderBy('created_at', 'desc')
      .get();

    return snapshot.docs.map(doc => FirestoreToPendingInput(doc.data() as Record<string, unknown>));
  }

  async resolve(activityId: string, inputData: Record<string, string>): Promise<void> {
    await this.db.collection('pending_inputs').doc(activityId).update({
      status: 2, // STATUS_COMPLETED
      input_data: inputData,
      updated_at: new Date()
    });
  }
}
