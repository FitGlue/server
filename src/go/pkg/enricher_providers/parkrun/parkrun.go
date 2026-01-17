package parkrun

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Shared location service instance - initialized at startup
var locationService *ParkrunLocationsService

func init() {
	locationService = NewParkrunLocationsService()
	enricher_providers.Register(NewParkrunProvider())
}

// ParkrunProvider detects if an activity is a Parkrun event.
type ParkrunProvider struct {
	locationService *ParkrunLocationsService
	service         *bootstrap.Service
}

func NewParkrunProvider() *ParkrunProvider {
	return &ParkrunProvider{
		locationService: locationService,
	}
}

// NewParkrunProviderWithService creates a provider with a custom location service (for testing).
func NewParkrunProviderWithService(svc *ParkrunLocationsService) *ParkrunProvider {
	return &ParkrunProvider{
		locationService: svc,
	}
}

// SetService injects the bootstrap service for database access (resume mode support)
func (p *ParkrunProvider) SetService(s *bootstrap.Service) {
	p.service = s
}

func (p *ParkrunProvider) Name() string {
	return "parkrun"
}

func (p *ParkrunProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_PARKRUN
}

// EnrichResume is called during resume mode to apply resolved pending input data
func (p *ParkrunProvider) EnrichResume(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, pendingInput *pb.PendingInput) (*enricher_providers.EnrichmentResult, error) {
	// Extract resolved data from the pending input
	description := pendingInput.InputData["description"]
	position := pendingInput.InputData["position"]
	timeStr := pendingInput.InputData["time"]
	ageGrade := pendingInput.InputData["age_grade"]

	result := &enricher_providers.EnrichmentResult{
		Description: description,
		Metadata: map[string]string{
			"status":                "success",
			"is_parkrun":            "true",
			"results_applied":       "true",
			"parkrun_position":      position,
			"parkrun_time":          timeStr,
			"parkrun_age_grade":     ageGrade,
			"parkrun_results_state": "COMPLETE",
		},
	}

	return result, nil
}

