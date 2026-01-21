package muscle_heatmap

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// MuscleHeatmapProvider generates an emoji-based "heatmap" of muscle volume.
type MuscleHeatmapProvider struct {
	// Coefficient map to skew effort based on muscle size/strength.
	// Smaller muscles (arms, shoulders) have lower max loads, so we multiplier their volume to match legs/back.
	// Baseline: Legs (Squat) = 1.0.
	// Arms: 100kg curl is impossible, 100kg squat is warmup.
	// Ratio: World Record Curl ~115kg. WR Squat ~500kg. Ratio ~4-5x.
	// So Arms volume * 4 = Equivalent Leg Volume?
	// User said: "100kg x 3 Squats (legs) is much easier than 100kg x 3 Bicep Curls"
	// So if weight is constant, Curls should score higher.
	// Score = Volume * Coefficient.
	coefficients map[pb.MuscleGroup]float64
}

func init() {
	enricher_providers.Register(NewMuscleHeatmapProvider())
}

func NewMuscleHeatmapProvider() *MuscleHeatmapProvider {
	return &MuscleHeatmapProvider{
		coefficients: map[pb.MuscleGroup]float64{
			pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS: 1.0,
			pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS: 1.0,
			pb.MuscleGroup_MUSCLE_GROUP_GLUTES:     1.0,
			pb.MuscleGroup_MUSCLE_GROUP_CALVES:     1.0,
			pb.MuscleGroup_MUSCLE_GROUP_ADDUCTORS:  1.0,
			pb.MuscleGroup_MUSCLE_GROUP_ABDUCTORS:  1.0,

			pb.MuscleGroup_MUSCLE_GROUP_LATS:       1.2,
			pb.MuscleGroup_MUSCLE_GROUP_UPPER_BACK: 1.2,
			pb.MuscleGroup_MUSCLE_GROUP_LOWER_BACK: 1.2,
			pb.MuscleGroup_MUSCLE_GROUP_NECK:       1.2,
			pb.MuscleGroup_MUSCLE_GROUP_TRAPS:      1.2,

			pb.MuscleGroup_MUSCLE_GROUP_CHEST:     1.5,
			pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS: 2.5,

			pb.MuscleGroup_MUSCLE_GROUP_BICEPS:   4.0,
			pb.MuscleGroup_MUSCLE_GROUP_TRICEPS:  4.0,
			pb.MuscleGroup_MUSCLE_GROUP_FOREARMS: 4.0,

			pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS: 3.0,
			pb.MuscleGroup_MUSCLE_GROUP_CARDIO:     0.5,
			pb.MuscleGroup_MUSCLE_GROUP_FULL_BODY:  1.0,
		},
	}
}

func (p *MuscleHeatmapProvider) Name() string {
	return "muscle-heatmap"
}

func (p *MuscleHeatmapProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_MUSCLE_HEATMAP
}

func getMuscleCoefficient(coeffs map[pb.MuscleGroup]float64, muscle pb.MuscleGroup) float64 {
	if v, ok := coeffs[muscle]; ok {
		return v
	}
	// Fallback logic could go here if we had grouped enums (e.g. all legs)
	// For now, if not in map, return 1.0
	return 1.0
}

