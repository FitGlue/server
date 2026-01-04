package auto_increment

import (
	"context"
	"testing"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/testing/mocks"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestAutoIncrement_Enrich(t *testing.T) {
	ctx := context.Background()

	t.Run("Skips if title filter mismatch", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{}
		provider := &AutoIncrementProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Name: "Afternoon Walk"}
		user := &pb.UserRecord{UserId: "u1"}
		inputs := map[string]string{
			"counter_key":    "parkrun",
			"title_contains": "Parkrun",
		}

		res, err := provider.Enrich(ctx, activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res != nil {
			t.Errorf("Expected nil result (skip), got %v", res)
		}
	})

	t.Run("Creates new counter if missing", func(t *testing.T) {
		var setCounter *pb.Counter
		mockDB := &mocks.MockDatabase{
			GetCounterFunc: func(ctx context.Context, userId, id string) (*pb.Counter, error) {
				return nil, nil // Not found
			},
			SetCounterFunc: func(ctx context.Context, userId string, counter *pb.Counter) error {
				setCounter = counter
				return nil
			},
		}

		provider := &AutoIncrementProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Name: "Parkrun"}
		user := &pb.UserRecord{UserId: "u1"}
		inputs := map[string]string{
			"counter_key":    "parkrun",
			"title_contains": "Parkrun",
		}

		res, err := provider.Enrich(ctx, activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("Expected result, got nil")
		}

		// Verify result
		if res.NameSuffix != " (#1)" {
			t.Errorf("Expected suffix ' (#1)', got '%s'", res.NameSuffix)
		}

		// Verify DB persistence
		if setCounter == nil {
			t.Fatal("Expected SetCounter to be called")
		}
		if setCounter.Count != 1 {
			t.Errorf("Expected persisted count 1, got %d", setCounter.Count)
		}
	})

	t.Run("Increments existing counter", func(t *testing.T) {
		var setCounter *pb.Counter
		mockDB := &mocks.MockDatabase{
			GetCounterFunc: func(ctx context.Context, userId, id string) (*pb.Counter, error) {
				return &pb.Counter{Id: "parkrun", Count: 5}, nil
			},
			SetCounterFunc: func(ctx context.Context, userId string, counter *pb.Counter) error {
				setCounter = counter
				return nil
			},
		}

		provider := &AutoIncrementProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Name: "Parkrun"}
		user := &pb.UserRecord{UserId: "u1"}
		inputs := map[string]string{
			"counter_key": "parkrun",
		}

		res, err := provider.Enrich(ctx, activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Verify result
		if res.NameSuffix != " (#6)" {
			t.Errorf("Expected suffix ' (#6)', got '%s'", res.NameSuffix)
		}

		// Verify DB persistence
		if setCounter.Count != 6 {
			t.Errorf("Expected persisted count 6, got %d", setCounter.Count)
		}
	})
}
