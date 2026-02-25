package power_summary

import (
	user "github.com/fitglue/server/src/go/pkg/domain/user"

	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"

	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"

	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPowerSummary_Enrich_Success(t *testing.T) {
	provider := NewPowerSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity with power data
	activity := &pbactivity.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Ride",
		Sessions: []*pbactivity.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pbactivity.Lap{
					{
						Records: []*pbactivity.Record{
							{Power: 200},
							{Power: 250},
							{Power: 300},
							{Power: 350},
							{Power: 400},
						},
					},
				},
			},
		},
	}

	user := &user.Record{UserProfile: &pbuser.UserProfile{UserId: "test-user"}}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify metadata
	if result.Metadata["power_summary_status"] != "success" {
		t.Errorf("Expected power_summary_status=success, got %s", result.Metadata["power_summary_status"])
	}
	if result.Metadata["power_avg"] != "300" {
		t.Errorf("Expected power_avg=300, got %s", result.Metadata["power_avg"])
	}
	if result.Metadata["power_max"] != "400" {
		t.Errorf("Expected power_max=400, got %s", result.Metadata["power_max"])
	}
	if result.Metadata["power_sample_count"] != "5" {
		t.Errorf("Expected power_sample_count=5, got %s", result.Metadata["power_sample_count"])
	}

	// Verify description is appended
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	if result.Description == "Morning Ride" {
		t.Error("Expected description to be appended with power summary")
	}
}

func TestPowerSummary_Enrich_NoPowerData(t *testing.T) {
	provider := NewPowerSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity without power data
	activity := &pbactivity.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pbactivity.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pbactivity.Lap{
					{
						Records: []*pbactivity.Record{
							{Power: 0},
							{Power: 0},
						},
					},
				},
			},
		},
	}

	user := &user.Record{UserProfile: &pbuser.UserProfile{UserId: "test-user"}}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result.Metadata["power_summary_status"] != "skipped" {
		t.Errorf("Expected power_summary_status=skipped, got %s", result.Metadata["power_summary_status"])
	}
}

func TestPowerSummary_Name(t *testing.T) {
	provider := NewPowerSummary()
	expected := "power-summary"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestPowerSummary_ProviderType(t *testing.T) {
	provider := NewPowerSummary()
	expected := pbplugin.EnricherProviderType_ENRICHER_PROVIDER_POWER_SUMMARY
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}
