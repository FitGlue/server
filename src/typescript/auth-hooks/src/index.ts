import { CloudEvent } from 'cloudevents';
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
 * Eventarc Logic: google.firebase.auth.user.v1.created
 */
export const authOnCreate = async (cloudEvent: CloudEvent<AuthUserData>) => {
  try {
    const { subject } = cloudEvent;
    const uid = subject?.replace('users/', '');

    if (!uid) {
      console.error('No UID found in CloudEvent subject', cloudEvent);
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

// Partial interface for the Auth CloudEvent data
// We mainly care about the UID which is in the subject, but data contains more info
interface AuthUserData {
  uid: string;
  email?: string;
  displayName?: string;
}
