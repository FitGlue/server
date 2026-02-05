import * as admin from 'firebase-admin';
import { UserStore, ActivityStore, PipelineStore } from '../../storage/firestore';
import { UserRecord, UserIntegrations, ProcessedActivityRecord, UserTier } from '../../types/pb/user';
import { FirestoreTokenSource, OAuthProvider } from '../../infrastructure/oauth/token-source';

/**
 * UserService provides business logic for user operations.
 */
export class UserService {

    constructor(
        private userStore: UserStore,
        private activityStore: ActivityStore,
        public readonly pipelineStore: PipelineStore
    ) { }

    /**
     * Get a user by ID.
     */
    async get(userId: string): Promise<UserRecord | null> {
        return this.userStore.get(userId);
    }

    /**
     * Find a user by Fitbit ID.
     */
    async findByFitbitId(fitbitUserId: string): Promise<{ id: string; data: UserRecord } | null> {
        return this.userStore.findByFitbitId(fitbitUserId);
    }

    /**
     * Find a user by Strava Athlete ID.
     */
    async findByStravaId(athleteId: number): Promise<{ id: string; data: UserRecord } | null> {
        return this.userStore.findByStravaId(athleteId);
    }

    /**
     * Find a user by Polar User ID.
     */
    async findByPolarId(polarUserId: string): Promise<{ id: string; data: UserRecord } | null> {
        return this.userStore.findByPolarId(polarUserId);
    }

    /**
     * Find a user by Oura User ID.
     */
    async findByOuraId(ouraUserId: string): Promise<{ id: string; data: UserRecord } | null> {
        return this.userStore.findByOuraId(ouraUserId);
    }


    /**
     * Load connector configuration for a user.
     */
    async loadConnectorConfig(userId: string, connectorName: string): Promise<Record<string, unknown>> {
        const user = await this.get(userId);
        if (!user) {
            throw new Error(`User ${userId} not found`);
        }

        const config = (user.integrations as Record<string, Record<string, unknown>>)?.[connectorName];
        if (!config || !config.enabled) {
            throw new Error(`${connectorName} integration not enabled for user ${userId}`);
        }

        return config;
    }

    /**
     * Get a valid OAuth token for a provider, refreshing if necessary.
     */
    async getValidToken(userId: string, provider: OAuthProvider, forceRefresh = false): Promise<string> {
        const tokenSource = new FirestoreTokenSource(this.userStore, userId, provider);
        const token = await tokenSource.getToken(forceRefresh);
        return token.accessToken;
    }

    /**
     * Get OAuth tokens for a user integration (raw access).
     */
    async getOAuthTokens(userId: string, provider: string): Promise<Record<string, unknown> | null> {
        const user = await this.get(userId);
        if (!user || !user.integrations) return null;
        return (user.integrations as Record<string, unknown>)[provider] as Record<string, unknown> | undefined || null;
    }


    /**
     * Check if an activity has been processed for a user.
     * Activities are stored in users/{userId}/raw_activities subcollection.
     * Activity IDs are scoped by connector to prevent clashes: {connectorName}_{activityId}
     */
    async hasProcessedActivity(userId: string, connectorName: string, activityId: string): Promise<boolean> {
        const scopedId = `${connectorName}_${activityId}`;
        return this.activityStore.isProcessed(userId, scopedId);
    }

    /**
     * Mark an activity as processed for a user.
     * Activity IDs are scoped by connector: {connectorName}_{activityId}
     */
    async markActivityAsProcessed(userId: string, connectorName: string, activityId: string, metadata: { processedAt: Date; source: string; externalId: string }): Promise<void> {
        const scopedId = `${connectorName}_${activityId}`;
        return this.activityStore.markProcessed(userId, scopedId, {
            source: metadata.source,
            externalId: metadata.externalId,
            processedAt: metadata.processedAt
        });
    }

    /**
     * Create or ensure a user exists.
     * New users get a 30-day Pro trial.
     */
    async createUser(userId: string): Promise<void> {
        const now = new Date();
        const trialEndsAt = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000); // 30 days

