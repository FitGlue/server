package enricher_providers

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// WorkoutSummaryProvider generates a text summary of a strength workout.
type WorkoutSummaryProvider struct{}

func init() {
	Register(NewWorkoutSummaryProvider())
}

func NewWorkoutSummaryProvider() *WorkoutSummaryProvider {
	return &WorkoutSummaryProvider{}
}

func (p *WorkoutSummaryProvider) Name() string {
	return "workout-summary"
}

func (p *WorkoutSummaryProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_WORKOUT_SUMMARY
}

func (p *WorkoutSummaryProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*EnrichmentResult, error) {
	// Aggregate all sets from all sessions
	var allSets []*pb.StrengthSet
	for _, s := range activity.Sessions {
		allSets = append(allSets, s.StrengthSets...)
	}

	if len(allSets) == 0 {
		return &EnrichmentResult{}, nil
	}

	// Parse format style from config (default: detailed)
	formatStyle := pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_DETAILED
	if formatStr, ok := inputConfig["format"]; ok {
		switch formatStr {
		case "compact":
			formatStyle = pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_COMPACT
		case "verbose":
			formatStyle = pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_VERBOSE
		}
	}

	// Check if stats should be shown (default: true)
	showStats := true
	if statsStr, ok := inputConfig["show_stats"]; ok {
		showStats = statsStr == "true"
	}

	// Calculate headline stats
	var stats struct {
		totalSets     int
		totalVolume   float64
		totalReps     int
		totalDistance float64
		heaviestLift  struct {
			weight   float64
			exercise string
		}
	}

	for _, set := range allSets {
		stats.totalSets++
		if set.Reps > 0 {
			stats.totalReps += int(set.Reps)
		}
		if set.WeightKg > 0 {
			if set.Reps > 0 {
				stats.totalVolume += set.WeightKg * float64(set.Reps)
			} else if set.DistanceMeters > 0 {
				stats.totalVolume += set.WeightKg * set.DistanceMeters
			}
			if set.WeightKg > stats.heaviestLift.weight {
				stats.heaviestLift.weight = set.WeightKg
				stats.heaviestLift.exercise = set.ExerciseName
			}
		}
		if set.DistanceMeters > 0 {
			stats.totalDistance += set.DistanceMeters
		}
	}

	// Group by Exercise Name
	type ExerciseBlock struct {
		Name         string
		Sets         []*pb.StrengthSet
		MuscleGroups []pb.MuscleGroup
	}

	var blocks []*ExerciseBlock
	exerciseMap := make(map[string]*ExerciseBlock)

	for _, set := range allSets {
		key := set.ExerciseName
		if key == "" {
			key = "Unknown Exercise"
		}

		if _, exists := exerciseMap[key]; !exists {
			blo := &ExerciseBlock{
				Name:         key,
				Sets:         []*pb.StrengthSet{},
				MuscleGroups: []pb.MuscleGroup{set.PrimaryMuscleGroup},
			}
			blocks = append(blocks, blo)
			exerciseMap[key] = blo
		}
		exerciseMap[key].Sets = append(exerciseMap[key].Sets, set)
	}

	var sb strings.Builder
	sb.WriteString("Workout Summary:\n")

	if showStats {
		var statParts []string
		if stats.totalSets > 0 {
			statParts = append(statParts, fmt.Sprintf("%d sets", stats.totalSets))
		}
		if stats.totalVolume > 0 {
			statParts = append(statParts, fmt.Sprintf("%.0fkg volume", stats.totalVolume))
		}
		if stats.totalReps > 0 {
			statParts = append(statParts, fmt.Sprintf("%d reps", stats.totalReps))
		}
		if stats.totalDistance > 0 {
			distKm := stats.totalDistance / 1000.0
			statParts = append(statParts, fmt.Sprintf("%.1fkm distance", distKm))
		}
		if stats.heaviestLift.weight > 0 {
			statParts = append(statParts, fmt.Sprintf("Heaviest: %.0fkg (%s)", stats.heaviestLift.weight, stats.heaviestLift.exercise))
		}

		if len(statParts) > 0 {
			sb.WriteString("📊 " + strings.Join(statParts, " • ") + "\n\n")
		}
	}

	// Check if any supersets exist
	hasSupersets := false
	for _, b := range blocks {
		if len(b.Sets) > 0 && b.Sets[0].SupersetId != "" {
			hasSupersets = true
			break
		}
	}

	// Check if any non-normal set types exist
	hasSetTypes := false
	for _, b := range blocks {
		for _, s := range b.Sets {
			if s.SetType != "" && s.SetType != "normal" {
				hasSetTypes = true
				break
			}
		}
		if hasSetTypes {
			break
		}
	}

	// Add explanatory notes if needed
	if hasSupersets {
		sb.WriteString("(Exercises with matching numbers are supersets - performed back-to-back)\n")
	}
	if hasSetTypes {
		sb.WriteString("([W]=Warmup, [F]=Failure, [D]=Dropset)\n")
	}
	if hasSupersets || hasSetTypes {
		sb.WriteString("\n")
	}

	// Map superset IDs to emoji numbers
	supersetNumbers := make(map[string]string)
	supersetCounter := 0
	emojiNumbers := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

	for _, b := range blocks {
		if len(b.Sets) > 0 && b.Sets[0].SupersetId != "" {
			supersetId := b.Sets[0].SupersetId
			if _, exists := supersetNumbers[supersetId]; !exists {
				if supersetCounter < len(emojiNumbers) {
					supersetNumbers[supersetId] = emojiNumbers[supersetCounter]
					supersetCounter++
				}
			}
		}
	}

	for _, b := range blocks {
		// Check if this exercise is part of a superset
		var supersetMarker string
		if len(b.Sets) > 0 && b.Sets[0].SupersetId != "" {
			supersetId := b.Sets[0].SupersetId
			if marker, exists := supersetNumbers[supersetId]; exists {
				supersetMarker = marker + " "
			}
		}

		sb.WriteString(fmt.Sprintf("- %s%s: ", supersetMarker, b.Name))

		// Format sets based on style
		var setStrs []string
		for _, s := range b.Sets {
			formatted := p.formatSet(s, formatStyle)
			// Add set type indicator if not normal
			if s.SetType != "" && s.SetType != "normal" {
				indicator := getSetTypeIndicator(s.SetType)
				formatted = indicator + formatted
			}
			setStrs = append(setStrs, formatted)
		}

		// Collapse identical sets
		allSame := true
		if len(setStrs) > 1 {
			first := setStrs[0]
			for _, str := range setStrs[1:] {
				if str != first {
					allSame = false
					break
				}
			}
			if allSame {
				sb.WriteString(p.formatCollapsedSets(len(setStrs), setStrs[0], formatStyle))
			} else {
				sb.WriteString(strings.Join(setStrs, ", "))
			}
		} else if len(setStrs) == 1 {
			sb.WriteString(setStrs[0])
		}
		sb.WriteString("\n")
	}

	return &EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"exercise_count": fmt.Sprintf("%d", len(blocks)),
			"total_sets":     fmt.Sprintf("%d", len(allSets)),
			"total_volume":   fmt.Sprintf("%.2f", stats.totalVolume),
			"total_reps":     fmt.Sprintf("%d", stats.totalReps),
			"has_stats":      fmt.Sprintf("%t", showStats),
			"has_supersets":  fmt.Sprintf("%t", hasSupersets),
			"has_set_types":  fmt.Sprintf("%t", hasSetTypes),
		},
	}, nil
}

