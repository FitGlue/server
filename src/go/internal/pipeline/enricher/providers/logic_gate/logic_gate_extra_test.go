// nolint:proto-json
package logic_gate

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// enrich is a shorthand for running the logic gate with a JSON config
func enrich(t *testing.T, cfg Config, act *pbactivity.StandardizedActivity) bool {
	t.Helper()
	cfgBytes, _ := json.Marshal(cfg)
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	res, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res.HaltPipeline
}

func actAt(hour, minute int) *pbactivity.StandardizedActivity {
	ts := time.Date(2026, 1, 1, hour, minute, 0, 0, time.UTC)
	return &pbactivity.StandardizedActivity{StartTime: timestamppb.New(ts)}
}

func actOnDay(weekday time.Weekday) *pbactivity.StandardizedActivity {
	// Find next occurrence of weekday from a known Monday 2026-01-05
	base := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC) // Monday
	offset := (int(weekday) - int(base.Weekday()) + 7) % 7
	ts := base.Add(time.Duration(offset) * 24 * time.Hour)
	return &pbactivity.StandardizedActivity{StartTime: timestamppb.New(ts)}
}

// --- time_start operator tests ---

func TestLogicGate_TimeStart_GT(t *testing.T) {
	// after 09:00 -> match -> continue
	cfg := Config{
		Rules:     []Rule{{Field: "time_start", Op: "gt", Values: []string{"09:00"}}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	if halt := enrich(t, cfg, actAt(10, 0)); halt {
		t.Error("expected continue for time_start > 09:00 at 10:00")
	}
	if halt := enrich(t, cfg, actAt(8, 0)); !halt {
		t.Error("expected halt for time_start > 09:00 at 08:00")
	}
}

func TestLogicGate_TimeStart_EQ(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "time_start", Op: "eq", Values: []string{"09:00"}}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	if halt := enrich(t, cfg, actAt(9, 0)); halt {
		t.Error("expected continue for exact time match")
	}
	if halt := enrich(t, cfg, actAt(9, 1)); !halt {
		t.Error("expected halt for non-matching time")
	}
}

func TestLogicGate_TimeStart_DefaultOp(t *testing.T) {
	// no op means >=
	cfg := Config{
		Rules:     []Rule{{Field: "time_start", Op: "", Values: []string{"09:00"}}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	if halt := enrich(t, cfg, actAt(9, 0)); halt {
		t.Error("expected continue for time_start >= 09:00 at 09:00")
	}
}

func TestLogicGate_TimeStart_InvalidFormat(t *testing.T) {
	cfgBytes, _ := json.Marshal(Config{
		Rules:     []Rule{{Field: "time_start", Op: "lt", Values: []string{"notatime"}}},
		MatchMode: "all", OnMatch: "halt", OnNoMatch: "continue",
	})
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	res, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), actAt(10, 0), nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Rule error treated as non-match -> OnNoMatch=continue -> no halt
	if res.HaltPipeline {
		t.Error("expected continue when time format is invalid (rule error = non-match)")
	}
}

// --- time_end tests ---

func TestLogicGate_TimeEnd_Match(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "time_end", Op: "lt", Values: []string{"12:00"}}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	if halt := enrich(t, cfg, actAt(10, 0)); halt {
		t.Error("expected continue for time_end before 12:00 at 10:00")
	}
	if halt := enrich(t, cfg, actAt(14, 0)); !halt {
		t.Error("expected halt for time_end before 12:00 at 14:00")
	}
}

// --- days rule tests ---

func TestLogicGate_Days_StringMatch(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "days", Op: "eq", Values: []string{"Mon", "Wed", "Fri"}}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	if halt := enrich(t, cfg, actOnDay(time.Monday)); halt {
		t.Error("expected continue on Monday")
	}
	if halt := enrich(t, cfg, actOnDay(time.Tuesday)); !halt {
		t.Error("expected halt on Tuesday")
	}
}

