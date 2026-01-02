import * as admin from 'firebase-admin';
import * as crypto from 'crypto';
import { UserRecord } from '../../types/pb/user';
import { getUsersCollection, getIngressApiKeysCollection } from '../../storage/firestore';
import { Timestamp } from 'firebase-admin/firestore';

export class UserService {
    // We expect the consumer to have initialized the app (e.g. via framework) or we use lazy loading via `storage`.
    // The explicit constructor with `db` is legacy but we should keep signature if possible or deprecate.
    constructor(private db: admin.firestore.Firestore) { }

    async createUser(userId: string): Promise<UserRecord> {
        const userRef = getUsersCollection().doc(userId);
        const doc = await userRef.get();
        if (doc.exists) {
            return doc.data()!;
        }

        const now = new Date();
        // Typed creation
        const userStub: UserRecord = {
            userId: userId,
            createdAt: now,
            integrations: undefined,
            pipelines: []
        };

        await userRef.set(userStub);
        return userStub;
    }

    async getUser(userId: string): Promise<UserRecord | null> {
        const doc = await getUsersCollection().doc(userId).get();
        if (!doc.exists) return null;
        return doc.data() || null;
    }

    /**
     * Creates an Ingress API Key, hashes it, stores the hash, and returns the plaintext key.
     */
    async createIngressApiKey(userId: string, label: string, scopes: string[]): Promise<string> {
        // 1. Generate Opaque Key: fg_sk_<32_random_bytes_hex>
        const randomBytes = crypto.randomBytes(32).toString('hex');
        const apiKey = `fg_sk_${randomBytes}`;

        // 2. Hash: SHA-256
        const hash = crypto.createHash('sha256').update(apiKey).digest('hex');

        // 3. Store Hash (using typed collection)
        const now = new Date();

        await getIngressApiKeysCollection().doc(hash).set({
            userId,
            label,
            scopes,
            createdAt: now,
            lastUsedAt: undefined
        });

        return apiKey;
    }

    async setHevyIntegration(userId: string, hevyApiKey: string, hevyUserId?: string): Promise<void> {
        const updateStub: any = {
            'integrations.hevy.api_key': hevyApiKey,
            'integrations.hevy.enabled': true
        };
        if (hevyUserId) {
            updateStub['integrations.hevy.user_id'] = hevyUserId;
        }

        await this.db.collection('users').doc(userId).update(updateStub);
    }

    async setStravaIntegration(userId: string, accessToken: string, refreshToken: string, expiresAtSeconds: number, athleteId: number): Promise<void> {
        const expiresAt = Timestamp.fromMillis(expiresAtSeconds * 1000);

        const updateStub: any = {
            'integrations.strava.enabled': true,
            'integrations.strava.access_token': accessToken,
            'integrations.strava.refresh_token': refreshToken,
            'integrations.strava.expires_at': expiresAt,
            'integrations.strava.athlete_id': athleteId
        };

        await this.db.collection('users').doc(userId).update(updateStub);
    }

    async setFitbitIntegration(userId: string, accessToken: string, refreshToken: string, expiresAtSeconds: number, fitbitUserId: string): Promise<void> {
        const expiresAt = Timestamp.fromMillis(expiresAtSeconds * 1000);

        const updateStub: any = {
            'integrations.fitbit.enabled': true,
            'integrations.fitbit.access_token': accessToken,
            'integrations.fitbit.refresh_token': refreshToken,
            'integrations.fitbit.expires_at': expiresAt,
            'integrations.fitbit.fitbit_user_id': fitbitUserId
        };

        await this.db.collection('users').doc(userId).update(updateStub);
    }

    async addPipeline(userId: string, source: string, enrichers: { providerType: number, inputs?: Record<string, string> }[], destinations: string[]): Promise<string> {
        const pipelineId = crypto.randomUUID();
        const pipeline = {
            id: pipelineId,
            source: source,
            enrichers: enrichers.map(e => ({
                provider_type: e.providerType, // snake_case
                inputs: e.inputs || {}
            })),
            destinations: destinations
        };

        await this.db.collection('users').doc(userId).update({
            pipelines: admin.firestore.FieldValue.arrayUnion(pipeline)
        });

        return pipelineId;
    }

    async removePipeline(userId: string, pipelineId: string): Promise<void> {
        const userRef = this.db.collection('users').doc(userId);
        const doc = await userRef.get();
        if (!doc.exists) {
            throw new Error(`User ${userId} not found`);
        }

        const data = doc.data();
        const pipelines = data?.pipelines || [];
        const newPipelines = pipelines.filter((p: any) => p.id !== pipelineId);

        if (pipelines.length === newPipelines.length) {
            throw new Error(`Pipeline ${pipelineId} not found for user ${userId}`);
        }

        await userRef.update({ pipelines: newPipelines });
    }

