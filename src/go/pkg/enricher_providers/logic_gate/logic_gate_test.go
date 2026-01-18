package logic_gate

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLogicGate_AllMatchContinue(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "activity_type", Op: "eq", Values: []string{"RUNNING"}}},
		MatchMode: "all",
		OnMatch:   "continue",
		OnNoMatch: "halt",
	}
	cfgBytes, _ := json.Marshal(cfg)
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	act := &pb.StandardizedActivity{Type: pb.ActivityType_ACTIVITY_TYPE_RUN, StartTime: timestamppb.New(time.Now())}
	res, err := NewLogicGateProvider().Enrich(context.Background(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HaltPipeline {
		t.Fatalf("expected pipeline to continue, got halt")
	}
}

func TestLogicGate_NegatedTimeHalt(t *testing.T) {
	// Rule: start time before 09:00, negated => halt if after 09:00
	cfg := Config{
		Rules:     []Rule{{Field: "time_start", Op: "lt", Values: []string{"09:00"}, Negate: true}},
		MatchMode: "all",
		OnMatch:   "halt",
		OnNoMatch: "continue",
	}
	cfgBytes, _ := json.Marshal(cfg)
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	// activity at 10:00
	start := time.Date(2026, 1, 18, 10, 0, 0, 0, time.UTC)
	act := &pb.StandardizedActivity{StartTime: timestamppb.New(start)}
	res, err := NewLogicGateProvider().Enrich(context.Background(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.HaltPipeline {
		t.Fatalf("expected pipeline to halt due to negated rule")
	}
}

func TestLogicGate_AnyMatchContinue(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "title_contains", Op: "contains", Values: []string{"morning"}}, {Field: "activity_type", Op: "eq", Values: []string{"RUNNING"}}},
		MatchMode: "any",
		OnMatch:   "continue",
		OnNoMatch: "halt",
	}
	cfgBytes, _ := json.Marshal(cfg)
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	act := &pb.StandardizedActivity{Name: "Morning Run", Type: pb.ActivityType_ACTIVITY_TYPE_RUN, StartTime: timestamppb.New(time.Now())}
	res, err := NewLogicGateProvider().Enrich(context.Background(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HaltPipeline {
		t.Fatalf("expected continue on any match")
	}
}

func TestLogicGate_NoneMatchContinue(t *testing.T) {
	cfg := Config{
		Rules:     []Rule{{Field: "activity_type", Op: "eq", Values: []string{"RUNNING"}}},
		MatchMode: "none",
		OnMatch:   "halt",
		OnNoMatch: "continue",
	}
	cfgBytes, _ := json.Marshal(cfg)
	inputs := map[string]string{"logic_config": string(cfgBytes)}
	act := &pb.StandardizedActivity{Type: pb.ActivityType_ACTIVITY_TYPE_RUN, StartTime: timestamppb.New(time.Now())}
	res, err := NewLogicGateProvider().Enrich(context.Background(), act, nil, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HaltPipeline {
		t.Fatalf("expected continue when none match and on_no_match=continue")
	}
}
