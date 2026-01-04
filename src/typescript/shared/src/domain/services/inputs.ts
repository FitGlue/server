import { InputStore } from '../../storage/firestore/inputs';
import { PendingInput } from '../../types/pb/pending_input';

export class InputService {
  constructor(private store: InputStore) { }

  async getPendingInput(activityId: string): Promise<PendingInput | null> {
    return this.store.getPending(activityId);
  }

  async listPendingInputs(userId: string): Promise<PendingInput[]> {
    return this.store.listPending(userId);
  }

  async resolveInput(activityId: string, userId: string, inputData: Record<string, string>): Promise<void> {
    const pending = await this.store.getPending(activityId);
    if (!pending) {
      throw new Error(`Pending input ${activityId} not found`);
    }

    if (pending.userId !== userId) {
      throw new Error('Unauthorized');
    }

    if (pending.status !== 1) { // STATUS_WAITING
      throw new Error('Input already resolved or invalid status');
    }

    await this.store.resolve(activityId, inputData);
  }
}
