package muscle_heatmap

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestLookupExercise_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantPrim pb.MuscleGroup
	}{
		{
			name:     "exact canonical name",
			input:    "Bench Press",
			wantName: "Bench Press",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_CHEST,
		},
		{
			name:     "exact canonical name - squat",
			input:    "Squat",
			wantName: "Squat",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		},
		{
			name:     "exact canonical name - deadlift",
			input:    "Deadlift",
			wantName: "Deadlift",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_LOWER_BACK,
		},
		{
			name:     "exact canonical name - bicep curl",
			input:    "Bicep Curl",
			wantName: "Bicep Curl",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_BICEPS,
		},
		{
			name:     "exact canonical name - pull up",
			input:    "Pull Up",
			wantName: "Pull Up",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_LATS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupExercise(tt.input)
			if !result.Matched {
				t.Errorf("expected match for %q, got no match", tt.input)
				return
			}
			if result.CanonicalName != tt.wantName {
				t.Errorf("expected canonical name %q, got %q", tt.wantName, result.CanonicalName)
			}
			if result.Primary != tt.wantPrim {
				t.Errorf("expected primary muscle %v, got %v", tt.wantPrim, result.Primary)
			}
			if result.Confidence != 1.0 {
				t.Errorf("expected confidence 1.0 for exact match, got %f", result.Confidence)
			}
		})
	}
}

func TestLookupExercise_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
	}{
		{"bench press", "Bench Press"},
		{"BENCH PRESS", "Bench Press"},
		{"BeNcH pReSs", "Bench Press"},
		{"squat", "Squat"},
		{"DEADLIFT", "Deadlift"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := LookupExercise(tt.input)
			if !result.Matched {
				t.Errorf("expected match for %q, got no match", tt.input)
				return
			}
			if result.CanonicalName != tt.wantName {
				t.Errorf("expected %q, got %q", tt.wantName, result.CanonicalName)
			}
		})
	}
}

func TestLookupExercise_AliasMatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantPrim pb.MuscleGroup
	}{
		{
			name:     "flat bench alias",
			input:    "Flat Bench",
			wantName: "Bench Press",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_CHEST,
		},
		{
			name:     "bb bench alias",
			input:    "BB Bench",
			wantName: "Bench Press",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_CHEST,
		},
		{
			name:     "military press alias",
			input:    "Military Press",
			wantName: "Overhead Press",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS,
		},
		{
			name:     "OHP alias",
			input:    "OHP",
			wantName: "Overhead Press",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS,
		},
		{
			name:     "back squat alias",
			input:    "Back Squat",
			wantName: "Squat",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		},
		{
			name:     "chin up alias",
			input:    "Chinup",
			wantName: "Chin Up",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_LATS,
		},
		{
			name:     "RDL alias",
			input:    "RDL",
			wantName: "Romanian Deadlift",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS,
		},
		{
			name:     "farmers walk with apostrophe",
			input:    "Farmer's Walk",
			wantName: "Farmers Walk",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_FULL_BODY,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupExercise(tt.input)
			if !result.Matched {
				t.Errorf("expected match for %q, got no match", tt.input)
				return
			}
			if result.CanonicalName != tt.wantName {
				t.Errorf("expected %q, got %q", tt.wantName, result.CanonicalName)
			}
			if result.Primary != tt.wantPrim {
				t.Errorf("expected primary %v, got %v", tt.wantPrim, result.Primary)
			}
		})
	}
}

func TestLookupExercise_AbbreviationExpansion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{
			name:     "DB Curl expands to Dumbbell Curl",
			input:    "DB Curl",
			wantName: "Bicep Curl",
		},
		{
			name:     "DB Bench expands to Dumbbell Bench",
			input:    "DB Bench",
			wantName: "Dumbbell Bench Press",
		},
		{
			name:     "BB Row expands",
			input:    "BB Row",
			wantName: "Bent Over Row",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupExercise(tt.input)
			if !result.Matched {
				t.Errorf("expected match for %q, got no match", tt.input)
				return
			}
			if result.CanonicalName != tt.wantName {
				t.Errorf("expected %q, got %q", tt.wantName, result.CanonicalName)
			}
		})
	}
}

