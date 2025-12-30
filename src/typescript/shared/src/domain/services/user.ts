import * as admin from 'firebase-admin';
import * as crypto from 'crypto';
import { UserRecord } from '../../types/pb/user';

import { Timestamp } from 'firebase-admin/firestore';

export class UserService {
    constructor(private db: admin.firestore.Firestore) { }

    async createUser(userId: string): Promise<UserRecord> {
        const userRef = this.db.collection('users').doc(userId);
        const doc = await userRef.get();
        if (doc.exists) {
            // Return existing? Or throw? For now just return it
            return doc.data() as UserRecord;
        }

        const now = Timestamp.now();
        // Construct using Proto shape. Note: Firestore stores protobuf timestamps as maps usually unless converted.
        // We'll stick to Firestore native Timestamp for storage if possible, or convert.
        // The generated types expect google.protobuf.Timestamp structure { seconds, nanos }.
        // Firestore client usually handles conversion if we pass plain objects.

        // Let's create the record
        const user: any = { // Using any cast to bypass strict Proto type vs Firestore type mismatch on Timestamp field initially
            userId: userId,
            createdAt: now,
            integrations: {}
        };

        await userRef.set(user);
        return user as UserRecord;
    }

    async getUser(userId: string): Promise<UserRecord | null> {
        const doc = await this.db.collection('users').doc(userId).get();
        if (!doc.exists) return null;
        return doc.data() as UserRecord;
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

        // 3. Store Hash
        const now = Timestamp.now();
        const record: any = {
            userId,
            label,
            scopes,
            createdAt: now,
            lastUsedAt: null
        };

        await this.db.collection('ingress_api_keys').doc(hash).set(record);

        return apiKey;
    }

    async setHevyIntegration(userId: string, hevyApiKey: string, hevyUserId?: string): Promise<void> {
        const updateStub: any = {
            'integrations.hevy.apiKey': hevyApiKey,
            'integrations.hevy.enabled': true
        };
        if (hevyUserId) {
            updateStub['integrations.hevy.userId'] = hevyUserId;
        }

        await this.db.collection('users').doc(userId).update(updateStub);
    }

    async setStravaIntegration(userId: string, accessToken: string, refreshToken: string, expiresAtSeconds: number, athleteId: number): Promise<void> {
        // Convert seconds to Firestore Timestamp (which is what we store, usually)
        // Or if we store raw seconds? Proto uses google.protobuf.Timestamp { seconds, nanos }
        // Firestore client accepts Date objects or Timestamp objects.
        const expiresAt = Timestamp.fromMillis(expiresAtSeconds * 1000);

        const updateStub: any = {
            'integrations.strava.enabled': true,
            'integrations.strava.accessToken': accessToken,
            'integrations.strava.refreshToken': refreshToken,
            'integrations.strava.expiresAt': expiresAt,
            'integrations.strava.athleteId': athleteId
        };

        await this.db.collection('users').doc(userId).update(updateStub);
    }

    async setFitbitIntegration(userId: string, accessToken: string, refreshToken: string, expiresAtSeconds: number, fitbitUserId: string): Promise<void> {
        const expiresAt = Timestamp.fromMillis(expiresAtSeconds * 1000);

        const updateStub: any = {
            'integrations.fitbit.enabled': true,
            'integrations.fitbit.accessToken': accessToken,
            'integrations.fitbit.refreshToken': refreshToken,
            'integrations.fitbit.expiresAt': expiresAt,
            'integrations.fitbit.fitbitUserId': fitbitUserId
        };

        await this.db.collection('users').doc(userId).update(updateStub);
    }

    async addPipeline(userId: string, source: string, enrichers: { providerType: number, inputs?: Record<string, string> }[], destinations: string[]): Promise<string> {
        const pipelineId = crypto.randomUUID();
        const pipeline = {
            id: pipelineId,
            source: source, // e.g. "SOURCE_HEVY"
            enrichers: enrichers.map(e => ({
                providerType: e.providerType,
                inputs: e.inputs || {}
            })),
            destinations: destinations // e.g. ["strava"]
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
                providerType: e.providerType,
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

                // Update DB
                if (provider === 'strava') {
                    // We don't have athleteId handy here to reuse setStravaIntegration nicely without fetching validation
                    // So we just patch fields.
                    await this.db.collection('users').doc(userId).update({
                        'integrations.strava.accessToken': newTokens.accessToken,
                        'integrations.strava.refreshToken': newTokens.refreshToken,
                        'integrations.strava.expiresAt': Timestamp.fromDate(newTokens.expiresAt)
                    });
                } else {
                    await this.db.collection('users').doc(userId).update({
                        'integrations.fitbit.accessToken': newTokens.accessToken,
                        'integrations.fitbit.refreshToken': newTokens.refreshToken,
                        'integrations.fitbit.expiresAt': Timestamp.fromDate(newTokens.expiresAt)
                    });
                }

                return newTokens.accessToken;
            } catch (err) {
                throw new Error(`Failed to refresh token for ${provider}: ${err}`);
            }
        }

        return access_token;
    }
}
