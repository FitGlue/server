package file_generators

import (
	"testing"
	"time"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestGenerateFitFile(t *testing.T) {
	// Setup input
	startTime := time.Now().Format(time.RFC3339)
	activity := &pb.StandardizedActivity{
		StartTime: startTime,
		Sessions: []*pb.Session{
			{
				StartTime:        startTime,
				TotalElapsedTime: 3600,
				StrengthSets: []*pb.StrengthSet{
					{
						ExerciseName:    "Bench Press",
						Reps:            10,
						WeightKg:        100,
						DurationSeconds: 60,
					},
				},
			},
		},
	}
	hrStream := []int{140, 145, 150}

	// Exec
	result, err := GenerateFitFile(activity, hrStream)

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty FIT file result")
	}

	// Basic check for FIT header (first 4 bytes are size, usually 12 or 14, then protocol version)
	// Byte 8-11 is ".FIT"
	if len(result) < 14 {
		t.Errorf("Result too short to be a FIT file: %d bytes", len(result))
	} else {
		fileType := string(result[8:12])
		if fileType != ".FIT" {
			t.Errorf("Expected .FIT file type in header, got %q", fileType)
		}
	}
}
