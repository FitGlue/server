// Package personal_records provides Personal Record (PR) detection for cardio and strength activities.
package personal_records

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/functions/enricher/providers/muscle_heatmap"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PersonalRecordsProvider detects and stores personal records for activities
type PersonalRecordsProvider struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewPersonalRecordsProvider())
}

// NewPersonalRecordsProvider creates a new PersonalRecordsProvider
func NewPersonalRecordsProvider() *PersonalRecordsProvider {
	return &PersonalRecordsProvider{}
}

// SetService sets the bootstrap service for Firestore access
func (p *PersonalRecordsProvider) SetService(service *bootstrap.Service) {
	p.Service = service
}

// Name returns the provider name
func (p *PersonalRecordsProvider) Name() string {
	return "personal-records"
}

// ProviderType returns the protobuf enum for this provider
func (p *PersonalRecordsProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_PERSONAL_RECORDS
}

// Enrich processes the activity and detects any new personal records
func (p *PersonalRecordsProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Parse config options
	trackCardio := inputs["cardio_records"] != "false"     // Default true
	trackStrength := inputs["strength_records"] != "false" // Default true
	celebrateInTitle := inputs["celebrate_in_title"] == "true"

	var newPRs []NewPRResult
	userID := user.UserId

	// Check cardio records
	if trackCardio && IsCardioActivity(activity.Type) {
		cardioPRs, err := p.checkCardioRecords(ctx, logger, activity, userID)
		if err != nil {
			logger.Warn("Failed to check cardio records", "error", err)
		} else {
			newPRs = append(newPRs, cardioPRs...)
		}
	}

	// Check strength records
	if trackStrength && IsStrengthActivity(activity.Type) {
		strengthPRs, err := p.checkStrengthRecords(ctx, logger, activity, userID)
		if err != nil {
			logger.Warn("Failed to check strength records", "error", err)
		} else {
			newPRs = append(newPRs, strengthPRs...)
		}
	}

	// Check hybrid race records (detects from tags/enrichment metadata)
	hybridRaceType := detectHybridRaceType(activity)
	if hybridRaceType != "" {
		hybridPRs, err := p.checkHybridRaceRecords(ctx, logger, activity, userID, hybridRaceType)
		if err != nil {
			logger.Warn("Failed to check hybrid race records", "error", err)
		} else {
			newPRs = append(newPRs, hybridPRs...)
		}
	}

	if len(newPRs) == 0 {
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"pr_status": "no_new_prs",
			},
		}, nil
	}

	// Build the output with section title (matching other enrichers like heart_rate_zones)
	var sb strings.Builder
	sb.WriteString("🏆 Personal Records:\n")
	for _, pr := range newPRs {
		sb.WriteString(pr.DisplayMessage)
		sb.WriteString("\n")
	}
	prDescription := sb.String()

	result := &providers.EnrichmentResult{
		Description: prDescription,
		Metadata: map[string]string{
			"pr_status": "pr_detected",
			"pr_count":  fmt.Sprintf("%d", len(newPRs)),
		},
	}

	// Optionally add celebration to name
	if celebrateInTitle && len(newPRs) > 0 {
		result.Name = "🎉 " + activity.Name
	}

	logger.Info("Personal records detected",
		"pr_count", len(newPRs),
		"activity_type", activity.Type.String(),
	)

	return result, nil
}

