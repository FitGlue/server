import { createCloudFunction, FrameworkContext, FirebaseAuthStrategy, getSecret, db, UserTier } from '@fitglue/shared';
import Stripe from 'stripe';
import { Request, Response } from 'express';

let stripe: Stripe;

async function getStripe(): Promise<Stripe> {
  if (!stripe) {
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-server-dev';
    const secretKey = await getSecret(projectId, 'stripe-secret-key');
    stripe = new Stripe(secretKey, {});
  }
  return stripe;
}

export const handler = async (req: Request, res: Response, ctx: FrameworkContext) => {
  const { logger, services } = ctx;
  const userId = ctx.userId;

  // Extract subpath: /billing/checkout or /billing/webhook
  const subPath = req.path.replace(/^\/api\/billing/, '') || '/';

  // POST /api/billing/checkout - Create Stripe checkout session
  if (subPath === '/checkout' && req.method === 'POST') {
    if (!userId) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    try {
      const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-server-dev';
      const stripeClient = await getStripe();
      const priceId = await getSecret(projectId, 'stripe-price-id');

      const user = await services.user.get(userId);
      if (!user) {
        res.status(404).json({ error: 'User not found' });
        return;
      }

      let customerId = user.stripeCustomerId;

      // Create Stripe customer if needed
      if (!customerId) {
        const customer = await stripeClient.customers.create({
          metadata: { fitglue_user_id: userId }
        });
        customerId = customer.id;
        // Update user with Stripe customer ID
        await ctx.stores.users.update(userId, { stripeCustomerId: customerId });
      }

      // Determine environment URL
      const env = projectId.includes('-prod') ? 'prod' : projectId.includes('-test') ? 'test' : 'dev';
      const baseUrl = env === 'prod' ? 'https://fitglue.tech' : `https://${env}.fitglue.tech`;

      // Create checkout session
      const session = await stripeClient.checkout.sessions.create({
        customer: customerId,
        payment_method_types: ['card'],
        line_items: [{ price: priceId, quantity: 1 }],
        mode: 'subscription',
        success_url: `${baseUrl}/app?billing=success`,
        cancel_url: `${baseUrl}/app?billing=cancelled`,
        metadata: { fitglue_user_id: userId },
      });

      logger.info('Checkout session created', { userId, sessionId: session.id });
      res.json({ url: session.url });
    } catch (error) {
      logger.error('Checkout error', { error, userId });
      res.status(500).json({ error: 'Failed to create checkout session' });
    }
    return;
  }

  // POST /api/billing/webhook - Handle Stripe webhook events
  if (subPath === '/webhook' && req.method === 'POST') {
    try {
      const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-dev';
      const stripeClient = await getStripe();
      const webhookSecret = await getSecret(projectId, 'stripe-webhook-secret');
      const sig = req.headers['stripe-signature'] as string;

      // Stripe expects raw body for signature verification
      const rawBody = req.body;
      const event = stripeClient.webhooks.constructEvent(rawBody, sig, webhookSecret);

      switch (event.type) {
        case 'checkout.session.completed': {
          const session = event.data.object as Stripe.Checkout.Session;
          const fitglueUserId = session.metadata?.fitglue_user_id;
          if (fitglueUserId) {
            await db.collection('users').doc(fitglueUserId).update({
              tier: UserTier.USER_TIER_ATHLETE,
              trial_ends_at: null,
            });
            logger.info('User upgraded to Athlete', { userId: fitglueUserId, sessionId: session.id });
          }
          break;
        }

        case 'customer.subscription.deleted': {
          const subscription = event.data.object as Stripe.Subscription;
          const customer = await stripeClient.customers.retrieve(subscription.customer as string);
          const fitglueUserId = (customer as Stripe.Customer).metadata?.fitglue_user_id;
          if (fitglueUserId) {
            await db.collection('users').doc(fitglueUserId).update({
              tier: UserTier.USER_TIER_HOBBYIST,
            });
            logger.info('User downgraded to Hobbyist', { userId: fitglueUserId });
          }
          break;
        }

        default:
          logger.info('Unhandled Stripe event', { type: event.type });
      }

      res.json({ received: true });
    } catch (error) {
      logger.error('Webhook error', { error });
      res.status(400).json({ error: 'Webhook signature verification failed' });
    }
    return;
  }
  // POST /api/billing/portal - Create Stripe billing portal session
  if (subPath === '/portal' && req.method === 'POST') {
    if (!userId) {
      res.status(401).json({ error: 'Unauthorized' });
      return;
    }

    try {
      const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-server-dev';
      const stripeClient = await getStripe();

      const user = await services.user.get(userId);
      if (!user?.stripeCustomerId) {
        res.status(400).json({ error: 'No billing account found. Please subscribe first.' });
        return;
      }

      // Determine environment URL
      const env = projectId.includes('-prod') ? 'prod' : projectId.includes('-test') ? 'test' : 'dev';
      const baseUrl = env === 'prod' ? 'https://fitglue.tech' : `https://${env}.fitglue.tech`;

      // Create portal session
      const session = await stripeClient.billingPortal.sessions.create({
        customer: user.stripeCustomerId,
        return_url: `${baseUrl}/settings/subscription`,
      });

      logger.info('Portal session created', { userId });
      res.json({ url: session.url });
    } catch (error) {
      logger.error('Portal error', { error, userId });
      res.status(500).json({ error: 'Failed to create billing portal session' });
    }
    return;
  }

  res.status(404).json({ error: 'Not Found' });
};

export const billingHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()],
  },
  skipExecutionLogging: true
});
