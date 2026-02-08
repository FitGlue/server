package hevyuploader

// Exercise template handling for Hevy
// This file provides template resolution via the Hevy API with strict matching and caching
// For Hyrox/ATHX/GymRace activities, we use strict matching to preserve exercise specificity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	hevy "github.com/fitglue/server/src/go/pkg/api/hevy"
	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
)

// ExerciseTypeConfig holds the exercise type and muscle group for custom templates
type ExerciseTypeConfig struct {
	ExerciseType string // Hevy CustomExerciseType: weight_reps, distance_duration, weight_duration, etc.
	MuscleGroup  string // Hevy MuscleGroup: full_body, quadriceps, etc.
}

// strictExerciseAliases maps normalized exercise names to acceptable alternatives
// These are intentionally restrictive - only truly equivalent exercises should be listed
// Key: normalized source name, Value: list of acceptable normalized Hevy names
var strictExerciseAliases = map[string][]string{
	"farmers carry":      {"farmers walk", "farmer walk", "farmer carry"},
	"farmers walk":       {"farmers carry", "farmer walk", "farmer carry"},
	"skierg":             {"ski erg"},
	"ski erg":            {"skierg"},
	"burpee broad jump":  {"burpee broad jumps"},
	"burpee broad jumps": {"burpee broad jump"},
	"sandbag lunges":     {"sandbag lunge", "weighted lunges", "weighted lunge"},
	"sandbag lunge":      {"sandbag lunges", "weighted lunges", "weighted lunge"},
	"sled push":          {"prowler push"},
	"sled pull":          {"prowler pull"},
	"wall balls":         {"wall ball"},
	"wall ball":          {"wall balls"},
	"rowing":             {"row", "rowing machine", "row machine"},
	"row":                {"rowing", "rowing machine", "row machine"},
}

// getExerciseTypeConfig returns the appropriate exercise_type and muscle_group for an exercise name
// This is used when creating custom templates to ensure the right measurement types are supported
func getExerciseTypeConfig(exerciseName string) ExerciseTypeConfig {
	normalized := strings.ToLower(exerciseName)

	// Distance + Duration exercises (Hyrox cardio stations, carries, sleds, etc.)
	// These exercises track distance covered and time taken
	distanceDurationPatterns := []string{
		"skierg", "ski erg",
		"rowing", "row",
		"sled push", "sled pull", "prowler",
		"burpee broad jump",
		"farmers carry", "farmers walk", "farmer carry", "farmer walk",
		"sandbag lunges", "sandbag lunge", "weighted lunges", "weighted lunge",
		"running", "cycling", "swimming", "walking",
	}

	for _, pattern := range distanceDurationPatterns {
		if strings.Contains(normalized, pattern) {
			return ExerciseTypeConfig{ExerciseType: "distance_duration", MuscleGroup: "full_body"}
		}
	}

	// Weight + Duration exercises (Wall Balls - we track weight and time, with reps in notes)
	// Hevy doesn't have reps+time+weight, so we use weight_duration and add reps to notes
	weightDurationPatterns := []string{"wall ball"}
	for _, pattern := range weightDurationPatterns {
		if strings.Contains(normalized, pattern) {
			return ExerciseTypeConfig{ExerciseType: "weight_duration", MuscleGroup: "full_body"}
		}
	}

	// Default to weight_reps for unknown strength exercises
	return ExerciseTypeConfig{ExerciseType: "weight_reps", MuscleGroup: "other"}
}

// TemplateResolver fetches, caches, and resolves exercise template IDs from Hevy
type TemplateResolver struct {
	apiKey    string
	templates []hevy.ExerciseTemplate
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
// 3. Strict match against fetched templates (exact or known aliases only)
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

	// Strict match against fetched templates (exact or known aliases only)
	if templateID := r.strictMatch(normalized); templateID != "" {
		r.cache[normalized] = templateID
		r.logger.Info("Template matched",
			"exerciseName", exerciseName,
			"templateID", templateID)
		return templateID, nil
	}

	// No match found - create custom template with appropriate exercise type
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
	r.templates = []hevy.ExerciseTemplate{}
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
			ExerciseTemplates []hevy.ExerciseTemplate `json:"exercise_templates"`
			Page              int                     `json:"page"`
			PageCount         int                     `json:"page_count"`
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

// strictMatch finds a template ID using exact matching or known aliases only
// This is intentionally restrictive to preserve exercise specificity for Hyrox/ATHX/GymRace
func (r *TemplateResolver) strictMatch(normalizedName string) string {
	// Exact match first
	for _, t := range r.templates {
		if t.Title != nil && normalizeExerciseName(*t.Title) == normalizedName {
			if t.Id != nil {
				return *t.Id
			}
		}
	}

	// Check known aliases (strict equivalents only)
	if aliases, ok := strictExerciseAliases[normalizedName]; ok {
		for _, alias := range aliases {
			for _, t := range r.templates {
				if t.Title != nil && normalizeExerciseName(*t.Title) == alias {
					r.logger.Debug("Matched via strict alias",
						"source", normalizedName,
						"alias", alias,
						"templateTitle", *t.Title)
					if t.Id != nil {
						return *t.Id
					}
				}
			}
		}
	}

	return ""
}

// createCustomTemplate creates a new custom exercise template in Hevy
// Uses getExerciseTypeConfig to determine the appropriate exercise_type
func (r *TemplateResolver) createCustomTemplate(ctx context.Context, exerciseName string) (string, error) {
	client := oauth.NewClientWithErrorLogging(r.logger, "hevy", 30*time.Second)

	// Get the appropriate exercise type for this exercise
	config := getExerciseTypeConfig(exerciseName)

	payload := map[string]interface{}{
		"exercise": map[string]interface{}{
			"title":         exerciseName,
			"exercise_type": config.ExerciseType,
			"muscle_group":  config.MuscleGroup,
		},
	}

	r.logger.Debug("Creating custom template",
		"exerciseName", exerciseName,
		"exerciseType", config.ExerciseType,
		"muscleGroup", config.MuscleGroup)

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
		ExerciseTemplate hevy.ExerciseTemplate `json:"exercise_template"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// Add to local cache
	r.templates = append(r.templates, result.ExerciseTemplate)

	if result.ExerciseTemplate.Id != nil {
		return *result.ExerciseTemplate.Id, nil
	}
	return "", fmt.Errorf("created template has no ID")
}

// normalizeExerciseName normalizes an exercise name for comparison
// This does NOT simplify Hyrox-specific exercises - we preserve their specificity
func normalizeExerciseName(name string) string {
	// Lowercase and trim
	name = strings.ToLower(strings.TrimSpace(name))

	// Common substitutions for punctuation/formatting only
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "'", "") // farmer's -> farmers
	name = strings.ReplaceAll(name, "'", "") // curly apostrophe

	// Normalize plural/singular for common variations
	name = strings.ReplaceAll(name, "carries", "carry")

	// Remove common equipment suffixes that vary between platforms
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
