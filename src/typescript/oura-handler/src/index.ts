import { createCloudFunction, createWebhookProcessor, PayloadUserStrategy } from '@fitglue/shared';
import { OuraConnector, OuraWebhookEvent } from './connector';

export const ouraWebhookHandler = createCloudFunction(
  createWebhookProcessor(OuraConnector),
  {
    auth: {
      strategies: [
        // Check for Notification Payloads (POST)
        new PayloadUserStrategy((payload, ctx) => {
          const connector = new OuraConnector(ctx);
          return connector.resolveUser(payload as OuraWebhookEvent, ctx);
        })
      ]
    }
  }
);
