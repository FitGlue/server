package streak_tracker

import (
	"context"
	"log/slog"
	"testing"

	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func makeActivity(dateStr string) *pb.StandardizedActivity {
	t, _ := time.Parse("2006-01-02", dateStr)
	return &pb.StandardizedActivity{
		Name:      "Morning Run",
		StartTime: timestamppb.New(t),
	}
}

func TestStreakTracker_FirstActivity(t *testing.T) {
	var savedData map[string]interface{}
	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return nil, nil // No existing data
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			savedData = data
			return nil
		},
	}

	provider := &StreakTracker{}
	provider.SetService(&bootstrap.Service{DB: mockDB})

	res, err := provider.Enrich(context.Background(), slog.Default(), makeActivity("2026-02-08"), &pb.UserRecord{UserId: "u1"}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Metadata["streak_current"] != "1" {
		t.Errorf("Expected streak_current=1, got %s", res.Metadata["streak_current"])
	}
	if savedData == nil {
		t.Fatal("Expected SetBoosterData to be called")
	}
	if savedData["current_streak"] != 1 {
		t.Errorf("Expected persisted current_streak=1, got %v", savedData["current_streak"])
	}
}

func TestStreakTracker_ConsecutiveDay(t *testing.T) {
	var savedData map[string]interface{}
	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"current_streak":     float64(5),
				"longest_streak":     float64(5),
				"last_activity_date": "2026-02-07",
			}, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			savedData = data
			return nil
		},
	}

	provider := &StreakTracker{}
	provider.SetService(&bootstrap.Service{DB: mockDB})

	res, err := provider.Enrich(context.Background(), slog.Default(), makeActivity("2026-02-08"), &pb.UserRecord{UserId: "u1"}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Metadata["streak_current"] != "6" {
		t.Errorf("Expected streak_current=6, got %s", res.Metadata["streak_current"])
	}
	if savedData["current_streak"] != 6 {
		t.Errorf("Expected persisted current_streak=6, got %v", savedData["current_streak"])
	}
}

func TestStreakTracker_SameDayDuplicate(t *testing.T) {
	var setCalled bool
	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"current_streak":     float64(5),
				"longest_streak":     float64(5),
				"last_activity_date": "2026-02-08",
			}, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			setCalled = true
			return nil
		},
	}

	provider := &StreakTracker{}
	provider.SetService(&bootstrap.Service{DB: mockDB})

	res, err := provider.Enrich(context.Background(), slog.Default(), makeActivity("2026-02-08"), &pb.UserRecord{UserId: "u1"}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Metadata["streak_current"] != "5" {
		t.Errorf("Expected streak_current=5 (no increment), got %s", res.Metadata["streak_current"])
	}
	if setCalled {
		t.Error("Expected SetBoosterData NOT to be called for same-day duplicate")
	}
}

func TestStreakTracker_StreakBroken(t *testing.T) {
	var savedData map[string]interface{}
	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"current_streak":     float64(10),
				"longest_streak":     float64(10),
				"last_activity_date": "2026-02-05", // 3 days ago
			}, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			savedData = data
			return nil
		},
	}

	provider := &StreakTracker{}
	provider.SetService(&bootstrap.Service{DB: mockDB})

	res, err := provider.Enrich(context.Background(), slog.Default(), makeActivity("2026-02-08"), &pb.UserRecord{UserId: "u1"}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Metadata["streak_current"] != "1" {
		t.Errorf("Expected streak_current=1 (reset), got %s", res.Metadata["streak_current"])
	}
	if savedData["current_streak"] != 1 {
		t.Errorf("Expected persisted current_streak=1, got %v", savedData["current_streak"])
	}
}

func TestStreakTracker_OutOfOrderActivity(t *testing.T) {
	var setCalled bool
	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"current_streak":     float64(5),
				"longest_streak":     float64(5),
				"last_activity_date": "2026-02-08", // Today already recorded
			}, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			setCalled = true
			return nil
		},
	}

	provider := &StreakTracker{}
	provider.SetService(&bootstrap.Service{DB: mockDB})

	// Activity from yesterday arriving late (out-of-order)
	res, err := provider.Enrich(context.Background(), slog.Default(), makeActivity("2026-02-07"), &pb.UserRecord{UserId: "u1"}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Metadata["streak_current"] != "5" {
		t.Errorf("Expected streak_current=5 (unchanged), got %s", res.Metadata["streak_current"])
	}
	if setCalled {
		t.Error("Expected SetBoosterData NOT to be called for out-of-order activity")
	}
}

func TestStreakTracker_LongestStreakUpdates(t *testing.T) {
	var savedData map[string]interface{}
	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"current_streak":     float64(9),
				"longest_streak":     float64(9),
				"last_activity_date": "2026-02-07",
			}, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			savedData = data
			return nil
		},
	}

	provider := &StreakTracker{}
	provider.SetService(&bootstrap.Service{DB: mockDB})

	res, err := provider.Enrich(context.Background(), slog.Default(), makeActivity("2026-02-08"), &pb.UserRecord{UserId: "u1"}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Metadata["streak_current"] != "10" {
		t.Errorf("Expected streak_current=10, got %s", res.Metadata["streak_current"])
	}
	if res.Metadata["streak_longest"] != "10" {
		t.Errorf("Expected streak_longest=10, got %s", res.Metadata["streak_longest"])
	}
	if savedData["longest_streak"] != 10 {
		t.Errorf("Expected persisted longest_streak=10, got %v", savedData["longest_streak"])
	}
}

func TestStreakTracker_LongestStreakPreserved(t *testing.T) {
	var savedData map[string]interface{}
	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"current_streak":     float64(3),
				"longest_streak":     float64(15),  // Previous best was higher
				"last_activity_date": "2026-02-05", // Streak broken
			}, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			savedData = data
			return nil
		},
	}

	provider := &StreakTracker{}
	provider.SetService(&bootstrap.Service{DB: mockDB})

	res, err := provider.Enrich(context.Background(), slog.Default(), makeActivity("2026-02-08"), &pb.UserRecord{UserId: "u1"}, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Metadata["streak_current"] != "1" {
		t.Errorf("Expected streak_current=1 (reset), got %s", res.Metadata["streak_current"])
	}
	if res.Metadata["streak_longest"] != "15" {
		t.Errorf("Expected streak_longest=15 (preserved), got %s", res.Metadata["streak_longest"])
	}
	if savedData["longest_streak"] != 15 {
		t.Errorf("Expected persisted longest_streak=15, got %v", savedData["longest_streak"])
	}
}
