import createClient, { Middleware } from 'openapi-fetch';
import type { paths, components } from './schema';
import type { UserStore } from '../../storage/firestore/user-store';
import type { Logger } from 'winston';
import { errorLoggingMiddleware } from '../../infrastructure/http/errors';

// Utility type to make a specific header optional in the paths definition
// This allows middleware to handle headers (like api-key) without forcing the caller to provide them.
type OmitHeader<T, K extends string> = {
    [Path in keyof T]: {
        [Method in keyof T[Path]]: T[Path][Method] extends { parameters: { header: infer H } }
        ? Omit<T[Path][Method], 'parameters'> & {
            parameters: Omit<T[Path][Method]['parameters'], 'header'> & {
                header?: Omit<H, K> & Partial<Pick<H, Extract<keyof H, K>>>;
            };
        }
        : T[Path][Method];
    };
};

export type ClientPaths = OmitHeader<paths, 'api-key'>;
export type HevyClient = ReturnType<typeof createClient<ClientPaths>>;
export type Workout = components['schemas']['Workout'];

export interface HevyClientOptions {
    apiKey: string;
}

const authMiddleware = (apiKey: string): Middleware => ({
    onRequest({ request }) {
        request.headers.set('api-key', apiKey);
        return request;
    },
});

const usageMiddleware = (userStore: UserStore, userId: string): Middleware => ({
    async onResponse({ response }) {
        if (response.ok) {
            // Fire and forget usage tracking
            userStore.updateLastUsed(userId, 'hevy').catch(err => {
                console.warn(`[HevyClient] Failed to track usage for user ${userId}`, err);
            });
        }
        return response;
    }
});

export interface HevyClientOptions {
    apiKey: string;
    usageTracking?: {
        userStore: UserStore;
        userId: string;
    };
    /** Optional logger for error response logging */
    logger?: Logger;
}

export function createHevyClient(options: HevyClientOptions): HevyClient {
    const client = createClient<ClientPaths>({
        baseUrl: 'https://api.hevyapp.com',
    });

    client.use(authMiddleware(options.apiKey));

    if (options.usageTracking) {
        client.use(usageMiddleware(options.usageTracking.userStore, options.usageTracking.userId));
    }

    // Add error logging middleware if logger provided
    if (options.logger) {
        client.use(errorLoggingMiddleware(options.logger, 'hevy-client'));
    }

    return client;
}

