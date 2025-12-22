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
    strava_access_token: 'valid_mock_token',
    strava_refresh_token: 'mock_refresh',
    strava_expires_at: admin.firestore.Timestamp.fromDate(new Date(Date.now() + 3600000)),
  });
};

export const cleanupTestUser = async (userId: string) => {
  console.log(`[Cleanup] Cleaning up user: ${userId}`);

  // Delete user document
  await db.collection('users').doc(userId).delete();

  // Delete execution records for this user
  console.log(`[Cleanup] Deleting executions for: ${userId}`);
  try {
    const executionsSnapshot = await db
      .collection('executions')
      .where('user_id', '==', userId)
      .get();

    const deletePromises = executionsSnapshot.docs.map(doc => doc.ref.delete());
    await Promise.all(deletePromises);
    console.log(`[Cleanup] Deleted ${executionsSnapshot.size} execution records`);
  } catch (e) {
    console.log(`[Cleanup] Warning: Execution cleanup failed: ${e}`);
  }

  // Delete GCS artifacts for this test user
  const bucket = storage.bucket(GCS_BUCKET);
  const prefix = `activities/${userId}/`;
  console.log(`[Cleanup] Deleting GCS folder: gs://${GCS_BUCKET}/${prefix}`);

  try {
    await bucket.deleteFiles({ prefix });
  } catch (e) {
    console.log(`[Cleanup] Warning: GCS delete failed (maybe empty?): ${e}`);
  }
};
