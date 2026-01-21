package trainingpeaksuploader

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestMapToTrainingPeaksType(t *testing.T) {
	tests := []struct {
		name         string
		activityType pb.ActivityType
		want         string
	}{
		{
			name:         "Run maps to Run",
			activityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
			want:         "Run",
		},
		{
			name:         "Trail Run maps to Run",
			activityType: pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN,
			want:         "Run",
		},
		{
			name:         "Virtual Run maps to Run",
			activityType: pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN,
			want:         "Run",
		},
		{
			name:         "Ride maps to Bike",
			activityType: pb.ActivityType_ACTIVITY_TYPE_RIDE,
			want:         "Bike",
		},
		{
			name:         "Virtual Ride maps to Bike",
			activityType: pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE,
			want:         "Bike",
		},
		{
			name:         "Mountain Bike Ride maps to Bike",
			activityType: pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE,
			want:         "Bike",
		},
		{
			name:         "Gravel Ride maps to Bike",
			activityType: pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE,
			want:         "Bike",
		},
		{
			name:         "eBike Ride maps to Bike",
			activityType: pb.ActivityType_ACTIVITY_TYPE_EBIKE_RIDE,
			want:         "Bike",
		},
		{
			name:         "eMountain Bike Ride maps to Bike",
			activityType: pb.ActivityType_ACTIVITY_TYPE_EMOUNTAIN_BIKE_RIDE,
			want:         "Bike",
		},
		{
			name:         "Swim maps to Swim",
			activityType: pb.ActivityType_ACTIVITY_TYPE_SWIM,
			want:         "Swim",
		},
		{
			name:         "Weight Training maps to Strength",
			activityType: pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
			want:         "Strength",
		},
		{
			name:         "Crossfit maps to Strength",
			activityType: pb.ActivityType_ACTIVITY_TYPE_CROSSFIT,
			want:         "Strength",
		},
		{
			name:         "HIIT maps to Strength",
			activityType: pb.ActivityType_ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING,
			want:         "Strength",
		},
		{
			name:         "Workout maps to Other",
			activityType: pb.ActivityType_ACTIVITY_TYPE_WORKOUT,
			want:         "Other",
		},
		{
			name:         "Walk maps to Other",
			activityType: pb.ActivityType_ACTIVITY_TYPE_WALK,
			want:         "Other",
		},
		{
			name:         "Hike maps to Other",
			activityType: pb.ActivityType_ACTIVITY_TYPE_HIKE,
			want:         "Other",
		},
		{
			name:         "Yoga maps to Other",
			activityType: pb.ActivityType_ACTIVITY_TYPE_YOGA,
			want:         "Other",
		},
		{
			name:         "Unspecified maps to Other",
			activityType: pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED,
			want:         "Other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapToTrainingPeaksType(tt.activityType)
			if got != tt.want {
				t.Errorf("mapToTrainingPeaksType(%v) = %q, want %q", tt.activityType, got, tt.want)
			}
		})
	}
}

func TestIsLoopOrigin(t *testing.T) {
	tests := []struct {
		name  string
		event *pb.EnrichedActivityEvent
		want  bool
	}{
		{
			name: "TrainingPeaks origin should be detected",
			event: &pb.EnrichedActivityEvent{
				EnrichmentMetadata: map[string]string{
					"origin_destination": "trainingpeaks",
				},
			},
			want: true,
		},
		{
			name: "Strava origin should not be detected",
			event: &pb.EnrichedActivityEvent{
				EnrichmentMetadata: map[string]string{
					"origin_destination": "strava",
				},
			},
			want: false,
		},
		{
			name: "No metadata should not be detected",
			event: &pb.EnrichedActivityEvent{
				EnrichmentMetadata: nil,
			},
			want: false,
		},
		{
			name: "Empty metadata should not be detected",
			event: &pb.EnrichedActivityEvent{
				EnrichmentMetadata: map[string]string{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLoopOrigin(tt.event)
			if got != tt.want {
				t.Errorf("isLoopOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildTrainingPeaksWorkout(t *testing.T) {
	tests := []struct {
		name  string
		event *pb.EnrichedActivityEvent
		check func(t *testing.T, w *TrainingPeaksWorkout)
	}{
		{
			name: "Basic workout with title and description",
			event: &pb.EnrichedActivityEvent{
				Name:         "Morning Run",
				Description:  "A nice morning jog",
				ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
			},
			check: func(t *testing.T, w *TrainingPeaksWorkout) {
				if w.Title != "Morning Run" {
					t.Errorf("Title = %q, want %q", w.Title, "Morning Run")
				}
				if w.Description != "A nice morning jog" {
					t.Errorf("Description = %q, want %q", w.Description, "A nice morning jog")
				}
				if w.WorkoutType != "Run" {
					t.Errorf("WorkoutType = %q, want %q", w.WorkoutType, "Run")
				}
			},
		},
		{
			name: "Workout with activity data",
			event: &pb.EnrichedActivityEvent{
				Name:         "Cycling Session",
				ActivityType: pb.ActivityType_ACTIVITY_TYPE_RIDE,
				ActivityData: &pb.StandardizedActivity{
					Sessions: []*pb.Session{
						{
							TotalElapsedTime: 3600,  // 1 hour
							TotalDistance:    25000, // 25km
						},
					},
				},
			},
			check: func(t *testing.T, w *TrainingPeaksWorkout) {
				if w.TotalTimePlanned != 3600 {
					t.Errorf("TotalTimePlanned = %v, want %v", w.TotalTimePlanned, 3600)
				}
				if w.DistancePlanned != 25000 {
					t.Errorf("DistancePlanned = %v, want %v", w.DistancePlanned, 25000)
				}
				if w.WorkoutType != "Bike" {
					t.Errorf("WorkoutType = %q, want %q", w.WorkoutType, "Bike")
				}
			},
		},
		{
			name: "Workout with heart rate data",
			event: &pb.EnrichedActivityEvent{
				Name:         "Interval Training",
				ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
				ActivityData: &pb.StandardizedActivity{
					Sessions: []*pb.Session{
						{
							Laps: []*pb.Lap{
								{
									Records: []*pb.Record{
										{HeartRate: 120},
										{HeartRate: 140},
										{HeartRate: 160},
										{HeartRate: 180},
									},
								},
							},
						},
					},
				},
			},
			check: func(t *testing.T, w *TrainingPeaksWorkout) {
				if w.HeartRateAvg == nil || *w.HeartRateAvg != 150 {
					var got interface{} = nil
					if w.HeartRateAvg != nil {
						got = *w.HeartRateAvg
					}
					t.Errorf("HeartRateAvg = %v, want %v", got, 150)
				}
				if w.HeartRateMax == nil || *w.HeartRateMax != 180 {
					var got interface{} = nil
					if w.HeartRateMax != nil {
						got = *w.HeartRateMax
					}
					t.Errorf("HeartRateMax = %v, want %v", got, 180)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := buildTrainingPeaksWorkout(tt.event)
			tt.check(t, w)
		})
	}
}