// checkCardioRecords checks for cardio PRs and persists them to Firestore
func (p *PersonalRecordsProvider) checkCardioRecords(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, userID string) ([]NewPRResult, error) {
	var results []NewPRResult

	// Calculate total distance and duration
	var totalDistanceM float64
	var totalDurationSec float64

	for _, session := range activity.Sessions {
		totalDistanceM += session.TotalDistance
		totalDurationSec += session.TotalElapsedTime
	}

	if totalDistanceM == 0 {
		return results, nil
	}

	// Check time-based records for running activities
	if IsRunningActivity(activity.Type) {
		// Fastest 5K
		if totalDistanceM >= Distance5K {
			time5K := calculate5KTime(totalDistanceM, totalDurationSec)
			if time5K > 0 {
				pr, err := p.checkAndUpdateRecord(ctx, userID, string(RecordFastest5K), time5K, "seconds", activity, true)
				if err != nil {
					logger.Warn("Failed to check 5K record", "error", err)
				} else if pr != nil {
					results = append(results, *pr)
				}
			}
		}

		// Fastest 10K
		if totalDistanceM >= Distance10K {
			time10K := calculate10KTime(totalDistanceM, totalDurationSec)
			if time10K > 0 {
				pr, err := p.checkAndUpdateRecord(ctx, userID, string(RecordFastest10K), time10K, "seconds", activity, true)
				if err != nil {
					logger.Warn("Failed to check 10K record", "error", err)
				} else if pr != nil {
					results = append(results, *pr)
				}
			}
		}

		// Fastest Half Marathon
		if totalDistanceM >= DistanceHalfMarathon {
			timeHM := calculateHalfMarathonTime(totalDistanceM, totalDurationSec)
			if timeHM > 0 {
				pr, err := p.checkAndUpdateRecord(ctx, userID, string(RecordFastestHalfMarathon), timeHM, "seconds", activity, true)
				if err != nil {
					logger.Warn("Failed to check half marathon record", "error", err)
				} else if pr != nil {
					results = append(results, *pr)
				}
			}
		}

		// Longest Run
		pr, err := p.checkAndUpdateRecord(ctx, userID, string(RecordLongestRun), totalDistanceM, "meters", activity, false)
		if err != nil {
			logger.Warn("Failed to check longest run record", "error", err)
		} else if pr != nil {
			results = append(results, *pr)
		}
	}

	// Longest Ride for cycling
	if IsCyclingActivity(activity.Type) {
		pr, err := p.checkAndUpdateRecord(ctx, userID, string(RecordLongestRide), totalDistanceM, "meters", activity, false)
		if err != nil {
			logger.Warn("Failed to check longest ride record", "error", err)
		} else if pr != nil {
			results = append(results, *pr)
		}
	}

	return results, nil
}

// checkStrengthRecords checks for strength PRs and persists them to Firestore
func (p *PersonalRecordsProvider) checkStrengthRecords(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, userID string) ([]NewPRResult, error) {
	var results []NewPRResult

	// Group sets by normalized exercise name
	exerciseData := make(map[string]struct {
		Best1RM     float64
		TotalVolume float64
		MaxReps     int32
	})

	for _, session := range activity.Sessions {
		for _, set := range session.StrengthSets {
			if set.WeightKg <= 0 || set.Reps <= 0 {
				continue
			}

			// Normalize exercise name using muscle_heatmap fuzzy matcher
			normalizedName := normalizeExerciseName(set.ExerciseName)
			if normalizedName == "" {
				normalizedName = strings.ToLower(strings.ReplaceAll(set.ExerciseName, " ", "_"))
			}

			data := exerciseData[normalizedName]

			// Calculate 1RM for this set
			estimated1RM := Calculate1RM(set.WeightKg, set.Reps)
			if estimated1RM > data.Best1RM {
				data.Best1RM = estimated1RM
			}

			// Accumulate volume
			data.TotalVolume += CalculateSetVolume(set.WeightKg, set.Reps)

			// Track max reps in a single set
			if set.Reps > data.MaxReps {
				data.MaxReps = set.Reps
			}

			exerciseData[normalizedName] = data
		}
	}

	// Check each exercise for PRs
	for exerciseName, data := range exerciseData {
		// Check 1RM
		if data.Best1RM > 0 {
			recordType := exerciseName + string(Suffix1RM)
			pr, err := p.checkAndUpdateRecord(ctx, userID, recordType, data.Best1RM, "kg", activity, false)
			if err != nil {
				logger.Warn("Failed to check 1RM record", "error", err, "exercise", exerciseName)
			} else if pr != nil {
				results = append(results, *pr)
			}
		}

		// Check volume
		if data.TotalVolume > 0 {
			recordType := exerciseName + string(SuffixVolume)
			pr, err := p.checkAndUpdateRecord(ctx, userID, recordType, data.TotalVolume, "kg", activity, false)
			if err != nil {
				logger.Warn("Failed to check volume record", "error", err, "exercise", exerciseName)
			} else if pr != nil {
				results = append(results, *pr)
			}
		}

		// Check max reps
		if data.MaxReps > 0 {
			recordType := exerciseName + string(SuffixReps)
			pr, err := p.checkAndUpdateRecord(ctx, userID, recordType, float64(data.MaxReps), "reps", activity, false)
			if err != nil {
				logger.Warn("Failed to check reps record", "error", err, "exercise", exerciseName)
			} else if pr != nil {
				results = append(results, *pr)
			}
		}
	}

	return results, nil
}

