package logic_gate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// LogicGateProvider evaluates configurable rules and can halt the pipeline.
type LogicGateProvider struct{}

func init() {
	enricher_providers.Register(NewLogicGateProvider())
}

func NewLogicGateProvider() *LogicGateProvider {
	return &LogicGateProvider{}
}

func (p *LogicGateProvider) Name() string { return "logic_gate" }

func (p *LogicGateProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_LOGIC_GATE
}

type Rule struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values"`
	Negate bool     `json:"negate,omitempty"`
}

type Config struct {
	Rules     []Rule `json:"rules"`
	MatchMode string `json:"match_mode"` // "all", "any", "none"
	OnMatch   string `json:"on_match"`   // "continue" or "halt"
	OnNoMatch string `json:"on_no_match"`
}

func (p *LogicGateProvider) Enrich(ctx context.Context, act *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	cfgStr, ok := inputs["logic_config"]
	if !ok || strings.TrimSpace(cfgStr) == "" {
		return nil, fmt.Errorf("logic_gate: missing logic_config input")
	}
	var cfg Config
	if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
		return nil, fmt.Errorf("logic_gate: invalid JSON config: %w", err)
	}
	// Default match mode is "all"
	if cfg.MatchMode == "" {
		cfg.MatchMode = "all"
	}
	// Evaluate each rule
	matches := make([]bool, len(cfg.Rules))
	for i, r := range cfg.Rules {
		result, err := evaluateRule(r, act)
		if err != nil {
			log.Printf("logic_gate rule evaluation error: %v", err)
			// Treat error as non‑match
			result = false
		}
		if r.Negate {
			result = !result
		}
		matches[i] = result
	}
	// Determine overall match based on mode
	overall := false
	switch strings.ToLower(cfg.MatchMode) {
	case "all":
		overall = true
		for _, m := range matches {
			if !m {
				overall = false
				break
			}
		}
	case "any":
		for _, m := range matches {
			if m {
				overall = true
				break
			}
		}
	case "none":
		overall = true
		for _, m := range matches {
			if m {
				overall = false
				break
			}
		}
	default:
		return nil, fmt.Errorf("logic_gate: unknown match_mode %s", cfg.MatchMode)
	}

	// Decide action
	halt := false
	if overall {
		if strings.ToLower(cfg.OnMatch) == "halt" {
			halt = true
		}
	} else {
		if strings.ToLower(cfg.OnNoMatch) == "halt" {
			halt = true
		}
	}

	result := &enricher_providers.EnrichmentResult{
		Metadata: map[string]string{
			"logic_gate_applied": "true",
			"logic_gate_match":   fmt.Sprintf("%v", overall),
			"logic_gate_config":  cfgStr,
		},
		HaltPipeline: halt,
	}
	return result, nil
}

// evaluateRule checks a single rule against the activity.
func evaluateRule(r Rule, act *pb.StandardizedActivity) (bool, error) {
	switch strings.ToLower(r.Field) {
	case "activity_type":
		if len(r.Values) == 0 {
			return false, fmt.Errorf("activity_type rule requires values")
		}
		expected := activity.ParseActivityTypeFromString(r.Values[0])
		return act.Type == expected, nil
	case "days":
		if len(r.Values) == 0 {
			return false, fmt.Errorf("days rule requires values")
		}
		start := act.StartTime.AsTime()
		curDay := start.Weekday().String()[:3]
		curIdx := int(start.Weekday())
		for _, v := range r.Values {
			v = strings.TrimSpace(v)
			if strings.EqualFold(v, curDay) {
				return true, nil
			}
			if idx, err := strconv.Atoi(v); err == nil && idx == curIdx {
				return true, nil
			}
		}
		return false, nil
	case "time_start":
		// Handle operator for time_start
		if len(r.Values) == 0 {
			return false, fmt.Errorf("time_start rule requires a value")
		}
		t := act.StartTime.AsTime()
		// parse limit
		parts := strings.Split(r.Values[0], ":")
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid time format %s", r.Values[0])
		}
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid time numbers")
		}
		limitMins := h*60 + m
		curMins := t.Hour()*60 + t.Minute()
		switch strings.ToLower(r.Op) {
		case "gt":
			return curMins > limitMins, nil
		case "lt":
			return curMins < limitMins, nil
		case "eq":
			return curMins == limitMins, nil
		default:
			// default to >=
			return curMins >= limitMins, nil
		}
	case "time_end":
		if len(r.Values) == 0 {
			return false, fmt.Errorf("time_end rule requires a value")
		}
		// Use start time as a fallback; more precise end time could be derived from sessions.
		t := act.StartTime.AsTime()
		return compareTime(t, r.Values[0], false)
	case "location":
		if len(r.Values) < 3 {
			return false, fmt.Errorf("location rule requires lat, long, radius")
		}
		lat, err1 := strconv.ParseFloat(r.Values[0], 64)
		lng, err2 := strconv.ParseFloat(r.Values[1], 64)
		rad, err3 := strconv.ParseFloat(r.Values[2], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			return false, fmt.Errorf("invalid location values")
		}
		actLat, actLng, ok := getStartLocation(act)
		if !ok {
			return false, nil
		}
		dist := distanceMeters(actLat, actLng, lat, lng)
		return dist <= rad, nil
	case "title_contains":
		if len(r.Values) == 0 {
			return false, fmt.Errorf("title_contains rule requires a value")
		}
		return strings.Contains(strings.ToLower(act.Name), strings.ToLower(r.Values[0])), nil
	case "description_contains":
		if len(r.Values) == 0 {
			return false, fmt.Errorf("description_contains rule requires a value")
		}
		return strings.Contains(strings.ToLower(act.Description), strings.ToLower(r.Values[0])), nil
	default:
		return false, fmt.Errorf("unsupported field %s", r.Field)
	}
}

// compareTime checks if t satisfies the limit string ("HH:MM")
func compareTime(t time.Time, limit string, isStart bool) (bool, error) {
	parts := strings.Split(limit, ":")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid time format %s", limit)
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false, fmt.Errorf("invalid time numbers")
	}
	limitMins := h*60 + m
	curMins := t.Hour()*60 + t.Minute()
	if isStart {
		return curMins >= limitMins, nil
	}
	return curMins <= limitMins, nil
}

// getStartLocation extracts the first GPS point from the activity.
func getStartLocation(act *pb.StandardizedActivity) (lat, lng float64, ok bool) {
	if len(act.Sessions) == 0 {
		return 0, 0, false
	}
	for _, sess := range act.Sessions {
		for _, lap := range sess.Laps {
			for _, rec := range lap.Records {
				if rec.PositionLat != 0 || rec.PositionLong != 0 {
					return rec.PositionLat, rec.PositionLong, true
				}
			}
		}
	}
	return 0, 0, false
}

// distanceMeters computes haversine distance.
func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	dphi := (lat2 - lat1) * math.Pi / 180
	dlambda := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dphi/2)*math.Sin(dphi/2) + math.Cos(phi1)*math.Cos(phi2)*math.Sin(dlambda/2)*math.Sin(dlambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