func (p *MuscleHeatmapProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	// Aggregate sets
	var allSets []*pb.StrengthSet
	for _, s := range activity.Sessions {
		allSets = append(allSets, s.StrengthSets...)
	}

	if len(allSets) == 0 {
		return &enricher_providers.EnrichmentResult{}, nil
	}

	// Parse config options
	style := pb.MuscleHeatmapStyle_MUSCLE_HEATMAP_STYLE_EMOJI_BARS
	if styleStr, ok := inputConfig["style"]; ok {
		switch styleStr {
		case "percentage":
			style = pb.MuscleHeatmapStyle_MUSCLE_HEATMAP_STYLE_PERCENTAGE
		case "text":
			style = pb.MuscleHeatmapStyle_MUSCLE_HEATMAP_STYLE_TEXT_ONLY
		}
	}

	barLength := 5
	if barLenStr, ok := inputConfig["bar_length"]; ok {
		if len, err := fmt.Sscanf(barLenStr, "%d", &barLength); err == nil && len == 1 {
			if barLength < 3 {
				barLength = 3
			} else if barLength > 10 {
				barLength = 10
			}
		}
	}

	// Apply coefficient preset
	coeffs := p.coefficients
	if preset, ok := inputConfig["preset"]; ok {
		coeffs = p.getPresetCoefficients(preset)
	}

	// Calculate Weighted Volume per Muscle Group
	volumeScores := make(map[string]float64)
	maxScore := 0.0

	for _, set := range allSets {
		// Process Primary Muscle
		primary := set.PrimaryMuscleGroup
		secondary := set.SecondaryMuscleGroups
		load := calculateLoad(set)

		// Fallback: if muscle group is unspecified, use taxonomy lookup
		if primary == pb.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED || primary == pb.MuscleGroup_MUSCLE_GROUP_OTHER {
			result := LookupExercise(set.ExerciseName)
			if result.Matched {
				primary = result.Primary
				secondary = result.Secondary
			}
		}

		if primary != pb.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED && primary != pb.MuscleGroup_MUSCLE_GROUP_OTHER {
			coeff := getMuscleCoefficient(coeffs, primary)
			score := load * coeff

			name := formatMuscleName(primary)
			volumeScores[name] += score
			if volumeScores[name] > maxScore {
				maxScore = volumeScores[name]
			}
		}

		// Process Secondary Muscles (0.5x impact)
		for _, sec := range secondary {
			if sec != pb.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED && sec != pb.MuscleGroup_MUSCLE_GROUP_OTHER {
				coeff := getMuscleCoefficient(coeffs, sec)
				score := load * coeff * 0.5

				name := formatMuscleName(sec)
				volumeScores[name] += score
				if volumeScores[name] > maxScore {
					maxScore = volumeScores[name]
				}
			}
		}
	}

	// Generate output based on style
	// Filter out muscle groups with zero volume
	keys := make([]string, 0, len(volumeScores))
	for k, score := range volumeScores {
		if score > 0 {
			keys = append(keys, k)
		}
	}

	// Sort by volume (descending order)
	sort.Slice(keys, func(i, j int) bool {
		return volumeScores[keys[i]] > volumeScores[keys[j]]
	})

	var sb strings.Builder
	sb.WriteString("🔥 Muscle Heatmap:\n")

	for _, k := range keys {
		score := volumeScores[k]
		rating := 0
		if maxScore > 0 {
			rating = int((score / maxScore) * float64(barLength))
		}
		if rating == 0 && score > 0 {
			rating = 1
		}

		sb.WriteString(p.formatMuscleRow(k, score, rating, maxScore, barLength, style))
	}

	return &enricher_providers.EnrichmentResult{
		Description: sb.String(),
		Metadata: map[string]string{
			"muscle_groups_displayed": fmt.Sprintf("%d", len(keys)),
			"max_score":               fmt.Sprintf("%.2f", maxScore),
		},
	}, nil
}

