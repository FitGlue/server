package infra

import "context"

// PaymentProvider defines an interface for interacting with a billing gateway like Stripe.
type PaymentProvider interface {
	CreateCheckoutSession(ctx context.Context, userID, planID, successURL, cancelURL string) (sessionURL string, err error)
	CancelSubscription(ctx context.Context, subscriptionID string) error
	VerifyWebhookSignature(payload []byte, signature string) error
}
