import { createCloudFunction, FrameworkContext, TOPICS } from '@fitglue/shared';
import { createHmac } from 'crypto';

const FITBIT_VERIFICATION_CODE = 'FITBIT_VERIFICATION_CODE';
const FITBIT_CLIENT_SECRET = 'FITBIT_CLIENT_SECRET';

const handler = async (req: any, res: any, ctx: FrameworkContext) => {
  const { logger, pubsub } = ctx;

  try {
    // 1. Verification Request (GET)
    if (req.method === 'GET') {
      const verifyCode = req.query.verify;
      if (!verifyCode) {
        logger.warn('Missing verify code in GET request');
        res.status(404).send('Not Found');
        return;
      }

      const expectedCode = process.env[FITBIT_VERIFICATION_CODE];
      if (verifyCode === expectedCode) {
        logger.info('Verification successful');
        res.status(204).send();
        return { action: 'verification', status: 'success' };
      } else {
        logger.warn('Invalid verification code');
        res.status(404).send('Not Found');
        throw new Error('Invalid verification code');
      }
    }

    // 2. Notification Request (POST)
    if (req.method === 'POST') {
      const signature = req.headers['x-fitbit-signature'];
      if (!signature) {
        logger.warn('Missing X-Fitbit-Signature header');
        res.status(400).send('Missing Signature');
        throw new Error('Missing X-Fitbit-Signature header');
      }

      const clientSecret = process.env[FITBIT_CLIENT_SECRET];
      if (!clientSecret) {
        logger.error('Missing FITBIT_CLIENT_SECRET env var');
        res.status(500).send('Configuration Error');
        throw new Error('Missing FITBIT_CLIENT_SECRET env var');
      }

      // Fitbit signature verification: HMAC-SHA1(body, secret + "&")
      const rawBody = (req as any).rawBody;

      if (!rawBody) {
        logger.error('Raw body not available for verification');
        res.status(500).send('Internal Server Error');
        return;
      }

      const hmac = createHmac('sha1', `${clientSecret}&`);
      hmac.update(rawBody);
      const expectedSignature = hmac.digest('base64');

      if (signature !== expectedSignature) {
        logger.warn('Invalid Signature', { expected: expectedSignature, received: signature });
        res.status(404).send('Not Found');
        throw new Error('Invalid Signature');
      }

      const body = req.body || [];
      if (!Array.isArray(body)) {
        logger.warn('Invalid body format', { body });
        res.status(400).send('Bad Request');
        throw new Error('Invalid body format');
      }

      logger.info(`Received ${body.length} updates`);

      // Filter and Publish
      const publishPromises = body.map(async (update: any) => {
        if (update.collectionType === 'activities') {
          // Use constant topic name
          await pubsub.topic(TOPICS.FITBIT_UPDATES).publishMessage({
            json: update,
          });
          logger.info('Published update', { ownerId: update.ownerId, date: update.date });
          return update; // Return for counting
        }
        return null;
      });

      const results = await Promise.all(publishPromises);
      const publishedCount = results.filter(r => r !== null).length;

      res.status(204).send();
      return {
        action: 'notification',
        received: body.length,
        published: publishedCount,
        updates: body
      };
    }

    res.status(405).send('Method Not Allowed');
    throw new Error(`Method Not Allowed: ${req.method}`);

  } catch (err: any) {
    logger.error('Handler error', { error: err });
    // If not already sent
    if (!res.headersSent) {
      res.status(500).send('Internal Error');
    }
    // Re-throw so framework catches it and logs failure
    throw err;
  }
};

export const fitbitWebhookHandler = createCloudFunction(handler);
