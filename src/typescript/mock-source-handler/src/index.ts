import { createCloudFunction, createWebhookProcessor, ApiKeyStrategy } from '@fitglue/shared';
import { MockConnector } from './connector';

export const mockSourceHandler = createCloudFunction(
  createWebhookProcessor(MockConnector),
  {
    auth: {
      strategies: [new ApiKeyStrategy()],
      requiredScopes: ['read:activity'] // Reusing existing scope
    }
  }
);
