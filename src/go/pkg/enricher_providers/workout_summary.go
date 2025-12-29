package enricher_providers

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// WorkoutSummaryProvider generates a text summary of a strength workout.
type WorkoutSummaryProvider struct{}

func NewWorkoutSummaryProvider() *WorkoutSummaryProvider {
	return &WorkoutSummaryProvider{}
}

func (p *WorkoutSummaryProvider) Name() string {
	return "workout-summary"
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

	for _, b := range blocks {
		sb.WriteString(fmt.Sprintf("- %s: ", b.Name))

		// Format sets based on style
		var setStrs []string
		for _, s := range b.Sets {
			setStrs = append(setStrs, p.formatSet(s, formatStyle))
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
	}, nil
}

// formatSet formats a single set based on the style
func (p *WorkoutSummaryProvider) formatSet(set *pb.StrengthSet, style pb.WorkoutSummaryFormat) string {
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
