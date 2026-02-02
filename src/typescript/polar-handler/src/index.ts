// Module-level imports for smart pruning
import { createCloudFunction, createWebhookProcessor, PayloadUserStrategy } from '@fitglue/shared/framework';
import { PolarConnector, PolarBody } from './connector';
import { PolarVerificationStrategy } from './auth';

export const polarWebhookHandler = createCloudFunction(
  createWebhookProcessor(PolarConnector),
  {
    auth: {
      strategies: [
        // 1. Check for Verification Requests (GET)
        new PolarVerificationStrategy(),

        // 2. Check for Notification Payloads (POST)
        new PayloadUserStrategy((payload, ctx) => {
          const connector = new PolarConnector(ctx);
          return connector.resolveUser(payload as PolarBody, ctx);
        })
      ]
    }
  }
);
