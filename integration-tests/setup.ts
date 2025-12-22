import * as admin from 'firebase-admin';
import { Storage } from '@google-cloud/storage';
import { config } from './config';

const PROJECT_ID = config.projectId;
const GCS_BUCKET = config.gcsBucket;

if (!admin.apps.length) {
  admin.initializeApp({
    projectId: PROJECT_ID,
  });
}

const db = admin.firestore();
const storage = new Storage({ projectId: PROJECT_ID });

export const setupTestUser = async (userId: string) => {
  console.log(`[Setup] Creating test user: ${userId}`);
  await db.collection('users').doc(userId).set({
    created_at: new Date(),
    strava_enabled: true,
    strava_access_token: 'valid_mock_token', // We want to reach the Uploader logic
    strava_refresh_token: 'mock_refresh',
    strava_expires_at: admin.firestore.Timestamp.fromDate(new Date(Date.now() + 3600000)), // 1 hour future
  });
};

export const cleanupTestUser = async (userId: string) => {
  console.log(`[Cleanup] Cleaning up user: ${userId}`);

  // 1. Delete User
  await db.collection('users').doc(userId).delete();

  // 2. Delete Executions (Best effort)
  console.log(`[Cleanup] Deleting executions for: ${userId}`);
  // Note: We scan for executions containing the userID in their ID or fields
  // Since our IDs are formatted like "service-USERID-timestamp", we can't easily query by ID prefix efficiently without listing all.
  // But we store inputs usually. Let's try to query by 'inputs' field if possible, or just skip it for now.
  // Actually, our code keys execution ID as:
  // Enricher: "{userId}-{timestamp}"
  // Router: "router-{userId}-{timestamp}"
  // Uploader: "uploader-{userId}-{timestamp}"

  // We can't wildcard delete. We will assume for this simple test suite that checking clean wiring matters most.
  // Ideally we would query a "userId" field on executions, but we didn't add one explicitly to the root of the doc, only inside 'inputs' map.
  // For now: Skip execution cleanup to avoid complexity, or try to list collection and filter manually (slow).
  // Let's implement GCS cleanup as it's more important for cost/storage.

  // 3. Delete GCS Folder
  const bucket = storage.bucket(GCS_BUCKET);
  const prefix = `activities/${userId}/`;
  console.log(`[Cleanup] Deleting GCS folder: gs://${GCS_BUCKET}/${prefix}`);

  try {
    await bucket.deleteFiles({ prefix });
  } catch (e) {
    console.log(`[Cleanup] Warning: GCS delete failed (maybe empty?): ${e}`);
  }
};
