// nolint:proto-json
package hybrid_race_tagger

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fitglue/server/src/go/pkg/domain/user"
	"log/slog"
	"sort"
	"strings"

	"github.com/fitglue/server/src/go/internal/pipeline/enricher/providers"
	"github.com/fitglue/server/src/go/internal/pipeline/enricher/providers/user_input"
	"github.com/fitglue/server/src/go/pkg/bootstrap"

	pendinginput "github.com/fitglue/server/src/go/pkg/pending_input"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"

	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"

	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	providers.Register(&HybridRaceTaggerProvider{})
}

// HybridRaceTaggerProvider allows users to tag and merge laps for hybrid races like Hyrox, ATHX.
type HybridRaceTaggerProvider struct {
	service *bootstrap.Service
}

func (p *HybridRaceTaggerProvider) SetService(s *bootstrap.Service) {
	p.service = s
}

func (p *HybridRaceTaggerProvider) Name() string { return "hybrid_race_tagger" }

func (p *HybridRaceTaggerProvider) ProviderType() pbplugin.EnricherProviderType {
	return pbplugin.EnricherProviderType_ENRICHER_PROVIDER_HYBRID_RACE_TAGGER
}

// LapInfo is sent as metadata to help the user tag laps
type LapInfo struct {
	Index    int     `json:"index"`
	Duration float64 `json:"duration_seconds"`
	Distance float64 `json:"distance_meters"`
}

// PresetOption is sent to the UI for the preset selector
type PresetOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UserSelection represents the user's input from the pending input
type UserSelection struct {
	PresetID      string  `json:"preset_id"`       // Selected preset ID, or empty if "not a hybrid race"
	MergedLaps    [][]int `json:"merged_laps"`     // Optional: custom lap merges (indices)
	NotHybridRace bool    `json:"not_hybrid_race"` // True if user dismisses as non-hybrid
}

// StationResult holds timing data for a processed station
type StationResult struct {
	Name         string
	Icon         string
	Duration     float64
	Distance     float64
	StartTime    *timestamppb.Timestamp
	IsRun        bool
	Weight       float64
	ExpectedReps int32 // Expected reps from preset (e.g., 100 for Wall Balls)
}

// Enrich is called on first run - returns WaitForInputError with lap metadata and preset options
func (p *HybridRaceTaggerProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pbactivity.StandardizedActivity, user *user.Record, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Info("hybrid_race_tagger: checking for laps to tag")

	if len(activity.Sessions) == 0 || len(activity.Sessions[0].Laps) == 0 {
		logger.Info("hybrid_race_tagger: no laps to tag")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"status": "skipped",
				"reason": "no_laps",
			},
		}, nil
	}

	laps := activity.Sessions[0].Laps

	// Build lap info for pending input metadata
	lapInfos := make([]LapInfo, len(laps))
	for i, lap := range laps {
		lapInfos[i] = LapInfo{
			Index:    i,
			Duration: lap.TotalElapsedTime,
			Distance: lap.TotalDistance,
		}
	}

	lapInfoJSON, err := json.Marshal(lapInfos)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lap info: %w", err)
	}

	// Build preset options for UI
	presetOptions := make([]PresetOption, 0)
	for _, preset := range GetPresetList() {
		presetOptions = append(presetOptions, PresetOption{
			ID:   preset.ID,
			Name: preset.Name,
		})
	}
	presetsJSON, err := json.Marshal(presetOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal presets: %w", err)
	}

	logger.Info("hybrid_race_tagger: requesting user input for race preset selection",
		"lap_count", len(laps),
		"preset_count", len(presetOptions),
	)

	// Return WaitForInputError to trigger pending input flow
	return nil, &user_input.WaitForInputError{
		ActivityID:         pendinginput.GenerateID(activity.Source.String(), activity.ExternalId, p.Name()),
		RequiredFields:     []string{"race_selection"},
		EnricherProviderID: p.Name(),
		Metadata: map[string]string{
			"laps":                 string(lapInfoJSON),
			"lap_count":            fmt.Sprintf("%d", len(laps)),
			"presets":              string(presetsJSON),
			"display.field_labels": `{"race_selection":"Race Type"}`,
			"display.field_types":  `{"race_selection":"custom:hybrid_race_tagger"}`,
			"display.summary":      "Select the race format for this activity",
			"display.title":        "Tag Hybrid Race",
		},
	}
}

