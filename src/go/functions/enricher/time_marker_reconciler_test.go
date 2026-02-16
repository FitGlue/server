package enricher

import (
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestReconcileTimeMarkerLabels(t *testing.T) {
	baseTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		activity       *pb.StandardizedActivity
		expectedLabels []string // expected labels after reconciliation
	}{
		{
			name:     "nil activity",
			activity: nil,
		},
		{
			name: "no markers",
			activity: &pb.StandardizedActivity{
				Sessions: []*pb.Session{{
					StrengthSets: []*pb.StrengthSet{
						{ExerciseName: "Bench Press", StartTime: timestamppb.New(baseTime)},
					},
				}},
			},
		},
		{
			name: "no strength sets - keeps original labels",
			activity: &pb.StandardizedActivity{
				TimeMarkers: []*pb.TimeMarker{
					{Label: "Cardio", Timestamp: timestamppb.New(baseTime), MarkerType: "exercise_start"},
				},
				Sessions: []*pb.Session{{}},
			},
			expectedLabels: []string{"Cardio"},
		},
		{
			name: "relabels markers with better names from StrengthSets",
			activity: &pb.StandardizedActivity{
				TimeMarkers: []*pb.TimeMarker{
					{Label: "Sit Up", Timestamp: timestamppb.New(baseTime), MarkerType: "exercise_start"},
					{Label: "Cardio", Timestamp: timestamppb.New(baseTime.Add(5 * time.Minute)), MarkerType: "exercise_start"},
					{Label: "Flye", Timestamp: timestamppb.New(baseTime.Add(10 * time.Minute)), MarkerType: "exercise_start"},
				},
				Sessions: []*pb.Session{{
					StrengthSets: []*pb.StrengthSet{
						{ExerciseName: "Weighted Crunches", StartTime: timestamppb.New(baseTime.Add(10 * time.Second))},
						{ExerciseName: "Weighted Crunches", StartTime: timestamppb.New(baseTime.Add(2 * time.Minute))},
						{ExerciseName: "Jump Rope", StartTime: timestamppb.New(baseTime.Add(5 * time.Minute))},
						{ExerciseName: "Dumbbell Fly", StartTime: timestamppb.New(baseTime.Add(10 * time.Minute))},
					},
				}},
			},
			expectedLabels: []string{"Weighted Crunches", "Jump Rope", "Dumbbell Fly"},
		},
		{
			name: "does not update if same name",
			activity: &pb.StandardizedActivity{
				TimeMarkers: []*pb.TimeMarker{
					{Label: "Bench Press", Timestamp: timestamppb.New(baseTime), MarkerType: "exercise_start"},
				},
				Sessions: []*pb.Session{{
					StrengthSets: []*pb.StrengthSet{
						{ExerciseName: "Bench Press", StartTime: timestamppb.New(baseTime)},
					},
				}},
			},
			expectedLabels: []string{"Bench Press"},
		},
		{
			name: "ignores sets beyond 5 minute window",
			activity: &pb.StandardizedActivity{
				TimeMarkers: []*pb.TimeMarker{
					{Label: "Original Label", Timestamp: timestamppb.New(baseTime), MarkerType: "exercise_start"},
				},
				Sessions: []*pb.Session{{
					StrengthSets: []*pb.StrengthSet{
						{ExerciseName: "Far Away Set", StartTime: timestamppb.New(baseTime.Add(10 * time.Minute))},
					},
				}},
			},
			expectedLabels: []string{"Original Label"},
		},
		{
			name: "position-based fallback when all sets share same timestamp (Hevy scenario)",
			activity: &pb.StandardizedActivity{
				TimeMarkers: []*pb.TimeMarker{
					{Label: "Sit Up", Timestamp: timestamppb.New(baseTime), MarkerType: "exercise_start"},
					{Label: "Cardio", Timestamp: timestamppb.New(baseTime.Add(5 * time.Minute)), MarkerType: "exercise_start"},
					{Label: "Flye", Timestamp: timestamppb.New(baseTime.Add(10 * time.Minute)), MarkerType: "exercise_start"},
				},
				Sessions: []*pb.Session{{
					StrengthSets: []*pb.StrengthSet{
						// All sets have the SAME startTime (workout start) - this is what Hevy does
						{ExerciseName: "Weighted Crunches", StartTime: timestamppb.New(baseTime)},
						{ExerciseName: "Weighted Crunches", StartTime: timestamppb.New(baseTime)},
						{ExerciseName: "Weighted Crunches", StartTime: timestamppb.New(baseTime)},
						{ExerciseName: "Jump Rope", StartTime: timestamppb.New(baseTime)},
						{ExerciseName: "Jump Rope", StartTime: timestamppb.New(baseTime)},
						{ExerciseName: "Dumbbell Fly", StartTime: timestamppb.New(baseTime)},
						{ExerciseName: "Dumbbell Fly", StartTime: timestamppb.New(baseTime)},
					},
				}},
			},
			expectedLabels: []string{"Weighted Crunches", "Jump Rope", "Dumbbell Fly"},
		},
		{
			name: "position-based fallback with more markers than exercise groups",
			activity: &pb.StandardizedActivity{
				TimeMarkers: []*pb.TimeMarker{
					{Label: "Exercise A", Timestamp: timestamppb.New(baseTime), MarkerType: "exercise_start"},
					{Label: "Exercise B", Timestamp: timestamppb.New(baseTime.Add(5 * time.Minute)), MarkerType: "exercise_start"},
					{Label: "Exercise C", Timestamp: timestamppb.New(baseTime.Add(10 * time.Minute)), MarkerType: "exercise_start"},
				},
				Sessions: []*pb.Session{{
					StrengthSets: []*pb.StrengthSet{
						{ExerciseName: "Bench Press", StartTime: timestamppb.New(baseTime)},
						{ExerciseName: "Squat", StartTime: timestamppb.New(baseTime)},
					},
				}},
			},
			// Only first 2 markers get relabeled; third keeps original
			expectedLabels: []string{"Bench Press", "Squat", "Exercise C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconcileTimeMarkerLabels(tt.activity)

			if tt.activity == nil || len(tt.expectedLabels) == 0 {
				return
			}

			if len(tt.activity.TimeMarkers) != len(tt.expectedLabels) {
				t.Fatalf("Expected %d markers, got %d", len(tt.expectedLabels), len(tt.activity.TimeMarkers))
			}

			for i, expected := range tt.expectedLabels {
				if tt.activity.TimeMarkers[i].Label != expected {
					t.Errorf("Marker %d: expected label %q, got %q", i, expected, tt.activity.TimeMarkers[i].Label)
				}
			}
		})
	}
}
