package distance_milestones

import (
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

func TestGetMilestoneEmoji(t *testing.T) {
	tests := []struct {
		km    float64
		emoji string
	}{
		{10000, "🏆🎉🏅"},
		{10001, "🏆🎉🏅"},
		{5000, "🏆🎉"},
		{7500, "🏆🎉"},
		{1000, "🎉🏅"},
		{2500, "🎉🏅"},
		{500, "🎉"},
		{750, "🎉"},
		{100, "✨"},
		{0, "✨"},
		{499, "✨"},
	}

	for _, tt := range tests {
		got := getMilestoneEmoji(tt.km)
		if got != tt.emoji {
			t.Errorf("getMilestoneEmoji(%.0f) = %q, want %q", tt.km, got, tt.emoji)
		}
	}
}

func TestGetSportLabel(t *testing.T) {
	tests := []struct {
		sport string
		want  string
	}{
		{"running", "running"},
		{"cycling", "cycling"},
		{"swimming", "swimming"},
		{"other", "distance"},
		{"", "distance"},
	}

	for _, tt := range tests {
		got := getSportLabel(tt.sport)
		if got != tt.want {
			t.Errorf("getSportLabel(%q) = %q, want %q", tt.sport, got, tt.want)
		}
	}
}

func TestGetNextMilestone(t *testing.T) {
	tests := []struct {
		current     float64
		wantAtLeast float64
	}{
		{0, 1},      // next milestone should be some positive integer
		{99, 100},   // should return 100
		{100, 200},  // return next after 100
		{499, 500},  // return 500
		{500, 1000}, // return 1000
	}

	for _, tt := range tests {
		got := getNextMilestone(tt.current)
		if got < tt.wantAtLeast {
			t.Errorf("getNextMilestone(%.0f) = %.0f, want >= %.0f", tt.current, got, tt.wantAtLeast)
		}
	}

	// Very high value — should cap at highest milestone
	veryHigh := getNextMilestone(999999)
	if veryHigh <= 0 {
		t.Errorf("getNextMilestone(999999) should be positive, got %.0f", veryHigh)
	}
}

func TestMatchesSport(t *testing.T) {
	tests := []struct {
		actType pbactivity.ActivityType
		sport   string
		want    bool
	}{
		{pbactivity.ActivityType_ACTIVITY_TYPE_RUN, "running", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_TRAIL_RUN, "running", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN, "running", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_RIDE, "cycling", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE, "cycling", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE, "cycling", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE, "cycling", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_SWIM, "swimming", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_RUN, "cycling", false},
		{pbactivity.ActivityType_ACTIVITY_TYPE_SWIM, "running", false},
		// "default" sport matches all types
		{pbactivity.ActivityType_ACTIVITY_TYPE_RUN, "all", true},
		{pbactivity.ActivityType_ACTIVITY_TYPE_SWIM, "", true},
	}

	for _, tt := range tests {
		got := matchesSport(tt.actType, tt.sport)
		if got != tt.want {
			t.Errorf("matchesSport(%v, %q) = %v, want %v", tt.actType, tt.sport, got, tt.want)
		}
	}
}

func TestDistanceMilestonesName(t *testing.T) {
	p := NewDistanceMilestones()
	if p.Name() == "" {
		t.Error("expected non-empty name")
	}
}

func TestDistanceMilestonesProviderType(t *testing.T) {
	p := NewDistanceMilestones()
	if p.ProviderType() == 0 {
		t.Error("expected non-zero provider type")
	}
}
