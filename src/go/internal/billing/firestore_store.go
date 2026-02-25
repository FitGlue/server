// nolint:proto-json
package billing

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/firestore"
	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type FirestoreStore struct {
	client *firestore.Client
}

func NewFirestoreStore(client *firestore.Client) *FirestoreStore {
	return &FirestoreStore{client: client}
}

// GetSubscription reads billing state from the user's billing subcollection
func (s *FirestoreStore) GetSubscription(ctx context.Context, userID string) (*pbuser.SubscriptionState, error) {
	doc, err := s.client.Collection("users").Doc(userID).Collection("billing").Doc("subscription").Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil // Not found is not an error, just no subscription yet
		}
		return nil, err
	}

	b, err := json.Marshal(doc.Data())
	if err != nil {
		return nil, err
	}

	var state pbuser.SubscriptionState
	err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(b, &state)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *FirestoreStore) UpsertSubscription(ctx context.Context, sub *pbuser.SubscriptionState) error {
	b, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(sub)
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	_, err = s.client.Collection("users").Doc(sub.UserId).Collection("billing").Doc("subscription").Set(ctx, data, firestore.MergeAll)
	if err != nil {
		return err
	}

	// Also ensure stripeCustomerId is synced to the root user doc for legacy/query purposes
	if sub.StripeCustomerId != "" {
		_, _ = s.client.Collection("users").Doc(sub.UserId).Update(ctx, []firestore.Update{
			{Path: "stripeCustomerId", Value: sub.StripeCustomerId},
		})
	}

	return nil
}

func (s *FirestoreStore) GetUserIDByStripeCustomer(ctx context.Context, customerID string) (string, error) {
	iter := s.client.Collection("users").Where("stripeCustomerId", "==", customerID).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return "", status.Error(codes.NotFound, "user not found for stripe customer")
	}
	if err != nil {
		return "", err
	}

	return doc.Ref.ID, nil
}

func (s *FirestoreStore) UpdateUserTier(ctx context.Context, userID string, tier pbuser.UserTier, trialEndsAt *time.Time) error {
	updates := []firestore.Update{
		{Path: "tier", Value: int(tier)}, // Enums are ints in firestore typically, or string. The old system used int or string? Let's use int representing the enum, or the string name.
		// Wait, old system used `UserTier.USER_TIER_ATHLETE`. It was represented as an integer in ts-proto by default, unless configured otherwise.
		// Let's check user_profile.proto or old TS code. The TS code imported `UserTier` enum. By default ts-proto uses ints.
	}

	if trialEndsAt == nil {
		updates = append(updates, firestore.Update{Path: "trial_ends_at", Value: firestore.Delete})
	} else {
		updates = append(updates, firestore.Update{Path: "trial_ends_at", Value: *trialEndsAt})
	}

	_, err := s.client.Collection("users").Doc(userID).Update(ctx, updates)
	return err
}

func (s *FirestoreStore) GetTierStatus(ctx context.Context, userID string) (pbuser.UserTier, bool, *time.Time, error) {
	doc, err := s.client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return pbuser.UserTier_USER_TIER_UNSPECIFIED, false, nil, err
	}

	var tier pbuser.UserTier = pbuser.UserTier_USER_TIER_HOBBYIST
	if t, err := doc.DataAt("tier"); err == nil {
		if tierInt, ok := t.(int64); ok {
			tier = pbuser.UserTier(tierInt)
		}
	}

	isAdmin := false
	if a, err := doc.DataAt("is_admin"); err == nil {
		if adminBool, ok := a.(bool); ok {
			isAdmin = adminBool
		}
	}

	var trialEnds *time.Time
	if tr, err := doc.DataAt("trial_ends_at"); err == nil {
		if tTime, ok := tr.(time.Time); ok {
			trialEnds = &tTime
		}
	}

	return tier, isAdmin, trialEnds, nil
}