// detectHybridRaceType checks activity tags for hybrid race type indicators.
// Only detects from explicit tags or activity name - NOT from exercise names.
// This ensures Hyrox PRs are only tracked during actual Hyrox-tagged events,
// not regular workouts that happen to include exercises like rowing or skierg.
func detectHybridRaceType(activity *pb.StandardizedActivity) string {
	// Check tags for hybrid race indicators
	for _, tag := range activity.Tags {
		lowerTag := strings.ToLower(tag)
		if strings.Contains(lowerTag, "hyrox") {
			return "hyrox"
		}
		if strings.Contains(lowerTag, "athx") {
			return "athx"
		}
	}

	// Check activity name
	lowerName := strings.ToLower(activity.Name)
	if strings.Contains(lowerName, "hyrox") {
		return "hyrox"
	}
	if strings.Contains(lowerName, "athx") {
		return "athx"
	}

	return ""
}

// checkHybridRaceRecords checks for hybrid race PRs (total time and individual stations)
func (p *PersonalRecordsProvider) checkHybridRaceRecords(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, userID, raceType string) ([]NewPRResult, error) {
	var results []NewPRResult

	// Calculate total activity time
	var totalDurationSec float64
	for _, session := range activity.Sessions {
		totalDurationSec += session.TotalElapsedTime
	}

	// Check total race time PR
	if totalDurationSec > 0 {
		recordType := FormatHybridRaceRecordType(raceType, "total_time")
		pr, err := p.checkAndUpdateRecord(ctx, userID, recordType, totalDurationSec, "seconds", activity, true)
		if err != nil {
			logger.Warn("Failed to check hybrid race total time", "error", err)
		} else if pr != nil {
			results = append(results, *pr)
		}
	}

	// Check individual station PRs (from StrengthSets)
	for _, session := range activity.Sessions {
		for _, set := range session.StrengthSets {
			if set.DurationSeconds <= 0 {
				continue
			}

			// Normalize station name for record key
			stationKey := normalizeStationName(set.ExerciseName)
			if stationKey == "" {
				continue
			}

			recordType := FormatHybridRaceRecordType(raceType, stationKey)
			pr, err := p.checkAndUpdateRecord(ctx, userID, recordType, float64(set.DurationSeconds), "seconds", activity, true)
			if err != nil {
				logger.Warn("Failed to check hybrid race station PR", "error", err, "station", stationKey)
			} else if pr != nil {
				results = append(results, *pr)
			}
		}
	}

	logger.Info("Checked hybrid race records",
		"race_type", raceType,
		"prs_found", len(results),
	)

	return results, nil
}

// normalizeStationName converts exercise names to station keys for PRs
func normalizeStationName(exerciseName string) string {
	lower := strings.ToLower(exerciseName)
	switch {
	case strings.Contains(lower, "skierg"):
		return "skierg"
	case strings.Contains(lower, "sled push"):
		return "sled_push"
	case strings.Contains(lower, "sled pull"):
		return "sled_pull"
	case strings.Contains(lower, "burpee"):
		return "burpee_broad_jump"
	case strings.Contains(lower, "row"):
		return "rowing"
	case strings.Contains(lower, "farmer"):
		return "farmers_carry"
	case strings.Contains(lower, "sandbag"), strings.Contains(lower, "lunge"):
		return "sandbag_lunges"
	case strings.Contains(lower, "wall ball"):
		return "wall_balls"
	default:
		return ""
	}
}

