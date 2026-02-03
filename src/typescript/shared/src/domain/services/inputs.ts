import { InputStore } from '../../storage/firestore/inputs';
import { PipelineRunStore } from '../../storage/firestore/pipeline-runs-store';
import { PendingInput } from '../../types/pb/pending_input';
import { PipelineRunStatus } from '../../types/pb/user';
import { AuthorizationService, ForbiddenError } from './authorization';

export class InputService {
  constructor(
    private store: InputStore,
    private authorization?: AuthorizationService,
    private pipelineRunStore?: PipelineRunStore
  ) { }

  async getPendingInput(userId: string, activityId: string): Promise<PendingInput | null> {
    return this.store.getPending(userId, activityId);
  }

  async listPendingInputs(userId: string): Promise<PendingInput[]> {
    return this.store.listPending(userId);
  }

  /**
   * Resolve a pending input.
   * Validates ownership: requesting user must own the input OR be admin.
   */
  async resolveInput(userId: string, activityId: string, requestingUserId: string, inputData: Record<string, string>): Promise<void> {
    const pending = await this.store.getPending(userId, activityId);
    if (!pending) {
      throw new Error(`Pending input ${activityId} not found`);
    }

    // Authorization check: requesting user must own the input or be admin
    if (this.authorization) {
      const canAccess = await this.authorization.canAccessUser(requestingUserId, pending.userId);
      if (!canAccess) {
        throw new ForbiddenError('You do not have permission to resolve this input');
      }
    } else {
      // Fallback to direct comparison if no authorization service
      if (pending.userId !== requestingUserId) {
        throw new Error('Unauthorized');
      }
    }

    if (pending.status !== 1) { // STATUS_WAITING
      throw new Error('Input already resolved or invalid status');
    }

    await this.store.resolve(userId, activityId, inputData);
  }

  /**
   * Dismiss a pending input.
   * Validates ownership: requesting user must own the input OR be admin.
   * Also updates the associated pipeline run to SKIPPED status.
   */
  async dismissInput(userId: string, activityId: string, requestingUserId: string): Promise<void> {
    const pending = await this.store.getPending(userId, activityId);
    if (!pending) {
      // Idempotent success if already gone
      return;
    }

    // Authorization check: requesting user must own the input or be admin
    if (this.authorization) {
      const canAccess = await this.authorization.canAccessUser(requestingUserId, pending.userId);
      if (!canAccess) {
        throw new ForbiddenError('You do not have permission to dismiss this input');
      }
    } else {
      // Fallback to direct comparison if no authorization service
      if (pending.userId !== requestingUserId) {
        throw new Error('Unauthorized');
      }
    }

    // Update the associated pipeline run to SKIPPED status if we have the store
    if (this.pipelineRunStore && pending.linkedActivityId) {
      const pipelineRun = await this.pipelineRunStore.findByActivityId(userId, pending.linkedActivityId);
      if (pipelineRun && pipelineRun.status === PipelineRunStatus.PIPELINE_RUN_STATUS_PENDING) {
        await this.pipelineRunStore.updateStatus(
          userId,
          pipelineRun.id,
          PipelineRunStatus.PIPELINE_RUN_STATUS_SKIPPED,
          'Dismissed by user'
        );
      }
    }

    await this.store.delete(userId, activityId);
  }
}
