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
     * Update a single destination outcome.
     */
    async updateDestinationOutcome(
        userId: string,
        pipelineRunId: string,
        outcome: DestinationOutcome
    ): Promise<void> {
        // Fetch current run, update destinations array, and save
        const run = await this.get(userId, pipelineRunId);
        if (!run) {
            throw new Error(`PipelineRun not found: ${pipelineRunId}`);
        }

        const destinations = run.destinations || [];
        const existingIndex = destinations.findIndex(d => d.destination === outcome.destination);

        if (existingIndex >= 0) {
            destinations[existingIndex] = outcome;
        } else {
            destinations.push(outcome);
        }

        await this.collection(userId).doc(pipelineRunId).update({
            destinations: destinations.map(d => ({
                destination: d.destination,
                status: d.status,
                external_id: d.externalId,
                error: d.error,
                completed_at: d.completedAt,
            })),
            updated_at: new Date(),
        });
    }

    /**
     * Set the enriched event on a pipeline run (called by router).
     */
    async setEnrichedEvent(userId: string, pipelineRunId: string, enrichedEvent: unknown): Promise<void> {
        await this.collection(userId).doc(pipelineRunId).update({
            enriched_event: JSON.stringify(enrichedEvent),
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
     * Delete a pipeline run.
     */
    async delete(userId: string, pipelineRunId: string): Promise<void> {
        await this.collection(userId).doc(pipelineRunId).delete();
    }
}
