// Module-level imports for smart pruning
import { createCloudFunction, createWebhookProcessor, ApiKeyStrategy } from '@fitglue/shared/framework';
import { MockConnector } from './connector';

export const mockSourceHandler = createCloudFunction(
  createWebhookProcessor(MockConnector),
  {
    auth: {
      strategies: [new ApiKeyStrategy()],
      requiredScopes: ['ingress'] // Ingress scope for API key authentication
    }
  }
);