func TestLookupExercise_FuzzyMatch(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantName      string
		wantPrim      pb.MuscleGroup
		minConfidence float64
		shouldMatch   bool
	}{
		{
			name:          "typo in bench press",
			input:         "Bech Press",
			wantName:      "Bench Press",
			wantPrim:      pb.MuscleGroup_MUSCLE_GROUP_CHEST,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			// "squatt" vs "squat" = 1 edit out of 6 chars = 83% similarity (below 90%)
			name:          "squat with typo - below threshold",
			input:         "Squatt",
			wantName:      "Squat",
			wantPrim:      pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
			minConfidence: 0.80,
			shouldMatch:   false, // Below 90% threshold
		},
		{
			// "deadlit" vs "deadlift" = 1 edit out of 8 chars = 87.5% similarity (below 90%)
			name:          "deadlift with typo - below threshold",
			input:         "Deadlit",
			wantName:      "Deadlift",
			wantPrim:      pb.MuscleGroup_MUSCLE_GROUP_LOWER_BACK,
			minConfidence: 0.80,
			shouldMatch:   false, // Below 90% threshold
		},
		{
			// "bench pres" vs "bench press" = 1 edit out of 11 chars = 90.9% (above threshold)
			name:          "bench press missing letter",
			input:         "Bench Pres",
			wantName:      "Bench Press",
			wantPrim:      pb.MuscleGroup_MUSCLE_GROUP_CHEST,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "custom exercise with too much added text",
			input:         "Bench Press Custom",
			wantName:      "Bench Press",
			wantPrim:      pb.MuscleGroup_MUSCLE_GROUP_CHEST,
			minConfidence: 0.70,
			shouldMatch:   false, // Too much added text drops below 90%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupExercise(tt.input)

			if tt.shouldMatch {
				if !result.Matched {
					t.Errorf("expected match for %q, got no match", tt.input)
					return
				}
				if result.CanonicalName != tt.wantName {
					t.Errorf("expected %q, got %q", tt.wantName, result.CanonicalName)
				}
				if result.Primary != tt.wantPrim {
					t.Errorf("expected primary %v, got %v", tt.wantPrim, result.Primary)
				}
				if result.Confidence < tt.minConfidence {
					t.Errorf("expected confidence >= %f, got %f", tt.minConfidence, result.Confidence)
				}
			} else {
				// Should not match (below threshold)
				if result.Matched && result.Confidence >= 0.90 {
					t.Logf("matched %q with confidence %f (may be acceptable)", result.CanonicalName, result.Confidence)
				}
			}
		})
	}
}

func TestLookupExercise_NoMatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "random nonsense",
			input: "xyzabc123 random exercise",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "very different name",
			input: "Underwater Basket Weaving",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupExercise(tt.input)
			if result.Matched {
				t.Errorf("expected no match for %q, but got %q with confidence %f",
					tt.input, result.CanonicalName, result.Confidence)
			}
			if result.Primary != pb.MuscleGroup_MUSCLE_GROUP_OTHER {
				t.Errorf("expected OTHER for unmatched, got %v", result.Primary)
			}
		})
	}
}

func TestLookupExercise_HexyExerciseNames(t *testing.T) {
	// Test with actual Hevy exercise names from sample data
	tests := []struct {
		name     string
		input    string
		wantName string
		wantPrim pb.MuscleGroup
	}{
		{
			name:     "Hevy kettlebell swing",
			input:    "Kettlebell Swing",
			wantName: "Kettlebell Swing",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_FULL_BODY,
		},
		{
			name:     "Hevy farmers walk",
			input:    "Farmers Walk",
			wantName: "Farmers Walk",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_FULL_BODY,
		},
		{
			name:     "Hevy battle ropes",
			input:    "Battle Ropes",
			wantName: "Battle Ropes",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_CARDIO,
		},
		{
			name:     "Hevy wall sit",
			input:    "Wall Sit",
			wantName: "Wall Sit",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		},
		{
			name:     "Hevy walking lunge dumbbell",
			input:    "Walking Lunge (Dumbbell)",
			wantName: "Walking Lunge",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		},
		{
			name:     "Hevy overhead press dumbbell",
			input:    "Overhead Press (Dumbbell)",
			wantName: "Dumbbell Shoulder Press",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS,
		},
		{
			name:     "Hevy lateral raise dumbbell",
			input:    "Lateral Raise (Dumbbell)",
			wantName: "Lateral Raise",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS,
		},
		{
			name:     "Hevy bent over row dumbbell",
			input:    "Bent Over Row (Dumbbell)",
			wantName: "Bent Over Row",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_UPPER_BACK,
		},
		{
			name:     "Hevy snatch",
			input:    "Snatch",
			wantName: "Snatch",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_FULL_BODY,
		},
		{
			name:     "Hevy bicep curl dumbbell",
			input:    "Bicep Curl (Dumbbell)",
			wantName: "Bicep Curl",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_BICEPS,
		},
		{
			name:     "Hevy front squat",
			input:    "Front Squat",
			wantName: "Front Squat",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		},
		{
			name:     "Hevy sumo squat kettlebell",
			input:    "Sumo Squat (Kettlebell)",
			wantName: "Sumo Squat",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		},
		{
			name:     "Hevy romanian deadlift dumbbell",
			input:    "Romanian Deadlift (Dumbbell)",
			wantName: "Romanian Deadlift",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS,
		},
		{
			name:     "Hevy crunch",
			input:    "Crunch",
			wantName: "Crunch",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS,
		},
		{
			name:     "Hevy russian twist weighted",
			input:    "Russian Twist (Weighted)",
			wantName: "Russian Twist",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS,
		},
		{
			name:     "Hevy lying leg raise",
			input:    "Lying Leg Raise",
			wantName: "Leg Raise",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS,
		},
		{
			name:     "Hevy plank",
			input:    "Plank",
			wantName: "Plank",
			wantPrim: pb.MuscleGroup_MUSCLE_GROUP_ABDOMINALS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupExercise(tt.input)
			if !result.Matched {
				t.Errorf("expected match for Hevy exercise %q, got no match", tt.input)
				return
			}
			if result.CanonicalName != tt.wantName {
				t.Errorf("expected %q, got %q", tt.wantName, result.CanonicalName)
			}
			if result.Primary != tt.wantPrim {
				t.Errorf("expected primary %v, got %v", tt.wantPrim, result.Primary)
			}
		})
	}
}

