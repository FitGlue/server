package user_input

import (
	user "github.com/fitglue/server/src/go/pkg/domain/user"

	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"

	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"

	"context"
	"log/slog"
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"
)

func TestUserInput_Enrich(t *testing.T) {
	ctx := context.Background()

	t.Run("Returns WaitError if no pending input", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{
			GetPendingInputFunc: func(ctx context.Context, userId string, id string) (*pbpipeline.PendingInput, error) {
				return nil, nil // Not found
			},
		}
		provider := &UserInputProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pbactivity.StandardizedActivity{Source: pbactivity.ActivitySource_SOURCE_HEVY, ExternalId: "123"}
		user := &user.Record{UserProfile: &pbuser.UserProfile{UserId: "test-user-1"}}
		inputs := map[string]string{}

		_, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)

		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		waitErr, ok := err.(*WaitForInputError)
		if !ok {
			t.Fatalf("Expected WaitForInputError, got %T", err)
		}
		if waitErr.ActivityID != "SOURCE_HEVY:123:user_input" {
			t.Errorf("Expected ActivityID 'SOURCE_HEVY:123:user_input', got %s", waitErr.ActivityID)
		}
		if waitErr.EnricherProviderID != "user_input" {
			t.Errorf("Expected EnricherProviderID 'user_input', got %s", waitErr.EnricherProviderID)
		}
	})

	t.Run("Returns WaitError if status WAITING", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{
			GetPendingInputFunc: func(ctx context.Context, userId string, id string) (*pbpipeline.PendingInput, error) {
				return &pbpipeline.PendingInput{
					ActivityId: id,
					Status:     pbpipeline.PendingInput_STATUS_WAITING,
				}, nil
			},
		}
		provider := &UserInputProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pbactivity.StandardizedActivity{Source: pbactivity.ActivitySource_SOURCE_HEVY, ExternalId: "123"}
		user := &user.Record{UserProfile: &pbuser.UserProfile{UserId: "test-user-1"}}

		_, err := provider.Enrich(ctx, slog.Default(), activity, user, nil, false)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if _, ok := err.(*WaitForInputError); !ok {
			t.Fatalf("Expected WaitForInputError, got %T", err)
		}
	})

	t.Run("Returns Result if status COMPLETED", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{
			GetPendingInputFunc: func(ctx context.Context, userId string, id string) (*pbpipeline.PendingInput, error) {
				return &pbpipeline.PendingInput{
					ActivityId: id,
					Status:     pbpipeline.PendingInput_STATUS_COMPLETED,
					InputData: map[string]string{
						"title":       "User Title",
						"description": "User Desc",
					},
				}, nil
			},
		}
		provider := &UserInputProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pbactivity.StandardizedActivity{Source: pbactivity.ActivitySource_SOURCE_HEVY, ExternalId: "123"}
		user := &user.Record{UserProfile: &pbuser.UserProfile{UserId: "test-user-1"}}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, nil, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("Expected result, got nil")
		}
		if res.Name != "User Title" {
			t.Errorf("Expected Name 'User Title', got %s", res.Name)
		}
		if res.Description != "User Desc" {
			t.Errorf("Expected Desc 'User Desc', got %s", res.Description)
		}
	})
}
