import { createCloudFunction, FirebaseAuthStrategy, db, UserTier, FrameworkHandler, HttpError, routeRequest, RouteMatch, FrameworkContext, RoutableRequest } from '@fitglue/shared';
import Stripe from 'stripe';

let stripe: Stripe;

function getStripe(): Stripe {
  if (!stripe) {
    const secretKey = process.env.STRIPE_SECRET_KEY;
    if (!secretKey) {
      throw new Error('STRIPE_SECRET_KEY not found in environment variables');
    }
    stripe = new Stripe(secretKey, {});
  }
  return stripe;
}

// ========================================
// Route Handlers
// ========================================

async function handleCheckout(_match: RouteMatch, _req: RoutableRequest, ctx: FrameworkContext) {
  const { logger, services, userId } = ctx;

  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  try {
    const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-server-dev';
    const stripeClient = getStripe();
    const priceId = process.env.STRIPE_PRICE_ID;
    if (!priceId) {
      throw new Error('STRIPE_PRICE_ID not found in environment variables');
    }

    const user = await services.user.get(userId);
    if (!user) {
      throw new HttpError(404, 'User not found');
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
    return { url: session.url };
  } catch (error: any) {
    logger.error('Checkout error', { error, userId });
    // Re-throw HttpError as-is (like 404)
    if (error.statusCode) {
      throw error;
    }
    // Wrap other errors
    throw new Error('Failed to create checkout session');
  }
}

async function handleWebhook(_match: RouteMatch, req: any, ctx: FrameworkContext) {
  const { logger } = ctx;

  try {
    const stripeClient = getStripe();
    const webhookSecret = process.env.STRIPE_WEBHOOK_SECRET;
    if (!webhookSecret) {
      throw new Error('STRIPE_WEBHOOK_SECRET not found in environment variables');
    }
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

    return { received: true };
  } catch (error: any) {
    logger.error('Webhook error', { error });
    throw new HttpError(400, 'Webhook signature verification failed');
  }
}

async function handlePortal(_match: RouteMatch, _req: RoutableRequest, ctx: FrameworkContext) {
  const { logger, services, userId } = ctx;

  if (!userId) {
    throw new HttpError(401, 'Unauthorized');
  }

  const projectId = process.env.GOOGLE_CLOUD_PROJECT || 'fitglue-server-dev';
  const stripeClient = getStripe();

  const user = await services.user.get(userId);
  if (!user?.stripeCustomerId) {
    throw new HttpError(400, 'No billing account found. Please subscribe first.');
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
  return { url: session.url };
}

// ========================================
// Main Handler
// ========================================

export const handler: FrameworkHandler = async (req, ctx) => {
  return await routeRequest(req, ctx, [
    {
      method: 'POST',
      pattern: '/api/billing/checkout',
      handler: handleCheckout
    },
    {
      method: 'POST',
      pattern: '/api/billing/webhook',
      handler: handleWebhook
    },
    {
      method: 'POST',
      pattern: '/api/billing/portal',
      handler: handlePortal
    }
  ]);
};

export const billingHandler = createCloudFunction(handler, {
  auth: {
    strategies: [new FirebaseAuthStrategy()],
  },
  skipExecutionLogging: true
});
