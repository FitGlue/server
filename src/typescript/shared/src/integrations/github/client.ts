import createClient, { Middleware } from 'openapi-fetch';
import type { paths, components } from './schema';
import type { UserStore } from '../../storage/firestore/user-store';
import type { Logger } from 'winston';
import { errorLoggingMiddleware } from '../../infrastructure/http/errors';

// GitHub API uses Bearer token auth in the Authorization header
export type GitHubClient = ReturnType<typeof createClient<paths>>;
export type GitHubComponents = components;

const authMiddleware = (accessToken: string): Middleware => ({
    onRequest({ request }) {
        request.headers.set('Authorization', `Bearer ${accessToken}`);
        request.headers.set('Accept', 'application/vnd.github.v3+json');
        request.headers.set('User-Agent', 'FitGlue/1.0');
        request.headers.set('X-GitHub-Api-Version', '2022-11-28');
        return request;
    },
});

const usageMiddleware = (userStore: UserStore, userId: string): Middleware => ({
    async onResponse({ response }) {
        if (response.ok) {
            // Fire and forget usage tracking
            userStore.updateLastUsed(userId, 'github').catch(err => {
                console.warn(`[GitHubClient] Failed to track usage for user ${userId}`, err);
            });
        }
        return response;
    }
});

export interface GitHubClientOptions {
    accessToken: string;
    usageTracking?: {
        userStore: UserStore;
        userId: string;
    };
    /** Optional logger for error response logging */
    logger?: Logger;
}

export function createGitHubClient(options: GitHubClientOptions): GitHubClient {
    const client = createClient<paths>({
        baseUrl: 'https://api.github.com',
    });

    client.use(authMiddleware(options.accessToken));

    if (options.usageTracking) {
        client.use(usageMiddleware(options.usageTracking.userStore, options.usageTracking.userId));
    }

    // Add error logging middleware if logger provided
    if (options.logger) {
        client.use(errorLoggingMiddleware(options.logger, 'github-client'));
    }

    return client;
}
