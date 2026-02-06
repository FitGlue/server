// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { CloudTasksClient } from '@google-cloud/tasks';
import { getMessaging } from 'firebase-admin/messaging';
import { Timestamp } from 'firebase-admin/firestore';

/**
 * Connection Actions Handler - One-off actions for integrations
 *
 * Endpoints:
 * - GET  /actions/:sourceId             - List available actions for a source
 * - POST /actions/:sourceId/:actionId   - Trigger an action (enqueues to Cloud Tasks)
 * - POST /execute/:jobId                - Execute a job (called by Cloud Tasks)
 */

// Action definitions per source
const SOURCE_ACTIONS: Record<string, ActionDefinition[]> = {
    strava: [
        {
            id: 'import_cardio_prs',
            label: 'Import Cardio PRs',
            description: 'Fetch your fastest 5K, 10K, and half marathon times from Strava history',
            icon: '🏃',
        },
    ],
    hevy: [
        {
            id: 'import_strength_prs',
            label: 'Import Strength PRs',
            description: 'Import 1RM and volume records from your last 12 months of Hevy workouts',
            icon: '💪',
        },
    ],
};

interface ActionDefinition {
    id: string;
    label: string;
    description: string;
    icon: string;
}

interface ActionJob {
    jobId: string;
    userId: string;
    sourceId: string;
    actionId: string;
    status: 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED';
    createdAt: string;
    completedAt?: string;
    result?: ActionResult;
    error?: string;
}

interface ActionResult {
    recordsImported: number;
    recordsSkipped: number;
    details: string[];
}

const tasksClient = new CloudTasksClient();

// eslint-disable-next-line complexity
export const handler: FrameworkHandler = async (req, ctx) => {
    if (!ctx.userId) {
        throw new HttpError(401, 'Unauthorized');
    }

    const path = req.path;
    const method = req.method;
    const userId = ctx.userId;

    // --- List Actions for Source ---
    // GET /actions/:sourceId
    if (method === 'GET' && path.includes('/actions/')) {
        const segments = path.split('/').filter(s => s.length > 0);
        const actionsIndex = segments.findIndex(s => s === 'actions');
        const sourceId = segments[actionsIndex + 1];

        if (!sourceId) {
            throw new HttpError(400, 'Missing source ID');
        }

        const actions = SOURCE_ACTIONS[sourceId] ?? [];
        return { actions };
    }

    // --- Trigger Action (enqueue to Cloud Tasks) ---
    // POST /actions/:sourceId/:actionId
    if (method === 'POST' && path.includes('/actions/') && !path.includes('/execute/')) {
        const segments = path.split('/').filter(s => s.length > 0);
        const actionsIndex = segments.findIndex(s => s === 'actions');
        const sourceId = segments[actionsIndex + 1];
        const actionId = segments[actionsIndex + 2];

        if (!sourceId || !actionId) {
            throw new HttpError(400, 'Missing source ID or action ID');
        }

        // Validate action exists
        const actions = SOURCE_ACTIONS[sourceId];
        if (!actions || !actions.find(a => a.id === actionId)) {
            throw new HttpError(404, `Action ${actionId} not found for source ${sourceId}`);
        }

        // Create job document
        const jobId = `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
        const job: ActionJob = {
            jobId,
            userId,
            sourceId,
            actionId,
            status: 'PENDING',
            createdAt: new Date().toISOString(),
        };

        // Store job in Firestore
        await db.collection('users').doc(userId).collection('action_jobs').doc(jobId).set({
            ...job,
            created_at: Timestamp.now(),
        });

        // Enqueue to Cloud Tasks
        const projectId = process.env.GCP_PROJECT || process.env.GCLOUD_PROJECT;
        const location = process.env.FUNCTION_REGION || 'us-central1';
        const queueName = 'connection-actions';

        const parent = tasksClient.queuePath(projectId!, location, queueName);
        const serviceUrl = process.env.CONNECTION_ACTIONS_URL || `https://${location}-${projectId}.cloudfunctions.net/connection-actions-handler`;

        await tasksClient.createTask({
            parent,
            task: {
                httpRequest: {
                    httpMethod: 'POST',
                    url: `${serviceUrl}/execute/${jobId}`,
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: Buffer.from(JSON.stringify({
                        jobId,
                        userId,
                        sourceId,
                        actionId,
                    })).toString('base64'),
                    oidcToken: {
                        serviceAccountEmail: `${projectId}@appspot.gserviceaccount.com`,
                    },
                },
            },
        });

        ctx.logger.info('Enqueued connection action', { userId, sourceId, actionId, jobId });
        return { success: true, jobId, message: 'Action queued for processing' };
    }

    // --- Execute Job (called by Cloud Tasks) ---
    // POST /execute/:jobId
    if (method === 'POST' && path.includes('/execute/')) {
        const segments = path.split('/').filter(s => s.length > 0);
        const executeIndex = segments.findIndex(s => s === 'execute');
        const jobId = segments[executeIndex + 1];

        if (!jobId) {
            throw new HttpError(400, 'Missing job ID');
        }

        const body = req.body as { userId: string; sourceId: string; actionId: string };
        const { userId: jobUserId, sourceId, actionId } = body;

        // Update job status to RUNNING
        const jobRef = db.collection('users').doc(jobUserId).collection('action_jobs').doc(jobId);
        await jobRef.update({ status: 'RUNNING' });

        try {
            // Execute the action
            const result = await executeAction(ctx, jobUserId, sourceId, actionId);

            // Update job with result
            await jobRef.update({
                status: 'COMPLETED',
                completed_at: Timestamp.now(),
                result,
            });

            // Send push notification
            await sendCompletionNotification(jobUserId, sourceId, actionId, result);

            ctx.logger.info('Connection action completed', { jobId, sourceId, actionId, recordsImported: result.recordsImported });
            return { success: true, result };

        } catch (error) {
            const errorMessage = error instanceof Error ? error.message : String(error);
            await jobRef.update({
                status: 'FAILED',
                completed_at: Timestamp.now(),
                error: errorMessage,
            });

            // Send failure notification
            await sendFailureNotification(jobUserId, sourceId, actionId, errorMessage);

            ctx.logger.error('Connection action failed', { jobId, error: errorMessage });
            throw error;
        }
    }

    throw new HttpError(404, 'Not found');
};

