import { createCloudFunction, createWebhookProcessor, HevyConnector, ApiKeyStrategy } from '@fitglue/shared';

// The HevyConnector encapsulates specific logic (ID extraction, API interaction, Mapping).
// The createWebhookProcessor encapsulation standardizes the flow:
// Auth -> Extract ID -> Load Config -> Dedup -> Fetch/Map -> Publish -> Mark Processed.

export const hevyWebhookHandler = createCloudFunction(
    createWebhookProcessor(new HevyConnector()),
    {
        auth: {
            strategies: [new ApiKeyStrategy()],
            requiredScopes: ['read:activity']
        }
    }
);
