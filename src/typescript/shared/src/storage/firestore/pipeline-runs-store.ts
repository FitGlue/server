import * as admin from 'firebase-admin';
import * as converters from './converters';
import { PipelineRun, PipelineRunStatus, DestinationOutcome } from '../../types/pb/user';

/**
 * PipelineRunStore provides typed access to pipeline run operations.
 * Stored in: users/{userId}/pipeline_runs/{pipelineRunId}
 */
export class PipelineRunStore {
    constructor(private db: admin.firestore.Firestore) { }

    /**
     * Get the pipeline_runs collection for a specific user.
     */
    private collection(userId: string) {
        return this.db.collection('users').doc(userId).collection('pipeline_runs').withConverter(converters.pipelineRunConverter);
    }

    /**
     * Create a new pipeline run.
     */
    async create(userId: string, run: PipelineRun): Promise<void> {
        await this.collection(userId).doc(run.id).set(run);
    }

    /**
     * Get a pipeline run by ID.
     */
    async get(userId: string, pipelineRunId: string): Promise<PipelineRun | null> {
        const doc = await this.collection(userId).doc(pipelineRunId).get();
        if (!doc.exists) {
            return null;
        }
        return doc.data() || null;
    }

    /**
     * List pipeline runs for a user.
     */
    async list(
        userId: string,
        options?: {
            limit?: number;
            status?: PipelineRunStatus;
            startAfter?: Date;
        }
    ): Promise<PipelineRun[]> {
        let q: admin.firestore.Query = this.collection(userId);

        if (options?.status !== undefined) {
            q = q.where('status', '==', options.status);
        }

        q = q.orderBy('created_at', 'desc');

        if (options?.startAfter) {
            q = q.startAfter(options.startAfter);
        }

        if (options?.limit) {
            q = q.limit(options.limit);
        } else {
            q = q.limit(50);
        }

        const snapshot = await q.get();
        return snapshot.docs.map(doc => doc.data() as PipelineRun);
    }

    /**
     * Update a pipeline run's status.
     */
    async updateStatus(
        userId: string,
        pipelineRunId: string,
        status: PipelineRunStatus,
        errorMessage?: string
    ): Promise<void> {
        const update: Record<string, unknown> = {
            status,
            updated_at: new Date(),
        };
        if (errorMessage) {
            update.error_message = errorMessage;
        }
        await this.collection(userId).doc(pipelineRunId).update(update);
    }

    /**
     * Get the destination_outcomes subcollection for a specific pipeline run.
     * Each destination has its own document to avoid race conditions when parallel uploaders update.
     */
    private destinationOutcomesCollection(userId: string, pipelineRunId: string) {
        return this.db.collection('users').doc(userId)
            .collection('pipeline_runs').doc(pipelineRunId)
            .collection('destination_outcomes');
    }

    /**
     * Set a single destination outcome in the subcollection.
     * This is an atomic write that doesn't conflict with other destination updates.
     */
    async setDestinationOutcome(
        userId: string,
        pipelineRunId: string,
        outcome: DestinationOutcome
    ): Promise<void> {
        const docId = String(outcome.destination);
        await this.destinationOutcomesCollection(userId, pipelineRunId).doc(docId).set({
            destination: outcome.destination,
            status: outcome.status,
            external_id: outcome.externalId,
            error: outcome.error,
            completed_at: outcome.completedAt,
            updated_at: new Date(),
        }, { merge: true });
    }

    /**
     * Get all destination outcomes from the subcollection.
     */
    async getDestinationOutcomes(
        userId: string,
        pipelineRunId: string
    ): Promise<DestinationOutcome[]> {
        const snapshot = await this.destinationOutcomesCollection(userId, pipelineRunId).get();
        return snapshot.docs.map(doc => {
            const data = doc.data();
            return {
                destination: data.destination,
                status: data.status,
                externalId: data.external_id,
                error: data.error,
                completedAt: data.completed_at?.toDate?.() || data.completed_at,
            };
        });
    }