// getPresetCoefficients returns coefficient map for a given preset
func (p *MuscleHeatmapProvider) getPresetCoefficients(preset string) map[pb.MuscleGroup]float64 {
	switch preset {
	case "powerlifting":
		// Emphasize compounds (squat, deadlift, bench)
		return map[pb.MuscleGroup]float64{
			pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS: 1.0,
			pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS: 1.0,
			pb.MuscleGroup_MUSCLE_GROUP_GLUTES:     1.0,
			pb.MuscleGroup_MUSCLE_GROUP_LOWER_BACK: 1.0,
			pb.MuscleGroup_MUSCLE_GROUP_CHEST:      1.0,
			pb.MuscleGroup_MUSCLE_GROUP_LATS:       1.2,
			pb.MuscleGroup_MUSCLE_GROUP_UPPER_BACK: 1.2,
			pb.MuscleGroup_MUSCLE_GROUP_TRAPS:      1.2,
			pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS:  2.0,
			pb.MuscleGroup_MUSCLE_GROUP_TRICEPS:    3.0,
			pb.MuscleGroup_MUSCLE_GROUP_BICEPS:     3.5,
			pb.MuscleGroup_MUSCLE_GROUP_FOREARMS:   3.5,
			pb.MuscleGroup_MUSCLE_GROUP_CALVES:     2.0,
			pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS: 2.5,
		}
	case "bodybuilding":
		// Emphasize isolation and hypertrophy
		return map[pb.MuscleGroup]float64{
			pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS: 1.0,
			pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS: 1.0,
			pb.MuscleGroup_MUSCLE_GROUP_GLUTES:     1.0,
			pb.MuscleGroup_MUSCLE_GROUP_CALVES:     0.8,
			pb.MuscleGroup_MUSCLE_GROUP_CHEST:      1.2,
			pb.MuscleGroup_MUSCLE_GROUP_LATS:       1.2,
			pb.MuscleGroup_MUSCLE_GROUP_UPPER_BACK: 1.2,
			pb.MuscleGroup_MUSCLE_GROUP_LOWER_BACK: 1.5,
			pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS:  2.0,
			pb.MuscleGroup_MUSCLE_GROUP_TRAPS:      2.0,
			pb.MuscleGroup_MUSCLE_GROUP_BICEPS:     3.5,
			pb.MuscleGroup_MUSCLE_GROUP_TRICEPS:    3.5,
			pb.MuscleGroup_MUSCLE_GROUP_FOREARMS:   4.0,
			pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS: 2.5,
		}
	default: // standard
		return p.coefficients
	}
}

// formatMuscleRow formats a single muscle row based on style
func (p *MuscleHeatmapProvider) formatMuscleRow(name string, score float64, rating int, maxScore float64, barLength int, style pb.MuscleHeatmapStyle) string {
	switch style {
	case pb.MuscleHeatmapStyle_MUSCLE_HEATMAP_STYLE_PERCENTAGE:
		percentage := 0
		if maxScore > 0 {
			percentage = int((score / maxScore) * 100)
		}
		return fmt.Sprintf("- %s: %d%%\n", name, percentage)

	case pb.MuscleHeatmapStyle_MUSCLE_HEATMAP_STYLE_TEXT_ONLY:
		level := "Low"
		if rating >= barLength*3/4 {
			level = "Very High"
		} else if rating >= barLength/2 {
			level = "High"
		} else if rating >= barLength/4 {
			level = "Medium"
		}
		return fmt.Sprintf("- %s: %s\n", name, level)

	default: // EMOJI_BARS
		bar := ""
		for i := 0; i < barLength; i++ {
			if i < rating {
				bar += "🟪"
			} else {
				bar += "⬜"
			}
		}
		return fmt.Sprintf("- %s: %s\n", name, bar)
	}
}

func formatMuscleName(m pb.MuscleGroup) string {
	// E.g. MUSCLE_GROUP_UPPER_BACK -> "Upper Back"
	s := m.String()
	s = strings.TrimPrefix(s, "MUSCLE_GROUP_")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Title(strings.ToLower(s))
}

func calculateLoad(set *pb.StrengthSet) float64 {
	// Handle distance-based exercises (running, cycling, rowing, etc.)
	if set.DistanceMeters > 0 {
		// Use distance as primary metric: 10m = 1 unit of load
		return set.DistanceMeters * 0.1
	}

	// Handle duration-based exercises (without distance)
	if set.DurationSeconds > 0 && set.Reps == 0 && set.WeightKg == 0 {
		// Use duration as metric: 2 seconds = 1 unit of load
		return float64(set.DurationSeconds) * 0.5
	}

	// Handle weight-based exercises
	load := set.WeightKg * float64(set.Reps)
	if set.WeightKg == 0 && set.Reps > 0 {
		// Bodyweight exercises: use heuristic
		load = float64(set.Reps) * 40.0
	}
	return load
}
