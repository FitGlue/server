package ai_banner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/tier"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"golang.org/x/oauth2/google"
)

// AIBannerProvider generates custom header images for activities using Vertex AI Imagen.
// This is an Athlete-tier only feature.
// Generated images are stored in Cloud Storage and referenced in activity metadata.
type AIBannerProvider struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewAIBannerProvider())
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

func (p *AIBannerProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Tier check - Athlete tier only
	if tier.GetEffectiveTier(user) != tier.TierAthlete {
		logger.Info("AI Banner skipped: user not on athlete tier",
			"user_id", user.UserId,
			"tier", tier.GetEffectiveTier(user),
		)
		return &providers.EnrichmentResult{
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

	// Get asset folder ID for storage path
	// Use pipeline_execution_id (unique per pipeline execution) with fallback to activity.ExternalId
	assetFolderID := inputs["pipeline_execution_id"]
	if assetFolderID == "" {
		assetFolderID = activity.ExternalId
	}
	if assetFolderID == "" {
		logger.Warn("AI Banner skipped: no pipeline execution ID or activity ID")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"status":        "skipped",
				"reason":        "no_asset_folder_id",
				"status_detail": "Pipeline execution ID or Activity ID is required for image storage",
			},
		}, nil
	}

	// Get Gemini API key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		logger.Warn("GEMINI_API_KEY not set, skipping AI banner")
		return &providers.EnrichmentResult{
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
		logger.Error("Failed to generate AI banner", "error", err)
		return &providers.EnrichmentResult{
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

	objectPath := fmt.Sprintf("%s/banner.png", assetFolderID)
	bannerURL, err := p.storeImage(ctx, bucketName, objectPath, imageData)
	if err != nil {
		logger.Error("Failed to store AI banner", "error", err)
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"status":        "error",
				"reason":        "storage_failed",
				"status_detail": err.Error(),
			},
		}, nil
	}

	logger.Info("AI Banner generated successfully",
		"asset_folder_id", assetFolderID,
		"banner_url", bannerURL,
		"style", style,
	)

	return &providers.EnrichmentResult{
		Metadata: map[string]string{
			"status":          "success",
			"asset_ai_banner": bannerURL,
			"style":           style,
		},
	}, nil
}

// ImagenRequest represents the request body for Vertex AI Imagen API
type ImagenRequest struct {
	Instances  []ImagenInstance `json:"instances"`
	Parameters ImagenParameters `json:"parameters"`
}

type ImagenInstance struct {
	Prompt string `json:"prompt"`
}

type ImagenParameters struct {
	SampleCount      int    `json:"sampleCount"`
	AspectRatio      string `json:"aspectRatio"`
	AddWatermark     bool   `json:"addWatermark"`
	PersonGeneration string `json:"personGeneration"`
	IncludeRaiReason bool   `json:"includeRaiReason"`
}

// ImagenResponse represents the response from Vertex AI Imagen API
type ImagenResponse struct {
	Predictions       []ImagenPrediction `json:"predictions"`
	RaiFilteredReason string             `json:"raiFilteredReason,omitempty"`
}

type ImagenPrediction struct {
	BytesBase64Encoded string `json:"bytesBase64Encoded"`
	MimeType           string `json:"mimeType"`
}

func (p *AIBannerProvider) generateBannerWithGemini(ctx context.Context, apiKey, prompt string) ([]byte, error) {
	// Get GCP project ID and region from environment
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if projectID == "" {
		return nil, fmt.Errorf("GCP_PROJECT_ID or GOOGLE_CLOUD_PROJECT environment variable not set")
	}

	region := os.Getenv("GCP_REGION")
	if region == "" {
		region = "us-central1" // Default region
	}

	// Use imagen-3.0-generate-002 model as specified in documentation
	modelVersion := "imagen-3.0-generate-002"

	// Build Vertex AI Imagen endpoint
	endpoint := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		region, projectID, region, modelVersion,
	)

	// Prepare request body
	reqBody := ImagenRequest{
		Instances: []ImagenInstance{
			{Prompt: prompt},
		},
		Parameters: ImagenParameters{
			SampleCount:      1,
			AspectRatio:      "3:4",        // Standard photograph ratio
			AddWatermark:     false,        // Disable watermark for cleaner banners
			PersonGeneration: "dont_allow", // No people/faces in abstract banners
			IncludeRaiReason: true,         // Include RAI filtering reasons for debugging
		},
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Get access token for authentication using Application Default Credentials
	// Note: Vertex AI requires OAuth 2.0 access tokens, not ID tokens
	tokenSource, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to create token source: %w", err)
	}

	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("imagen API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var imagenResp ImagenResponse
	if err := json.Unmarshal(respBody, &imagenResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(imagenResp.Predictions) == 0 {
		// Include RAI filter reason and full response for debugging
		raiReason := imagenResp.RaiFilteredReason
		if raiReason == "" {
			raiReason = "unknown"
		}
		return nil, fmt.Errorf("no predictions in response (RAI reason: %s, full response: %s)", raiReason, string(respBody))
	}

	// Decode base64 image data
	imageData, err := base64.StdEncoding.DecodeString(imagenResp.Predictions[0].BytesBase64Encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	return imageData, nil
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

	// Base prompt for banner generation (aspect ratio is set via API parameter, not in prompt)
	parts = append(parts, "Generate an artistic banner image for a fitness activity.")

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

	// General guidance - explicitly abstract to avoid person generation conflicts
	parts = append(parts, "No text, watermarks, or people. Abstract landscape or geometric patterns. High quality, professional digital art.")

	return strings.Join(parts, "\n")
}
