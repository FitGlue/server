
import { initializeApp } from 'firebase-admin/app';
import { getFirestore, Timestamp } from 'firebase-admin/firestore';
import { UserService } from '@fitglue/shared/domain/services';

// Initialize Firebase Admin
initializeApp();
import { UserStore, ActivityStore, PipelineStore, PluginDefaultsStore, ShowcaseProfileStore } from '@fitglue/shared/storage';


// Initialize Firebase Admin
const db = getFirestore();
const userStore = new UserStore(db);
const activityStore = new ActivityStore(db);
const pipelineStore = new PipelineStore(db);
const pluginDefaultsStore = new PluginDefaultsStore(db);
const showcaseProfileStore = new ShowcaseProfileStore(db);
const userService = new UserService(userStore, activityStore, pipelineStore, pluginDefaultsStore);

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

    // Create a base showcase profile so the management page is accessible immediately.
    // Slug is the first 8 chars of the uid (unique since UIDs are unique).
    // Profile starts hidden (visible: false) until the user opts in.
    try {
      const slug = uid.substring(0, 8).toLowerCase();
      const now = Timestamp.now();

      await showcaseProfileStore.set(slug, {
        slug,
        user_id: uid,
        display_name: '',
        entries: [],
        total_activities: 0,
        total_distance_meters: 0,
        total_duration_seconds: 0,
        total_sets: 0,
        total_reps: 0,
        total_weight_kg: 0,
        subtitle: '',
        bio: '',
        profile_picture_url: '',
        visible: false,
        created_at: now,
        updated_at: now,
      });

      console.log(`Created base showcase profile for ${uid} with slug: ${slug}`);
    } catch (showcaseError) {
      // Non-fatal: user document is the critical path
      console.error('Failed to create base showcase profile (non-fatal):', showcaseError);
    }

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
