package notifications

import (
	"context"
	"fmt"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

type FCMAdapter struct {
	client *messaging.Client
}

func NewFCMAdapter(ctx context.Context, app *firebase.App) (*FCMAdapter, error) {
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}
	return &FCMAdapter{client: client}, nil
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
		// TODO: Cleanup dead tokens if we get "registration-token-not-registered" error
	}

	return nil
}