func (p *ParkrunProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {

	// 0. Ensure locations are loaded
	if err := p.locationService.EnsureLoaded(ctx); err != nil {
		if doNotRetry {
			return &enricher_providers.EnrichmentResult{
				Metadata: map[string]string{
					"status":       "skipped",
					"reason":       "location_data_unavailable",
					"error_detail": err.Error(),
				},
			}, nil
		}
		return nil, enricher_providers.NewRetryableError(err, 5*time.Minute, "failed to load Parkrun locations")
	}

	// 1. Parse Inputs
	enableTitling := inputs["enable_titling"] != "false" // Default true
	titlePattern := inputs["title_pattern"]
	if titlePattern == "" {
		titlePattern = "{location}"
	}
	specialTitlePattern := inputs["special_title_pattern"]
	if specialTitlePattern == "" {
		specialTitlePattern = "{location} - {special} Edition"
	}
	tagValueStr := inputs["tags"]
	if _, ok := inputs["tags"]; !ok {
		tagValueStr = "Parkrun"
	}

	// 2. Basic Checks - Only care about Runs
	if activity.Type != pb.ActivityType_ACTIVITY_TYPE_RUN &&
		activity.Type != pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN &&
		activity.Type != pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN {
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "not_run_activity_type", "activity_type": activity.Type.String()},
		}, nil
	}

	// 3. Location Check
	lat, long, found := getStartLocation(activity)
	if !found {
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "no_gps_data"},
		}, nil
	}

	// 4. Time Check
	startTime := activity.StartTime.AsTime()
	if startTime.IsZero() {
		return nil, fmt.Errorf("invalid start time: zero")
	}

	// 5. Find nearest Parkrun location (200m threshold)
	matchedLocation := p.locationService.FindNearest(lat, long, 200.0)
	if matchedLocation == nil {
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "not_near_parkrun"},
		}, nil
	}

	// 6. Estimate local time based on longitude
	estimatedOffsetHours := matchedLocation.Longitude / 15.0
	estimatedLocalTime := startTime.Add(time.Duration(estimatedOffsetHours * float64(time.Hour)))

	// 7. Check for Parkrun day (Saturday or special event)
	isSaturday := estimatedLocalTime.Weekday() == time.Saturday
	specialDay := getSpecialDay(estimatedLocalTime)
	isSpecial := specialDay != ""

	if !isSaturday && !isSpecial {
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{
				"status": "skipped",
				"reason": "not_parkrun_day",
				"day":    estimatedLocalTime.Weekday().String(),
			},
		}, nil
	}

	// 8. Time Window Check (07:30 to 11:00 local time)
	hour := estimatedLocalTime.Hour()
	minute := estimatedLocalTime.Minute()
	totalMinutes := hour*60 + minute

	startWindow := 7*60 + 30 // 07:30
	endWindow := 11*60 + 0   // 11:00

	if totalMinutes < startWindow || totalMinutes > endWindow {
		return &enricher_providers.EnrichmentResult{
			Metadata: map[string]string{
				"status": "skipped",
				"reason": "outside_time_window",
				"time":   fmt.Sprintf("%02d:%02d", hour, minute),
			},
		}, nil
	}

	// 9. Match Found! Build result
	result := &enricher_providers.EnrichmentResult{
		Metadata: map[string]string{
			"status":             "success",
			"is_parkrun":         "true",
			"parkrun_event":      matchedLocation.Name,
			"parkrun_slug":       matchedLocation.EventSlug,
			"parkrun_country":    matchedLocation.CountryURL,
			"parkrun_is_special": fmt.Sprintf("%v", isSpecial),
		},
	}

	if isSpecial {
		result.Metadata["parkrun_special_day"] = specialDay
	}

	// 10. Apply title using pattern
	if enableTitling {
		result.Name = applyTitlePattern(titlePattern, specialTitlePattern, matchedLocation.Name, estimatedLocalTime, specialDay)
	}

	// 11. Apply tags
	if tagValueStr != "" {
		tags := strings.Split(tagValueStr, ",")
		result.Tags = make([]string, 0, len(tags))
		for _, t := range tags {
			if val := strings.TrimSpace(t); val != "" {
				result.Tags = append(result.Tags, val)
			}
		}
	}

	// 12. Results Enrichment: Create auto-populated pending input if enabled
	enableResultsEnrichment := inputs["enable_results_enrichment"] == "true"

	if enableResultsEnrichment && p.service != nil {
		// Check if user has Parkrun integration configured
		if user.Integrations != nil && user.Integrations.Parkrun != nil && user.Integrations.Parkrun.Enabled {
			// Create an auto-populated pending input to be resolved later by parkrun-results-source
			stableID := fmt.Sprintf("%s:%s", activity.Source, activity.ExternalId)

			// Calculate auto deadline (48 hours from now - Parkrun results usually come within 24h)
			autoDeadline := time.Now().Add(48 * time.Hour)

			pendingInput := &pb.PendingInput{
				ActivityId:                 stableID,
				UserId:                     user.UserId,
				Status:                     pb.PendingInput_STATUS_WAITING,
				RequiredFields:             []string{"description", "position", "time", "age_grade"},
				AutoPopulated:              true,
				ContinuedWithoutResolution: true,
				EnricherProviderId:         "parkrun",
				AutoDeadline:               timestamppb.New(autoDeadline),
				LinkedActivityId:           stableID, // Link to this activity
				OriginalPayload: &pb.ActivityPayload{
					UserId: user.UserId,
					Metadata: map[string]string{
						"parkrun_event_slug":   matchedLocation.EventSlug,
						"parkrun_event_name":   matchedLocation.Name,
						"parkrun_country":      matchedLocation.CountryURL,
						"source_activity_id":   activity.ExternalId,
						"source_activity_type": activity.Source,
					},
				},
				CreatedAt: timestamppb.Now(),
				UpdatedAt: timestamppb.Now(),
			}

			if err := p.service.DB.CreatePendingInput(ctx, pendingInput); err != nil {
				// Log but don't fail - we can still continue without results enrichment
				// slog would be nice here but we don't have access to logger
				result.Metadata["results_pending_input_error"] = err.Error()
			} else {
				result.Metadata["results_pending_input_created"] = "true"
				result.Metadata["results_auto_deadline"] = autoDeadline.Format(time.RFC3339)
			}

			result.Metadata["parkrun_results_state"] = "PENDING"
		} else {
			result.Metadata["parkrun_results_state"] = "DISABLED"
			result.Metadata["results_enrichment_skipped"] = "no_parkrun_integration"
		}
	} else {
		// Results enrichment not enabled or no service - mark as immediate (no pending results)
		result.Metadata["parkrun_results_state"] = "IMMEDIATE"
	}

	return result, nil
}

// getSpecialDay returns the special day name if applicable, or empty string.
func getSpecialDay(t time.Time) string {
	month := t.Month()
	day := t.Day()

	if month == time.December && day == 25 {
		return "Christmas Day"
	}
	if month == time.January && day == 1 {
		return "New Year's Day"
	}
	return ""
}

// applyTitlePattern applies the appropriate title pattern.
func applyTitlePattern(normalPattern, specialPattern, location string, t time.Time, specialDay string) string {
	pattern := normalPattern
	if specialDay != "" {
		pattern = specialPattern
	}

	dateFormatted := t.Format("2 Jan 2006")

	result := pattern
	result = strings.ReplaceAll(result, "{location}", location)
	result = strings.ReplaceAll(result, "{date}", dateFormatted)
	result = strings.ReplaceAll(result, "{special}", specialDay)

	return result
}

func getStartLocation(activity *pb.StandardizedActivity) (float64, float64, bool) {
	if len(activity.Sessions) == 0 {
		return 0, 0, false
	}
	for _, session := range activity.Sessions {
		if len(session.Laps) == 0 {
			continue
		}
		for _, lap := range session.Laps {
			if len(lap.Records) == 0 {
				continue
			}
			for _, rec := range lap.Records {
				if rec.PositionLat != 0 || rec.PositionLong != 0 {
					return rec.PositionLat, rec.PositionLong, true
				}
			}
		}
	}
	return 0, 0, false
}

// Haversine formula for distance calculation
func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}
