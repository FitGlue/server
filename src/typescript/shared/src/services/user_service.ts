import * as admin from 'firebase-admin';
import * as crypto from 'crypto';
import { UserRecord } from '../types/pb/user';

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
}
