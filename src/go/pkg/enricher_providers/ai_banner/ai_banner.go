package ai_banner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/tier"
	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// AIBannerProvider generates custom header images for activities using Gemini 2.0 Flash.
// This is an Athlete-tier only feature.
// Generated images are stored in Cloud Storage and referenced in activity metadata.
type AIBannerProvider struct {
	Service *bootstrap.Service
}

func init() {
	enricher_providers.Register(NewAIBannerProvider())
}

func NewAIBannerProvider() *AIBannerProvider {
	return &AIBannerProvider{}
}

func (p *AIBannerProvider) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *AIBannerProvider) Name() string {
	return "ai-banner"
}

func (p *AIBannerProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_AI_BANNER
}

func (p *AIBannerProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	// Tier check - Athlete tier only
	if tier.GetEffectiveTier(user) != tier.TierAthlete {
		slog.Info("AI Banner skipped: user not on athlete tier",
			"user_id", user.UserId,
			"tier", tier.GetEffectiveTier(user),
		)
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{
				"status":        "skipped",
				"reason":        "tier_restricted",
				"required_tier": "athlete",
			},
		}, nil
	}

	// Get style configuration
	style := inputs["style"]
	if style == "" {
		style = "vibrant" // Default
	}

	// Get activity ID for storage path
	activityID := activity.ExternalId
	if activityID == "" {
		slog.Warn("AI Banner skipped: no activity ID")
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{
				"status":        "skipped",
				"reason":        "no_activity_id",
				"status_detail": "Activity ID is required for image storage",
			},
		}, nil
	}

	// Get Gemini API key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		slog.Warn("GEMINI_API_KEY not set, skipping AI banner")
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{
				"status":        "skipped",
				"reason":        "api_key_not_configured",
				"status_detail": "GEMINI_API_KEY environment variable not set",
			},
		}, nil
	}

	// Build context-aware prompt
	prompt := buildImagePrompt(activity, style)

	// Generate image using Gemini
	imageData, err := p.generateBannerWithGemini(ctx, apiKey, prompt)
	if err != nil {
		slog.Error("Failed to generate AI banner", "error", err)
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{
				"status":        "error",
				"reason":        "generation_failed",
				"status_detail": err.Error(),
			},
		}, nil // Don't return error to avoid pipeline failure
	}

	// Store image in Cloud Storage
	bucketName := os.Getenv("SHOWCASE_ASSETS_BUCKET")
	if bucketName == "" {
		bucketName = "fitglue-showcase-assets" // Default bucket name
	}

	objectPath := fmt.Sprintf("%s/banner.png", activityID)
	bannerURL, err := p.storeImage(ctx, bucketName, objectPath, imageData)
	if err != nil {
		slog.Error("Failed to store AI banner", "error", err)
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{
				"status":        "error",
				"reason":        "storage_failed",
				"status_detail": err.Error(),
			},
		}, nil
	}

	slog.Info("AI Banner generated successfully",
		"activity_id", activityID,
		"banner_url", bannerURL,
		"style", style,
	)

	return &enricher_providers.EnrichmentResult{
		Metadata: map[string]string{
			"status":          "success",
			"asset_ai_banner": bannerURL,
			"style":           style,
		},
	}, nil
}

func (p *AIBannerProvider) generateBannerWithGemini(ctx context.Context, apiKey, prompt string) ([]byte, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	// Use Gemini 2.0 Flash for image generation
	model := client.GenerativeModel("gemini-2.0-flash-exp")

	// Configure for image generation
	model.SetTemperature(0.8)
	model.GenerationConfig.ResponseMIMEType = "image/png"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate image: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no image generated")
	}

	// Extract image data from response
	for _, part := range resp.Candidates[0].Content.Parts {
		if blob, ok := part.(genai.Blob); ok {
			return blob.Data, nil
		}
		// Handle base64-encoded image data
		if text, ok := part.(genai.Text); ok {
			data, err := base64.StdEncoding.DecodeString(string(text))
			if err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("no image data in response")
}

func (p *AIBannerProvider) storeImage(ctx context.Context, bucketName, objectPath string, data []byte) (string, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectPath)

	writer := obj.NewWriter(ctx)
	writer.ContentType = "image/png"
	writer.CacheControl = "public, max-age=31536000, immutable"

	if _, err := bytes.NewReader(data).WriteTo(writer); err != nil {
		return "", fmt.Errorf("failed to write image data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// Build URL using custom domain if configured, otherwise raw GCS URL
	// ASSETS_BASE_URL should be set per environment:
	//   - Dev: https://assets.dev.fitglue.tech
	//   - Prod: https://assets.fitglue.tech
	assetsBaseURL := os.Getenv("ASSETS_BASE_URL")
	if assetsBaseURL != "" {
		return fmt.Sprintf("%s/%s", assetsBaseURL, objectPath), nil
	}

	// Fallback to raw GCS URL
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectPath), nil
}

func buildImagePrompt(activity *pb.StandardizedActivity, style string) string {
	var parts []string

	// Base prompt for banner generation
	parts = append(parts, "Generate a wide banner image (1200x400 pixels) for a fitness activity.")

	// Activity type context
	activityType := strings.ToLower(strings.ReplaceAll(activity.Type.String(), "ACTIVITY_TYPE_", ""))
	activityType = strings.ReplaceAll(activityType, "_", " ")
	if activityType != "unspecified" {
		parts = append(parts, fmt.Sprintf("Activity type: %s", activityType))
	}

	// Time of day context
	if activity.StartTime != nil {
		startTime := activity.StartTime.AsTime()
		hour := startTime.Hour()
		var timeOfDay string
		switch {
		case hour >= 5 && hour < 9:
			timeOfDay = "early morning, sunrise colors"
		case hour >= 9 && hour < 12:
			timeOfDay = "morning, bright daylight"
		case hour >= 12 && hour < 17:
			timeOfDay = "afternoon, warm sunlight"
		case hour >= 17 && hour < 20:
			timeOfDay = "evening, golden hour"
		default:
			timeOfDay = "night, dark with city lights"
		}
		parts = append(parts, fmt.Sprintf("Time of day: %s", timeOfDay))
	}

	// Style guidance
	switch style {
	case "minimal":
		parts = append(parts, "Style: minimalist, clean lines, muted colors, simple geometric shapes")
	case "dramatic":
		parts = append(parts, "Style: dramatic, bold contrast, dynamic composition, intense colors")
	default: // "vibrant"
		parts = append(parts, "Style: vibrant, energetic, bold colors, athletic and dynamic mood")
	}

	// General guidance
	parts = append(parts, "No text or watermarks. Abstract or semi-abstract athletic imagery. High quality, professional look.")

	return strings.Join(parts, "\n")
}

// getTimeOfDay returns a descriptive time of day string from hour
func getTimeOfDay(hour int) string {
	switch {
	case hour >= 5 && hour < 9:
		return "early morning"
	case hour >= 9 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 20:
		return "evening"
	default:
		return "night"
	}
}
