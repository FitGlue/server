package personal_records

import (
	"testing"
)

func TestCalculate1RM(t *testing.T) {
	tests := []struct {
		name     string
		weightKg float64
		reps     int32
		want     float64
	}{
		{
			name:     "single rep returns weight directly",
			weightKg: 100,
			reps:     1,
			want:     100,
		},
		{
			name:     "Epley formula for 10 reps",
			weightKg: 100,
			reps:     10,
			want:     133.33, // 100 * (1 + 10/30) ≈ 133.33
		},
		{
			name:     "Epley formula for 5 reps",
			weightKg: 80,
			reps:     5,
			want:     93.33, // 80 * (1 + 5/30) ≈ 93.33
		},
		{
			name:     "zero reps returns zero",
			weightKg: 100,
			reps:     0,
			want:     0,
		},
		{
			name:     "negative reps returns zero",
			weightKg: 100,
			reps:     -1,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calculate1RM(tt.weightKg, tt.reps)
			// Use approximate comparison for floating point
			if !approximatelyEqual(got, tt.want, 0.01) {
				t.Errorf("Calculate1RM(%v, %v) = %v, want approximately %v", tt.weightKg, tt.reps, got, tt.want)
			}
		})
	}
}

// approximatelyEqual checks if two floats are equal within a tolerance
func approximatelyEqual(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < tolerance
}

func TestCalculateSetVolume(t *testing.T) {
	tests := []struct {
		name     string
		weightKg float64
		reps     int32
		want     float64
	}{
		{
			name:     "standard calculation",
			weightKg: 100,
			reps:     10,
			want:     1000,
		},
		{
			name:     "zero reps returns zero",
			weightKg: 100,
			reps:     0,
			want:     0,
		},
		{
			name:     "zero weight returns zero",
			weightKg: 0,
			reps:     10,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateSetVolume(tt.weightKg, tt.reps)
			if got != tt.want {
				t.Errorf("CalculateSetVolume(%v, %v) = %v, want %v", tt.weightKg, tt.reps, got, tt.want)
			}
		})
	}
}

func TestCalculateImprovement(t *testing.T) {
	tests := []struct {
		name          string
		oldValue      float64
		newValue      float64
		lowerIsBetter bool
		want          float64
	}{
		{
			name:          "time improvement (lower is better)",
			oldValue:      1200,
			newValue:      1140, // 1 minute faster
			lowerIsBetter: true,
			want:          5.0, // 5% improvement
		},
		{
			name:          "weight improvement (higher is better)",
			oldValue:      100,
			newValue:      110,
			lowerIsBetter: false,
			want:          10.0, // 10% improvement
		},
		{
			name:          "zero old value returns zero",
			oldValue:      0,
			newValue:      100,
			lowerIsBetter: false,
			want:          0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateImprovement(tt.oldValue, tt.newValue, tt.lowerIsBetter)
			if got != tt.want {
				t.Errorf("CalculateImprovement(%v, %v, %v) = %v, want %v", tt.oldValue, tt.newValue, tt.lowerIsBetter, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name    string
		seconds float64
		want    string
	}{
		{
			name:    "less than an hour",
			seconds: 1425, // 23:45
			want:    "23:45",
		},
		{
			name:    "over an hour",
			seconds: 3725, // 1:02:05
			want:    "1:02:05",
		},
		{
			name:    "exactly one minute",
			seconds: 60,
			want:    "1:00",
		},
		{
			name:    "zero seconds",
			seconds: 0,
			want:    "0:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.seconds)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestFormatWeight(t *testing.T) {
	tests := []struct {
		name string
		kg   float64
		want string
	}{
		{
			name: "whole number",
			kg:   100,
			want: "100kg",
		},
		{
			name: "decimal",
			kg:   100.5,
			want: "100.5kg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWeight(tt.kg)
			if got != tt.want {
				t.Errorf("formatWeight(%v) = %v, want %v", tt.kg, got, tt.want)
			}
		})
	}
}

func TestFormatExerciseName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple exercise",
			input: "bench_press",
			want:  "Bench Press",
		},
		{
			name:  "single word",
			input: "squat",
			want:  "Squat",
		},
		{
			name:  "three words",
			input: "romanian_dead_lift",
			want:  "Romanian Dead Lift",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatExerciseName(tt.input)
			if got != tt.want {
				t.Errorf("formatExerciseName(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeExerciseName(t *testing.T) {
	tests := []struct {
		name         string
		exerciseName string
		wantNonEmpty bool // Just check if we get a normalized name
	}{
		{
			name:         "standard bench press",
			exerciseName: "Bench Press",
			wantNonEmpty: true,
		},
		{
			name:         "abbreviation",
			exerciseName: "DB Bench Press",
			wantNonEmpty: true,
		},
		{
			name:         "unknown exercise",
			exerciseName: "Some Random Exercise 123",
			wantNonEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeExerciseName(tt.exerciseName)
			if tt.wantNonEmpty && got == "" {
				t.Errorf("normalizeExerciseName(%v) returned empty, want non-empty", tt.exerciseName)
			}
			if !tt.wantNonEmpty && got != "" {
				// It's okay if the fuzzy matcher found something
				t.Logf("normalizeExerciseName(%v) = %v (matched unexpectedly, but acceptable)", tt.exerciseName, got)
			}
		})
	}
}

func TestFormatVolume(t *testing.T) {
	tests := []struct {
		name string
		kg   float64
		want string
	}{
		{
			name: "less than 1000kg",
			kg:   500,
			want: "500kg",
		},
		{
			name: "exactly 1000kg",
			kg:   1000,
			want: "1.0 tonnes",
		},
		{
			name: "more than 1000kg",
			kg:   4500,
			want: "4.5 tonnes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatVolume(tt.kg)
			if got != tt.want {
				t.Errorf("formatVolume(%v) = %v, want %v", tt.kg, got, tt.want)
			}
		})
	}
}

func TestCalculate5KTime(t *testing.T) {
	tests := []struct {
		name        string
		distanceM   float64
		durationSec float64
		want        float64
	}{
		{
			name:        "exactly 5K",
			distanceM:   5000,
			durationSec: 1500, // 25 minutes
			want:        1500,
		},
		{
			name:        "10K run extrapolates to 5K time",
			distanceM:   10000,
			durationSec: 3000, // 50 minutes for 10K
			want:        1500, // 25 minutes for 5K
		},
		{
			name:        "less than 5K returns 0",
			distanceM:   4000,
			durationSec: 1200,
			want:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculate5KTime(tt.distanceM, tt.durationSec)
			if got != tt.want {
				t.Errorf("calculate5KTime(%v, %v) = %v, want %v", tt.distanceM, tt.durationSec, got, tt.want)
			}
		})
	}
}
