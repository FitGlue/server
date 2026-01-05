import { createCloudFunction, createWebhookProcessor, PayloadUserStrategy } from '@fitglue/shared';
import { FitbitConnector, FitbitBody } from './connector';
import { FitbitVerificationStrategy } from './auth';

export const fitbitWebhookHandler = createCloudFunction(
  createWebhookProcessor(FitbitConnector),
  {
    auth: {
      strategies: [
        // 1. Check for Verification Requests (GET)
        new FitbitVerificationStrategy(),

        // 2. Check for Notification Payloads (POST)
        new PayloadUserStrategy((payload, ctx) => {
          const connector = new FitbitConnector(ctx);
          return connector.resolveUser(payload as FitbitBody, ctx);
        })
      ]
    }
  }
);
