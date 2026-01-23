package muscle_heatmap_image

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"text/template"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/functions/enricher/providers/muscle_heatmap"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/tier"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// MuscleHeatmapImageProvider generates an anatomical SVG heatmap of muscle activation
// Athlete tier only - produces a visual asset for Showcase
type MuscleHeatmapImageProvider struct {
	service *bootstrap.Service
}

func init() {
	providers.Register(NewMuscleHeatmapImageProvider())
}

func NewMuscleHeatmapImageProvider() *MuscleHeatmapImageProvider {
	return &MuscleHeatmapImageProvider{}
}

func (p *MuscleHeatmapImageProvider) SetService(svc *bootstrap.Service) {
	p.service = svc
}

func (p *MuscleHeatmapImageProvider) Name() string {
	return "muscle-heatmap-image"
}

func (p *MuscleHeatmapImageProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_MUSCLE_HEATMAP_IMAGE
}

// MuscleScore represents activation level for a muscle group
type MuscleScore struct {
	Name       string
	Percentage float64 // 0.0 to 1.0
	Color      string  // Hex color
}

func (p *MuscleHeatmapImageProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Tier check - Athlete only
	if tier.GetEffectiveTier(user) != tier.TierAthlete {
		slog.Info("Skipping muscle heatmap image - Athlete tier required")
		return &providers.EnrichmentResult{}, nil
	}

	// Aggregate strength sets
	var allSets []*pb.StrengthSet
	for _, s := range activity.Sessions {
		allSets = append(allSets, s.StrengthSets...)
	}

	if len(allSets) == 0 {
		return &providers.EnrichmentResult{}, nil
	}

	// Calculate muscle activation scores
	scores := p.calculateMuscleScores(allSets)
	if len(scores) == 0 {
		return &providers.EnrichmentResult{}, nil
	}

	// Generate SVG
	svgContent, err := p.generateSVG(scores)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SVG: %w", err)
	}

	// Store in Cloud Storage if service is available
	var assetURL string
	if p.service != nil && p.service.Store != nil {
		// Use dedicated showcase assets bucket
		bucketName := os.Getenv("SHOWCASE_ASSETS_BUCKET")
		if bucketName == "" {
			bucketName = "fitglue-showcase-assets" // Default bucket name
		}

		activityID := activity.ExternalId
		if activityID == "" {
			activityID = "unknown"
		}

		objectPath := fmt.Sprintf("%s/muscle-heatmap.svg", activityID)
		if err := p.service.Store.Write(ctx, bucketName, objectPath, []byte(svgContent)); err != nil {
			slog.Warn("Failed to store SVG asset", "error", err)
		} else {
			// Build URL using custom domain if configured, otherwise raw GCS URL
			// ASSETS_BASE_URL should be set per environment:
			//   - Dev: https://assets.dev.fitglue.tech
			//   - Prod: https://assets.fitglue.tech
			assetsBaseURL := os.Getenv("ASSETS_BASE_URL")
			if assetsBaseURL != "" {
				assetURL = fmt.Sprintf("%s/%s", assetsBaseURL, objectPath)
			} else {
				// Fallback to raw GCS URL
				assetURL = fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectPath)
			}
		}
	}

	metadata := map[string]string{
		"muscle_groups_activated": fmt.Sprintf("%d", len(scores)),
	}

	if assetURL != "" {
		metadata["asset_muscle_heatmap"] = assetURL
	}

	return &providers.EnrichmentResult{
		Metadata: metadata,
	}, nil
}

