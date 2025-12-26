import {
    ActivityPayload,
    ActivitySource,
    TOPICS,
    createCloudFunction,
    FrameworkContext,
    decryptCredentials,
} from '@fitglue/shared';
import Metrics from '@keiser/metrics-sdk';

const TOPIC_NAME = TOPICS.RAW_ACTIVITY;
const TOKEN_VALIDITY_DAYS = 90;

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
    const { db, logger, pubsub } = ctx;
    const projectId = process.env.GOOGLE_CLOUD_PROJECT!;

    // Query users with Keiser enabled
    const snapshot = await db
        .collection('users')
        .where('integrations.keiser.enabled', '==', true)
        .limit(50)
        .get();

    if (snapshot.empty) {
        logger.info('No users with Keiser integration found.');
        res.status(200).json({ status: 'NO_USERS' });
        return { status: 'NO_USERS' };
    }

    let totalSessions = 0;
    const errors: string[] = [];
    const skippedUsers: string[] = [];

    // Process each user
    const userPromises = snapshot.docs.map(async (doc) => {
        const userId = doc.id;
        const userData = doc.data();
        const keiserIntegration = userData.integrations?.keiser;

        if (!keiserIntegration?.enabled) return;

        // Skip users requiring re-authentication
        if (keiserIntegration.requiresReauth) {
            logger.warn(`User ${userId} requires re-authentication, skipping`);
            skippedUsers.push(userId);
            return;
        }

        try {
            // Get cursor
            const cursorRef = db.collection('cursors').doc(`${userId}_keiser`);
            const cursorSnap = await cursorRef.get();
            let lastSync = new Date(0);
            if (cursorSnap.exists) {
                lastSync = new Date(cursorSnap.data()!.lastSync);
            }

            // Initialize Keiser SDK
            const metrics = new Metrics();
            let userSession;

            try {
                // Try refresh token first
                userSession = await metrics.authenticateWithToken({
                    token: keiserIntegration.refreshToken,
                });
                logger.info(`Authenticated user ${userId} with refresh token`);
            } catch (err: any) {
                // Token expired - handle based on auth provider
                logger.warn(`Refresh token expired for user ${userId}, provider: ${keiserIntegration.authProvider}`);

                if (keiserIntegration.authProvider === 'email') {
                    // Auto-refresh using decrypted credentials
                    logger.info(`Re-authenticating user ${userId} with email/password`);

                    const credentials = await decryptCredentials(
                        keiserIntegration.encryptedCredentials,
                        projectId
                    );

                    userSession = await metrics.authenticateWithCredentials({
                        email: credentials.email,
                        password: credentials.password,
                    });

                    // Update refresh token in Firestore
                    const expiresAt = new Date(Date.now() + TOKEN_VALIDITY_DAYS * 24 * 60 * 60 * 1000);
                    await db.collection('users').doc(userId).update({
                        'integrations.keiser.refreshToken': userSession.refreshToken,
                        'integrations.keiser.refreshTokenExpiresAt': expiresAt,
                    });

                    logger.info(`Updated refresh token for user ${userId}`);
                } else {
                    // OAuth - cannot auto-refresh, mark as requiring re-auth
                    logger.warn(`OAuth token expired for user ${userId}, marking requiresReauth`);
                    await db.collection('users').doc(userId).update({
                        'integrations.keiser.requiresReauth': true,
                    });
                    skippedUsers.push(userId);
                    return;
                }
            }

            // Fetch M Series bike sessions since last sync
            logger.info(`Fetching Keiser M Series sessions for user ${userId} since ${lastSync.toISOString()}`);

            const mSeriesSessions = await userSession.getMSeriesDataSets({
                from: lastSync,
                sort: 'startedAt',
                ascending: true,
                limit: 100,
            });

            // Publish sessions to Pub/Sub
            if (mSeriesSessions.length > 0) {
                const publishPromises = mSeriesSessions.map(async (session) => {
                    const payload: ActivityPayload = {
                        source: ActivitySource.SOURCE_KEISER,
                        userId: userId,
                        timestamp: session.startedAt?.toISOString() || new Date().toISOString(),
                        originalPayloadJson: JSON.stringify({
                            id: session.id,
                            startedAt: session.startedAt,
                            endedAt: session.endedAt,
                            ordinalId: session.ordinalId,
                            // Include relevant session data
                            data: session,
                        }),
                        metadata: {
                            sessionId: session.id?.toString() || '',
                            ordinalId: session.ordinalId?.toString() || '',
                        },
                        standardizedActivity: undefined, // TODO: Map Keiser M Series to StandardizedActivity
                    };
                    return pubsub.topic(TOPIC_NAME).publishMessage({ json: payload });
                });

                await Promise.all(publishPromises);
                totalSessions += mSeriesSessions.length;

                // Update cursor
                const lastSession = mSeriesSessions[mSeriesSessions.length - 1];
                const newLastSync = lastSession.startedAt || new Date();
                await cursorRef.set({ lastSync: newLastSync.toISOString() }, { merge: true });
                logger.info(`Published ${mSeriesSessions.length} M Series sessions for user ${userId}`);
            } else {
                logger.info(`No new M Series sessions for user ${userId}`);
            }
        } catch (err: any) {
            logger.error(`Failed to sync user ${userId}`, { error: err.message, stack: err.stack });
            errors.push(`${userId}: ${err.message}`);
        }
    });

    await Promise.all(userPromises);

    const result = {
        status: 'COMPLETED',
        usersProcessed: snapshot.size,
        sessionsFound: totalSessions,
        skippedUsers: skippedUsers.length,
        errors: errors.length,
        errorDetails: errors,
    };

    logger.info('Keiser M Series polling completed', result);
    res.status(200).json(result);
    return result;
};

export const keiserPoller = createCloudFunction(handler);
