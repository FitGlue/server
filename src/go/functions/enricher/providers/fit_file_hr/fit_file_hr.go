package fit_file_hr

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/functions/enricher/providers/user_input"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/fit_parser"
	pendinginput "github.com/fitglue/server/src/go/pkg/pending_input"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// FitFileHRProvider allows users to upload a FIT file containing heart rate data
// which is then merged with the activity. This is useful for activities recorded
// without HR (e.g., treadmill with GPS watch) where HR was captured separately
// (e.g., chest strap syncing to a different device or Peloton).
type FitFileHRProvider struct {
	service *bootstrap.Service
}

func init() {
	providers.Register(NewFitFileHRProvider())
}

func NewFitFileHRProvider() *FitFileHRProvider {
	return &FitFileHRProvider{}
}

func (p *FitFileHRProvider) SetService(s *bootstrap.Service) {
	p.service = s
}

func (p *FitFileHRProvider) Name() string {
	return "fit-file-heart-rate"
}

func (p *FitFileHRProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_FIT_FILE_HEART_RATE
}

// Enrich checks if the activity already has heart rate data. If not, it creates
// a pending input requesting a FIT file upload.
func (p *FitFileHRProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Check force option - skip if activity already has heartrate data and force is not set
	forceOverwrite := inputs["force"] == "true"
	if !forceOverwrite && hasExistingHeartRateData(activity) {
		logger.Info("Skipping FIT file HR enrichment: activity already has heartrate data and force=false")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"hr_source":     "skipped",
				"status_detail": "Activity already has heartrate data",
				"force":         "false",
			},
		}, nil
	}

	if p.service == nil {
		logger.Debug("fit-file-hr: error - service not initialized")
		return nil, fmt.Errorf("service not initialized")
	}

	stableID := pendinginput.GenerateID(activity.Source, activity.ExternalId, p.Name())

	// Check if there's already a pending input for this activity
	pending, err := p.service.DB.GetPendingInput(ctx, user.UserId, stableID)
	if err == nil && pending != nil {
		if pending.Status == pb.PendingInput_STATUS_WAITING {
			logger.Debug("fit-file-hr: already waiting for user input")
			return &providers.EnrichmentResult{
				Metadata: map[string]string{
					"hr_source":     "pending",
					"status_detail": "Waiting for FIT file upload",
				},
			}, nil
		}
	}

	// Use the pre-generated activity_id from orchestrator for LinkedActivityId
	linkedActivityId := inputs["activity_id"]
	if linkedActivityId == "" {
		logger.Error("fit-file-hr: activity_id not in inputs - orchestrator bug")
		return nil, fmt.Errorf("activity_id not provided in enricher inputs")
	}

	logger.Info("fit-file-hr: requesting FIT file upload via pending input",
		"activity_id", stableID,
		"linked_activity_id", linkedActivityId)

	// Return WaitForInputError to halt the pipeline - the orchestrator's handleWaitError
	// will create the pending input with the original_payload_uri (GCS URI for payload retrieval)
	return nil, &user_input.WaitForInputError{
		ActivityID:         stableID,
		RequiredFields:     []string{"fit_file_base64"},
		EnricherProviderID: p.Name(),
		Metadata: map[string]string{
			"source_activity_id":   activity.ExternalId,
			"source_activity_type": activity.Source,
			"linked_activity_id":   linkedActivityId,
			"pipeline_id":          inputs["pipeline_id"],
		},
	}
}

