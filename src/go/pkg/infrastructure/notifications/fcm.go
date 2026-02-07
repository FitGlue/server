package notifications

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

type FCMAdapter struct {
	client *messaging.Client
	fs     *firestore.Client
}

func NewFCMAdapter(ctx context.Context, app *firebase.App, fs *firestore.Client) (*FCMAdapter, error) {
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}
	return &FCMAdapter{client: client, fs: fs}, nil
}

func (a *FCMAdapter) SendPushNotification(ctx context.Context, userID string, title, body string, tokens []string, data map[string]string) error {
	if len(tokens) == 0 {
		slog.Debug("No tokens for user, skipping notification", "user_id", userID)
		return nil
	}

	slog.Info("Sending push notification", "user_id", userID, "token_count", len(tokens), "title", title)

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	response, err := a.client.SendEachForMulticast(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send multicast message: %w", err)
	}

	if response.FailureCount > 0 {
		slog.Warn("Some push notifications failed to send",
			"user_id", userID,
			"failure_count", response.FailureCount,
			"success_count", response.SuccessCount,
		)
		a.cleanupDeadTokens(ctx, userID, tokens, response.Responses)
	}

	return nil
}

// cleanupDeadTokens removes FCM tokens that returned NotRegistered from the user document.
func (a *FCMAdapter) cleanupDeadTokens(ctx context.Context, userID string, tokens []string, responses []*messaging.SendResponse) {
	var deadTokens []interface{}
	for i, resp := range responses {
		if resp.Error != nil && messaging.IsRegistrationTokenNotRegistered(resp.Error) {
			deadTokens = append(deadTokens, tokens[i])
		}
	}

	if len(deadTokens) == 0 {
		return
	}

	slog.Info("Removing dead FCM tokens", "user_id", userID, "count", len(deadTokens))
	_, err := a.fs.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "fcm_tokens", Value: firestore.ArrayRemove(deadTokens...)},
	})
	if err != nil {
		slog.Error("Failed to remove dead FCM tokens", "user_id", userID, "error", err)
	}
}
