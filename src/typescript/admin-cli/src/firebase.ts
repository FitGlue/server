
import * as admin from 'firebase-admin';

// Initialize Firebase if not already initialized
if (admin.apps.length === 0) {
  admin.initializeApp({
    credential: admin.credential.applicationDefault()
  });
}

export const adminDb = admin.firestore();
