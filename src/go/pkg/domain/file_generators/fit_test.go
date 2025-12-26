package file_generators

import (
	"bytes"
	"testing"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/typedef"
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

	// Verify FIT file content by decoding it
	fitDecoder := decoder.New(bytes.NewReader(result))
	fitData, err := fitDecoder.Decode()
	if err != nil {
		t.Fatalf("Failed to decode generated FIT file: %v", err)
	}

	// Count messages
	var recordCount, setCount, sessionCount, activityCount int
	for _, msg := range fitData.Messages {
		switch msg.Num {
		case typedef.MesgNumRecord:
			recordCount++
		case typedef.MesgNumSet:
			setCount++
		case typedef.MesgNumSession:
			sessionCount++
		case typedef.MesgNumActivity:
			activityCount++
		}
	}

	// Expectations
	if recordCount != len(hrStream) {
		t.Errorf("Expected %d Record messages, got %d", len(hrStream), recordCount)
	}
	if setCount != 1 {
		t.Errorf("Expected 1 Set message, got %d", setCount)
	}
	if sessionCount != 1 {
		t.Errorf("Expected 1 Session message, got %d", sessionCount)
	}
	if activityCount != 1 {
		t.Errorf("Expected 1 Activity message, got %d", activityCount)
	}
}
