package condition_matcher

import (
	"context"
	"testing"
	"time"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConditionMatcher_Enrich(t *testing.T) {
	ctx := context.Background()
	provider := NewConditionMatcherProvider()

	// Helper for time
	makeTime := func(day string, hour int) *timestamppb.Timestamp {
		// 2024-01-01 is Monday
		d := 1
		if day == "Tue" {
			d = 2
		} else if day == "Sat" {
			d = 6
		}
		return timestamppb.New(time.Date(2024, 1, d, hour, 0, 0, 0, time.UTC))
	}

	t.Run("Matches Type", func(t *testing.T) {
		act := &pb.StandardizedActivity{StartTime: makeTime("Mon", 10), Type: pb.ActivityType_ACTIVITY_TYPE_RUN}
		inputs := map[string]string{
			"activity_type":  "run", // Case insensitive check
			"title_template": "Matched Run",
		}
		res, _ := provider.Enrich(ctx, act, nil, inputs, false)
		if res == nil {
			t.Fatal("Expected match")
		}
		if res.Name != "Matched Run" {
			t.Errorf("Expected name 'Matched Run', got %s", res.Name)
		}
	})

	t.Run("Fails Type Mismatch", func(t *testing.T) {
		act := &pb.StandardizedActivity{StartTime: makeTime("Mon", 10), Type: pb.ActivityType_ACTIVITY_TYPE_SWIM}
		inputs := map[string]string{
			"activity_type": "run",
		}
		res, _ := provider.Enrich(ctx, act, nil, inputs, false)
		if res != nil {
			t.Errorf("Expected nil (no match), got %v", res)
		}
	})

	t.Run("Matches Day", func(t *testing.T) {
		// Sat 10am
		act := &pb.StandardizedActivity{StartTime: makeTime("Sat", 10)}
		inputs := map[string]string{
			"days":           "Sat,Sun",
			"title_template": "Weekend Warrior",
		}
		res, _ := provider.Enrich(ctx, act, nil, inputs, false)
		if res == nil {
			t.Fatal("Expected match")
		}
	})

	t.Run("Fails Day Mismatch", func(t *testing.T) {
		// Mon 10am
		act := &pb.StandardizedActivity{StartTime: makeTime("Mon", 10)}
		inputs := map[string]string{
			"days": "Sat,Sun",
		}
		res, _ := provider.Enrich(ctx, act, nil, inputs, false)
		if res != nil {
			t.Errorf("Expected nil, got %v", res)
		}
	})

	t.Run("Matches Location", func(t *testing.T) {
		// Bushy Park Coordinates (Roughly 51.41, -0.34)
		act := &pb.StandardizedActivity{
			StartTime: makeTime("Sat", 9),
			Sessions: []*pb.Session{
				{
					Laps: []*pb.Lap{
						{
							Records: []*pb.Record{
								{PositionLat: 51.41, PositionLong: -0.34},
							},
						},
					},
				},
			},
		}
		inputs := map[string]string{
			"location_lat":   "51.4101",
			"location_long":  "-0.3401",
			"radius_m":       "1000",
			"title_template": "Bushy Park",
		}
		res, _ := provider.Enrich(ctx, act, nil, inputs, false)
		if res == nil {
			t.Fatal("Expected match")
		}
	})

	t.Run("Fails Location Mismatch", func(t *testing.T) {
		// Far away
		act := &pb.StandardizedActivity{
			StartTime: makeTime("Sat", 9),
			Sessions: []*pb.Session{
				{
					Laps: []*pb.Lap{
						{
							Records: []*pb.Record{
								{PositionLat: 52.0, PositionLong: 0.0},
							},
						},
					},
				},
			},
		}
		inputs := map[string]string{
			"location_lat":  "51.41",
			"location_long": "-0.34",
			"radius_m":      "1000",
		}
		res, _ := provider.Enrich(ctx, act, nil, inputs, false)
		if res != nil {
			t.Errorf("Expected nil, got %v", res)
		}
	})

	t.Run("Matches Time Range", func(t *testing.T) {
		// 9am
		act := &pb.StandardizedActivity{StartTime: makeTime("Sat", 9)}
		inputs := map[string]string{
			"time_start":     "08:30",
			"time_end":       "09:30",
			"title_template": "Morning Run",
		}
		res, _ := provider.Enrich(ctx, act, nil, inputs, false)
		if res == nil {
			t.Fatal("Expected match")
		}
	})

	t.Run("Fails validation when only lat is provided", func(t *testing.T) {
		act := &pb.StandardizedActivity{StartTime: makeTime("Sat", 9)}
		inputs := map[string]string{
			"location_lat": "51.41",
		}
		_, err := provider.Enrich(ctx, act, nil, inputs, false)
		if err == nil {
			t.Fatal("Expected error when location_long is missing")
		}
		if err.Error() != "both location_lat and location_long are required for location proximity matching" {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Matches with Aliases", func(t *testing.T) {
		act := &pb.StandardizedActivity{
			StartTime: makeTime("Sat", 9),
			Sessions: []*pb.Session{
				{
					Laps: []*pb.Lap{
						{
							Records: []*pb.Record{
								{PositionLat: 51.41, PositionLong: -0.34},
							},
						},
					},
				},
			},
		}
		inputs := map[string]string{
			"days_of_week":    "Sat",
			"start_time":      "08:00",
			"end_time":        "10:00",
			"location_lat":    "51.41",
			"location_long":   "-0.34",
			"location_radius": "500",
			"title_template":  "Alias Match",
		}
		res, err := provider.Enrich(ctx, act, nil, inputs, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("Expected match with aliases")
		}
		if res.Name != "Alias Match" {
			t.Errorf("Expected name 'Alias Match', got %s", res.Name)
		}
	})
}
