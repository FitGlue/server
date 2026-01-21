import { createCloudFunction, createWebhookProcessor, PayloadUserStrategy } from '@fitglue/shared';
import { StravaConnector, StravaWebhookEvent } from './connector';
import { StravaVerificationStrategy } from './auth';

export const stravaWebhookHandler = createCloudFunction(
  createWebhookProcessor(StravaConnector),
  {
    auth: {
      strategies: [
        // 1. Check for Verification Requests (GET)
        new StravaVerificationStrategy(),

        // 2. Check for Notification Payloads (POST)
        new PayloadUserStrategy((payload, ctx) => {
          const connector = new StravaConnector(ctx);
          return connector.resolveUser(payload as StravaWebhookEvent, ctx);
        })
      ]
    }
  }
);
