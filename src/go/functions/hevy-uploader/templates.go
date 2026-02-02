package hevyuploader

// Exercise template handling for Hevy
// This file provides template resolution via the Hevy API with fuzzy matching and caching

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
)

// HevyExerciseTemplate represents an exercise template from the Hevy API
type HevyExerciseTemplate struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Type          string `json:"type"`           // strength, cardio, etc.
	PrimaryMuscle string `json:"primary_muscle"` // optional
	IsCustom      bool   `json:"is_custom"`
}

// TemplateResolver fetches, caches, and resolves exercise template IDs from Hevy
type TemplateResolver struct {
	apiKey    string
	templates []HevyExerciseTemplate
	cache     map[string]string // normalized name -> template ID
	fetched   bool
	logger    *slog.Logger
}

// NewTemplateResolver creates a resolver with the user's Hevy API key
func NewTemplateResolver(apiKey string, logger *slog.Logger) *TemplateResolver {
	return &TemplateResolver{
		apiKey: apiKey,
		cache:  make(map[string]string),
		logger: logger,
	}
}

// ResolveTemplateID resolves an exercise name to a valid Hevy template ID
// It will:
// 1. Check local cache first
// 2. Fetch templates from API if not yet fetched
// 3. Fuzzy match against fetched templates
// 4. Create a custom template if no match found
func (r *TemplateResolver) ResolveTemplateID(ctx context.Context, exerciseName string) (string, error) {
	normalized := normalizeExerciseName(exerciseName)

	// Check cache first
	if id, ok := r.cache[normalized]; ok {
		r.logger.Debug("Template cache hit",
			"exerciseName", exerciseName,
			"templateID", id)
		return id, nil
	}

	// Fetch templates from API if not yet done
	if !r.fetched {
		if err := r.fetchAllTemplates(ctx); err != nil {
			r.logger.Warn("Failed to fetch templates, will create custom",
				"error", err)
			// Continue to create custom template
		}
	}

	// Fuzzy match against fetched templates
	if templateID := r.fuzzyMatch(normalized); templateID != "" {
		r.cache[normalized] = templateID
		r.logger.Info("Template matched",
			"exerciseName", exerciseName,
			"templateID", templateID)
		return templateID, nil
	}

	// No match found - create custom template
	r.logger.Info("No template match, creating custom",
		"exerciseName", exerciseName)
	templateID, err := r.createCustomTemplate(ctx, exerciseName)
	if err != nil {
		return "", fmt.Errorf("failed to create custom template for %q: %w", exerciseName, err)
	}

	r.cache[normalized] = templateID
	r.logger.Info("Created custom template",
		"exerciseName", exerciseName,
		"templateID", templateID)

	return templateID, nil
}

// fetchAllTemplates retrieves all exercise templates from Hevy API (paginated)
func (r *TemplateResolver) fetchAllTemplates(ctx context.Context) error {
	r.templates = []HevyExerciseTemplate{}
	page := 1
	pageSize := 100

	client := oauth.NewClientWithErrorLogging(r.logger, "hevy", 30*time.Second)

	for {
		url := fmt.Sprintf("https://api.hevyapp.com/v1/exercise_templates?page=%d&page_size=%d", page, pageSize)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("api-key", r.apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			var errorBody bytes.Buffer
			errorBody.ReadFrom(resp.Body)
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, errorBody.String())
		}

		var result struct {
			ExerciseTemplates []HevyExerciseTemplate `json:"exercise_templates"`
			Page              int                    `json:"page"`
			PageCount         int                    `json:"page_count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		r.templates = append(r.templates, result.ExerciseTemplates...)
		r.logger.Debug("Fetched exercise templates page",
			"page", page,
			"count", len(result.ExerciseTemplates),
			"totalSoFar", len(r.templates))

		if page >= result.PageCount || len(result.ExerciseTemplates) == 0 {
			break
		}
		page++
	}

	r.fetched = true
	r.logger.Info("Fetched all exercise templates",
		"totalCount", len(r.templates))

	return nil
}

// fuzzyMatch finds a template ID by matching against fetched templates
func (r *TemplateResolver) fuzzyMatch(normalizedName string) string {
	// Exact match first
	for _, t := range r.templates {
		if normalizeExerciseName(t.Title) == normalizedName {
			return t.ID
		}
	}

	// Partial/contains match
	for _, t := range r.templates {
		normalizedTitle := normalizeExerciseName(t.Title)
		if strings.Contains(normalizedTitle, normalizedName) ||
			strings.Contains(normalizedName, normalizedTitle) {
			return t.ID
		}
	}

	return ""
}

// createCustomTemplate creates a new custom exercise template in Hevy
func (r *TemplateResolver) createCustomTemplate(ctx context.Context, exerciseName string) (string, error) {
	client := oauth.NewClientWithErrorLogging(r.logger, "hevy", 30*time.Second)

	payload := map[string]interface{}{
		"exercise": map[string]interface{}{
			"title":         exerciseName,
			"exercise_type": "weight_reps", // default to weight/reps for strength
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.hevyapp.com/v1/exercise_templates", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("api-key", r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errorBody bytes.Buffer
		errorBody.ReadFrom(resp.Body)
		return "", fmt.Errorf("create template failed (status %d): %s", resp.StatusCode, errorBody.String())
	}

	var result struct {
		ExerciseTemplate HevyExerciseTemplate `json:"exercise_template"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// Add to local cache
	r.templates = append(r.templates, result.ExerciseTemplate)

	return result.ExerciseTemplate.ID, nil
}

// normalizeExerciseName normalizes an exercise name for comparison
func normalizeExerciseName(name string) string {
	// Lowercase and trim
	name = strings.ToLower(strings.TrimSpace(name))

	// Common substitutions
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "'", "") // farmer's -> farmers
	name = strings.ReplaceAll(name, "'", "") // curly apostrophe

	// Normalize synonyms for better matching
	name = strings.ReplaceAll(name, "carry", "walk")   // farmer carry -> farmer walk
	name = strings.ReplaceAll(name, "carries", "walk") // farmers carries -> farmers walk

	// Hyrox-specific normalizations
	name = strings.ReplaceAll(name, "skierg", "ski erg")           // SkiErg -> Ski Erg
	name = strings.ReplaceAll(name, "wall balls", "wall ball")     // Wall Balls -> Wall Ball
	name = strings.ReplaceAll(name, "burpee broad jump", "burpee") // Simplified to base exercise
	name = strings.ReplaceAll(name, "sandbag lunges", "lunges")    // Simplified to base exercise

	// Remove common suffixes that vary between platforms
	suffixes := []string{"(barbell)", "(dumbbell)", "(machine)", "(cable)", "(smith)", "(outdoor)", "(treadmill)"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	return strings.TrimSpace(name)
}

// CardioTemplateNames maps FitGlue activity types to Hevy exercise search terms
var CardioTemplateNames = map[string][]string{
	"run":  {"Running (Outdoor)", "Running", "Running (Treadmill)"},
	"walk": {"Walking", "Walk"},
	"ride": {"Cycling (Outdoor)", "Cycling", "Cycling (Stationary)"},
	"swim": {"Swimming", "Swim"},
	"row":  {"Rowing", "Rowing Machine"},
}
