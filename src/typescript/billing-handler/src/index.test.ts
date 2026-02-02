import { handler } from './index';
import type { FrameworkContext } from '@fitglue/shared/framework';

// Mock @fitglue/shared/framework
jest.mock('@fitglue/shared/framework', () => ({
  createCloudFunction: (handler: any, _opts?: any) => handler,
  FirebaseAuthStrategy: jest.fn(),
  FrameworkHandler: jest.fn(),
  FrameworkContext: jest.fn(),
  db: {
    collection: jest.fn(() => ({
      doc: jest.fn(() => ({
        update: jest.fn(),
      })),
    })),
  },
}));

// Mock @fitglue/shared/errors
jest.mock('@fitglue/shared/errors', () => {
  class HttpError extends Error {
    statusCode: number;
    constructor(statusCode: number, message: string) {
      super(message);
      this.statusCode = statusCode;
      this.name = 'HttpError';
    }
  }
  return { HttpError };
});

// Mock @fitglue/shared/routing
jest.mock('@fitglue/shared/routing', () => {
  class HttpError extends Error {
    statusCode: number;
    constructor(statusCode: number, message: string) {
      super(message);
      this.statusCode = statusCode;
      this.name = 'HttpError';
    }
  }

  async function routeRequest(
    req: { method: string; path: string; query: Record<string, unknown> },
    ctx: unknown,
    routes: Array<{ method: string; pattern: string; handler: (match: { params: Record<string, string>; query: Record<string, string> }, req: unknown, ctx: unknown) => Promise<unknown> }>
  ): Promise<unknown> {
    for (const route of routes) {
      if (route.method !== req.method) continue;

      const patternParts = route.pattern.split('/').filter(Boolean);
      const pathParts = req.path.split('/').filter(Boolean);

      if (patternParts.length !== pathParts.length) continue;

      const params: Record<string, string> = {};
      let matched = true;

      for (let i = 0; i < patternParts.length; i++) {
        if (patternParts[i].startsWith(':')) {
          params[patternParts[i].slice(1)] = pathParts[i];
        } else if (patternParts[i] !== pathParts[i]) {
          matched = false;
          break;
        }
      }

      if (matched) {
        return await route.handler({ params, query: req.query as Record<string, string> }, req, ctx);
      }
    }

    throw new HttpError(404, 'Not found');
  }

  return {
    routeRequest,
    RouteMatch: jest.fn(),
    RoutableRequest: jest.fn(),
  };
});

// Mock @fitglue/shared/types
jest.mock('@fitglue/shared/types', () => ({
  UserTier: {
    USER_TIER_UNSPECIFIED: 0,
    USER_TIER_HOBBYIST: 1,
    USER_TIER_ATHLETE: 2,
  },
}));

// Mock Stripe
const mockStripeCheckoutSessionsCreate = jest.fn();
const mockStripeCustomersCreate = jest.fn();
const mockStripeCustomersRetrieve = jest.fn();
const mockStripeWebhooksConstructEvent = jest.fn();

jest.mock('stripe', () => {
  return jest.fn().mockImplementation(() => ({
    checkout: {
      sessions: {
        create: mockStripeCheckoutSessionsCreate,
      },
    },
    customers: {
      create: mockStripeCustomersCreate,
      retrieve: mockStripeCustomersRetrieve,
    },
    webhooks: {
      constructEvent: mockStripeWebhooksConstructEvent,
    },
  }));
});

import { db } from '@fitglue/shared/framework';

// Helper to create request objects - casts to handler's expected request type
const createRequest = (overrides: Record<string, unknown> = {}): Parameters<typeof handler>[0] => ({
  method: 'POST',
  path: '/api/billing/checkout',
  body: {},
  headers: {},
  query: {},
  ...overrides,
} as Parameters<typeof handler>[0]);