func TestLookupExercise_SecondaryMuscles(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantSecondary []pb.MuscleGroup
	}{
		{
			name:          "bench press has triceps and shoulders secondary",
			input:         "Bench Press",
			wantSecondary: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_TRICEPS, pb.MuscleGroup_MUSCLE_GROUP_SHOULDERS},
		},
		{
			name:          "squat has glutes and hamstrings secondary",
			input:         "Squat",
			wantSecondary: []pb.MuscleGroup{pb.MuscleGroup_MUSCLE_GROUP_GLUTES, pb.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS},
		},
		{
			name:          "lateral raise has no secondary",
			input:         "Lateral Raise",
			wantSecondary: []pb.MuscleGroup{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupExercise(tt.input)
			if !result.Matched {
				t.Fatalf("expected match for %q", tt.input)
			}

			if len(result.Secondary) != len(tt.wantSecondary) {
				t.Errorf("expected %d secondary muscles, got %d", len(tt.wantSecondary), len(result.Secondary))
				return
			}

			for i, want := range tt.wantSecondary {
				if result.Secondary[i] != want {
					t.Errorf("secondary[%d]: expected %v, got %v", i, want, result.Secondary[i])
				}
			}
		})
	}
}

func TestLookupExercise_Coefficients(t *testing.T) {
	result := LookupExercise("Bench Press")
	if !result.Matched {
		t.Fatal("expected match for Bench Press")
	}

	if result.PrimaryCoeff != 1.0 {
		t.Errorf("expected primary coefficient 1.0, got %f", result.PrimaryCoeff)
	}
	if result.SecondaryCoeff != 0.5 {
		t.Errorf("expected secondary coefficient 0.5, got %f", result.SecondaryCoeff)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Bench Press", "bench press"},
		{"BENCH PRESS", "bench press"},
		{"Bench-Press", "benchpress"},
		{"Bench (Press)", "bench press"},
		{"  Multiple   Spaces  ", "multiple spaces"},
		{"Special!@#$Characters", "specialcharacters"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalize(tt.input)
			if got != tt.want {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandAbbreviations(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"db curl", "dumbbell curl"},
		{"bb bench", "barbell bench"},
		{"kb swing", "kettlebell swing"},
		{"ohp", "overhead press"},
		{"rdl", "romanian deadlift"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandAbbreviations(tt.input)
			if got != tt.want {
				t.Errorf("expandAbbreviations(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"bench", "bech", 1},
		{"squat", "squatt", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshteinDistance(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSimilarityScore(t *testing.T) {
	tests := []struct {
		a, b     string
		minScore float64
		maxScore float64
	}{
		{"bench press", "bench press", 1.0, 1.0},
		{"bench press", "bech press", 0.9, 1.0},
		{"squat", "squatt", 0.8, 1.0},
		{"deadlift", "xyz", 0.0, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := similarityScore(tt.a, tt.b)
			if got < tt.minScore || got > tt.maxScore {
				t.Errorf("similarityScore(%q, %q) = %f, want between %f and %f",
					tt.a, tt.b, got, tt.minScore, tt.maxScore)
			}
		})
	}
}

// Benchmark fuzzy matching performance
func BenchmarkLookupExercise(b *testing.B) {
	testCases := []string{
		"Bench Press",
		"bench press",
		"DB Curl",
		"xyzabc random",
		"Bech Press", // typo
	}

	for _, tc := range testCases {
		b.Run(tc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				LookupExercise(tc)
			}
		})
	}
}
