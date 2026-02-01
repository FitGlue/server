package running_dynamics

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func intPointer(v int32) *int32 {
	return &v
}

func floatPointer(v float64) *float64 {
	return &v
}

func TestRunningDynamics_Enrich_Success(t *testing.T) {
	provider := NewRunningDynamics()
	provider.Service = &bootstrap.Service{}

	// Create activity with running dynamics data
	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now()),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{
								GroundContactTime:   intPointer(220),
								VerticalOscillation: intPointer(80),
								StepLength:          floatPointer(1.05),
							},
							{
								GroundContactTime:   intPointer(230),
								VerticalOscillation: intPointer(90),
								StepLength:          floatPointer(1.15),
							},
						},
					},
				},
			},
		},
	}

	user := &pb.UserRecord{UserId: "test-user"}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify metadata
	if result.Metadata["running_dynamics_status"] != "success" {
		t.Errorf("Expected running_dynamics_status=success, got %s", result.Metadata["running_dynamics_status"])
	}

	// Verify description contains expected strings
	// GCT: (220+230)/2 = 225
	// SL: (1.05+1.15)/2 = 1.10
	// VO: (80+90)/2 = 85 -> 8.5 cm (actually I had avg/10.0 in my coach, let's check)
	// My code was: fmt.Sprintf("↕️ Vert: %.1f cm", avg/10.0)
	// For 85 mm average, avg/10.0 is 8.5.

	expectedParts := []string{"225 ms", "1.10 m", "8.5 cm"}
	for _, part := range expectedParts {
		if !contains(result.Description, part) {
			t.Errorf("Expected description to contain %q, but got %q", part, result.Description)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}

func TestRunningDynamics_Enrich_NoData(t *testing.T) {
	provider := NewRunningDynamics()
	provider.Service = &bootstrap.Service{}

	activity := &pb.StandardizedActivity{
		Sessions: []*pb.Session{
			{
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{},
							{},
						},
					},
				},
			},
		},
	}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, nil, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result.Metadata["running_dynamics_status"] != "skipped" {
		t.Errorf("Expected skipped status, got %s", result.Metadata["running_dynamics_status"])
	}
}