func (p *MuscleHeatmapImageProvider) calculateMuscleScores(sets []*pb.StrengthSet) []MuscleScore {
	volumeScores := make(map[pb.MuscleGroup]float64)
	maxScore := 0.0

	for _, set := range sets {
		primary := set.PrimaryMuscleGroup
		secondary := set.SecondaryMuscleGroups
		load := calculateLoad(set)

		// Fallback to taxonomy lookup if muscle is unspecified
		if primary == pb.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED || primary == pb.MuscleGroup_MUSCLE_GROUP_OTHER {
			result := muscle_heatmap.LookupExercise(set.ExerciseName)
			if result.Matched {
				primary = result.Primary
				secondary = result.Secondary
			}
		}

		if primary != pb.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED && primary != pb.MuscleGroup_MUSCLE_GROUP_OTHER {
			volumeScores[primary] += load
			if volumeScores[primary] > maxScore {
				maxScore = volumeScores[primary]
			}
		}

		// Secondary muscles at 50% impact
		for _, sec := range secondary {
			if sec != pb.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED && sec != pb.MuscleGroup_MUSCLE_GROUP_OTHER {
				volumeScores[sec] += load * 0.5
				if volumeScores[sec] > maxScore {
					maxScore = volumeScores[sec]
				}
			}
		}
	}

	// Convert to percentage-based scores
	var scores []MuscleScore
	for muscle, score := range volumeScores {
		if score > 0 {
			pct := score / maxScore
			scores = append(scores, MuscleScore{
				Name:       muscleToSVGID(muscle),
				Percentage: pct,
				Color:      percentageToColor(pct),
			})
		}
	}

	// Sort by percentage descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Percentage > scores[j].Percentage
	})

	return scores
}

func calculateLoad(set *pb.StrengthSet) float64 {
	if set.DistanceMeters > 0 {
		return set.DistanceMeters * 0.1
	}

	if set.DurationSeconds > 0 && set.Reps == 0 && set.WeightKg == 0 {
		return float64(set.DurationSeconds) * 0.5
	}

	load := set.WeightKg * float64(set.Reps)
	if set.WeightKg == 0 && set.Reps > 0 {
		load = float64(set.Reps) * 40.0
	}
	return load
}

func muscleToSVGID(m pb.MuscleGroup) string {
	switch m {
	case pb.MuscleGroup_MUSCLE_GROUP_CHEST:
		return "chest"
	case pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS:
		return "shoulders"
	case pb.MuscleGroup_MUSCLE_GROUP_BICEPS:
		return "biceps"
	case pb.MuscleGroup_MUSCLE_GROUP_TRICEPS:
		return "triceps"
	case pb.MuscleGroup_MUSCLE_GROUP_FOREARMS:
		return "forearms"
	case pb.MuscleGroup_MUSCLE_GROUP_LATS:
		return "lats"
	case pb.MuscleGroup_MUSCLE_GROUP_UPPER_BACK:
		return "upper_back"
	case pb.MuscleGroup_MUSCLE_GROUP_TRAPS:
		return "traps"
	case pb.MuscleGroup_MUSCLE_GROUP_LOWER_BACK:
		return "lower_back"
	case pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS:
		return "abs"
	case pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS:
		return "quads"
	case pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS:
		return "hamstrings"
	case pb.MuscleGroup_MUSCLE_GROUP_GLUTES:
		return "glutes"
	case pb.MuscleGroup_MUSCLE_GROUP_CALVES:
		return "calves"
	default:
		return "other"
	}
}

func percentageToColor(pct float64) string {
	if pct <= 0 {
		return "#444444" // No activation: gray
	} else if pct < 0.25 {
		return "#9370DB" // Low: light purple
	} else if pct < 0.5 {
		return "#8B5CF6" // Medium: purple
	} else if pct < 0.75 {
		return "#7C3AED" // High: bright purple
	} else {
		return "#EC4899" // Intense: hot pink
	}
}

func (p *MuscleHeatmapImageProvider) generateSVG(scores []MuscleScore) (string, error) {
	// Build data map for template
	data := make(map[string]string)

	// Initialize all muscles with default gray color
	for key := range MusclePathIDs {
		data[key] = "#444444"
	}

	// Override with actual colors for activated muscles
	for _, score := range scores {
		data[score.Name] = score.Color
	}

	tmpl, err := template.New("body").Parse(BodySVGTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse SVG template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute SVG template: %w", err)
	}

	return buf.String(), nil
}