// EnrichResume is called during resume mode to apply resolved pending input data.
// It parses the uploaded FIT file and extracts heart rate data for merging.
func (p *FitFileHRProvider) EnrichResume(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, pendingInput *pb.PendingInput) (*providers.EnrichmentResult, error) {
	fitFileBase64 := pendingInput.InputData["fit_file_base64"]
	if fitFileBase64 == "" {
		return nil, fmt.Errorf("fit_file_base64 not provided in pending input")
	}

	// Decode base64
	fitBytes, err := base64.StdEncoding.DecodeString(fitFileBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FIT file base64: %w", err)
	}

	// Parse the FIT file
	parsedActivity, err := fit_parser.ParseFitFile(fitBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse FIT file: %w", err)
	}

	// Extract heart rate data from parsed activity
	hrSamples := extractHRSamples(parsedActivity)
	if len(hrSamples) == 0 {
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"hr_source":     "fit_file",
				"status_detail": "No heart rate data found in uploaded FIT file",
				"points_found":  "0",
			},
		}, nil
	}

	// Build heart rate stream aligned to the target activity
	var stream []int
	alignmentMetadata := make(map[string]string)

	// Get target activity duration
	durationSec := 3600 // Default 1 hour
	if len(activity.Sessions) > 0 {
		durationSec = int(activity.Sessions[0].TotalElapsedTime)
	}

	// Check if GPS data exists for alignment
	if hasGPSData(activity) {
		// Use elastic matching for GPS+HR alignment
		gpsTimestamps := extractGPSTimestamps(activity)

		if len(gpsTimestamps) > 0 {
			alignResult, err := providers.AlignTimeSeries(gpsTimestamps, hrSamples, providers.DefaultAlignmentConfig, slog.Default())
			if err != nil {
				// Fallback to simple time-based mapping
				stream = buildStreamTimeBased(hrSamples, activity.StartTime.AsTime(), durationSec)
				alignmentMetadata["alignment_status"] = "fallback_time_based"
				alignmentMetadata["alignment_error"] = err.Error()
			} else {
				stream = alignResult.AlignedHR
				for k, v := range alignResult.Metadata {
					alignmentMetadata[k] = v
				}
				if alignResult.WarningMessage != "" {
					alignmentMetadata["alignment_warning"] = alignResult.WarningMessage
				}
			}
		} else {
			stream = buildStreamTimeBased(hrSamples, activity.StartTime.AsTime(), durationSec)
			alignmentMetadata["alignment_status"] = "time_based_no_gps_timestamps"
		}
	} else {
		// No GPS data - use time-based mapping
		stream = buildStreamTimeBased(hrSamples, activity.StartTime.AsTime(), durationSec)
		alignmentMetadata["alignment_status"] = "time_based_no_gps"
	}

	return &providers.EnrichmentResult{
		HeartRateStream: stream,
		Metadata: mergeMetadata(map[string]string{
			"hr_source":     "fit_file",
			"status_detail": "Success",
			"points_found":  fmt.Sprintf("%d", len(hrSamples)),
			"stream_length": fmt.Sprintf("%d", len(stream)),
		}, alignmentMetadata),
	}, nil
}

// hasExistingHeartRateData checks if the activity already has heart rate data in its records
func hasExistingHeartRateData(activity *pb.StandardizedActivity) bool {
	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.HeartRate > 0 {
					return true
				}
			}
		}
	}
	return false
}

// hasGPSData checks if any record in the activity has GPS coordinates
func hasGPSData(activity *pb.StandardizedActivity) bool {
	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.PositionLat != 0 || record.PositionLong != 0 {
					return true
				}
			}
		}
	}
	return false
}

// extractGPSTimestamps extracts all record timestamps from the activity
func extractGPSTimestamps(activity *pb.StandardizedActivity) []time.Time {
	var timestamps []time.Time
	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.Timestamp != nil {
					timestamps = append(timestamps, record.Timestamp.AsTime())
				}
			}
		}
	}
	return timestamps
}

// extractHRSamples extracts timed heart rate samples from a parsed FIT activity
func extractHRSamples(activity *pb.StandardizedActivity) []providers.TimedSample {
	var samples []providers.TimedSample
	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.HeartRate > 0 && record.Timestamp != nil {
					samples = append(samples, providers.TimedSample{
						Timestamp: record.Timestamp.AsTime(),
						Value:     int(record.HeartRate),
					})
				}
			}
		}
	}
	return samples
}

// buildStreamTimeBased creates an HR stream aligned by timestamps
func buildStreamTimeBased(samples []providers.TimedSample, activityStart time.Time, durationSec int) []int {
	stream := make([]int, durationSec)

	for _, sample := range samples {
		offset := int(sample.Timestamp.Sub(activityStart).Seconds())
		if offset >= 0 && offset < durationSec {
			stream[offset] = sample.Value
		}
	}

	// Forward fill gaps
	lastVal := 0
	for i := 0; i < len(stream); i++ {
		if stream[i] != 0 {
			lastVal = stream[i]
		} else {
			stream[i] = lastVal
		}
	}

	return stream
}

// mergeMetadata combines two metadata maps, with second map taking precedence
func mergeMetadata(base, overlay map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}