describe('billing-handler', () => {

  let ctx: any;
  let mockDbUpdate: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();

    // Setup db mock with chained methods
    mockDbUpdate = jest.fn().mockResolvedValue(undefined);
    (db.collection as jest.Mock).mockReturnValue({
      doc: jest.fn().mockReturnValue({
        update: mockDbUpdate,
      }),
    });

    // Setup environment variables for secrets
    process.env.STRIPE_SECRET_KEY = 'sk_test_fake_key';
    process.env.STRIPE_PRICE_ID = 'price_test_123';
    process.env.STRIPE_WEBHOOK_SECRET = 'whsec_test_secret';



    ctx = {
      userId: 'user-123',
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
        debug: jest.fn(),
      },
      services: {
        user: {
          get: jest.fn(),
        },
      },
      stores: {
        users: {
          update: jest.fn(),
        },
      },
    };
  });

  describe('POST /api/billing/checkout', () => {
    it('returns 401 if no user is authenticated', async () => {
      ctx.userId = undefined;
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await expect(handler(req, ctx as FrameworkContext)).rejects.toThrow(expect.objectContaining({ statusCode: 401 }));
    });

    it('returns 404 if user not found', async () => {
      ctx.services.user.get.mockResolvedValue(null);
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await expect(handler(req, ctx as FrameworkContext)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('creates a new Stripe customer if user has no stripeCustomerId', async () => {
      ctx.services.user.get.mockResolvedValue({ id: 'user-123', tier: 1 }); // USER_TIER_HOBBYIST
      mockStripeCustomersCreate.mockResolvedValue({ id: 'cus_new_123' });
      mockStripeCheckoutSessionsCreate.mockResolvedValue({
        id: 'cs_test_123',
        url: 'https://checkout.stripe.com/session123',
      });
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await handler(req, ctx as FrameworkContext);

      expect(mockStripeCustomersCreate).toHaveBeenCalledWith({
        metadata: { fitglue_user_id: 'user-123' },
      });
      expect(ctx.stores.users.update).toHaveBeenCalledWith('user-123', {
        stripeCustomerId: 'cus_new_123',
      });
    });

    it('uses existing stripeCustomerId if user already has one', async () => {
      ctx.services.user.get.mockResolvedValue({
        id: 'user-123',
        tier: 1, // USER_TIER_HOBBYIST
        stripeCustomerId: 'cus_existing_456',
      });
      mockStripeCheckoutSessionsCreate.mockResolvedValue({
        id: 'cs_test_123',
        url: 'https://checkout.stripe.com/session123',
      });
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await handler(req, ctx as FrameworkContext);

      expect(mockStripeCustomersCreate).not.toHaveBeenCalled();
      expect(ctx.stores.users.update).not.toHaveBeenCalled();
      expect(mockStripeCheckoutSessionsCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          customer: 'cus_existing_456',
        })
      );
    });

    it('creates checkout session with correct parameters', async () => {
      ctx.services.user.get.mockResolvedValue({
        id: 'user-123',
        stripeCustomerId: 'cus_existing_456',
      });
      mockStripeCheckoutSessionsCreate.mockResolvedValue({
        id: 'cs_test_123',
        url: 'https://checkout.stripe.com/session123',
      });

      // Mock dev environment
      process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-dev';
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await handler(req, ctx as FrameworkContext);

      expect(mockStripeCheckoutSessionsCreate).toHaveBeenCalledWith({
        customer: 'cus_existing_456',
        payment_method_types: ['card'],
        line_items: [{ price: 'price_test_123', quantity: 1 }],
        mode: 'subscription',
        success_url: 'https://dev.fitglue.tech/app?billing=success',
        cancel_url: 'https://dev.fitglue.tech/app?billing=cancelled',
        metadata: { fitglue_user_id: 'user-123' },
      });
    });

    it('returns checkout session URL on success', async () => {
      ctx.services.user.get.mockResolvedValue({
        id: 'user-123',
        stripeCustomerId: 'cus_existing_456',
      });
      mockStripeCheckoutSessionsCreate.mockResolvedValue({
        id: 'cs_test_123',
        url: 'https://checkout.stripe.com/session123',
      });
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      const result = await handler(req, ctx as FrameworkContext);

      expect(result).toEqual({
        url: 'https://checkout.stripe.com/session123',
      });
      expect(ctx.logger.info).toHaveBeenCalledWith('Checkout session created', {
        userId: 'user-123',
        sessionId: 'cs_test_123',
      });
    });

    it('uses prod URL for production environment', async () => {
      ctx.services.user.get.mockResolvedValue({
        id: 'user-123',
        stripeCustomerId: 'cus_existing_456',
      });
      mockStripeCheckoutSessionsCreate.mockResolvedValue({
        id: 'cs_test_123',
        url: 'https://checkout.stripe.com/session123',
      });

      process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-prod';
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await handler(req, ctx as FrameworkContext);

      expect(mockStripeCheckoutSessionsCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          success_url: 'https://fitglue.tech/app?billing=success',
          cancel_url: 'https://fitglue.tech/app?billing=cancelled',
        })
      );
    });

    it('uses test URL for test environment', async () => {
      ctx.services.user.get.mockResolvedValue({
        id: 'user-123',
        stripeCustomerId: 'cus_existing_456',
      });
      mockStripeCheckoutSessionsCreate.mockResolvedValue({
        id: 'cs_test_123',
        url: 'https://checkout.stripe.com/session123',
      });

      process.env.GOOGLE_CLOUD_PROJECT = 'fitglue-server-test';
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await handler(req, ctx as FrameworkContext);

      expect(mockStripeCheckoutSessionsCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          success_url: 'https://test.fitglue.tech/app?billing=success',
          cancel_url: 'https://test.fitglue.tech/app?billing=cancelled',
        })
      );
    });

    it('returns 500 on Stripe API error', async () => {
      ctx.services.user.get.mockResolvedValue({
        id: 'user-123',
        stripeCustomerId: 'cus_existing_456',
      });
      mockStripeCheckoutSessionsCreate.mockRejectedValue(new Error('Stripe API error'));
      const req = createRequest({ path: '/api/billing/checkout', method: 'POST' });

      await expect(handler(req, ctx as FrameworkContext)).rejects.toThrow('Failed to create checkout session');

      expect(ctx.logger.error).toHaveBeenCalledWith('Checkout error', expect.objectContaining({
        userId: 'user-123',
      }));
    });
  });

  describe('POST /api/billing/webhook', () => {
    it('handles checkout.session.completed event - upgrades user to Pro', async () => {
      const mockEvent = {
        type: 'checkout.session.completed',
        data: {
          object: {
            id: 'cs_test_123',
            metadata: { fitglue_user_id: 'user-abc' },
          },
        },
      };
      mockStripeWebhooksConstructEvent.mockReturnValue(mockEvent);
      const req = createRequest({
        path: '/api/billing/webhook',
        method: 'POST',
        headers: { 'stripe-signature': 'sig_test_signature' },
        body: 'raw_webhook_body',
      });

      const result = await handler(req, ctx as FrameworkContext);

      expect(mockStripeWebhooksConstructEvent).toHaveBeenCalledWith(
        'raw_webhook_body',
        'sig_test_signature',
        'whsec_test_secret'
      );
      expect(db.collection).toHaveBeenCalledWith('users');
      expect(mockDbUpdate).toHaveBeenCalledWith({
        tier: 2, // USER_TIER_ATHLETE
        trial_ends_at: null,
      });
      expect(ctx.logger.info).toHaveBeenCalledWith('User upgraded to Athlete', {
        userId: 'user-abc',
        sessionId: 'cs_test_123',
      });
      expect(result).toEqual({ received: true });
    });

    it('handles checkout.session.completed without fitglue_user_id', async () => {
      const mockEvent = {
        type: 'checkout.session.completed',
        data: {
          object: {
            id: 'cs_test_123',
            metadata: {},
          },
        },
      };
      mockStripeWebhooksConstructEvent.mockReturnValue(mockEvent);
      const req = createRequest({
        path: '/api/billing/webhook',
        method: 'POST',
        headers: { 'stripe-signature': 'sig_test_signature' },
        body: 'raw_webhook_body',
      });

      const result = await handler(req, ctx as FrameworkContext);

      expect(mockDbUpdate).not.toHaveBeenCalled();
      expect(result).toEqual({ received: true });
    });

    it('handles customer.subscription.deleted event - downgrades user to Free', async () => {
      const mockEvent = {
        type: 'customer.subscription.deleted',
        data: {
          object: {
            id: 'sub_test_123',
            customer: 'cus_customer_123',
          },
        },
      };
      mockStripeWebhooksConstructEvent.mockReturnValue(mockEvent);
      mockStripeCustomersRetrieve.mockResolvedValue({
        id: 'cus_customer_123',
        metadata: { fitglue_user_id: 'user-xyz' },
      });
      const req = createRequest({
        path: '/api/billing/webhook',
        method: 'POST',
        headers: { 'stripe-signature': 'sig_test_signature' },
        body: 'raw_webhook_body',
      });

      const result = await handler(req, ctx as FrameworkContext);

      expect(mockStripeCustomersRetrieve).toHaveBeenCalledWith('cus_customer_123');
      expect(db.collection).toHaveBeenCalledWith('users');
      expect(mockDbUpdate).toHaveBeenCalledWith({
        tier: 1, // USER_TIER_HOBBYIST
      });
      expect(ctx.logger.info).toHaveBeenCalledWith('User downgraded to Hobbyist', {
        userId: 'user-xyz',
      });
      expect(result).toEqual({ received: true });
    });

    it('handles customer.subscription.deleted without fitglue_user_id', async () => {
      const mockEvent = {
        type: 'customer.subscription.deleted',
        data: {
          object: {
            id: 'sub_test_123',
            customer: 'cus_customer_123',
          },
        },
      };
      mockStripeWebhooksConstructEvent.mockReturnValue(mockEvent);
      mockStripeCustomersRetrieve.mockResolvedValue({
        id: 'cus_customer_123',
        metadata: {},
      });
      const req = createRequest({
        path: '/api/billing/webhook',
        method: 'POST',
        headers: { 'stripe-signature': 'sig_test_signature' },
        body: 'raw_webhook_body',
      });

      const result = await handler(req, ctx as FrameworkContext);

      expect(mockDbUpdate).not.toHaveBeenCalled();
      expect(result).toEqual({ received: true });
    });

    it('handles unhandled event types gracefully', async () => {
      const mockEvent = {
        type: 'invoice.payment_succeeded',
        data: { object: {} },
      };
      mockStripeWebhooksConstructEvent.mockReturnValue(mockEvent);
      const req = createRequest({
        path: '/api/billing/webhook',
        method: 'POST',
        headers: { 'stripe-signature': 'sig_test_signature' },
        body: 'raw_webhook_body',
      });

      const result = await handler(req, ctx as FrameworkContext);

      expect(ctx.logger.info).toHaveBeenCalledWith('Unhandled Stripe event', {
        type: 'invoice.payment_succeeded',
      });
      expect(result).toEqual({ received: true });
    });

    it('returns 400 on webhook signature verification failure', async () => {
      mockStripeWebhooksConstructEvent.mockImplementation(() => {
        throw new Error('Webhook signature verification failed');
      });
      const req = createRequest({
        path: '/api/billing/webhook',
        method: 'POST',
        headers: { 'stripe-signature': 'sig_test_signature' },
        body: 'raw_webhook_body',
      });

      await expect(handler(req, ctx as FrameworkContext)).rejects.toThrow(expect.objectContaining({ statusCode: 400 }));

      expect(ctx.logger.error).toHaveBeenCalledWith('Webhook error', expect.any(Object));
    });
  });

  describe('Unknown routes', () => {
    it('returns 404 for unknown GET routes', async () => {
      const req = createRequest({ method: 'GET', path: '/api/billing/unknown' });

      await expect(handler(req, ctx as FrameworkContext)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('returns 404 for unknown POST routes', async () => {
      const req = createRequest({ method: 'POST', path: '/api/billing/unknown' });

      await expect(handler(req, ctx as FrameworkContext)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });

    it('returns 404 for root billing path', async () => {
      const req = createRequest({ method: 'GET', path: '/api/billing' });

      await expect(handler(req, ctx as FrameworkContext)).rejects.toThrow(expect.objectContaining({ statusCode: 404 }));
    });
  });
});
