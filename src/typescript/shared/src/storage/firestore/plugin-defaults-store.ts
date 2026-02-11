import * as admin from 'firebase-admin';
import { PluginDefault } from '../../types/pb/user';

/**
 * Firestore converter for plugin default documents.
 * Maps between PluginDefault (camelCase) and Firestore (snake_case).
 */
export const pluginDefaultConverter: admin.firestore.FirestoreDataConverter<PluginDefault> = {
    toFirestore(model: PluginDefault): admin.firestore.DocumentData {
        const data: admin.firestore.DocumentData = {
            plugin_id: model.pluginId,
            config: model.config || {},
        };
        if (model.createdAt) {
            data.created_at = model.createdAt;
        }
        if (model.updatedAt) {
            data.updated_at = model.updatedAt;
        }
        return data;
    },

    fromFirestore(snapshot: admin.firestore.QueryDocumentSnapshot): PluginDefault {
        const data = snapshot.data();
        return {
            pluginId: (data.plugin_id as string) || snapshot.id,
            config: (data.config || {}) as Record<string, string>,
            createdAt: data.created_at?.toDate?.() ?? undefined,
            updatedAt: data.updated_at?.toDate?.() ?? undefined,
        };
    }
};

/**
 * PluginDefaultsStore provides typed access to plugin defaults sub-collection.
 * Plugin defaults are stored at: users/{userId}/plugin_defaults/{pluginId}
 */
export class PluginDefaultsStore {
    constructor(private db: admin.firestore.Firestore) { }

    private collection(userId: string) {
        return this.db
            .collection('users')
            .doc(userId)
            .collection('plugin_defaults')
            .withConverter(pluginDefaultConverter);
    }

    /**
     * Get a single plugin default by plugin ID.
     */
    async get(userId: string, pluginId: string): Promise<PluginDefault | null> {
        const doc = await this.collection(userId).doc(pluginId).get();
        return doc.exists ? doc.data() || null : null;
    }

    /**
     * List all plugin defaults for a user.
     */
    async list(userId: string): Promise<PluginDefault[]> {
        const snapshot = await this.collection(userId).get();
        return snapshot.docs.map(doc => doc.data());
    }

    /**
     * Set (create or update) a plugin default.
     * Uses the pluginId (registry ID) as document ID.
     */
    async set(userId: string, pluginDefault: PluginDefault): Promise<void> {
        await this.collection(userId).doc(pluginDefault.pluginId).set(pluginDefault);
    }

    /**
     * Set a plugin default only if one doesn't already exist.
     * Returns true if a new default was created, false if one already existed.
     */
    async setIfNotExists(userId: string, pluginDefault: PluginDefault): Promise<boolean> {
        const existing = await this.get(userId, pluginDefault.pluginId);
        if (existing) {
            return false;
        }
        await this.set(userId, pluginDefault);
        return true;
    }

    /**
     * Delete a plugin default.
     */
    async delete(userId: string, pluginId: string): Promise<void> {
        await this.collection(userId).doc(pluginId).delete();
    }
}
