import { createCloudFunction, createWebhookProcessor, ApiKeyStrategy } from '@fitglue/shared';
import { HevyConnector } from './connector';

// The HevyConnector encapsulates specific logic (ID extraction, API interaction, Mapping).
// The createWebhookProcessor encapsulation standardizes the flow:
// Auth -> Extract ID -> Load Config -> Dedup -> Fetch/Map -> Publish -> Mark Processed.

export const hevyHandler = createCloudFunction(
    createWebhookProcessor(HevyConnector),
    {
        auth: {
            strategies: [new ApiKeyStrategy()],
            requiredScopes: ['ingress']
        }
    }
);
