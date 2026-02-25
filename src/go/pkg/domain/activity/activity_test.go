package activity

import (
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

// --- GetIntervalsActivityType ---

func TestGetIntervalsActivityType_KnownTypes(t *testing.T) {
	cases := []struct {
		input    pbactivity.ActivityType
		expected string
	}{
		{pbactivity.ActivityType_ACTIVITY_TYPE_RUN, "Run"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_RIDE, "Ride"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_SWIM, "Swim"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_WALK, "Walk"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_HIKE, "Hike"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING, "WeightTraining"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_YOGA, "Yoga"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_WORKOUT, "Workout"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING, "HIIT"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_CROSSFIT, "Crossfit"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_TRAIL_RUN, "TrailRun"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE, "VirtualRide"},
		{pbactivity.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE, "MountainBikeRide"},
	}
	for _, c := range cases {
		got := GetIntervalsActivityType(c.input)
		if got != c.expected {
			t.Errorf("GetIntervalsActivityType(%v) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestGetIntervalsActivityType_UnknownType(t *testing.T) {
	// Types not in the intervals map fall back to "Workout"
	got := GetIntervalsActivityType(pbactivity.ActivityType_ACTIVITY_TYPE_GOLF)
	if got != "Workout" {
		t.Errorf("expected Workout fallback for unknown type, got %q", got)
	}
}

// --- GetStravaActivityType ---

func TestGetStravaActivityType_Run(t *testing.T) {
	got := GetStravaActivityType(pbactivity.ActivityType_ACTIVITY_TYPE_RUN)
	if got == "" {
		t.Error("expected non-empty strava activity type for RUN")
	}
}

func TestGetStravaActivityType_Unspecified(t *testing.T) {
	got := GetStravaActivityType(pbactivity.ActivityType_ACTIVITY_TYPE_UNSPECIFIED)
	if got != "Workout" {
		t.Errorf("expected 'Workout' fallback for UNSPECIFIED, got %q", got)
	}
}

// --- ParseActivityTypeFromString ---

func TestParseActivityTypeFromString_ExactProtoName(t *testing.T) {
	got := ParseActivityTypeFromString("ACTIVITY_TYPE_RUN")
	if got != pbactivity.ActivityType_ACTIVITY_TYPE_RUN {
		t.Errorf("expected ACTIVITY_TYPE_RUN, got %v", got)
	}
}

func TestParseActivityTypeFromString_DisplayName(t *testing.T) {
	got := ParseActivityTypeFromString("Run")
	if got != pbactivity.ActivityType_ACTIVITY_TYPE_RUN {
		t.Errorf("expected ACTIVITY_TYPE_RUN for 'Run', got %v", got)
	}
}

func TestParseActivityTypeFromString_Unknown(t *testing.T) {
	got := ParseActivityTypeFromString("definitely_not_an_activity_xyz")
	if got != pbactivity.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
		t.Errorf("expected UNSPECIFIED for unknown input, got %v", got)
	}
}

// --- ParseGCSURI ---

func TestParseGCSURI_Valid(t *testing.T) {
	bucket, object, ok := ParseGCSURI("gs://my-bucket/path/to/file.json")
	if !ok {
		t.Error("expected valid GCS URI to parse successfully")
	}
	if bucket != "my-bucket" {
		t.Errorf("expected bucket 'my-bucket', got %q", bucket)
	}
	if object != "path/to/file.json" {
		t.Errorf("expected object 'path/to/file.json', got %q", object)
	}
}

func TestParseGCSURI_Invalid(t *testing.T) {
	cases := []string{
		"",
		"https://example.com/file",
		"gs://",
		"gs://bucket",
		"/local/path",
	}
	for _, uri := range cases {
		_, _, ok := ParseGCSURI(uri)
		if ok {
			t.Errorf("expected invalid parse for URI %q, but got ok=true", uri)
		}
	}
}

// --- ShouldOffloadActivityData ---

func TestShouldOffloadActivityData_NilActivity(t *testing.T) {
	if ShouldOffloadActivityData(nil) {
		t.Error("expected false for nil activity")
	}
}

func TestShouldOffloadActivityData_SmallActivity(t *testing.T) {
	// A minimal activity should be well below 5MB
	activity := &pbactivity.StandardizedActivity{
		Name: "Test Run",
	}
	if ShouldOffloadActivityData(activity) {
		t.Error("expected false for small activity below 5MB threshold")
	}
}
