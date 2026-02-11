import * as admin from 'firebase-admin';
import { FirestoreToShowcaseProfile } from './converters';
import type { ShowcaseProfile } from '../../types/pb/user';

/**
 * ShowcaseProfileStore provides typed access to showcase profile operations.
 * Showcase profiles are materialized documents for a user's public homepage.
 */
export class ShowcaseProfileStore {
    constructor(private db: admin.firestore.Firestore) { }

    /**
     * Get the showcase profiles collection.
     */
    private collection() {
        return this.db.collection('showcase_profiles');
    }

    /**
     * Get a showcase profile by its URL slug.
     */
    async get(slug: string): Promise<ShowcaseProfile | null> {
        const doc = await this.collection().doc(slug).get();
        if (!doc.exists) {
            return null;
        }
        const rawData = doc.data() as Record<string, unknown>;
        return FirestoreToShowcaseProfile(rawData);
    }
}
