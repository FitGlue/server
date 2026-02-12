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

    /**
     * Get a showcase profile by its owner's user ID.
     */
    async getByUserId(userId: string): Promise<ShowcaseProfile | null> {
        const snap = await this.collection()
            .where('user_id', '==', userId)
            .limit(1)
            .get();
        if (snap.empty) {
            return null;
        }
        const rawData = snap.docs[0].data() as Record<string, unknown>;
        return FirestoreToShowcaseProfile(rawData);
    }

    /**
     * Check if a slug already exists (for collision prevention).
     */
    async exists(slug: string): Promise<boolean> {
        const doc = await this.collection().doc(slug).get();
        return doc.exists;
    }

    /**
     * Partial update of a showcase profile by slug.
     */
    async update(slug: string, data: Partial<Record<string, unknown>>): Promise<void> {
        await this.collection().doc(slug).update(data);
    }

    /**
     * Full write of a showcase profile (used for slug migration).
     */
    async set(slug: string, data: Record<string, unknown>): Promise<void> {
        await this.collection().doc(slug).set(data);
    }

    /**
     * Delete a showcase profile by slug (used during slug migration).
     */
    async delete(slug: string): Promise<void> {
        await this.collection().doc(slug).delete();
    }
}