// Execute the specified action
async function executeAction(
    ctx: { logger: { info: (msg: string, data?: Record<string, unknown>) => void } },
    userId: string,
    sourceId: string,
    actionId: string
): Promise<ActionResult> {
    // Import action handlers dynamically to keep the main handler lean
    if (sourceId === 'strava' && actionId === 'import_cardio_prs') {
        const { importStravaCardioPRs } = await import('./actions/strava');
        return importStravaCardioPRs(userId, ctx.logger);
    }

    if (sourceId === 'hevy' && actionId === 'import_strength_prs') {
        const { importHevyStrengthPRs } = await import('./actions/hevy');
        return importHevyStrengthPRs(userId, ctx.logger);
    }

    throw new Error(`Unknown action: ${sourceId}/${actionId}`);
}

// Send push notification on completion
async function sendCompletionNotification(
    userId: string,
    sourceId: string,
    actionId: string,
    result: ActionResult
): Promise<void> {
    const userDoc = await db.collection('users').doc(userId).get();
    const userData = userDoc.data();
    const fcmTokens: string[] = userData?.fcm_tokens ?? [];

    if (fcmTokens.length === 0) {
        return;
    }

    const sourceName = sourceId.charAt(0).toUpperCase() + sourceId.slice(1);
    const title = `${sourceName} Import Complete`;
    const body = result.recordsImported > 0
        ? `Imported ${result.recordsImported} new personal record${result.recordsImported > 1 ? 's' : ''}`
        : 'No new records found to import';

    const messaging = getMessaging();
    for (const token of fcmTokens) {
        try {
            await messaging.send({
                token,
                notification: { title, body },
                data: {
                    type: 'CONNECTION_ACTION',
                    sourceId,
                    actionId,
                    recordsImported: String(result.recordsImported),
                },
            });
        } catch (err) {
            // Token might be invalid, continue with others
            console.warn('Failed to send notification to token', { token, error: err });
        }
    }
}

// Send push notification on failure
async function sendFailureNotification(
    userId: string,
    sourceId: string,
    actionId: string,
    errorMessage: string
): Promise<void> {
    const userDoc = await db.collection('users').doc(userId).get();
    const userData = userDoc.data();
    const fcmTokens: string[] = userData?.fcm_tokens ?? [];

    if (fcmTokens.length === 0) {
        return;
    }

    const sourceName = sourceId.charAt(0).toUpperCase() + sourceId.slice(1);
    const title = `${sourceName} Import Failed`;
    const body = 'There was an error importing your records. Please try again later.';

    const messaging = getMessaging();
    for (const token of fcmTokens) {
        try {
            await messaging.send({
                token,
                notification: { title, body },
                data: {
                    type: 'CONNECTION_ACTION_FAILED',
                    sourceId,
                    actionId,
                    error: errorMessage.substring(0, 100), // Truncate for payload limits
                },
            });
        } catch (err) {
            console.warn('Failed to send notification to token', { token, error: err });
        }
    }
}

// Export the wrapped function
export const connectionActionsHandler = createCloudFunction(handler, {
    auth: {
        strategies: [new FirebaseAuthStrategy()]
    },
    skipExecutionLogging: true
});
