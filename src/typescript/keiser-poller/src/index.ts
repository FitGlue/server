import { getSecret, ActivityPayload, ActivitySource, TOPICS, createCloudFunction, FrameworkContext } from '@fitglue/shared';

// NOTE: Since we cannot strictly install the SDK in this environment,
// we interface the expected behavior based on the research doc.
interface KeiserSession {
    id: string;
    userId: string;
    startTime: string; // ISO
    data: any;
}

const TOPIC_NAME = TOPICS.RAW_ACTIVITY;

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
    const { db, logger } = ctx;

    // 1. Fetch Users with Keiser Enabled (Multi-Tenancy)
    const snapshot = await db.collection('users').limit(50).get();

    if (snapshot.empty) {
        logger.info('No users found.');
        res.status(200).json({ status: 'NO_USERS' });
        return { status: 'NO_USERS' };
    }

    let totalSessions = 0;
    const errors: string[] = [];

    // 2. Process Each User
    const userPromises = snapshot.docs.map(async (doc) => {
        const userId = doc.id;

        // Check if Keiser is enabled
        if (!doc.data().integrations?.keiser?.enabled) return;

        try {
            // A. Get Cursor
            const cursorRef = db.collection('cursors').doc(`${userId}_keiser`);
            const cursorSnap = await cursorRef.get();
            let lastSync = new Date(0).toISOString();
            if (cursorSnap.exists) {
                lastSync = cursorSnap.data()!.lastSync;
            }

            // B. Initialize SDK (Secret fetched from Secret Manager)
            const secretName = `keiser-${userId}`;
            const token = await getSecret(process.env.GOOGLE_CLOUD_PROJECT || 'dev-project', secretName);
            logger.info(`Initialized SDK with token for ${secretName}: ${token ? 'FOUND' : 'MISSING'}`);

            // C. Fetch Sessions from Real API
            // Placeholder for real logic to avoid build error on missing SDK
            logger.info(`Polling Keiser for user ${userId} since ${lastSync}`);
            const sessions: KeiserSession[] = []; // Real implementation would populate this

            // D. Push to Pub/Sub
            if (sessions.length > 0) {
                const publishPromises = sessions.map(async (session) => {
                    const payload: ActivityPayload = {
                        source: ActivitySource.SOURCE_KEISER,
                        userId: userId,
                        timestamp: session.startTime,
                        originalPayloadJson: JSON.stringify(session),
                        metadata: {},
                        standardizedActivity: undefined // Todo: Map Keiser to StandardizedActivity
                    };
                    return ctx.pubsub.topic(TOPIC_NAME).publishMessage({ json: payload });
                });
                await Promise.all(publishPromises);
                totalSessions += sessions.length;

                // E. Update Cursor
                const newLastSync = sessions[sessions.length - 1].startTime;
                await cursorRef.set({ lastSync: newLastSync }, { merge: true });
            }

        } catch (err: any) {
            logger.error(`Failed to sync user ${userId}`, { error: err.message });
            errors.push(`${userId}: ${err.message}`);
        }
    });

    await Promise.all(userPromises);

    const result = {
        status: 'Processed',
        usersProcessed: snapshot.size,
        sessionsFound: totalSessions,
        errors: errors
    }

    res.status(200).json(result); // Framework adds x-execution-id

    return result;
};

export const keiserPoller = createCloudFunction(handler);