// getSetTypeIndicator returns a visual indicator for set types
func getSetTypeIndicator(setType string) string {
	switch setType {
	case "warmup":
		return "[W] "
	case "failure":
		return "[F] "
	case "dropset":
		return "[D] "
	default:
		return ""
	}
}

// formatSet formats a single set based on the style
func (p *WorkoutSummaryProvider) formatSet(set *pb.StrengthSet, style pb.WorkoutSummaryFormat) string {
	// Handle distance/duration exercises (cardio, running, cycling, etc.)
	if set.DistanceMeters > 0 || set.DurationSeconds > 0 {
		return p.formatDistanceDuration(set, style)
	}

	// Handle weight-based or bodyweight exercises
	switch style {
	case pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_COMPACT:
		// "10×100kg" or "10 reps"
		if set.WeightKg > 0 {
			return fmt.Sprintf("%d×%.0fkg", set.Reps, set.WeightKg)
		}
		return fmt.Sprintf("%d reps", set.Reps)

	case pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_VERBOSE:
		// "10 reps at 100.0 kilograms" or "10 reps"
		if set.WeightKg > 0 {
			return fmt.Sprintf("%d reps at %.1f kilograms", set.Reps, set.WeightKg)
		}
		return fmt.Sprintf("%d reps", set.Reps)

	default: // DETAILED
		// "10 × 100.0kg" or "10 reps"
		if set.WeightKg > 0 {
			return fmt.Sprintf("%d × %.1fkg", set.Reps, set.WeightKg)
		}
		return fmt.Sprintf("%d reps", set.Reps)
	}
}

// formatDistanceDuration formats distance/duration exercises
func (p *WorkoutSummaryProvider) formatDistanceDuration(set *pb.StrengthSet, style pb.WorkoutSummaryFormat) string {
	hasDistance := set.DistanceMeters > 0
	hasDuration := set.DurationSeconds > 0

	switch style {
	case pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_COMPACT:
		if hasDistance && hasDuration {
			return fmt.Sprintf("%.0fm in %s", set.DistanceMeters, formatDuration(set.DurationSeconds))
		} else if hasDistance {
			return fmt.Sprintf("%.0fm", set.DistanceMeters)
		} else {
			return formatDuration(set.DurationSeconds)
		}

	case pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_VERBOSE:
		if hasDistance && hasDuration {
			return fmt.Sprintf("%.1f meters in %s", set.DistanceMeters, formatDuration(set.DurationSeconds))
		} else if hasDistance {
			return fmt.Sprintf("%.1f meters", set.DistanceMeters)
		} else {
			return formatDuration(set.DurationSeconds)
		}

	default: // DETAILED
		if hasDistance && hasDuration {
			return fmt.Sprintf("%.0fm in %s", set.DistanceMeters, formatDuration(set.DurationSeconds))
		} else if hasDistance {
			return fmt.Sprintf("%.0fm", set.DistanceMeters)
		} else {
			return formatDuration(set.DurationSeconds)
		}
	}
}

// formatDuration formats seconds into a readable duration string (e.g., "5:30")
func formatDuration(seconds int32) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	secs := seconds % 60
	if secs == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

// formatCollapsedSets formats multiple identical sets
func (p *WorkoutSummaryProvider) formatCollapsedSets(count int, singleSet string, style pb.WorkoutSummaryFormat) string {
	switch style {
	case pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_COMPACT:
		// "3×(10×100kg)" -> "3×10×100kg"
		return fmt.Sprintf("%d×%s", count, singleSet)

	case pb.WorkoutSummaryFormat_WORKOUT_SUMMARY_FORMAT_VERBOSE:
		// "3 sets of 10 reps at 100.0 kilograms"
		return fmt.Sprintf("%d sets of %s", count, singleSet)

	default: // DETAILED
		// "3 x 10 × 100.0kg"
		return fmt.Sprintf("%d x %s", count, singleSet)
	}
}
