package file_generators

import (
	"bytes"
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/typedef"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGenerateFitFile(t *testing.T) {
	// Setup input
	startTime := timestamppb.New(time.Now())
	activity := &pb.StandardizedActivity{
		StartTime: startTime,
		Type:      pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
		Sessions: []*pb.Session{
			{
				StartTime:        startTime,
				TotalElapsedTime: 3,
				Laps: []*pb.Lap{
					{
						Records: []*pb.Record{
							{Timestamp: startTime, HeartRate: 140},
							{Timestamp: startTime, HeartRate: 145}, // Logic usually adds seconds, simplified here
							{Timestamp: startTime, HeartRate: 150},
						},
					},
				},
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
	// Exec
	result, err := GenerateFitFile(activity)

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
	var recordCount, setCount, sessionCount, activityCount, lapCount int
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
		case typedef.MesgNumLap:
			lapCount++
		}
	}

	// Expectations
	if recordCount != 3 {
		t.Errorf("Expected 3 Record messages, got %d", recordCount)
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
	if lapCount != 1 {
		t.Errorf("Expected 1 Lap message, got %d", lapCount)
	}
}

func TestGenerateFitFile_NoHR(t *testing.T) {
	startTime := timestamppb.New(time.Now())
	activity := &pb.StandardizedActivity{
		StartTime: startTime,
		Sessions: []*pb.Session{
			{
				StartTime:        startTime,
				TotalElapsedTime: 10,
				StrengthSets: []*pb.StrengthSet{
					{
						ExerciseName:    "Bench Press",
						Reps:            10,
						WeightKg:        100,
						DurationSeconds: 10,
					},
				},
			},
		},
	}
	// Empty HR stream - no longer needed as an argument

	// Generate FIT file
	fitFileBytes, err := GenerateFitFile(activity)
	if err != nil {
		t.Fatalf("GenerateFitFile failed: %v", err)
	}

	fitDecoder := decoder.New(bytes.NewReader(fitFileBytes))
	fitData, err := fitDecoder.Decode()
	if err != nil {
		t.Fatalf("Failed to decode generated FIT file: %v", err)
	}

	var recordCount int
	for _, msg := range fitData.Messages {
		if msg.Num == typedef.MesgNumRecord {
			recordCount++
		}
	}

	if recordCount != 10 {
		t.Errorf("Expected 10 Record messages (synthesized), got %d", recordCount)
	}
}
