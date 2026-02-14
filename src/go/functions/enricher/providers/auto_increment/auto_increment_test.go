package auto_increment

import (
	"context"
	"log/slog"
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
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

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("Expected non-nil result for skip, got nil")
		}
		if res.Metadata["auto_increment_applied"] != "false" {
			t.Errorf("Expected auto_increment_applied=false, got %v", res.Metadata["auto_increment_applied"])
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

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
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

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
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

	t.Run("New counter starts at 1", func(t *testing.T) {
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
			"counter_key": "parkrun",
		}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if res.NameSuffix != " (#1)" {
			t.Errorf("Expected suffix ' (#1)', got '%s'", res.NameSuffix)
		}

		if setCounter == nil {
			t.Fatal("Expected SetCounter to be called")
		}
		if setCounter.Count != 1 {
			t.Errorf("Expected persisted count 1, got %d", setCounter.Count)
		}
	})
	t.Run("Matches title case insensitive", func(t *testing.T) {
		var setCounter *pb.Counter
		mockDB := &mocks.MockDatabase{
			GetCounterFunc: func(ctx context.Context, userId, id string) (*pb.Counter, error) {
				return &pb.Counter{Id: "parkrun", Count: 0}, nil
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
			"title_contains": "parkrun", // Lowercase filter vs Uppercase Activity
		}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res == nil {
			t.Error("Expected result, got nil (failed case sensitivity)")
		} else if res.NameSuffix != " (#1)" {
			t.Errorf("Expected suffix ' (#1)', got '%s'", res.NameSuffix)
		}

		if setCounter == nil {
			t.Error("Expected SetCounter to be called")
		}
	})
}

func TestAutoIncrement_CounterRules(t *testing.T) {
	ctx := context.Background()

	t.Run("Matches counter_rules single rule", func(t *testing.T) {
		var setCounter *pb.Counter
		mockDB := &mocks.MockDatabase{
			GetCounterFunc: func(ctx context.Context, userId, id string) (*pb.Counter, error) {
				return nil, nil
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
			"counter_rules": `{"parkrun": "parkrun_counter"}`,
		}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.NameSuffix != " (#1)" {
			t.Errorf("Expected suffix ' (#1)', got '%s'", res.NameSuffix)
		}
		if res.Metadata["auto_increment_key"] != "parkrun_counter" {
			t.Errorf("Expected key 'parkrun_counter', got '%s'", res.Metadata["auto_increment_key"])
		}
		if setCounter == nil {
			t.Fatal("Expected SetCounter to be called")
		}
	})

	t.Run("No rule matches skips increment", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{}
		provider := &AutoIncrementProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Name: "Weight Training"}
		user := &pb.UserRecord{UserId: "u1"}
		inputs := map[string]string{
			"counter_rules": `{"parkrun": "parkrun_counter"}`,
		}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Metadata["auto_increment_applied"] != "false" {
			t.Errorf("Expected auto_increment_applied=false, got %v", res.Metadata["auto_increment_applied"])
		}
	})

	t.Run("Case-insensitive counter_rules matching", func(t *testing.T) {
		var setCounter *pb.Counter
		mockDB := &mocks.MockDatabase{
			GetCounterFunc: func(ctx context.Context, userId, id string) (*pb.Counter, error) {
				return &pb.Counter{Id: "leg_day", Count: 3}, nil
			},
			SetCounterFunc: func(ctx context.Context, userId string, counter *pb.Counter) error {
				setCounter = counter
				return nil
			},
		}

		provider := &AutoIncrementProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Name: "LEG DAY Session"}
		user := &pb.UserRecord{UserId: "u1"}
		inputs := map[string]string{
			"counter_rules": `{"leg day": "leg_day"}`,
		}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.NameSuffix != " (#4)" {
			t.Errorf("Expected suffix ' (#4)', got '%s'", res.NameSuffix)
		}
		if setCounter == nil {
			t.Fatal("Expected SetCounter to be called")
		}
	})

	t.Run("Empty counter_rules skips", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{}
		provider := &AutoIncrementProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Name: "Parkrun"}
		user := &pb.UserRecord{UserId: "u1"}
		inputs := map[string]string{
			"counter_rules": `{}`,
		}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Metadata["auto_increment_applied"] != "false" {
			t.Errorf("Expected auto_increment_applied=false, got %v", res.Metadata["auto_increment_applied"])
		}
	})

	t.Run("Invalid counter_rules JSON skips", func(t *testing.T) {
		mockDB := &mocks.MockDatabase{}
		provider := &AutoIncrementProvider{}
		provider.SetService(&bootstrap.Service{DB: mockDB})

		activity := &pb.StandardizedActivity{Name: "Parkrun"}
		user := &pb.UserRecord{UserId: "u1"}
		inputs := map[string]string{
			"counter_rules": `{invalid}`,
		}

		res, err := provider.Enrich(ctx, slog.Default(), activity, user, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.Metadata["auto_increment_applied"] != "false" {
			t.Errorf("Expected auto_increment_applied=false, got %v", res.Metadata["auto_increment_applied"])
		}
	})

}
