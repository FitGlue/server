package recovery_advisor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/testing/mocks"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func makeActivity(durationMinutes int, heartRate int32) *pb.StandardizedActivity {
	var records []*pb.Record
	for i := 0; i < durationMinutes; i++ {
		records = append(records, &pb.Record{
			HeartRate: heartRate,
		})
	}
	return &pb.StandardizedActivity{
		Name: "Test Activity",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: float64(durationMinutes * 60),
				Laps: []*pb.Lap{
					{Records: records},
				},
			},
		},
	}
}

func TestRecoveryAdvisor_SingleActivity(t *testing.T) {
	provider := NewRecoveryAdvisor()
	provider.Service = &bootstrap.Service{}

	activity := makeActivity(30, 150)
	user := &pb.UserRecord{UserId: "test-user"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if !strings.Contains(result.Description, "💤 Recovery Advisor") {
		t.Errorf("Missing header in description: %s", result.Description)
	}
	if !strings.Contains(result.Description, "Session load:") {
		t.Errorf("Missing session load in description: %s", result.Description)
	}
	if !strings.Contains(result.Description, "7-day load:") {
		t.Errorf("Missing 7-day load in description: %s", result.Description)
	}
	if !strings.Contains(result.Description, "Suggested recovery:") {
		t.Errorf("Missing recovery suggestion in description: %s", result.Description)
	}
	if result.Metadata["trimp"] == "" || result.Metadata["trimp"] == "0" {
		t.Errorf("Expected non-zero trimp, got %s", result.Metadata["trimp"])
	}
}

func TestRecoveryAdvisor_MultiActivitySameDay(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	existingTrimp := 80.0

	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			return map[string]interface{}{
				today:         existingTrimp,
				"last_update": time.Now().Format(time.RFC3339),
			}, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			savedLoad, ok := data[today].(float64)
			if !ok {
				t.Errorf("Expected today's load to be float64, got %T", data[today])
				return nil
			}
			// The saved load must be greater than the existing TRIMP (accumulated)
			if savedLoad <= existingTrimp {
				t.Errorf("Expected accumulated load > %.0f, got %.0f (overwrite bug!)", existingTrimp, savedLoad)
			}
			return nil
		},
	}

	provider := NewRecoveryAdvisor()
	provider.Service = &bootstrap.Service{DB: mockDB}

	// Second activity of the day: 20 min at 140 bpm
	activity := makeActivity(20, 140)
	user := &pb.UserRecord{UserId: "test-user"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// The 7-day load should include the existing TRIMP for today
	weeklyLoad := result.Metadata["weekly_load"]
	if weeklyLoad == "" {
		t.Fatal("Expected weekly_load in metadata")
	}
	// Weekly load should be at least the existing TRIMP (80) + this activity's TRIMP
	var weekly float64
	fmt.Sscanf(weeklyLoad, "%f", &weekly)
	if weekly <= existingTrimp {
		t.Errorf("Weekly load %.0f should be > existing today TRIMP %.0f", weekly, existingTrimp)
	}
}

func TestRecoveryAdvisor_NoHeartRate(t *testing.T) {
	provider := NewRecoveryAdvisor()
	provider.Service = &bootstrap.Service{}

	// Activity with no HR data - should fall back to duration-based estimation
	activity := &pb.StandardizedActivity{
		Name: "No HR Activity",
		Sessions: []*pb.Session{
			{
				TotalElapsedTime: 1800, // 30 minutes
				Laps: []*pb.Lap{
					{Records: []*pb.Record{{HeartRate: 0}}},
				},
			},
		},
	}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, &pb.UserRecord{UserId: "test"}, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Duration-based: 30 min * 0.5 = 15 TRIMP
	if result.Metadata["trimp"] != "15" {
		t.Errorf("Expected trimp 15 for no-HR fallback, got %s", result.Metadata["trimp"])
	}
	if result.Metadata["intensity"] != "Easy" {
		t.Errorf("Expected Easy intensity, got %s", result.Metadata["intensity"])
	}
}

func TestRecoveryAdvisor_HighWeeklyLoad(t *testing.T) {
	now := time.Now()

	mockDB := &mocks.MockDatabase{
		GetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string) (map[string]interface{}, error) {
			// Simulate heavy training week: ~100 TRIMP per day for 6 days = 600
			data := map[string]interface{}{}
			for i := 1; i <= 6; i++ {
				dateKey := now.AddDate(0, 0, -i).Format("2006-01-02")
				data[dateKey] = 100.0
			}
			return data, nil
		},
		SetBoosterDataFunc: func(ctx context.Context, userId string, boosterId string, data map[string]interface{}) error {
			return nil
		},
	}

	provider := NewRecoveryAdvisor()
	provider.Service = &bootstrap.Service{DB: mockDB}

	// Hard session: 60 min at 170 bpm
	activity := makeActivity(60, 170)
	user := &pb.UserRecord{UserId: "test-user"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Weekly load should be > 500, triggering +12h adjustment
	var weekly float64
	fmt.Sscanf(result.Metadata["weekly_load"], "%f", &weekly)
	if weekly <= 500 {
		t.Errorf("Expected weekly load > 500, got %.0f", weekly)
	}

	// Recovery hours should include the +12h high-load adjustment
	// 60min@170bpm ≈ 88 TRIMP (Moderate = 24h base + 12h high-load = 36h)
	var hours float64
	fmt.Sscanf(result.Metadata["recovery_hours"], "%f", &hours)
	if hours < 36 {
		t.Errorf("Expected >= 36h recovery with high weekly load, got %.0f", hours)
	}
}

func TestGetRecoveryRecommendation(t *testing.T) {
	tests := []struct {
		trimp      float64
		weeklyLoad float64
		wantHours  float64
		wantLabel  string
	}{
		{30, 200, 12, "Easy"},
		{75, 300, 24, "Moderate"},
		{120, 400, 36, "Hard"},
		{200, 400, 48, "Very Hard"},
		// High weekly load adjustments
		{30, 600, 24, "Easy"},       // 12 + 12
		{120, 600, 48, "Hard"},      // 36 + 12
		{200, 600, 60, "Very Hard"}, // 48 + 12
	}

	for _, tt := range tests {
		hours, intensity := getRecoveryRecommendation(tt.trimp, tt.weeklyLoad)
		if hours != tt.wantHours {
			t.Errorf("trimp=%.0f weeklyLoad=%.0f: got hours=%.0f, want %.0f", tt.trimp, tt.weeklyLoad, hours, tt.wantHours)
		}
		if intensity != tt.wantLabel {
			t.Errorf("trimp=%.0f weeklyLoad=%.0f: got intensity=%s, want %s", tt.trimp, tt.weeklyLoad, intensity, tt.wantLabel)
		}
	}
}

func TestFormatRecoveryTime(t *testing.T) {
	tests := []struct {
		hours    float64
		expected string
	}{
		{12, "12 hours"},
		{24, "24 hours (1 day)"},
		{36, "36 hours (1 day)"},
		{48, "48 hours (2 days)"},
		{60, "60 hours (2 days)"},
	}

	for _, tt := range tests {
		result := formatRecoveryTime(tt.hours)
		if result != tt.expected {
			t.Errorf("formatRecoveryTime(%.0f) = %q, want %q", tt.hours, result, tt.expected)
		}
	}
}