// checkAndUpdateRecord compares the new value with the existing record and updates if it's a PR
func (p *PersonalRecordsProvider) checkAndUpdateRecord(ctx context.Context, userID, recordType string, newValue float64, unit string, activity *pb.StandardizedActivity, lowerIsBetter bool) (*NewPRResult, error) {
	// Get existing record from Firestore
	existingRecord, err := p.Service.DB.GetPersonalRecord(ctx, userID, recordType)
	if err != nil {
		// Check if it's a "not found" error (which is OK - first record)
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
			return nil, fmt.Errorf("failed to get existing record: %w", err)
		}
		existingRecord = nil
	}

	// Determine if this is a new PR
	isNewPR := false
	if existingRecord == nil {
		isNewPR = true
	} else if lowerIsBetter {
		isNewPR = newValue < existingRecord.Value
	} else {
		isNewPR = newValue > existingRecord.Value
	}

	if !isNewPR {
		return nil, nil
	}

	// Calculate improvement
	var previousValue *float64
	var improvement *float64
	if existingRecord != nil {
		pv := existingRecord.Value
		previousValue = &pv
		imp := CalculateImprovement(pv, newValue, lowerIsBetter)
		improvement = &imp
	}

	// Create new record
	newRecord := &pb.PersonalRecord{
		RecordType:   recordType,
		Value:        newValue,
		Unit:         unit,
		ActivityId:   activity.ExternalId,
		AchievedAt:   timestamppb.Now(),
		ActivityType: activity.Type,
	}
	if previousValue != nil {
		newRecord.PreviousValue = previousValue
	}
	if improvement != nil {
		newRecord.Improvement = improvement
	}

	// Save to Firestore
	if err := p.Service.DB.SetPersonalRecord(ctx, userID, newRecord); err != nil {
		return nil, fmt.Errorf("failed to save record: %w", err)
	}

	// Format display message
	displayMessage := p.formatPRMessage(recordType, newValue, previousValue, improvement, unit, lowerIsBetter)

	return &NewPRResult{
		RecordType:     recordType,
		NewValue:       newValue,
		PreviousValue:  previousValue,
		Improvement:    improvement,
		Unit:           unit,
		DisplayMessage: displayMessage,
	}, nil
}

// formatPRMessage creates a user-friendly PR announcement
func (p *PersonalRecordsProvider) formatPRMessage(recordType string, newValue float64, previousValue, improvement *float64, unit string, lowerIsBetter bool) string {
	// Determine emoji based on record type
	emoji := "🏆"
	if strings.Contains(recordType, "_volume") {
		emoji = "💪"
	} else if strings.HasPrefix(recordType, "fastest_") {
		emoji = "🎉"
	}

	// Format record type for display
	displayName := formatRecordTypeForDisplay(recordType)

	// Format value based on unit
	var valueStr string
	switch unit {
	case "seconds":
		valueStr = formatDuration(newValue)
	case "meters":
		if newValue >= 1000 {
			valueStr = fmt.Sprintf("%.2fkm", newValue/1000)
		} else {
			valueStr = fmt.Sprintf("%.0fm", newValue)
		}
	case "kg":
		valueStr = formatWeight(newValue)
	case "reps":
		valueStr = fmt.Sprintf("%d reps", int(newValue))
	default:
		valueStr = fmt.Sprintf("%.2f %s", newValue, unit)
	}

	// Build message
	message := fmt.Sprintf("%s NEW PR! %s: %s", emoji, displayName, valueStr)

	// Add comparison with previous value
	if previousValue != nil && improvement != nil {
		var prevStr string
		switch unit {
		case "seconds":
			prevStr = formatDuration(*previousValue)
		case "meters":
			if *previousValue >= 1000 {
				prevStr = fmt.Sprintf("%.2fkm", *previousValue/1000)
			} else {
				prevStr = fmt.Sprintf("%.0fm", *previousValue)
			}
		case "kg":
			prevStr = formatWeight(*previousValue)
		case "reps":
			prevStr = fmt.Sprintf("%d reps", int(*previousValue))
		default:
			prevStr = fmt.Sprintf("%.2f", *previousValue)
		}

		impSign := "+"
		impVal := *improvement
		if lowerIsBetter {
			// For time records, positive improvement = faster = better
			impSign = "-"
		} else if impVal < 0 {
			impSign = ""
		}
		message += fmt.Sprintf(" (previous: %s, %s%.1f%%)", prevStr, impSign, math.Abs(impVal))
	}

	return message
}

