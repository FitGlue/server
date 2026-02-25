package billing

import (
	"context"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
)

type StripeClient interface {
	CreateCustomer(ctx context.Context, userID string) (*stripe.Customer, error)
	CreateCheckoutSession(ctx context.Context, customerID string, priceID string, successURL string, cancelURL string, userID string) (*stripe.CheckoutSession, error)
	GetSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error)
	GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error)
	CancelSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error)
}

type LiveStripeClient struct {
	secretKey string
}

func NewLiveStripeClient(secretKey string) *LiveStripeClient {
	stripe.Key = secretKey
	return &LiveStripeClient{secretKey: secretKey}
}

func (s *LiveStripeClient) CreateCustomer(ctx context.Context, userID string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Metadata: map[string]string{
			"fitglue_user_id": userID,
		},
	}
	return customer.New(params)
}

func (s *LiveStripeClient) CreateCheckoutSession(ctx context.Context, customerID string, priceID string, successURL string, cancelURL string, userID string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata: map[string]string{
			"fitglue_user_id": userID,
		},
	}
	return session.New(params)
}

func (s *LiveStripeClient) GetSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	return subscription.Get(subscriptionID, nil)
}

func (s *LiveStripeClient) GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	return customer.Get(customerID, nil)
}

func (s *LiveStripeClient) CancelSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionCancelParams{}
	return subscription.Cancel(subscriptionID, params)
}
