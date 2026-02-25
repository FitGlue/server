package virtual_gps

import (
	user "github.com/fitglue/server/src/go/pkg/domain/user"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"

	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestVirtualGPS_GeneratesDescription(t *testing.T) {
	provider := NewVirtualGPSProvider()

	activity := &pbactivity.StandardizedActivity{
		Sessions: []*pbactivity.Session{
			{
				TotalElapsedTime: 1800, // 30 minutes
				TotalDistance:    5000, // 5km
				Laps: []*pbactivity.Lap{
					{
						Records: []*pbactivity.Record{
							{Timestamp: timestamppb.New(time.Now())},
						},
					},
				},
			},
		},
	}

	config := map[string]string{
		"route": "london",
	}

	result, err := provider.Enrich(context.Background(), slog.Default(), activity, &user.Record{}, config, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Verify description is present
	if result.Description == "" {
		t.Error("Expected description to be set, got empty string")
	}

	// Verify description contains route name
	expectedSubstring := "London Hyde Park"
	if !strings.Contains(result.Description, expectedSubstring) {
		t.Errorf("Expected description to contain %q, got: %s", expectedSubstring, result.Description)
	}

	// Verify GPS streams are generated
	if len(result.PositionLatStream) != 1800 {
		t.Errorf("Expected 1800 lat points, got %d", len(result.PositionLatStream))
	}
	if len(result.PositionLongStream) != 1800 {
		t.Errorf("Expected 1800 long points, got %d", len(result.PositionLongStream))
	}
}
