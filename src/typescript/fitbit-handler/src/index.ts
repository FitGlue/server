import { createCloudFunction, createWebhookProcessor, FitbitConnector, PayloadUserStrategy } from '@fitglue/shared';

const connector = new FitbitConnector();

export const fitbitWebhookHandler = createCloudFunction(
  createWebhookProcessor(connector),
  {
    auth: {
      strategies: [new PayloadUserStrategy((payload, ctx) => connector.resolveUser!(payload, ctx))]
    }
  }
);
