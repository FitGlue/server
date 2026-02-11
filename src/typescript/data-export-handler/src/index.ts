// Module-level imports for smart pruning
import { createCloudFunction, FirebaseAuthStrategy, PayloadUserStrategy, FrameworkHandler, db } from '@fitglue/shared/framework';
import { HttpError } from '@fitglue/shared/errors';
import { CloudTasksClient } from '@google-cloud/tasks';

import { getMessaging } from 'firebase-admin/messaging';
import { getAuth } from 'firebase-admin/auth';
import { getStorage } from 'firebase-admin/storage';
import { Timestamp, FieldValue } from 'firebase-admin/firestore';
import * as nodemailer from 'nodemailer';
import JSZip from 'jszip';

/**
 * Data Export Handler - GDPR Article 20 data portability + per-run export
 *
 * User-facing endpoints (Firebase Auth):
 * - POST /export/full           - Trigger full account export (background)
 * - GET  /export/status/:jobId  - Poll export job status
 * - GET  /export/run/:runId     - Synchronous single-run export
 *
 * Cloud Tasks callback (PayloadUserStrategy):
 * - POST /execute/:jobId        - Background worker that builds the ZIP
 */

interface ExportJob {
    jobId: string;
    userId: string;
    type: 'data_export';
    status: 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED';
    createdAt: string;
    completedAt?: string;
    downloadUrl?: string;
    error?: string;
}

const tasksClient = new CloudTasksClient();


const SENDER_EMAIL = 'system@fitglue.tech';

// Sub-collections to export under users/{userId}
const USER_SUBCOLLECTIONS = [
    'pipelines',
    'pipeline_runs',
    'counters',
    'personal_records',
    'booster_data',
    'raw_activities',
    'synchronized_activities',
    'executions',
    'pending_inputs',
    'action_jobs',
] as const;

// Fields to mask in exported data (tokens, secrets)
const SENSITIVE_FIELDS = [
    'access_token', 'refresh_token', 'token', 'secret',
    'api_key', 'hashed_key', 'stripe_customer_id',
];

function maskSensitiveData(data: Record<string, unknown>): Record<string, unknown> {
    const masked = { ...data };
    for (const key of Object.keys(masked)) {
        if (SENSITIVE_FIELDS.some(f => key.toLowerCase().includes(f))) {
            if (typeof masked[key] === 'string' && (masked[key] as string).length > 0) {
                const val = masked[key] as string;
                masked[key] = val.substring(0, 4) + '***REDACTED***';
            }
        } else if (masked[key] && typeof masked[key] === 'object' && !Array.isArray(masked[key])) {
            masked[key] = maskSensitiveData(masked[key] as Record<string, unknown>);
        }
    }
    return masked;
}

async function fetchSubcollection(
    userId: string,
    subcollection: string
): Promise<Record<string, unknown>[]> {
    const query = db.collection('users').doc(userId).collection(subcollection)
        .orderBy('__name__');

    const snapshot = await query.get();
    return snapshot.docs.map((doc: FirebaseFirestore.QueryDocumentSnapshot) => ({
        _id: doc.id,
        ...maskSensitiveData(doc.data()),
    }));
}

async function fetchGcsBlob(uri: string): Promise<{ content: string; found: boolean }> {
    if (!uri || !uri.startsWith('gs://')) {
        return { content: '', found: false };
    }

    try {
        const withoutProtocol = uri.replace('gs://', '');
        const slashIndex = withoutProtocol.indexOf('/');
        const bucketName = withoutProtocol.substring(0, slashIndex);
        const filePath = withoutProtocol.substring(slashIndex + 1);

        const storage = getStorage();
        const file = storage.bucket(bucketName).file(filePath);
        const [exists] = await file.exists();

        if (!exists) {
            return { content: '', found: false };
        }

        const [buffer] = await file.download();
        return { content: buffer.toString('utf-8'), found: true };
    } catch {
        return { content: '', found: false };
    }
}

function getEmailPassword(): string {
    const password = process.env.EMAIL_APP_PASSWORD;
    if (!password) {
        throw new Error('EMAIL_APP_PASSWORD environment variable is not set');
    }
    return password;
}