// formatRecordTypeForDisplay converts record type to human-readable format
func formatRecordTypeForDisplay(recordType string) string {
	// Handle special cardio record types
	switch recordType {
	case string(RecordFastest5K):
		return "Fastest 5K"
	case string(RecordFastest10K):
		return "Fastest 10K"
	case string(RecordFastestHalfMarathon):
		return "Fastest Half Marathon"
	case string(RecordLongestRun):
		return "Longest Run"
	case string(RecordLongestRide):
		return "Longest Ride"
	case string(RecordHighestElevationGain):
		return "Highest Elevation Gain"
	}

	// Handle strength record types
	if strings.HasSuffix(recordType, string(Suffix1RM)) {
		exerciseName := strings.TrimSuffix(recordType, string(Suffix1RM))
		return formatExerciseName(exerciseName) + " 1RM"
	}
	if strings.HasSuffix(recordType, string(SuffixVolume)) {
		exerciseName := strings.TrimSuffix(recordType, string(SuffixVolume))
		return formatExerciseName(exerciseName) + " Volume"
	}
	if strings.HasSuffix(recordType, string(SuffixReps)) {
		exerciseName := strings.TrimSuffix(recordType, string(SuffixReps))
		return formatExerciseName(exerciseName) + " Max Reps"
	}

	// Handle hybrid race record types
	if strings.HasPrefix(recordType, "hybrid_race_") {
		raceType, category := ParseHybridRaceRecordType(recordType)
		if raceType != "" && category != "" {
			raceDisplay := strings.ToUpper(raceType) // HYROX, ATHX
			categoryDisplay := formatExerciseName(category)
			if category == "total_time" {
				return raceDisplay + " Total Time"
			}
			return raceDisplay + " " + categoryDisplay
		}
	}

	// Fallback: convert snake_case to Title Case
	parts := strings.Split(recordType, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

// normalizeExerciseName uses the muscle_heatmap fuzzy matcher to normalize exercise names
func normalizeExerciseName(name string) string {
	result := muscle_heatmap.LookupExercise(name)
	if result.Matched {
		// Convert canonical name to snake_case for record type
		normalized := strings.ToLower(result.CanonicalName)
		normalized = strings.ReplaceAll(normalized, " ", "_")
		return normalized
	}
	return ""
}

// formatExerciseName converts snake_case to Title Case
func formatExerciseName(name string) string {
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

// formatDuration formats seconds into MM:SS or HH:MM:SS
func formatDuration(seconds float64) string {
	totalSeconds := int(math.Round(seconds))
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	secs := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

// formatWeight formats weight in kg with appropriate precision
func formatWeight(kg float64) string {
	if kg == float64(int(kg)) {
		return fmt.Sprintf("%dkg", int(kg))
	}
	return fmt.Sprintf("%.1fkg", kg)
}

// formatVolume formats volume with appropriate units (kg or tonnes)
func formatVolume(kg float64) string {
	if kg >= 1000 {
		return fmt.Sprintf("%.1f tonnes", kg/1000)
	}
	return fmt.Sprintf("%.0fkg", kg)
}

// calculate5KTime estimates the 5K time from total distance and duration
func calculate5KTime(totalDistanceM, totalDurationSec float64) float64 {
	if totalDistanceM < Distance5K {
		return 0
	}
	// Proportional time for 5K based on total activity
	return (Distance5K / totalDistanceM) * totalDurationSec
}

// calculate10KTime estimates the 10K time from total distance and duration
func calculate10KTime(totalDistanceM, totalDurationSec float64) float64 {
	if totalDistanceM < Distance10K {
		return 0
	}
	return (Distance10K / totalDistanceM) * totalDurationSec
}

// calculateHalfMarathonTime estimates the half marathon time from total distance and duration
func calculateHalfMarathonTime(totalDistanceM, totalDurationSec float64) float64 {
	if totalDistanceM < DistanceHalfMarathon {
		return 0
	}
	return (DistanceHalfMarathon / totalDistanceM) * totalDurationSec
}
