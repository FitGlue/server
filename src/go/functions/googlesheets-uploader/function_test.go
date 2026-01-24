package googlesheetsuploader

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestBuildSheetRow_BasicActivity(t *testing.T) {
	event := &pb.EnrichedActivityEvent{
		ActivityId:   "test-123",
		Name:         "Morning Run",
		Description:  "A quick morning jog",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
	}

	row := buildSheetRow(event, false, false)

	if len(row) == 0 {
		t.Error("Expected non-empty row")
	}

	// Check that activity name is in row
	found := false
	for _, val := range row {
		if val == "Morning Run" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected activity name in row")
	}
}
