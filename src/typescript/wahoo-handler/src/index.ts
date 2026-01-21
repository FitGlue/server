import { createCloudFunction, createWebhookProcessor, PayloadUserStrategy } from '@fitglue/shared';
import { WahooConnector, WahooWebhookEvent } from './connector';
import { WahooVerificationStrategy } from './auth';

export const wahooWebhookHandler = createCloudFunction(
  createWebhookProcessor(WahooConnector),
  {
    auth: {
      strategies: [
        // 1. Check for Verification Requests (GET)
        new WahooVerificationStrategy(),

        // 2. Check for Notification Payloads (POST)
        new PayloadUserStrategy((payload, ctx) => {
          const connector = new WahooConnector(ctx);
          return connector.resolveUser(payload as WahooWebhookEvent, ctx);
        })
      ]
    }
  }
);
