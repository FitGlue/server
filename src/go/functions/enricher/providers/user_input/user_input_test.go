package user_input

import (
	"context"
	"log/slog"
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestUserInput_Enrich(t *testing.T) {
	ctx := context.Background()

	t.Run("Returns WaitError if no pending input", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{
			GetPendingInputFunc: func(ctx context.Context, userId string, id string) (*pb.PendingInput, error) {
				return nil, nil // Not found
			},
		}
		provider := &UserInputProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Source: "HEVY", ExternalId: "123"}
		user := &pb.UserRecord{UserId: "test-user-1"}
		inputs := map[string]string{}

		_, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)

		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		waitErr, ok := err.(*WaitForInputError)
		if !ok {
			t.Fatalf("Expected WaitForInputError, got %T", err)
		}
		if waitErr.ActivityID != "HEVY:123:user_input" {
			t.Errorf("Expected ActivityID 'HEVY:123:user_input', got %s", waitErr.ActivityID)
		}
		if waitErr.EnricherProviderID != "user_input" {
			t.Errorf("Expected EnricherProviderID 'user_input', got %s", waitErr.EnricherProviderID)
		}
	})

	t.Run("Returns WaitError if status WAITING", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{
			GetPendingInputFunc: func(ctx context.Context, userId string, id string) (*pb.PendingInput, error) {
				return &pb.PendingInput{
					ActivityId: id,
					Status:     pb.PendingInput_STATUS_WAITING,
				}, nil
			},
		}
		provider := &UserInputProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Source: "HEVY", ExternalId: "123"}
		user := &pb.UserRecord{UserId: "test-user-1"}

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
			GetPendingInputFunc: func(ctx context.Context, userId string, id string) (*pb.PendingInput, error) {
				return &pb.PendingInput{
					ActivityId: id,
					Status:     pb.PendingInput_STATUS_COMPLETED,
					InputData: map[string]string{
						"title":       "User Title",
						"description": "User Desc",
					},
				}, nil
			},
		}
		provider := &UserInputProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Source: "HEVY", ExternalId: "123"}
		user := &pb.UserRecord{UserId: "test-user-1"}

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