    async replacePipeline(userId: string, pipelineId: string, source: string, enrichers: { providerType: number, inputs?: Record<string, string> }[], destinations: string[]): Promise<void> {
        const userRef = this.db.collection('users').doc(userId);
        const doc = await userRef.get();
        if (!doc.exists) {
            throw new Error(`User ${userId} not found`);
        }

        const data = doc.data();
        const pipelines = data?.pipelines || [];
        const index = pipelines.findIndex((p: any) => p.id === pipelineId);

        if (index === -1) {
            throw new Error(`Pipeline ${pipelineId} not found for user ${userId}`);
        }

        const newPipeline = {
            id: pipelineId, // Keep same ID
            source: source,
            enrichers: enrichers.map(e => ({
                provider_type: e.providerType,
                inputs: e.inputs || {}
            })),
            destinations: destinations
        };

        const newPipelines = [...pipelines];
        newPipelines[index] = newPipeline;

        await userRef.update({ pipelines: newPipelines });
    }

    /**
     * Returns a valid access token for the given provider.
     * Refreshes the token if expired or expiring soon.
     * @param userId The User ID
     * @param provider The provider name ('strava' or 'fitbit')
     * @param forceRefresh If true, forces a refresh regardless of expiry (e.g. after a 401)
     */
    async getValidToken(userId: string, provider: 'strava' | 'fitbit', forceRefresh = false): Promise<string> {
        // We import refreshOAuthToken dynamically to avoid circular dependencies if any,
        // essentially just keeping it clean. But `oauth.ts` is in same package.
        // Actually UserService -> oauth.ts is fine.
        const { refreshOAuthToken } = await import('../../infrastructure/oauth');

        const userDoc = await this.db.collection('users').doc(userId).get();
        if (!userDoc.exists) {
            throw new Error(`User ${userId} not found`);
        }

        const userData = userDoc.data();
        const integration = userData?.integrations?.[provider];

        if (!integration || !integration.enabled) {
            throw new Error(`Integration ${provider} not enabled for user ${userId}`);
        }

        const { access_token, refresh_token, expires_at } = integration;

        // Check Expiry
        let expiryDate: Date;
        if (expires_at instanceof Timestamp) {
            expiryDate = expires_at.toDate();
        } else if (typeof expires_at === 'string') {
            expiryDate = new Date(expires_at); // Should probably use Timestamp but being safe
        } else {
            // Assume it's a date or timestamp-like object or missing
            expiryDate = new Date(expires_at.seconds * 1000); // Protobuf style?
            // We'll trust Firestore/Timestamp mostly.
        }

        // Proactive Refresh: Refresh if expiring in next 2 minutes
        const expiringSoon = new Date(Date.now() + 2 * 60 * 1000) > expiryDate;

        if (forceRefresh || expiringSoon) {
            console.log(`Refreshing ${provider} token for user ${userId} (Force: ${forceRefresh}, Expiring: ${expiringSoon})`);

            try {
                const newTokens = await refreshOAuthToken(provider, refresh_token);

                // Update DB with snake_case
                if (provider === 'strava') {
                    await this.db.collection('users').doc(userId).update({
                        'integrations.strava.access_token': newTokens.accessToken,
                        'integrations.strava.refresh_token': newTokens.refreshToken,
                        'integrations.strava.expires_at': Timestamp.fromDate(newTokens.expiresAt)
                    });
                } else {
                    await this.db.collection('users').doc(userId).update({
                        'integrations.fitbit.access_token': newTokens.accessToken,
                        'integrations.fitbit.refresh_token': newTokens.refreshToken,
                        'integrations.fitbit.expires_at': Timestamp.fromDate(newTokens.expiresAt)
                    });
                }

                return newTokens.accessToken;
            } catch (err) {
                throw new Error(`Failed to refresh token for ${provider}: ${err}`);
            }
        }

        return access_token;
    }
    /**
     * Checks if a raw activity has already been processed for the user.
     * @param userId The User ID
     * @param source The source provider (e.g. 'fitbit')
     * @param activityId The unique activity ID from the provider (e.g. logId)
     */
    async hasProcessedActivity(userId: string, source: string, activityId: string): Promise<boolean> {
        const id = `${source}_${activityId}`;
        const doc = await this.db.collection('users').doc(userId).collection('raw_activities').doc(id).get();
        return doc.exists;
    }

    /**
     * Marks a raw activity as processed to prevent duplicate ingestion.
     * @param userId The User ID
     * @param source The source provider
     * @param activityId The unique activity ID
     */
    async markActivityAsProcessed(userId: string, source: string, activityId: string): Promise<void> {
        const id = `${source}_${activityId}`;
        const now = Timestamp.now();
        await this.db.collection('users').doc(userId).collection('raw_activities').doc(id).set({
            source,
            external_id: activityId, // snake_case
            processed_at: now
        });
    }
}
