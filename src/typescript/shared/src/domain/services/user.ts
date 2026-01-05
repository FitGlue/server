import { UserStore, ActivityStore } from '../../storage/firestore';
import { UserRecord, UserIntegrations, EnricherConfig, ProcessedActivityRecord } from '../../types/pb/user';
import { FirestoreTokenSource } from '../../infrastructure/oauth/token-source';

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
    async getValidToken(userId: string, provider: 'strava' | 'fitbit', forceRefresh = false): Promise<string> {
        const tokenSource = new FirestoreTokenSource(this.userStore, userId, provider);
        const token = await tokenSource.getToken(forceRefresh);
        return token.accessToken;
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
     */
    async createUser(userId: string): Promise<void> {
        await this.userStore.create(userId, {
            userId: userId,
            createdAt: new Date(),
            integrations: {} as UserIntegrations,
            pipelines: [],
            fcmTokens: []
        });
    }

    /**
     * Set Hevy integration for a user.
     */
    async setHevyIntegration(userId: string, apiKey: string): Promise<void> {
        await this.userStore.setIntegration(userId, 'hevy', {
            enabled: true,
            apiKey: apiKey,
            userId: userId // Enforcing userId requirement from HevyIntegration interface
        });
    }

    async setStravaIntegration(userId: string, accessToken: string, refreshToken: string, expiresAt: number, athleteId: number): Promise<void> {
        await this.userStore.setIntegration(userId, 'strava', {
            enabled: true,
            accessToken,
            refreshToken,
            expiresAt: new Date(expiresAt * 1000), // Convert to Date
            athleteId
        });
    }

    async setFitbitIntegration(userId: string, accessToken: string, refreshToken: string, expiresAt: number, fitbitUserId: string): Promise<void> {
        await this.userStore.setIntegration(userId, 'fitbit', {
            enabled: true,
            accessToken,
            refreshToken,
            expiresAt: new Date(expiresAt * 1000), // Convert to Date
            fitbitUserId,
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

    // Pipeline methods (legacy support)
    async addPipeline(userId: string, source: string, enrichers: EnricherConfig[], destinations: string[]): Promise<string> {
        const id = `pipe_${Date.now()}`;
        await this.userStore.addPipeline(userId, {
            id, source, enrichers, destinations
        });
        return id;
    }

    async removePipeline(userId: string, pipelineId: string): Promise<void> {
        const user = await this.get(userId);
        if (!user || !user.pipelines) return;
        const newPipelines = user.pipelines.filter(p => p.id !== pipelineId);
        await this.userStore.updatePipelines(userId, newPipelines);
    }

    async replacePipeline(userId: string, pipelineId: string, source: string, enrichers: EnricherConfig[], destinations: string[]): Promise<void> {
        await this.removePipeline(userId, pipelineId);
        await this.userStore.addPipeline(userId, {
            id: pipelineId, source, enrichers, destinations
        });
    }
}