func TestLogicGate_Days_NumericMatch(t *testing.T) {
	// Sunday = 0 in Go's time.Weekday
	cfg := Config{
		Rules:     []Rule{{Field: "days", Op: "eq", Values: []string{"0"}}}, // Sunday
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	if halt := enrich(t, cfg, actOnDay(time.Sunday)); halt {
		t.Error("expected continue on Sunday (index 0)")
	}
}

func TestLogicGate_Days_NoValues(t *testing.T) {
	cfgBytes, _ := json.Marshal(Config{
		Rules:     []Rule{{Field: "days", Op: "eq", Values: []string{}}},
		MatchMode: "all", OnMatch: "halt", OnNoMatch: "continue",
	})
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	res, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), actAt(10, 0), nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// empty values -> error -> non-match -> OnNoMatch=continue
	if res.HaltPipeline {
		t.Error("expected continue with empty days values")
	}
}

// --- title_contains and description_contains ---

func TestLogicGate_TitleContains(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "title_contains", Op: "contains", Values: []string{"parkrun"}}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	act := &pbactivity.StandardizedActivity{Name: "Bingham Parkrun", StartTime: timestamppb.New(time.Now())}
	if halt := enrich(t, cfg, act); halt {
		t.Error("expected continue when title contains 'parkrun'")
	}
	act.Name = "Morning Run"
	if halt := enrich(t, cfg, act); !halt {
		t.Error("expected halt when title doesn't contain 'parkrun'")
	}
}

func TestLogicGate_DescriptionContains(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "description_contains", Op: "contains", Values: []string{"recovery"}}},
		MatchMode: "all", OnMatch: "halt", OnNoMatch: "continue",
	}
	act := &pbactivity.StandardizedActivity{
		Description: "Easy recovery run today",
		StartTime:   timestamppb.New(time.Now()),
	}
	if halt := enrich(t, cfg, act); !halt {
		t.Error("expected halt when description contains 'recovery'")
	}
}

// --- location rule ---

func TestLogicGate_Location_WithinRadius(t *testing.T) {
	// 51.5074, -0.1278 is central London — test with a nearby point
	cfg := Config{
		Rules: []Rule{{
			Field:  "location",
			Op:     "within",
			Values: []string{"51.5074", "-0.1278", "500"}, // 500m radius
		}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	// Activity with GPS record near that location
	act := &pbactivity.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{
				Records: []*pbactivity.Record{{
					PositionLat:  51.5075,
					PositionLong: -0.1279,
				}},
			}},
		}},
	}
	if halt := enrich(t, cfg, act); halt {
		t.Error("expected continue when within radius")
	}
}

func TestLogicGate_Location_OutsideRadius(t *testing.T) {
	cfg := Config{
		Rules: []Rule{{
			Field:  "location",
			Op:     "within",
			Values: []string{"51.5074", "-0.1278", "100"}, // 100m radius
		}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	// Activity far away (Manchester)
	act := &pbactivity.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Sessions: []*pbactivity.Session{{
			Laps: []*pbactivity.Lap{{
				Records: []*pbactivity.Record{{
					PositionLat:  53.4808,
					PositionLong: -2.2426,
				}},
			}},
		}},
	}
	if halt := enrich(t, cfg, act); !halt {
		t.Error("expected halt when outside radius")
	}
}

func TestLogicGate_Location_NoGPS(t *testing.T) {
	cfg := Config{
		Rules: []Rule{{
			Field:  "location",
			Op:     "within",
			Values: []string{"51.5074", "-0.1278", "500"},
		}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	}
	// No sessions = no GPS
	act := &pbactivity.StandardizedActivity{StartTime: timestamppb.New(time.Now())}
	if halt := enrich(t, cfg, act); !halt {
		t.Error("expected halt when no GPS data")
	}
}

func TestLogicGate_Location_InvalidValues(t *testing.T) {
	cfgBytes, _ := json.Marshal(Config{
		Rules:     []Rule{{Field: "location", Op: "within", Values: []string{"notlat", "-0.1278", "500"}}},
		MatchMode: "all", OnMatch: "continue", OnNoMatch: "halt",
	})
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	act := &pbactivity.StandardizedActivity{StartTime: timestamppb.New(time.Now())}
	res, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// parse error -> non-match -> halt
	if !res.HaltPipeline {
		t.Error("expected halt when location values are invalid")
	}
}

// --- edge cases ---

func TestLogicGate_UnknownField(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "unknown_field", Op: "eq", Values: []string{"foo"}}},
		MatchMode: "all", OnMatch: "halt", OnNoMatch: "continue",
	}
	// Unknown field -> error -> non-match -> continue
	if halt := enrich(t, cfg, actAt(10, 0)); halt {
		t.Error("expected continue for unknown field (treated as non-match)")
	}
}

