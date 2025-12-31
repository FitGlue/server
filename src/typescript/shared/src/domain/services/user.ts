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
            return this.mapFirestoreToUserRecord(doc.data());
        }

        const now = Timestamp.now();
        // Construct using snake_case for DB
        const userStub: any = {
            user_id: userId,
            created_at: now,
            integrations: {}
        };

        await userRef.set(userStub);
        return this.mapFirestoreToUserRecord(userStub);
    }

    async getUser(userId: string): Promise<UserRecord | null> {
        const doc = await this.db.collection('users').doc(userId).get();
        if (!doc.exists) return null;
        return this.mapFirestoreToUserRecord(doc.data());
    }

    /**
     * Maps Firestore snake_case data to UserRecord camelCase interface.
     * Prevents invisible fields issue by acting as strictly typed boundary.
     */
    private mapFirestoreToUserRecord(data: any): UserRecord {
        if (!data) throw new Error('Cannot map null data');

        // Helper to convert Timestamp/string to Date
        const toDate = (ts: any): Date | undefined => {
            if (!ts) return undefined;
            if (ts instanceof Timestamp) return ts.toDate();
            if (ts.seconds) return new Date(ts.seconds * 1000);
            if (typeof ts === 'string') return new Date(ts);
            return undefined;
        };

        const result: UserRecord = {
            userId: data.user_id || data.userId, // Fallback for transition
            createdAt: toDate(data.created_at || data.createdAt),
            integrations: undefined,
            pipelines: []
        };

        // INTEGRATIONS
        if (data.integrations) {
            result.integrations = {
                hevy: undefined,
                fitbit: undefined,
                strava: undefined
            };

            const i = data.integrations;

            // Hevy
            if (i.hevy) {
                result.integrations.hevy = {
                    enabled: !!i.hevy.enabled,
                    apiKey: i.hevy.api_key || i.hevy.apiKey,
                    userId: i.hevy.user_id || i.hevy.userId
                };
            }

            // Strava
            if (i.strava) {
                result.integrations.strava = {
                    enabled: !!i.strava.enabled,
                    accessToken: i.strava.access_token || i.strava.accessToken,
                    refreshToken: i.strava.refresh_token || i.strava.refreshToken,
                    expiresAt: toDate(i.strava.expires_at || i.strava.expiresAt),
                    athleteId: i.strava.athlete_id || i.strava.athleteId
                };
            }

            // Fitbit
            if (i.fitbit) {
                result.integrations.fitbit = {
                    enabled: !!i.fitbit.enabled,
                    accessToken: i.fitbit.access_token || i.fitbit.accessToken,
                    refreshToken: i.fitbit.refresh_token || i.fitbit.refreshToken,
                    expiresAt: toDate(i.fitbit.expires_at || i.fitbit.expiresAt),
                    fitbitUserId: i.fitbit.fitbit_user_id || i.fitbit.fitbitUserId
                };
            }
        }

        // PIPELINES
        if (data.pipelines && Array.isArray(data.pipelines)) {
            result.pipelines = data.pipelines.map((p: any) => ({
                id: p.id,
                source: p.source,
                destinations: p.destinations || [],
                enrichers: (p.enrichers || []).map((e: any) => ({
                    providerType: e.provider_type || e.providerType,
                    inputs: e.inputs || {}
                }))
            }));
        }

        return result;
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

        // 3. Store Hash (using snake_case)
        const now = Timestamp.now();
        const record: any = {
            user_id: userId,
            label,
            scopes,
            created_at: now,
            last_used_at: null
        };

        await this.db.collection('ingress_api_keys').doc(hash).set(record);

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
}
