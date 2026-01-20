package enricher_providers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/tier"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// AIDescriptionProvider generates AI-powered activity descriptions using Google Gemini.
// This is an Athlete-tier only feature.
type AIDescriptionProvider struct {
	Service *bootstrap.Service
}

func init() {
	Register(NewAIDescriptionProvider())
}

func NewAIDescriptionProvider() *AIDescriptionProvider {
	return &AIDescriptionProvider{}
}

func (p *AIDescriptionProvider) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *AIDescriptionProvider) Name() string {
	return "ai-description"
}

func (p *AIDescriptionProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_AI_DESCRIPTION
}

func (p *AIDescriptionProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*EnrichmentResult, error) {
	// Tier check - Athlete (pro) tier only
	if tier.GetEffectiveTier(user) != tier.TierPro {
		slog.Info("AI Description skipped: user not on pro tier",
			"user_id", user.UserId,
			"tier", tier.GetEffectiveTier(user),
		)
		return &EnrichmentResult{
			Metadata: map[string]string{
				"status":        "skipped",
				"reason":        "tier_restricted",
				"required_tier": "pro",
			},
		}, nil
	}

	// Get configuration
	mode := inputs["mode"]
	if mode == "" {
		mode = "description" // Default
	}

	// Build context from activity
	activityContext := buildActivityContext(activity)

	// Get Gemini API key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		slog.Warn("GEMINI_API_KEY not set, skipping AI description")
		return &EnrichmentResult{
			Metadata: map[string]string{
				"status":        "skipped",
				"reason":        "api_key_not_configured",
				"status_detail": "GEMINI_API_KEY environment variable not set",
			},
		}, nil
	}

	// Generate content using Gemini
	result, err := p.generateWithGemini(ctx, apiKey, mode, activityContext)
	if err != nil {
		slog.Error("Failed to generate AI description", "error", err)
		return &EnrichmentResult{
			Metadata: map[string]string{
				"status":        "error",
				"reason":        "generation_failed",
				"status_detail": err.Error(),
			},
		}, nil // Don't return error to avoid pipeline failure
	}

	slog.Info("AI Description generated successfully",
		"mode", mode,
		"has_title", result.Title != "",
		"has_description", result.Description != "",
	)

	return &EnrichmentResult{
		Name:        result.Title,
		Description: result.Description,
		Metadata: map[string]string{
			"status": "success",
			"mode":   mode,
		},
	}, nil
}

type aiResult struct {
	Title       string
	Description string
}

func (p *AIDescriptionProvider) generateWithGemini(ctx context.Context, apiKey, mode, activityContext string) (*aiResult, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash")

	// Configure model for short, punchy outputs
	model.SetTemperature(0.7)
	model.SetTopP(0.9)
	model.SetMaxOutputTokens(300)

	prompt := buildPrompt(mode, activityContext)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	// Parse response
	rawOutput := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			rawOutput += string(text)
		}
	}

	return parseAIResponse(mode, rawOutput), nil
}

func buildPrompt(mode, activityContext string) string {
	basePrompt := `You are a fitness app assistant. Generate engaging, motivational content for workout activities.

Activity Context:
%s

Guidelines:
- Be encouraging and positive
- Keep it concise and punchy
- Use fitness terminology naturally
- Don't be overly generic - reference specific details from the workout
- Match the tone of a premium fitness app (like Strava or Peloton)
`

	switch mode {
	case "title":
		return fmt.Sprintf(basePrompt+`
Generate a creative, engaging title for this workout (max 50 characters).
Respond with ONLY the title, nothing else.`, activityContext)
	case "both":
		return fmt.Sprintf(basePrompt+`
Generate both a title and description for this workout.
Format your response exactly as:
TITLE: [creative title, max 50 chars]
DESCRIPTION: [engaging description, 2-3 sentences max]`, activityContext)
	default: // "description"
		return fmt.Sprintf(basePrompt+`
Generate an engaging description for this workout (2-3 sentences max).
Respond with ONLY the description, nothing else.`, activityContext)
	}
}

func buildActivityContext(activity *pb.StandardizedActivity) string {
	var parts []string

	// Activity type
	if activity.Type != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
		parts = append(parts, fmt.Sprintf("Type: %s", activity.Type.String()))
	}

	// Activity name (original)
	if activity.Name != "" {
		parts = append(parts, fmt.Sprintf("Original Name: %s", activity.Name))
	}

	// Duration and distance from sessions
	var totalDuration float64
	var totalDistance float64
	var strengthSets []*pb.StrengthSet
	for _, session := range activity.Sessions {
		totalDuration += session.TotalElapsedTime
		totalDistance += session.TotalDistance
		strengthSets = append(strengthSets, session.StrengthSets...)
	}

	if totalDuration > 0 {
		mins := totalDuration / 60 // seconds to minutes
		parts = append(parts, fmt.Sprintf("Duration: %.0f minutes", mins))
	}

	if totalDistance > 0 {
		km := totalDistance / 1000
		parts = append(parts, fmt.Sprintf("Distance: %.2f km", km))
	}

	// Exercises (from strength sets)
	if len(strengthSets) > 0 {
		exerciseNames := make(map[string]bool)
		for _, set := range strengthSets {
			if set.ExerciseName != "" {
				exerciseNames[set.ExerciseName] = true
			}
		}
		if len(exerciseNames) > 0 {
			names := make([]string, 0, len(exerciseNames))
			for name := range exerciseNames {
				if len(names) < 5 {
					names = append(names, name)
				}
			}
			parts = append(parts, fmt.Sprintf("Exercises: %s", strings.Join(names, ", ")))
		}
		parts = append(parts, fmt.Sprintf("Total Sets: %d", len(strengthSets)))
	}

	// Heart rate data presence
	hasHR := false
	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.HeartRate > 0 {
					hasHR = true
					break
				}
			}
			if hasHR {
				break
			}
		}
		if hasHR {
			break
		}
	}
	if hasHR {
		parts = append(parts, "Heart Rate Data: Available")
	}

	return strings.Join(parts, "\n")
}

func parseAIResponse(mode, rawOutput string) *aiResult {
	result := &aiResult{}
	output := strings.TrimSpace(rawOutput)

	switch mode {
	case "title":
		result.Title = cleanOutput(output)
	case "both":
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToUpper(line), "TITLE:") {
				result.Title = cleanOutput(strings.TrimPrefix(line, "TITLE:"))
				result.Title = cleanOutput(strings.TrimPrefix(result.Title, "Title:"))
			} else if strings.HasPrefix(strings.ToUpper(line), "DESCRIPTION:") {
				result.Description = cleanOutput(strings.TrimPrefix(line, "DESCRIPTION:"))
				result.Description = cleanOutput(strings.TrimPrefix(result.Description, "Description:"))
			}
		}
	default: // "description"
		result.Description = cleanOutput(output)
	}

	return result
}

func cleanOutput(s string) string {
	s = strings.TrimSpace(s)
	// Remove markdown formatting if present
	s = strings.Trim(s, "*_`")
	// Limit title length
	if len(s) > 100 {
		s = s[:97] + "..."
	}
	return s
}
