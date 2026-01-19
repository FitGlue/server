package hevyuploader

// Exercise template handling for Hevy
// This file provides fuzzy matching and caching for exercise templates

import (
	"strings"
	"sync"
)

// TemplateCache caches exercise template IDs to avoid repeated API calls
type TemplateCache struct {
	mu        sync.RWMutex
	templates map[string]string // exerciseName -> templateID
}

// NewTemplateCache creates a new template cache
func NewTemplateCache() *TemplateCache {
	return &TemplateCache{
		templates: make(map[string]string),
	}
}

// Get retrieves a template ID from the cache
func (tc *TemplateCache) Get(exerciseName string) (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	id, ok := tc.templates[normalizeExerciseName(exerciseName)]
	return id, ok
}

// Set stores a template ID in the cache
func (tc *TemplateCache) Set(exerciseName, templateID string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.templates[normalizeExerciseName(exerciseName)] = templateID
}

// normalizeExerciseName normalizes an exercise name for comparison
func normalizeExerciseName(name string) string {
	// Lowercase and trim
	name = strings.ToLower(strings.TrimSpace(name))

	// Common substitutions
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Remove common suffixes that vary between platforms
	suffixes := []string{"(barbell)", "(dumbbell)", "(machine)", "(cable)", "(smith)"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	return strings.TrimSpace(name)
}

// CommonExerciseTemplates maps common exercise names to their Hevy template IDs
// These IDs should be verified against the actual Hevy API
var CommonExerciseTemplates = map[string]string{
	// Chest
	"bench press":         "D04AC939", // Example ID - needs verification
	"incline bench press": "incline-bench",
	"decline bench press": "decline-bench",
	"dumbbell press":      "dumbbell-press",
	"chest fly":           "chest-fly",
	"push up":             "push-up",

	// Back
	"lat pulldown":      "lat-pulldown",
	"pull up":           "pull-up",
	"chin up":           "chin-up",
	"barbell row":       "barbell-row",
	"dumbbell row":      "dumbbell-row",
	"cable row":         "cable-row",
	"deadlift":          "deadlift",
	"romanian deadlift": "romanian-deadlift",

	// Shoulders
	"overhead press": "overhead-press",
	"shoulder press": "shoulder-press",
	"lateral raise":  "lateral-raise",
	"front raise":    "front-raise",
	"face pull":      "face-pull",
	"shrug":          "shrug",

	// Arms
	"bicep curl":       "bicep-curl",
	"hammer curl":      "hammer-curl",
	"tricep extension": "tricep-extension",
	"tricep pushdown":  "tricep-pushdown",
	"skull crusher":    "skull-crusher",

	// Legs
	"squat":         "squat",
	"back squat":    "back-squat",
	"front squat":   "front-squat",
	"leg press":     "leg-press",
	"leg extension": "leg-extension",
	"leg curl":      "leg-curl",
	"lunge":         "lunge",
	"calf raise":    "calf-raise",
	"hip thrust":    "hip-thrust",

	// Core
	"plank":         "plank",
	"crunch":        "crunch",
	"russian twist": "russian-twist",
	"leg raise":     "leg-raise",
	"ab wheel":      "ab-wheel",

	// Cardio (distance-based)
	"running":         "running",
	"treadmill":       "treadmill",
	"cycling":         "cycling",
	"stationary bike": "stationary-bike",
	"rowing":          "rowing",
	"elliptical":      "elliptical",
	"stair climber":   "stair-climber",
	"walking":         "walking",
	"swimming":        "swimming",
}

// FuzzyMatchTemplate attempts to find a template ID for an exercise name
func FuzzyMatchTemplate(exerciseName string) (string, bool) {
	normalized := normalizeExerciseName(exerciseName)

	// Direct match
	if id, ok := CommonExerciseTemplates[normalized]; ok {
		return id, true
	}

	// Partial match - check if normalized name contains any known exercise
	for name, id := range CommonExerciseTemplates {
		if strings.Contains(normalized, name) || strings.Contains(name, normalized) {
			return id, true
		}
	}

	return "", false
}