// EnrichResume is called when the user has completed the pending input
func (p *HybridRaceTaggerProvider) EnrichResume(ctx context.Context, activity *pbactivity.StandardizedActivity, user *user.Record, pendingInput *pbpipeline.PendingInput) (*providers.EnrichmentResult, error) {
	selectionJSON := pendingInput.InputData["race_selection"]
	if selectionJSON == "" {
		return nil, fmt.Errorf("missing race_selection in pending input")
	}

	var selection UserSelection
	if err := json.Unmarshal([]byte(selectionJSON), &selection); err != nil {
		return nil, fmt.Errorf("failed to parse race_selection: %w", err)
	}

	// User said "not a hybrid race" - return without modifications
	if selection.NotHybridRace {
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"status": "skipped",
				"reason": "not_hybrid_race",
			},
		}, nil
	}

	if len(activity.Sessions) == 0 {
		return nil, fmt.Errorf("activity has no sessions")
	}

	session := activity.Sessions[0]
	originalLaps := session.Laps

	// Get the selected preset
	preset, ok := GetPreset(selection.PresetID)
	if !ok {
		return nil, fmt.Errorf("unknown preset: %s", selection.PresetID)
	}

	// Apply lap merges if provided
	effectiveLaps := originalLaps
	if len(selection.MergedLaps) > 0 {
		effectiveLaps = applyMerges(originalLaps, selection.MergedLaps)
	}

	// Map laps to stations using the preset
	newLaps, strengthSets, stationResults := mapLapsToPreset(effectiveLaps, preset)

	// Generate time markers for graph visualization
	timeMarkers := generateTimeMarkers(stationResults)

	// Generate description
	description := generateDescription(preset, stationResults)

	// Update session with transformed data
	session.Laps = newLaps
	session.StrengthSets = append(session.StrengthSets, strengthSets...)

	// Recalculate session total distance. Since we now preserve all laps (including strength stations),
	// we just sum the laps themselves.
	var totalDistance float64
	for _, lap := range newLaps {
		totalDistance += lap.TotalDistance
	}
	session.TotalDistance = totalDistance

	// Add time markers to activity
	activity.TimeMarkers = timeMarkers

	// Determine the tag to add based on race type
	// This allows personal_records enricher to detect Hyrox/ATHX events for PR tracking
	raceTypeTag := strings.ToUpper(preset.RaceType) // "HYROX", "ATHX"

	// Return description in EnrichmentResult so orchestrator can merge it properly
	// (don't modify activity.Description directly - orchestrator overwrites it)
	return &providers.EnrichmentResult{
		ActivityType: pbactivity.ActivityType_ACTIVITY_TYPE_WORKOUT,
		Description:  description,
		Tags:         []string{raceTypeTag},
		Metadata: map[string]string{
			"status":        "success",
			"preset":        preset.Name,
			"race_type":     preset.RaceType,
			"laps_count":    fmt.Sprintf("%d", len(newLaps)),
			"strength_sets": fmt.Sprintf("%d", len(strengthSets)),
			"time_markers":  fmt.Sprintf("%d", len(timeMarkers)),
		},
	}, nil
}

// generateTimeMarkers creates TimeMarker entries for each station transition
func generateTimeMarkers(results []StationResult) []*pbactivity.TimeMarker {
	markers := make([]*pbactivity.TimeMarker, 0, len(results))

	for _, result := range results {
		if result.StartTime == nil {
			continue
		}

		markerType := "station_start"
		if result.IsRun {
			markerType = "run_start"
		}

		markers = append(markers, &pbactivity.TimeMarker{
			Timestamp:       result.StartTime,
			Label:           result.Name,
			MarkerType:      markerType,
			DurationSeconds: int32(result.Duration),
		})
	}

	return markers
}

