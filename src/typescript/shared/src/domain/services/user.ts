import { UserStore, ActivityStore } from '../../storage/firestore';
import { UserRecord } from '../../types/pb/user';

/**
 * UserService provides business logic for user operations.
 */
export class UserService {
    constructor(
        private userStore: UserStore,
        private activityStore: ActivityStore
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
     * Load connector configuration for a user.
     */
    async loadConnectorConfig(userId: string, connectorName: string): Promise<any> {
        const user = await this.get(userId);
        if (!user) {
            throw new Error(`User ${userId} not found`);
        }

        const config = (user.integrations as any)?.[connectorName];
        if (!config || !config.enabled) {
            throw new Error(`${connectorName} integration not enabled for user ${userId}`);
        }

        return config;
    }

    /**
   * Get a valid OAuth token for a provider, refreshing if necessary.
   */
    async getValidToken(userId: string, provider: 'strava' | 'fitbit', forceRefresh = false): Promise<string> {
        const user = await this.get(userId);
        if (!user) {
            throw new Error(`User ${userId} not found`);
        }

        const integration = (user.integrations as any)?.[provider];
        if (!integration || !integration.enabled) {
            throw new Error(`${provider} integration not enabled for user ${userId}`);
        }

        const accessToken = integration.accessToken || integration.access_token;
        const expiresAt = integration.expiresAt || integration.expires_at;

        if (!accessToken) {
            throw new Error(`No access token found for ${provider}`);
        }

        // Check if token is expired
        const now = new Date();
        const tokenExpired = expiresAt && new Date(expiresAt) <= now;

        if (tokenExpired || forceRefresh) {
            // TODO: Implement token refresh when refreshOAuthToken is available
            throw new Error(`Token expired for ${provider}. Refresh not yet implemented.`);
        }

        return accessToken;
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
    async markActivityAsProcessed(userId: string, connectorName: string, activityId: string, metadata: { processedAt: Date; source: number }): Promise<void> {
        const scopedId = `${connectorName}_${activityId}`;
        return this.activityStore.markProcessed(userId, scopedId, metadata);
    }
}