    /**
     * Update a single destination outcome.
     * DEPRECATED: Use setDestinationOutcome for new code.
     * This method now delegates to setDestinationOutcome.
     */
    async updateDestinationOutcome(
        userId: string,
        pipelineRunId: string,
        outcome: DestinationOutcome
    ): Promise<void> {
        await this.setDestinationOutcome(userId, pipelineRunId, outcome);
    }


    /**
     * Set the enriched event URI on a pipeline run (called by router after GCS upload).
     */
    async setEnrichedEventUri(userId: string, pipelineRunId: string, enrichedEventUri: string): Promise<void> {
        await this.collection(userId).doc(pipelineRunId).update({
            enriched_event_uri: enrichedEventUri,
            updated_at: new Date(),
        });
    }

    /**
     * Count pipeline runs by status.
     */
    async countByStatus(userId: string, status: PipelineRunStatus): Promise<number> {
        const snapshot = await this.collection(userId)
            .where('status', '==', status)
            .count()
            .get();
        return snapshot.data().count;
    }

    /**
     * Count synced runs since a date.
     */
    async countSyncedSince(userId: string, since: Date): Promise<number> {
        const snapshot = await this.collection(userId)
            .where('status', '==', PipelineRunStatus.PIPELINE_RUN_STATUS_SYNCED)
            .where('created_at', '>=', since)
            .count()
            .get();
        return snapshot.data().count;
    }

    /**
     * Find a pipeline run by activity ID.
     * Returns the most recent pipeline run for the activity, or null if not found.
     */
    async findByActivityId(userId: string, activityId: string): Promise<PipelineRun | null> {
        const snapshot = await this.collection(userId)
            .where('activity_id', '==', activityId)
            .orderBy('created_at', 'desc')
            .limit(1)
            .get();

        if (snapshot.empty) {
            return null;
        }
        return snapshot.docs[0].data() || null;
    }

    /**
     * Delete a pipeline run.
     */
    async delete(userId: string, pipelineRunId: string): Promise<void> {
        await this.collection(userId).doc(pipelineRunId).delete();
    }

    // ============================================
    // Methods for synchronized_activities migration
    // ============================================

    /**
     * Count synced pipeline runs (total or since a date).
     * Replaces ActivityStore.countSynchronized()
     */
    async countSynced(userId: string, since?: Date): Promise<number> {
        let q: admin.firestore.Query = this.collection(userId)
            .where('status', '==', PipelineRunStatus.PIPELINE_RUN_STATUS_SYNCED);

        if (since) {
            q = q.where('created_at', '>=', since);
        }

        const snapshot = await q.count().get();
        return snapshot.data().count;
    }

    /**
     * List synced pipeline runs with pagination.
     * Replaces ActivityStore.listSynchronized()
     */
    async listSynced(
        userId: string,
        limit: number = 20,
        offset: number = 0
    ): Promise<PipelineRun[]> {
        let q: admin.firestore.Query = this.collection(userId)
            .where('status', '==', PipelineRunStatus.PIPELINE_RUN_STATUS_SYNCED)
            .orderBy('created_at', 'desc')
            .limit(limit);

        if (offset > 0) {
            q = q.offset(offset);
        }

        const snapshot = await q.get();
        return snapshot.docs.map(doc => doc.data() as PipelineRun);
    }

    /**
     * Get a synced run by activity ID.
     * Replaces ActivityStore.getSynchronized() - uses findByActivityId internally
     * but filters for SYNCED status.
     */
    async getSynced(userId: string, activityId: string): Promise<PipelineRun | null> {
        const snapshot = await this.collection(userId)
            .where('activity_id', '==', activityId)
            .where('status', '==', PipelineRunStatus.PIPELINE_RUN_STATUS_SYNCED)
            .orderBy('created_at', 'desc')
            .limit(1)
            .get();

        if (snapshot.empty) {
            return null;
        }
        return snapshot.docs[0].data() || null;
    }
}
