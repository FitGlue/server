package file_generators

import (
	"strings"

	"github.com/muktihari/fit/profile/typedef"
)

// MapExerciseToCategory maps exercise names to FIT exercise categories
// Returns the best matching category or a generic one if no match found
func MapExerciseToCategory(exerciseName string) typedef.ExerciseCategory {
	// Normalize the exercise name for matching
	name := strings.ToLower(strings.TrimSpace(exerciseName))

	// Chest exercises
	if strings.Contains(name, "bench press") || strings.Contains(name, "bench") {
		return typedef.ExerciseCategoryBenchPress
	}
	if strings.Contains(name, "chest press") || strings.Contains(name, "push up") || strings.Contains(name, "pushup") {
		return typedef.ExerciseCategoryBenchPress // Use bench press as closest match
	}
	if strings.Contains(name, "fly") || strings.Contains(name, "flye") {
		return typedef.ExerciseCategoryFlye
	}

	// Back exercises
	if strings.Contains(name, "deadlift") {
		return typedef.ExerciseCategoryDeadlift
	}
	if strings.Contains(name, "row") {
		return typedef.ExerciseCategoryRow
	}
	if strings.Contains(name, "pull up") || strings.Contains(name, "pullup") || strings.Contains(name, "chin up") {
		return typedef.ExerciseCategoryPullUp
	}
	if strings.Contains(name, "lat pulldown") || strings.Contains(name, "pulldown") {
		return typedef.ExerciseCategoryPullUp // Use pull up as closest match
	}

	// Leg exercises
	if strings.Contains(name, "squat") {
		return typedef.ExerciseCategorySquat
	}
	if strings.Contains(name, "lunge") {
		return typedef.ExerciseCategoryLunge
	}
	if strings.Contains(name, "leg press") {
		return typedef.ExerciseCategorySquat // Use squat as closest match
	}
	if strings.Contains(name, "leg curl") {
		return typedef.ExerciseCategoryLegCurl
	}
	if strings.Contains(name, "leg extension") {
		return typedef.ExerciseCategoryLegCurl // Use leg curl as closest match
	}
	if strings.Contains(name, "calf raise") {
		return typedef.ExerciseCategoryCalfRaise
	}

	// Shoulder exercises
	if strings.Contains(name, "shoulder press") || strings.Contains(name, "overhead press") || strings.Contains(name, "military press") {
		return typedef.ExerciseCategoryShoulderPress
	}
	if strings.Contains(name, "lateral raise") || strings.Contains(name, "side raise") {
		return typedef.ExerciseCategoryLateralRaise
	}
	if strings.Contains(name, "front raise") {
		return typedef.ExerciseCategoryLateralRaise // Use lateral raise as closest match
	}
	if strings.Contains(name, "rear delt") || strings.Contains(name, "reverse fly") {
		return typedef.ExerciseCategoryLateralRaise // Use lateral raise as closest match
	}
	if strings.Contains(name, "shrug") {
		return typedef.ExerciseCategoryShrug
	}

	// Arm exercises
	if strings.Contains(name, "bicep curl") || strings.Contains(name, "curl") {
		return typedef.ExerciseCategoryCurl // Generic curl category
	}
	if strings.Contains(name, "tricep extension") || strings.Contains(name, "triceps extension") {
		return typedef.ExerciseCategoryTricepsExtension
	}
	if strings.Contains(name, "tricep dip") || strings.Contains(name, "dip") {
		return typedef.ExerciseCategoryTricepsExtension // Use triceps extension as closest match
	}

	// Core exercises
	if strings.Contains(name, "crunch") || strings.Contains(name, "sit up") || strings.Contains(name, "situp") {
		return typedef.ExerciseCategoryCrunch
	}
	if strings.Contains(name, "plank") {
		return typedef.ExerciseCategoryPlank
	}

	// Olympic lifts
	if strings.Contains(name, "clean") {
		return typedef.ExerciseCategoryOlympicLift // Generic olympic lift
	}
	if strings.Contains(name, "snatch") {
		return typedef.ExerciseCategoryOlympicLift // Generic olympic lift
	}

	// Default to generic strength training
	return typedef.ExerciseCategoryTotalBody
}
