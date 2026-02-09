// Module-level imports for smart pruning
import { createCloudFunction, createWebhookProcessor, PayloadUserStrategy } from '@fitglue/shared/framework';
import { GitHubConnector, GitHubPushEvent } from './connector';
import { GitHubWebhookAuthStrategy } from './auth';

// The GitHubConnector encapsulates specific logic (ID extraction, API interaction, Mapping).
// The createWebhookProcessor encapsulation standardizes the flow:
// Auth -> Extract ID -> Load Config -> Dedup -> Fetch/Map -> Publish -> Mark Processed.

export const githubHandler = createCloudFunction(
    createWebhookProcessor(GitHubConnector),
    {
        auth: {
            strategies: [
                // 1. Verify webhook HMAC signature (X-Hub-Signature-256)
                new GitHubWebhookAuthStrategy(),

                // 2. Resolve FitGlue user from the repository in the payload
                new PayloadUserStrategy((payload, ctx) => {
                    const connector = new GitHubConnector(ctx);
                    return connector.resolveUser(payload as GitHubPushEvent, ctx);
                })
            ]
        }
    }
);
