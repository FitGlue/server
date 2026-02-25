// nolint:proto-json
package activity_filter

import (
	"context"
	"log/slog"
	"testing"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
)

func enrich(t *testing.T, act *pbactivity.StandardizedActivity, inputs map[string]string) (bool, string) {
	t.Helper()
	p := NewActivityFilterProvider()
	res, err := p.Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res.HaltPipeline, res.Metadata["filter_reason"]
}

func actOfType(actType pbactivity.ActivityType) *pbactivity.StandardizedActivity {
	return &pbactivity.StandardizedActivity{Type: actType}
}

// --- exclude_activity_types ---

func TestActivityFilter_ExcludeType_Match(t *testing.T) {
	halt, reason := enrich(t, actOfType(pbactivity.ActivityType_ACTIVITY_TYPE_WALK),
		map[string]string{"exclude_activity_types": "WALK"})
	if !halt || reason != "activity_type_excluded" {
		t.Errorf("expected halt with activity_type_excluded, got halt=%v reason=%q", halt, reason)
	}
}

func TestActivityFilter_ExcludeType_FullEnumName(t *testing.T) {
	halt, reason := enrich(t, actOfType(pbactivity.ActivityType_ACTIVITY_TYPE_YOGA),
		map[string]string{"exclude_activity_types": "ACTIVITY_TYPE_YOGA"})
	if !halt || reason != "activity_type_excluded" {
		t.Errorf("expected halt, got halt=%v reason=%q", halt, reason)
	}
}

func TestActivityFilter_ExcludeType_NoMatch(t *testing.T) {
	halt, _ := enrich(t, actOfType(pbactivity.ActivityType_ACTIVITY_TYPE_RUN),
		map[string]string{"exclude_activity_types": "WALK,YOGA"})
	if halt {
		t.Error("expected no halt for non-excluded activity type")
	}
}

func TestActivityFilter_ExcludeType_MultipleValues(t *testing.T) {
	halt, reason := enrich(t, actOfType(pbactivity.ActivityType_ACTIVITY_TYPE_YOGA),
		map[string]string{"exclude_activity_types": "WALK, YOGA, PILATES"})
	if !halt || reason != "activity_type_excluded" {
		t.Errorf("expected halt with activity_type_excluded for multiple values, got halt=%v reason=%q", halt, reason)
	}
}

// --- exclude_title_contains ---

func TestActivityFilter_ExcludeTitle_Match(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Name: "Easy Recovery Run"}
	halt, reason := enrich(t, act, map[string]string{"exclude_title_contains": "recovery"})
	if !halt || reason != "title_pattern_excluded" {
		t.Errorf("expected halt with title_pattern_excluded, got halt=%v reason=%q", halt, reason)
	}
}

func TestActivityFilter_ExcludeTitle_CaseInsensitive(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Name: "Lunch Break Walk"}
	halt, _ := enrich(t, act, map[string]string{"exclude_title_contains": "LUNCH"})
	if !halt {
		t.Error("expected halt for case-insensitive title match")
	}
}

func TestActivityFilter_ExcludeTitle_NoMatch(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Name: "Morning Run"}
	halt, _ := enrich(t, act, map[string]string{"exclude_title_contains": "recovery,rest"})
	if halt {
		t.Error("expected no halt when title doesn't match any pattern")
	}
}

// --- exclude_description_contains ---

func TestActivityFilter_ExcludeDescription_Match(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Description: "This was a rest day activity"}
	halt, reason := enrich(t, act, map[string]string{"exclude_description_contains": "rest day"})
	if !halt || reason != "description_pattern_excluded" {
		t.Errorf("expected halt with description_pattern_excluded, got halt=%v reason=%q", halt, reason)
	}
}

// --- include_activity_types ---

func TestActivityFilter_IncludeType_Match(t *testing.T) {
	halt, _ := enrich(t, actOfType(pbactivity.ActivityType_ACTIVITY_TYPE_RUN),
		map[string]string{"include_activity_types": "RUN,RIDE"})
	if halt {
		t.Error("expected no halt when activity type is in include list")
	}
}

func TestActivityFilter_IncludeType_NoMatch(t *testing.T) {
	halt, reason := enrich(t, actOfType(pbactivity.ActivityType_ACTIVITY_TYPE_WALK),
		map[string]string{"include_activity_types": "RUN,RIDE"})
	if !halt || reason != "type_not_included" {
		t.Errorf("expected halt with type_not_included, got halt=%v reason=%q", halt, reason)
	}
}

// --- include_title_contains ---

func TestActivityFilter_IncludeTitle_Match(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Name: "Parkrun Saturday"}
	halt, _ := enrich(t, act, map[string]string{"include_title_contains": "parkrun"})
	if halt {
		t.Error("expected no halt when title matches include pattern")
	}
}

func TestActivityFilter_IncludeTitle_NoMatch(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Name: "Morning Run"}
	halt, reason := enrich(t, act, map[string]string{"include_title_contains": "parkrun"})
	if !halt || reason != "title_not_included" {
		t.Errorf("expected halt with title_not_included, got halt=%v reason=%q", halt, reason)
	}
}

// --- include_description_contains ---

func TestActivityFilter_IncludeDescription_Match(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Description: "Felt great today, PR attempt"}
	halt, _ := enrich(t, act, map[string]string{"include_description_contains": "pr attempt"})
	if halt {
		t.Error("expected no halt when description matches include pattern")
	}
}

func TestActivityFilter_IncludeDescription_NoMatch(t *testing.T) {
	act := &pbactivity.StandardizedActivity{Description: "Easy jog"}
	halt, reason := enrich(t, act, map[string]string{"include_description_contains": "pr attempt"})
	if !halt || reason != "description_not_included" {
		t.Errorf("expected halt with description_not_included, got halt=%v reason=%q", halt, reason)
	}
}

// --- combined rules and pass-through ---

func TestActivityFilter_NoRules_Passes(t *testing.T) {
	halt, _ := enrich(t, actOfType(pbactivity.ActivityType_ACTIVITY_TYPE_RUN), map[string]string{})
	if halt {
		t.Error("expected pass-through when no filter rules defined")
	}
}

func TestActivityFilter_ProviderMetadata(t *testing.T) {
	p := NewActivityFilterProvider()
	if p.Name() != "activity_filter" {
		t.Errorf("expected 'activity_filter', got %q", p.Name())
	}
}