async function sendExportEmail(userEmail: string, downloadUrl: string): Promise<void> {
    const emailPassword = await getEmailPassword();

    const transporter = nodemailer.createTransport({
        host: 'smtp.gmail.com',
        port: 587,
        secure: false,
        auth: { user: SENDER_EMAIL, pass: emailPassword },
    });

    await transporter.sendMail({
        from: `"FitGlue" <${SENDER_EMAIL}>`,
        to: userEmail,
        subject: '[FitGlue] Your data export is ready',
        html: [
            '<h2>📦 Your Data Export is Ready</h2>',
            '<p>Your FitGlue data export has been prepared and is ready to download.</p>',
            `<p><a href="${downloadUrl}" style="display:inline-block;padding:12px 24px;background:#4F46E5;color:#fff;text-decoration:none;border-radius:8px;font-weight:bold;">Download My Data</a></p>`,
            '<p style="color:#666;font-size:12px;">This link will expire in 24 hours. If you did not request this export, please contact support@fitglue.tech.</p>',
            '<hr>',
            '<p style="color:#666;font-size:12px;">FitGlue — Your fitness data, your way.</p>',
        ].join('\n'),
    });
}

async function sendExportNotification(userId: string): Promise<void> {
    const userDoc = await db.collection('users').doc(userId).get();
    const userData = userDoc.data();
    const fcmTokens: string[] = userData?.fcm_tokens ?? [];

    for (const token of fcmTokens) {
        try {
            await getMessaging().send({
                token,
                notification: {
                    title: 'Data Export Ready',
                    body: 'Your FitGlue data export is ready to download.',
                },
                data: { type: 'DATA_EXPORT_READY' },
            });
        } catch (err) {
            const isNotRegistered = err instanceof Error &&
                'code' in err && (err as { code: string }).code === 'messaging/registration-token-not-registered';
            if (isNotRegistered) {
                await db.collection('users').doc(userId).update({
                    fcm_tokens: FieldValue.arrayRemove(token),
                });
            }
        }
    }
}

function extractSegmentAfter(path: string, keyword: string): string {
    const segments = path.split('/').filter((s: string) => s.length > 0);
    const idx = segments.findIndex((s: string) => s === keyword);
    return segments[idx + 1] || '';
}

async function addRunBlobsToFolder(
    runsFolder: JSZip,
    doc: FirebaseFirestore.QueryDocumentSnapshot
): Promise<void> {
    const runData = doc.data();

    const enrichedUri = runData.enriched_event_uri as string;
    if (enrichedUri) {
        const blob = await fetchGcsBlob(enrichedUri);
        if (blob.found) {
            runsFolder.file(`${doc.id}_enriched.json`, blob.content);
        } else {
            runsFolder.file(
                `${doc.id}_enriched_DELETED.txt`,
                'This enriched event data was auto-purged after 7 days.\n' +
                `Original GCS URI: ${enrichedUri}\n` +
                'The Firestore metadata for this run is still available in pipeline_runs.json.'
            );
        }
    }

    const payloadUri = runData.original_payload_uri as string;
    if (payloadUri) {
        const blob = await fetchGcsBlob(payloadUri);
        if (blob.found) {
            runsFolder.file(`${doc.id}_payload.json`, blob.content);
        } else {
            runsFolder.file(
                `${doc.id}_payload_DELETED.txt`,
                'This original payload was auto-purged after 7 days.\n' +
                `Original GCS URI: ${payloadUri}\n` +
                'The Firestore metadata for this run is still available in pipeline_runs.json.'
            );
        }
    }
}

