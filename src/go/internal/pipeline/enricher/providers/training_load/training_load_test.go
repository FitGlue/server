package training_load

import (
	user "github.com/fitglue/server/src/go/pkg/domain/user"

	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"

	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTrainingLoad_Enrich_Success(t *testing.T) {
	provider := NewTrainingLoad()
	provider.Service = &bootstrap.Service{}

	now := time.Now()
	// Create activity with 10 minutes of steady heart rate
	// 10 records, each 1 minute apart
	var records []*pbactivity.Record
	for i := 0; i < 11; i++ {
		records = append(records, &pbactivity.Record{
			Timestamp: timestamppb.New(now.Add(time.Duration(i) * time.Minute)),
			HeartRate: 150,
		})
	}

	activity := &pbactivity.StandardizedActivity{
		Description: "Stable Run",
		Sessions: []*pbactivity.Session{
			{
				Laps: []*pbactivity.Lap{
					{
						Records: records,
					},
				},
			},
		},
	}

	user := &user.Record{UserProfile: &pbuser.UserProfile{UserId: "test-user"}}
	inputs := map[string]string{
		"max_hr":  "190",
		"rest_hr": "60",
		"gender":  "male",
	}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, inputs, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// 10 minutes duration at 150bpm
	// HRR = (150-60)/130 = 0.6923
	// 1 min TRIMP = 1 * 0.6923 * 0.64 * e^(1.92 * 0.6923) = 1.675
	// 10 min TRIMP = 16.75
	// Expected output: "17 (Recovery)"

	if result.Metadata["training_load_status"] != "success" {
		t.Errorf("Expected success, got %s", result.Metadata["training_load_status"])
	}

	if result.Metadata["trimp"] != "17" {
		t.Errorf("Expected trimp 17, got %s", result.Metadata["trimp"])
	}

	if !strings.Contains(result.Description, "💪 Training Load: 17 (Recovery)") {
		t.Errorf("Description mismatch: %s", result.Description)
	}
}

func TestTrainingLoad_Enrich_Zones(t *testing.T) {
	tests := []struct {
		trimp    float64
		expected string
	}{
		{15, "Recovery"},
		{45, "Easy"},
		{75, "Moderate"},
		{120, "Hard"},
		{300, "Very Hard"},
	}

	for _, tt := range tests {
		zone := getTrainingLoadZone(tt.trimp)
		if zone != tt.expected {
			t.Errorf("For TRIMP %.0f expected zone %s, got %s", tt.trimp, tt.expected, zone)
		}
	}
}

func TestTrainingLoad_Enrich_NoData(t *testing.T) {
	provider := NewTrainingLoad()
	provider.Service = &bootstrap.Service{}

	activity := &pbactivity.StandardizedActivity{
		Description: "No HR Run",
		Sessions: []*pbactivity.Session{
			{
				Laps: []*pbactivity.Lap{
					{
						Records: []*pbactivity.Record{
							{HeartRate: 0},
						},
					},
				},
			},
		},
	}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, &user.Record{}, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result.Metadata["training_load_status"] != "skipped" {
		t.Errorf("Expected skipped, got %s", result.Metadata["training_load_status"])
	}
}
