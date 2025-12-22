import * as admin from 'firebase-admin';
import { Storage } from '@google-cloud/storage';
import { config } from './config';

const db = admin.firestore();
const storage = new Storage({ projectId: config.projectId });

/**
 * Wait for a Firestore document to exist or match a condition
 */
export async function waitForFirestoreDoc(
  collection: string,
  docId: string,
  options: {
    timeout?: number;
    checkInterval?: number;
    condition?: (doc: admin.firestore.DocumentSnapshot) => boolean;
  } = {}
): Promise<admin.firestore.DocumentSnapshot> {
  const timeout = options.timeout || 30000; // 30s default
  const checkInterval = options.checkInterval || 1000; // 1s default
  const condition = options.condition || ((doc) => doc.exists);

  const startTime = Date.now();

  while (Date.now() - startTime < timeout) {
    const doc = await db.collection(collection).doc(docId).get();

    if (condition(doc)) {
      return doc;
    }

    await new Promise((resolve) => setTimeout(resolve, checkInterval));
  }

  throw new Error(
    `Timeout waiting for Firestore document: ${collection}/${docId}`
  );
}

/**
 * Wait for a GCS file to exist
 */
export async function waitForGcsFile(
  bucket: string,
  path: string,
  options: {
    timeout?: number;
    checkInterval?: number;
  } = {}
): Promise<boolean> {
  const timeout = options.timeout || 30000; // 30s default
  const checkInterval = options.checkInterval || 1000; // 1s default

  const startTime = Date.now();
  const bucketObj = storage.bucket(bucket);
  const file = bucketObj.file(path);

  while (Date.now() - startTime < timeout) {
    const [exists] = await file.exists();

    if (exists) {
      return true;
    }

    await new Promise((resolve) => setTimeout(resolve, checkInterval));
  }

  throw new Error(`Timeout waiting for GCS file: gs://${bucket}/${path}`);
}

/**
 * Wait for any activity in Firestore executions collection
 * This is a simple check to verify functions are processing
 */
export async function waitForExecutionActivity(
  options: {
    timeout?: number;
    checkInterval?: number;
    minExecutions?: number;
  } = {}
): Promise<number> {
  const timeout = options.timeout || 30000; // 30s default
  const checkInterval = options.checkInterval || 2000; // 2s default
  const minExecutions = options.minExecutions || 1;

  const startTime = Date.now();

  while (Date.now() - startTime < timeout) {
    const snapshot = await db
      .collection('executions')
      .orderBy('timestamp', 'desc')
      .limit(10)
      .get();

    const recentExecutions = snapshot.docs.filter((doc) => {
      const timestamp = doc.data().timestamp?.toDate();
      if (!timestamp) return false;
      // Check if execution happened in the last minute
      return Date.now() - timestamp.getTime() < 60000;
    });

    if (recentExecutions.length >= minExecutions) {
      return recentExecutions.length;
    }

    await new Promise((resolve) => setTimeout(resolve, checkInterval));
  }

  throw new Error(
    `Timeout waiting for execution activity (expected ${minExecutions} executions)`
  );
}