// generateDescription creates a formatted breakdown of the race
// For hybrid races like Hyrox, distances are fixed (known), so we show:
// - Runs: just duration (1km is always the distance)
// - Stations with weight: duration + weight
// - Stations with reps (e.g., Wall Balls): duration + reps + weight
func generateDescription(preset RacePreset, results []StationResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🏁 %s:\n", preset.Name))

	var totalDuration float64
	runCount := 0

	for _, result := range results {
		totalDuration += result.Duration

		// Use station icon (with fallback to function lookup)
		icon := result.Icon
		if icon == "" {
			icon = getStationIcon(result.Name)
		}
		timeStr := formatDuration(result.Duration)

		if result.IsRun {
			// Runs: just show duration (distance is always 1km - known)
			runCount++
			sb.WriteString(fmt.Sprintf("%s Run %d: %s\n", icon, runCount, timeStr))
		} else if result.ExpectedReps > 0 && result.Weight > 0 {
			// Rep-based stations with weight (e.g., Wall Balls): show reps + weight
			sb.WriteString(fmt.Sprintf("%s %s: %s (%d reps @ %.0fkg)\n", icon, result.Name, timeStr, result.ExpectedReps, result.Weight))
		} else if result.ExpectedReps > 0 {
			// Rep-based stations without weight: show reps only
			sb.WriteString(fmt.Sprintf("%s %s: %s (%d reps)\n", icon, result.Name, timeStr, result.ExpectedReps))
		} else if result.Weight > 0 {
			// Distance-based stations with weight: show weight only (distance is known)
			sb.WriteString(fmt.Sprintf("%s %s: %s (%.0fkg)\n", icon, result.Name, timeStr, result.Weight))
		} else {
			// Distance-based stations without weight: just show time (distance is known)
			sb.WriteString(fmt.Sprintf("%s %s: %s\n", icon, result.Name, timeStr))
		}
	}

	sb.WriteString(fmt.Sprintf("⏱️ Total: %s", formatDuration(totalDuration)))

	return sb.String()
}

// getStationIcon returns an emoji for the station type
func getStationIcon(name string) string {
	switch {
	case strings.Contains(name, "Run"):
		return "🏃"
	case strings.Contains(name, "SkiErg"):
		return "⛷️"
	case strings.Contains(name, "Sled Push"):
		return "🛷"
	case strings.Contains(name, "Sled Pull"):
		return "🛷"
	case strings.Contains(name, "Burpee"):
		return "🏋️"
	case strings.Contains(name, "Row"):
		return "🚣"
	case strings.Contains(name, "Farmers"):
		return "🧳"
	case strings.Contains(name, "Sandbag"), strings.Contains(name, "Lunge"):
		return "🎒"
	case strings.Contains(name, "Wall"):
		return "🏐"
	default:
		return "▪️"
	}
}

// formatDuration converts seconds to MM:SS or HH:MM:SS
func formatDuration(seconds float64) string {
	totalSec := int(seconds)
	hours := totalSec / 3600
	mins := (totalSec % 3600) / 60
	secs := totalSec % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%d:%02d", mins, secs)
}

// applyMerges combines laps according to merge groups while preserving chronological order.
// Each merge group is placed at the position of its first (lowest index) lap.
// Merge groups must contain contiguous lap indices.
func applyMerges(laps []*pbactivity.Lap, mergeGroups [][]int) []*pbactivity.Lap {
	if len(mergeGroups) == 0 {
		return laps
	}

	// Build a map from lap index to its merge group (if any)
	// Key: lap index, Value: index of merge group in mergeGroups
	lapToGroup := make(map[int]int)
	for groupIdx, group := range mergeGroups {
		for _, lapIdx := range group {
			lapToGroup[lapIdx] = groupIdx
		}
	}

	// Find the minimum index in each merge group and validate contiguity
	groupMinIdx := make(map[int]int)
	for groupIdx, group := range mergeGroups {
		if len(group) == 0 {
			continue
		}

		// Sort indices to check contiguity
		sortedGroup := make([]int, len(group))
		copy(sortedGroup, group)
		sort.Ints(sortedGroup)

		// Check that indices are contiguous (each index is exactly 1 more than previous)
		for i := 1; i < len(sortedGroup); i++ {
			if sortedGroup[i] != sortedGroup[i-1]+1 {
				// Non-contiguous indices - return original laps unchanged
				// This shouldn't happen if UI enforces contiguous selection
				return laps
			}
		}

		groupMinIdx[groupIdx] = sortedGroup[0]
	}

	// Track which merge groups we've already processed
	processedGroups := make(map[int]bool)

	result := make([]*pbactivity.Lap, 0, len(laps))

	for i, lap := range laps {
		groupIdx, isInGroup := lapToGroup[i]

		if !isInGroup {
			// This lap is not part of any merge group - add it as-is
			result = append(result, lap)
			continue
		}

		if processedGroups[groupIdx] {
			// This lap is part of a group we've already merged - skip it
			continue
		}

		// This is the first time we're seeing this merge group
		// Check if this is the minimum index for the group (where we insert)
		if i == groupMinIdx[groupIdx] {
			// Merge all laps in this group and insert here
			mergedLap := mergeLaps(laps, mergeGroups[groupIdx])
			if mergedLap != nil {
				result = append(result, mergedLap)
			}
			processedGroups[groupIdx] = true
		}
		// If i != groupMinIdx[groupIdx], we'll process this group when we hit the min index
	}

	return result
}