func TestLogicGate_UnknownMatchMode(t *testing.T) {
	cfgBytes, _ := json.Marshal(Config{
		Rules:     []Rule{{Field: "activity_type", Op: "eq", Values: []string{"RUNNING"}}},
		MatchMode: "invalid_mode", OnMatch: "halt", OnNoMatch: "continue",
	})
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	act := &pbactivity.StandardizedActivity{
		Type:      pbactivity.ActivityType_ACTIVITY_TYPE_RUN,
		StartTime: timestamppb.New(time.Now()),
	}
	_, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err == nil {
		t.Error("expected error for unknown match_mode")
	}
}

func TestLogicGate_InvalidJSON(t *testing.T) {
	inputs := map[string]string{"logic_config": "{not valid json"}
	act := &pbactivity.StandardizedActivity{StartTime: timestamppb.New(time.Now())}
	_, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err == nil {
		t.Error("expected error for invalid JSON config")
	}
}

func TestLogicGate_IndividualFieldConfig(t *testing.T) {
	// Test the alternative config path (individual fields, not logic_config JSON)
	rulesBytes, _ := json.Marshal([]Rule{
		{Field: "activity_type", Op: "eq", Values: []string{"RUNNING"}},
	})
	inputs := map[string]string{
		"rules":       string(rulesBytes),
		"match_mode":  "all",
		"on_match":    "continue",
		"on_no_match": "halt",
	}
	act := &pbactivity.StandardizedActivity{
		Type:      pbactivity.ActivityType_ACTIVITY_TYPE_RUN,
		StartTime: timestamppb.New(time.Now()),
	}
	res, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HaltPipeline {
		t.Error("expected continue for run activity with run rule")
	}
}

func TestLogicGate_IndividualFieldConfig_InvalidRulesJSON(t *testing.T) {
	inputs := map[string]string{
		"rules":       "not valid json",
		"match_mode":  "all",
		"on_match":    "continue",
		"on_no_match": "halt",
	}
	act := &pbactivity.StandardizedActivity{StartTime: timestamppb.New(time.Now())}
	_, err := NewLogicGateProvider().Enrich(context.Background(), slog.Default(), act, nil, inputs, false)
	if err == nil {
		t.Error("expected error for invalid rules JSON")
	}
}

func TestLogicGate_EmptyRules_DefaultAll(t *testing.T) {
	// No rules + MatchMode="" -> defaults to "all" -> all() = true
	cfg := Config{OnMatch: "halt", OnNoMatch: "continue"}
	if halt := enrich(t, cfg, actAt(10, 0)); !halt {
		t.Error("expected halt when no rules and on_match=halt (empty rules all() = true)")
	}
}

func TestLogicGate_ProviderMetadata(t *testing.T) {
	p := NewLogicGateProvider()
	if p.Name() != "logic_gate" {
		t.Errorf("expected name 'logic_gate', got %q", p.Name())
	}
}
