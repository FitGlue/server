package cadence_summary

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

func TestCadenceSummary_Enrich_Success(t *testing.T) {
	provider := NewCadenceSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity with cadence data
	activity := &pbactivity.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Ride",
		Type:        pbactivity.ActivityType_ACTIVITY_TYPE_RIDE,
		Sessions: []*pbactivity.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pbactivity.Lap{
					{
						Records: []*pbactivity.Record{
							{Cadence: 80},
							{Cadence: 85},
							{Cadence: 90},
							{Cadence: 95},
							{Cadence: 100},
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
	if result.Metadata["cadence_summary_status"] != "success" {
		t.Errorf("Expected cadence_summary_status=success, got %s", result.Metadata["cadence_summary_status"])
	}
	if result.Metadata["cadence_avg"] != "90" {
		t.Errorf("Expected cadence_avg=90, got %s", result.Metadata["cadence_avg"])
	}
	if result.Metadata["cadence_max"] != "100" {
		t.Errorf("Expected cadence_max=100, got %s", result.Metadata["cadence_max"])
	}

	// Verify description is appended
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	if result.Description == "Morning Ride" {
		t.Error("Expected description to be appended with cadence summary")
	}
}

func TestCadenceSummary_Enrich_RunningActivity(t *testing.T) {
	provider := NewCadenceSummary()
	provider.Service = &bootstrap.Service{}

	// Create running activity with cadence data
	activity := &pbactivity.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Type:        pbactivity.ActivityType_ACTIVITY_TYPE_RUN,
		Sessions: []*pbactivity.Session{
			{
				TotalElapsedTime: 1800,
				Laps: []*pbactivity.Lap{
					{
						Records: []*pbactivity.Record{
							{Cadence: 170},
							{Cadence: 175},
							{Cadence: 180},
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

	// Verify description contains "spm" for running
	if result.Description == "" {
		t.Error("Expected non-empty description")
	}
	// The description should contain "spm" not "rpm"
	if !contains(result.Description, "spm") {
		t.Error("Expected description to contain 'spm' for running activity")
	}
}

func TestCadenceSummary_Enrich_NoCadenceData(t *testing.T) {
	provider := NewCadenceSummary()
	provider.Service = &bootstrap.Service{}

	// Create activity without cadence data
	activity := &pbactivity.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pbactivity.Session{
			{
				TotalElapsedTime: 3600,
				Laps: []*pbactivity.Lap{
					{
						Records: []*pbactivity.Record{
							{Cadence: 0},
							{Cadence: 0},
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

	if result.Metadata["cadence_summary_status"] != "skipped" {
		t.Errorf("Expected cadence_summary_status=skipped, got %s", result.Metadata["cadence_summary_status"])
	}
}

func TestCadenceSummary_Name(t *testing.T) {
	provider := NewCadenceSummary()
	expected := "cadence-summary"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestCadenceSummary_ProviderType(t *testing.T) {
	provider := NewCadenceSummary()
	expected := pbplugin.EnricherProviderType_ENRICHER_PROVIDER_CADENCE_SUMMARY
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