// --- Route: POST /export/full ---
async function handleTriggerExport(
    userId: string,
    logger: { info: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<Record<string, unknown>> {
    // Check for existing pending export
    const existingJobs = await db.collection('users').doc(userId)
        .collection('action_jobs')
        .where('type', '==', 'data_export')
        .where('status', 'in', ['PENDING', 'RUNNING'])
        .limit(1)
        .get();

    if (!existingJobs.empty) {
        const existing = existingJobs.docs[0].data() as ExportJob;
        return { jobId: existing.jobId, status: existing.status, message: 'Export already in progress' };
    }

    const jobId = `export-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
    const job: ExportJob = {
        jobId,
        userId,
        type: 'data_export',
        status: 'PENDING',
        createdAt: new Date().toISOString(),
    };

    await db.collection('users').doc(userId).collection('action_jobs').doc(jobId).set({
        ...job,
        created_at: Timestamp.now(),
    });

    const projectId = process.env.GOOGLE_CLOUD_PROJECT;
    if (!projectId) {
        throw new Error('GOOGLE_CLOUD_PROJECT environment variable is not set');
    }
    const location = process.env.FUNCTION_REGION || 'us-central1';
    const parent = tasksClient.queuePath(projectId, location, 'data-export');
    const serviceUrl = process.env.DATA_EXPORT_URL ||
        `https://${location}-${projectId}.cloudfunctions.net/data-export-handler`;

    await tasksClient.createTask({
        parent,
        task: {
            httpRequest: {
                httpMethod: 'POST',
                url: `${serviceUrl}/execute/${jobId}`,
                headers: { 'Content-Type': 'application/json' },
                body: Buffer.from(JSON.stringify({ jobId, userId })).toString('base64'),
                oidcToken: {
                    serviceAccountEmail: `${projectId}@appspot.gserviceaccount.com`,
                    audience: serviceUrl,
                },
            },
        },
    });

    logger.info('Data export enqueued', { userId, jobId });
    return { success: true, jobId, message: 'Export queued for processing' };
}

// --- Route: GET /export/status/:jobId ---
async function handlePollStatus(
    userId: string,
    path: string
): Promise<Record<string, unknown>> {
    const jobId = extractSegmentAfter(path, 'status');
    if (!jobId) {
        throw new HttpError(400, 'Missing job ID');
    }

    const jobDoc = await db.collection('users').doc(userId)
        .collection('action_jobs').doc(jobId).get();

    if (!jobDoc.exists) {
        throw new HttpError(404, 'Export job not found');
    }

    const job = jobDoc.data() as ExportJob;
    if (job.userId !== userId) {
        throw new HttpError(404, 'Export job not found');
    }

    return {
        jobId: job.jobId,
        status: job.status,
        downloadUrl: job.downloadUrl || null,
        error: job.error || null,
        createdAt: job.createdAt,
        completedAt: job.completedAt || null,
    };
}

// --- Route: GET /export/run/:runId ---
async function handleRunExport(
    userId: string,
    path: string
): Promise<Record<string, unknown>> {
    const runId = extractSegmentAfter(path, 'run');
    if (!runId) {
        throw new HttpError(400, 'Missing run ID');
    }

    const runDoc = await db.collection('users').doc(userId)
        .collection('pipeline_runs').doc(runId).get();

    if (!runDoc.exists) {
        throw new HttpError(404, 'Pipeline run not found');
    }

    const runData = runDoc.data() as Record<string, unknown>;
    const enrichedEventUri = runData.enriched_event_uri as string;
    const originalPayloadUri = runData.original_payload_uri as string;

    let enrichedEvent: unknown = null;
    let originalPayload: unknown = null;

    if (enrichedEventUri) {
        const blob = await fetchGcsBlob(enrichedEventUri);
        if (blob.found) {
            try { enrichedEvent = JSON.parse(blob.content); } catch { enrichedEvent = blob.content; }
        }
    }

    if (originalPayloadUri) {
        const blob = await fetchGcsBlob(originalPayloadUri);
        if (blob.found) {
            try { originalPayload = JSON.parse(blob.content); } catch { originalPayload = blob.content; }
        }
    }

    return {
        exportVersion: '1.0',
        exportedAt: new Date().toISOString(),
        pipelineRun: maskSensitiveData(runData),
        enrichedEvent,
        originalPayload,
        _meta: {
            enrichedEventAvailable: !!enrichedEvent,
            originalPayloadAvailable: !!originalPayload,
        },
    };
}

// --- Route: POST /execute/:jobId (Cloud Tasks callback) ---
async function handleExecuteExport(
    path: string,
    body: { userId: string },
    logger: { info: (msg: string, meta?: Record<string, unknown>) => void; warn: (msg: string, meta?: Record<string, unknown>) => void; error: (msg: string, meta?: Record<string, unknown>) => void }
): Promise<Record<string, unknown>> {
    const jobId = extractSegmentAfter(path, 'execute');
    if (!jobId) {
        throw new HttpError(400, 'Missing job ID');
    }

    const jobUserId = body.userId;
    const jobRef = db.collection('users').doc(jobUserId).collection('action_jobs').doc(jobId);
    await jobRef.update({ status: 'RUNNING' });

    try {
        const zip = new JSZip();
        const exportDate = new Date().toISOString().split('T')[0];
        const folder = zip.folder(`fitglue-export-${exportDate}`);
        if (!folder) throw new Error('Failed to create ZIP folder');

        // 1. User profile
        const userDoc = await db.collection('users').doc(jobUserId).get();
        if (userDoc.exists) {
            folder.file('profile.json', JSON.stringify(
                maskSensitiveData(userDoc.data() as Record<string, unknown>), null, 2
            ));
        }

        // 2. All sub-collections
        for (const subcollection of USER_SUBCOLLECTIONS) {
            const docs = await fetchSubcollection(jobUserId, subcollection);
            if (docs.length > 0) {
                folder.file(`${subcollection}.json`, JSON.stringify(docs, null, 2));
            }
        }

        // 3. Pipeline runs with GCS blobs
        const runsSnapshot = await db.collection('users').doc(jobUserId)
            .collection('pipeline_runs').get();

        if (!runsSnapshot.empty) {
            const runsFolder = folder.folder('pipeline_runs_data');
            if (runsFolder) {
                for (const doc of runsSnapshot.docs) {
                    await addRunBlobsToFolder(runsFolder, doc);
                }
            }
        }

        // 4. Showcased activities
        const showcaseSnapshot = await db.collection('showcased_activities')
            .where('user_id', '==', jobUserId).get();

        if (!showcaseSnapshot.empty) {
            const showcaseDocs = showcaseSnapshot.docs.map(
                (doc: FirebaseFirestore.QueryDocumentSnapshot) => ({
                    _id: doc.id,
                    ...maskSensitiveData(doc.data()),
                })
            );
            folder.file('showcases.json', JSON.stringify(showcaseDocs, null, 2));
        }

        // 5. Ingress API keys
        const apiKeysSnapshot = await db.collection('ingress_api_keys')
            .where('user_id', '==', jobUserId).get();

        if (!apiKeysSnapshot.empty) {
            const apiKeyDocs = apiKeysSnapshot.docs.map(
                (doc: FirebaseFirestore.QueryDocumentSnapshot) => ({
                    _id: doc.id,
                    ...maskSensitiveData(doc.data()),
                })
            );
            folder.file('api_keys.json', JSON.stringify(apiKeyDocs, null, 2));
        }

        // 6. Export metadata
        folder.file('_export_meta.json', JSON.stringify({
            exportVersion: '1.0',
            exportedAt: new Date().toISOString(),
            userId: jobUserId,
            note: 'Files ending in _DELETED.txt indicate data that was auto-purged from cloud storage after 7 days.',
        }, null, 2));

        // Generate, upload, sign
        const zipBuffer = await zip.generateAsync({ type: 'nodebuffer' });
        const projectId = process.env.GOOGLE_CLOUD_PROJECT || '';
        const storage = getStorage();
        const bucket = storage.bucket(`${projectId}-artifacts`);
        const gcsPath = `exports/${jobUserId}/${jobId}.zip`;
        const file = bucket.file(gcsPath);

        await file.save(zipBuffer, { contentType: 'application/zip', metadata: { cacheControl: 'no-cache' } });
        const [signedUrl] = await file.getSignedUrl({ action: 'read', expires: Date.now() + 24 * 60 * 60 * 1000 });

        await jobRef.update({ status: 'COMPLETED', completed_at: Timestamp.now(), downloadUrl: signedUrl });
        await sendExportNotification(jobUserId);

        try {
            const authUser = await getAuth().getUser(jobUserId);
            if (authUser.email) {
                await sendExportEmail(authUser.email, signedUrl);
            }
        } catch {
            logger.warn('Failed to send export email', { userId: jobUserId });
        }

        logger.info('Data export completed', { jobId, userId: jobUserId });
        return { success: true };

    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        await jobRef.update({ status: 'FAILED', completed_at: Timestamp.now(), error: errorMessage });
        logger.error('Data export failed', { jobId, error: errorMessage });
        throw error;
    }
}

// --- Main handler ---
export const handler: FrameworkHandler = async (req, ctx) => {
    if (!ctx.userId) {
        throw new HttpError(401, 'Unauthorized');
    }

    const path = req.path;
    const method = req.method;

    if (method === 'POST' && path.endsWith('/export/full')) {
        return handleTriggerExport(ctx.userId, ctx.logger);
    }

    if (method === 'GET' && path.includes('/export/status/')) {
        return handlePollStatus(ctx.userId, path);
    }

    if (method === 'GET' && path.includes('/export/run/')) {
        return handleRunExport(ctx.userId, path);
    }

    if (method === 'POST' && path.includes('/execute/')) {
        return handleExecuteExport(path, req.body as { userId: string }, ctx.logger);
    }

    throw new HttpError(404, 'Not found');
};

// Export the wrapped function
export const dataExportHandler = createCloudFunction(handler, {
    auth: {
        strategies: [
            new FirebaseAuthStrategy(),
            new PayloadUserStrategy(async (payload: unknown) => {
                const body = payload as { userId?: string };
                return body?.userId ?? null;
            }),
        ]
    },
    skipExecutionLogging: true
});
