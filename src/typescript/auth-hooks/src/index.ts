
import { initializeApp } from 'firebase-admin/app';
import { getFirestore } from 'firebase-admin/firestore';
import { UserService } from '@fitglue/shared/domain/services';

// Initialize Firebase Admin
initializeApp();
import { UserStore, ActivityStore, PipelineStore } from '@fitglue/shared/storage';


// Initialize Firebase Admin
const db = getFirestore();
const userStore = new UserStore(db);
const activityStore = new ActivityStore(db);
const pipelineStore = new PipelineStore(db);
const userService = new UserService(userStore, activityStore, pipelineStore);

/**
 * Cloud Function triggered by Firebase Auth User Creation.
 * Gen 1 trigger: providers/firebase.auth/eventTypes/user.create
 *
 * Note: Gen 1 Firebase Auth triggers pass the UserRecord directly (not as CloudEvent).
 * The UID is available directly on the event object.
 */
export const authOnCreate = async (event: AuthUserRecord) => {
  try {
    // Gen 1 Firebase Auth triggers pass uid directly on the event
    const uid = event.uid;

    if (!uid) {
      console.error('No UID found in event', event);
      return;
    }

    console.log(`Detected new user registration: ${uid}`);

    // Ensure user exists in Firestore
    // UserService.createUser is idempotent (checks existence first)
    await userService.createUser(uid);

    console.log(`Successfully ensured user document for ${uid}`);

  } catch (error) {
    console.error('Error in authOnCreate:', error);
    throw error; // Rethrow to trigger retry if configured
  }
};

// Interface for Gen 1 Firebase Auth User Record
// Matches the structure passed by providers/firebase.auth/eventTypes/user.create trigger
interface AuthUserRecord {
  uid: string;
  email?: string;
  displayName?: string;
}
