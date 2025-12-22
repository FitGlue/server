import * as admin from 'firebase-admin';
import { config } from './config';

const PROJECT_ID = config.projectId;

if (!admin.apps.length) {
  admin.initializeApp({
    projectId: PROJECT_ID,
  });
}

const db = admin.firestore();

import * as crypto from 'crypto';

export const setupTestUser = async (userId: string) => {
  console.log(`[Setup] Creating test user: ${userId}`);
  await db.collection('users').doc(userId).set({
    created_at: new Date(),
    strava_enabled: true,
    strava_access_token: 'valid_mock_token',
    strava_refresh_token: 'mock_refresh',
    strava_expires_at: admin.firestore.Timestamp.fromDate(new Date(Date.now() + 3600000)),
    integrations: {
        hevy: {
            apiKey: 'mock-hevy-api-key-123', // For Active Fetch (Egrees)
            enabled: true
        }
    }
  });
};

export const setupTestApiKey = async (userId: string): Promise<string> => {
    // Generate Opaque Token
    const randomBytes = crypto.randomBytes(32).toString('hex');
    const token = `fg_sk_${randomBytes}`;

    // Hash
    const hash = crypto.createHash('sha256').update(token).digest('hex');

    // Store with test scope
    await db.collection('ingress_api_keys').doc(hash).set({
        userId,
        label: 'Integration Test Key',
        scopes: ['test:mock_fetch'], // REQUIRED for mock fetch
        createdAt: admin.firestore.Timestamp.now(),
        lastUsed: null
    });
    console.log(`[Setup] Created API Key for ${userId}`);

    return token;
};