        await this.userStore.create(userId, {
            userId: userId,
            createdAt: now,
            integrations: {} as UserIntegrations,
            fcmTokens: [],
            // Initialize with 30-day Pro trial
            tier: UserTier.USER_TIER_ATHLETE,
            trialEndsAt: trialEndsAt,
            isAdmin: false,
            syncCountThisMonth: 0,
            preventedSyncCount: 0,
            syncCountResetAt: now,
            stripeCustomerId: '', // Will be set when user subscribes
            accessEnabled: false, // Waitlisted until admin enables
        });
    }



    /**
     * Set Hevy integration for a user.
     */
    async setIntegration<K extends keyof UserIntegrations>(
        userId: string,
        key: K,
        data: UserIntegrations[K]
    ): Promise<void> {
        return this.userStore.setIntegration(userId, key, data);
    }

    async setHevyIntegration(userId: string, apiKey: string): Promise<void> {
        await this.setIntegration(userId, 'hevy', {
            enabled: true,
            apiKey: apiKey,
            userId: userId,
            createdAt: new Date(),
            lastUsedAt: new Date()
        });
    }

    async setStravaIntegration(userId: string, accessToken: string, refreshToken: string, expiresAt: number, athleteId: number): Promise<void> {
        await this.setIntegration(userId, 'strava', {
            enabled: true,
            accessToken,
            refreshToken,
            expiresAt: new Date(expiresAt * 1000),
            athleteId,
            createdAt: new Date(),
            lastUsedAt: new Date()
        });
    }

    async setFitbitIntegration(userId: string, accessToken: string, refreshToken: string, expiresAt: number, fitbitUserId: string): Promise<void> {
        await this.setIntegration(userId, 'fitbit', {
            enabled: true,
            accessToken,
            refreshToken,
            expiresAt: new Date(expiresAt * 1000),
            fitbitUserId,
            createdAt: new Date(),
            lastUsedAt: new Date()
        });
    }

    async setMockIntegration(userId: string, enabled: boolean): Promise<void> {
        await this.setIntegration(userId, 'mock', {
            enabled,
            createdAt: new Date(),
            lastUsedAt: new Date()
        });
    }

    async updateLastUsed(userId: string, provider: string): Promise<void> {
        return this.userStore.updateLastUsed(userId, provider);
    }

    async getUser(userId: string): Promise<UserRecord | null> {
        return this.get(userId);
    }

    async listUsers(): Promise<UserRecord[]> {
        return this.userStore.list();
    }

    async deleteUser(userId: string): Promise<void> {
        // Delete Firebase Auth user FIRST to prevent orphan auth users
        // If this fails, the user can still login and retry deletion
        // If Firestore deletion fails after Auth deletion, cleanup is harder
        // but at least the user can't login as an orphan
        await admin.auth().deleteUser(userId);

        // Then delete Firestore user document
        return this.userStore.delete(userId);
    }

    async deleteAllUsers(): Promise<number> {
        return this.userStore.deleteAll();
    }

    async listProcessedActivities(userId: string): Promise<ProcessedActivityRecord[]> {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        return this.activityStore.list(userId);
    }

    async deleteProcessedActivity(userId: string, activityId: string): Promise<void> {
        return this.activityStore.delete(userId, activityId);
    }

    /**
     * Check if an incoming activity is a "bounceback" from our own upload.
     * Used for source-level loop prevention - when a destination sends a webhook,
     * we check if we recently uploaded an activity with this destination + destinationId.
     *
     * @param userId - User to check
     * @param destination - The destination enum value (e.g., Destination.DESTINATION_HEVY)
     * @param destinationId - The external ID from the webhook (which IS the destination's ID)
     * @returns true if this activity was uploaded by us (should skip processing)
     */
    async isBounceback(userId: string, destination: number, destinationId: string): Promise<boolean> {
        return this.activityStore.isBounceback(userId, destination, destinationId);
    }

}