// mapLapsToPreset maps laps to preset stations, creating StrengthSets for strength stations
func mapLapsToPreset(laps []*pbactivity.Lap, preset RacePreset) ([]*pbactivity.Lap, []*pbactivity.StrengthSet, []StationResult) {
	newLaps := make([]*pbactivity.Lap, 0)
	strengthSets := make([]*pbactivity.StrengthSet, 0)
	stationResults := make([]StationResult, 0)

	stationCount := len(preset.Stations)

	for i, lap := range laps {
		// Match lap to station (simple 1:1 mapping)
		stationIdx := i
		if stationIdx >= stationCount {
			// Extra laps at end - keep as generic laps
			newLaps = append(newLaps, lap)
			continue
		}

		station := preset.Stations[stationIdx]

		// Record station result for time markers and description
		result := StationResult{
			Name:         station.Name,
			Icon:         station.Icon,
			Duration:     lap.TotalElapsedTime,
			Distance:     lap.TotalDistance,
			StartTime:    lap.StartTime,
			IsRun:        station.Type == StationTypeRun,
			Weight:       station.WeightKg,
			ExpectedReps: station.Reps,
		}
		stationResults = append(stationResults, result)

		switch station.Type {
		case StationTypeRun:
			// Keep as lap (running segment)
			lap.ExerciseName = station.Name
			lap.TotalDistance = station.DistanceMeters
			newLaps = append(newLaps, lap)

		case StationTypeCardio:
			// Keep as lap but with exercise name (SkiErg, Rowing)
			lap.ExerciseName = station.Name
			lap.TotalDistance = station.DistanceMeters
			newLaps = append(newLaps, lap)

		case StationTypeStrength:
			// Keep the actual lap in the session so we don't permanently discard all the
			// internal second-by-second records (heart rate, power, cadence, etc.) for this duration.
			lap.ExerciseName = station.Name

			// Rep-based stations (Wall Balls) have no actual trackable distance, whereas moving
			// strength sets (Sleds, Farmers, Lunges) do have the preset distance.
			if station.Reps > 0 {
				lap.TotalDistance = 0
			} else {
				lap.TotalDistance = station.DistanceMeters
			}
			newLaps = append(newLaps, lap)

			// Additionally generate the StrengthSet so that volume/reps/weight metadata is properly attached
			// for the frontend UI and the FIT file StrengthSet export.
			set := &pbactivity.StrengthSet{
				ExerciseName:    station.Name,
				StartTime:       lap.StartTime,
				DurationSeconds: int32(lap.TotalElapsedTime),
				DistanceMeters:  lap.TotalDistance,
				WeightKg:        station.WeightKg,
				SetType:         "normal",
			}

			// Use preset reps if specified
			if station.Reps > 0 {
				set.Reps = station.Reps
			}

			strengthSets = append(strengthSets, set)
		}
	}

	return newLaps, strengthSets, stationResults
}

// mergeLaps merges multiple laps into one, combining records and summing totals.
// Indices are sorted to ensure StartTime comes from the earliest lap.
func mergeLaps(allLaps []*pbactivity.Lap, indices []int) *pbactivity.Lap {
	if len(indices) == 0 {
		return nil
	}

	// Sort indices to ensure chronological order
	sortedIndices := make([]int, len(indices))
	copy(sortedIndices, indices)
	sort.Ints(sortedIndices)

	// Validate first index
	firstIdx := sortedIndices[0]
	if firstIdx < 0 || firstIdx >= len(allLaps) {
		return nil
	}

	merged := &pbactivity.Lap{
		StartTime:        allLaps[firstIdx].StartTime,
		TotalElapsedTime: 0,
		TotalDistance:    0,
		Records:          make([]*pbactivity.Record, 0),
	}

	for _, idx := range sortedIndices {
		if idx < 0 || idx >= len(allLaps) {
			continue
		}
		lap := allLaps[idx]
		merged.TotalElapsedTime += lap.TotalElapsedTime
		merged.TotalDistance += lap.TotalDistance
		merged.Records = append(merged.Records, lap.Records...)
	}

	return merged
}
