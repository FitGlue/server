package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	shared "github.com/fitglue/server/src/go/pkg"
	fit "github.com/fitglue/server/src/go/pkg/domain/file_generators"
	"github.com/fitglue/server/src/go/pkg/domain/tier"
	infrasentry "github.com/fitglue/server/src/go/pkg/infrastructure/sentry"
	pendinginput "github.com/fitglue/server/src/go/pkg/pending_input"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/functions/enricher/providers/user_input"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// temporarilyUnavailableEnrichers is a skip-list for enrichers that are awaiting API access.
// When an enricher is added here, it will be skipped during pipeline execution even if configured.
// Remove entries from this map once API access is granted and the enricher is ready.
var temporarilyUnavailableEnrichers = map[pb.EnricherProviderType]bool{
	// Example: pb.EnricherProviderType_ENRICHER_PROVIDER_POLAR_TRACKS: true,
}

type Orchestrator struct {
	database        shared.Database
	storage         shared.BlobStore
	bucketName      string
	providersByName map[string]providers.Provider
	providersByType map[pb.EnricherProviderType]providers.Provider
	notifications   shared.NotificationService
}

func NewOrchestrator(db shared.Database, storage shared.BlobStore, bucketName string, notifications shared.NotificationService) *Orchestrator {
	return &Orchestrator{
		database:        db,
		storage:         storage,
		bucketName:      bucketName,
		providersByName: make(map[string]providers.Provider),
		providersByType: make(map[pb.EnricherProviderType]providers.Provider),
		notifications:   notifications,
	}
}

func (o *Orchestrator) Register(p providers.Provider) {
	o.providersByName[p.Name()] = p
	if t := p.ProviderType(); t != pb.EnricherProviderType_ENRICHER_PROVIDER_UNSPECIFIED {
		o.providersByType[t] = p
	}
}

// ProcessResult contains detailed information about the enrichment process
type ProcessResult struct {
	Events             []*pb.EnrichedActivityEvent
	ProviderExecutions []ProviderExecution
	Status             pb.ExecutionStatus
}

// ProviderExecution tracks a single provider's execution
type ProviderExecution struct {
	ProviderName string
	ExecutionID  string
	Status       string
	Error        string
	DurationMs   int64
	Metadata     map[string]string
}

// Process executes the enrichment pipelines for the activity
func (o *Orchestrator) Process(ctx context.Context, logger *slog.Logger, payload *pb.ActivityPayload, parentExecutionID string, basePipelineExecutionID string, doNotRetry bool) (*ProcessResult, error) {
	// 1. Fetch User Config
	userRec, err := o.database.GetUser(ctx, payload.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user config: %w", err)
	}

	// 1.1. Check Tier Limits
	if tier.ShouldResetSyncCount(userRec) {
		// Reset monthly counter
		if err := o.database.ResetSyncCount(ctx, payload.UserId); err != nil {
			logger.Warn("Failed to reset sync count", "error", err, "userId", payload.UserId)
		}
		userRec.SyncCountThisMonth = 0
	}

	allowed, reason := tier.CanSync(userRec)
	if !allowed {
		logger.Info("Sync blocked by tier limit", "userId", payload.UserId, "reason", reason)
		// Track prevented sync
		if err := o.database.IncrementPreventedSyncCount(ctx, payload.UserId); err != nil {
			logger.Warn("Failed to increment prevented sync count", "error", err, "userId", payload.UserId)
		}

		// Create a visible TIER_BLOCKED PipelineRun so user sees the blocked activity
		// and can be prompted to upgrade
		if payload.StandardizedActivity != nil &&
			len(payload.StandardizedActivity.Sessions) > 0 &&
			payload.PipelineId != nil && *payload.PipelineId != "" {

			activity := payload.StandardizedActivity
			activityId := uuid.NewString()

			blockedRun := &pb.PipelineRun{
				Id:               basePipelineExecutionID,
				PipelineId:       *payload.PipelineId,
				ActivityId:       activityId,
				Source:           payload.Source.String(),
				SourceActivityId: activity.GetExternalId(),
				Title:            activity.GetName(),
				Description:      activity.GetDescription(),
				Type:             activity.GetType(),
				StartTime:        activity.GetSessions()[0].GetStartTime(),
				Status:           pb.PipelineRunStatus_PIPELINE_RUN_STATUS_TIER_BLOCKED,
				StatusMessage:    &reason,
				CreatedAt:        timestamppb.Now(),
				UpdatedAt:        timestamppb.Now(),
				Destinations:     []*pb.DestinationOutcome{}, // No destinations for blocked runs
			}

			if err := o.database.CreatePipelineRun(ctx, payload.UserId, blockedRun); err != nil {
				logger.Warn("Failed to create tier-blocked pipeline run", "error", err)
			} else {
				logger.Info("Created tier-blocked pipeline run", "pipeline_run_id", blockedRun.Id, "activity_id", activityId)
			}
		}

		return &ProcessResult{
			Events:             []*pb.EnrichedActivityEvent{},
			ProviderExecutions: []ProviderExecution{},
			Status:             pb.ExecutionStatus_STATUS_SKIPPED,
		}, nil // Return nil error - this is a controlled halt, not an exception
	}

	// 1.5. Validate Payload
	if payload.StandardizedActivity == nil {
		return nil, fmt.Errorf("standardized activity is nil")
	}
	if len(payload.StandardizedActivity.Sessions) != 1 {
		logger.Error("Activity does not have exactly one session", "count", len(payload.StandardizedActivity.Sessions))
		return nil, fmt.Errorf("multiple sessions not supported")
	}
	if payload.StandardizedActivity.Sessions[0].TotalElapsedTime == 0 {
		logger.Error("Activity session has 0 elapsed time")
		return nil, fmt.Errorf("session total elapsed time is 0")
	}

	// 2. MANDATORY: Pipeline ID is required (Rule E25: Per-Pipeline Isolation via Splitter)
	// The enricher ONLY receives targeted messages from the pipeline-splitter.
	// Each invocation processes exactly one pipeline with clean memory and a dedicated trace.
	if payload.PipelineId == nil || *payload.PipelineId == "" {
		logger.Error("pipeline_id is required - enricher only accepts targeted messages from splitter")
		return nil, fmt.Errorf("pipeline_id is required")
	}

	pipelineID := *payload.PipelineId
	logger.Info("Processing targeted pipeline", "pipeline_id", pipelineID, "is_resume", payload.IsResume)

	// 2.1 Resolve the targeted pipeline by ID
	pipeline, err := o.resolvePipeline(ctx, pipelineID, userRec.UserId, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve pipeline: %w", err)
	}
	if pipeline == nil {
		logger.Error("Targeted pipeline not found or disabled", "pipeline_id", pipelineID)
		return &ProcessResult{
			Events:             []*pb.EnrichedActivityEvent{},
			ProviderExecutions: []ProviderExecution{},
			Status:             pb.ExecutionStatus_STATUS_SKIPPED,
		}, nil
	}

	// 2.2 Handle Resume Mode flags
	isResumeMode := payload.IsResume
	resumeOnlyEnrichers := payload.ResumeOnlyEnrichers
	useUpdateMethod := payload.UseUpdateMethod

	if isResumeMode {
		logger.Info("Resume mode activated",
			"resume_only_enrichers", resumeOnlyEnrichers,
			"use_update_method", useUpdateMethod,
			"resume_pending_input_id", payload.ResumePendingInputId,
			"pipeline_id", pipelineID)
	}

	var providerExecutions []ProviderExecution

	// 3. Execute the Pipeline (Single Pipeline Mode)
	// Note: basePipelineExecutionID already contains the pipeline ID (appended by pipeline-splitter)
	pipelineExecutionID := basePipelineExecutionID
	logger.Info("Executing pipeline", "id", pipeline.ID, "pipelineExecutionId", pipelineExecutionID)

	// Pre-generate ActivityId so enrichers can use it for pending input linking
	// In resume mode, use the provided ActivityId; otherwise generate a new one
	var activityId string
	if isResumeMode {
		if payload.ActivityId == nil || *payload.ActivityId == "" {
			return nil, fmt.Errorf("resume mode requires activity_id to be set")
		}
		activityId = *payload.ActivityId
	} else {
		activityId = uuid.NewString()
	}
	logger.Debug("Activity ID for pipeline", "activity_id", activityId, "is_resume", isResumeMode)

	// Create initial pipeline run document for lifecycle tracking (RUNNING status)
	// This ensures we track the pipeline execution even if it fails partway through
	o.createInitialPipelineRun(ctx, logger, payload.UserId, pipelineExecutionID, pipeline.ID, activityId, payload, pipeline.Destinations)

	// Upload original payload to GCS for Magic Actions (retry/repost) BEFORE any mutations
	// This ensures the stored payload has the clean original description (Rule E22: Reset-on-Repost)
	originalPayloadUri := ""
	if o.storage != nil && o.bucketName != "" {
		payloadPath := fmt.Sprintf("payloads/%s/%s.json", payload.UserId, activityId)
		payloadBytes, err := protojson.Marshal(payload)
		if err != nil {
			logger.Warn("Failed to marshal original payload for GCS", "error", err)
		} else if err := o.storage.Write(ctx, o.bucketName, payloadPath, payloadBytes); err != nil {
			logger.Warn("Failed to upload original payload to GCS", "error", err)
		} else {
			originalPayloadUri = fmt.Sprintf("gs://%s/%s", o.bucketName, payloadPath)
			logger.Debug("Uploaded original payload to GCS", "uri", originalPayloadUri)

			// Update pipeline run with GCS URI immediately so it's available even if pipeline fails early
			// This ensures full-pipeline repost can always retrieve the original payload
			if err := o.database.UpdatePipelineRun(ctx, payload.UserId, pipelineExecutionID, map[string]interface{}{
				"original_payload_uri": originalPayloadUri,
			}); err != nil {
				logger.Warn("Failed to update pipeline run with original payload URI", "error", err)
			}
		}
	}

	// 3a. Execute Enrichers Sequentially
	configs := pipeline.Enrichers
	results := make([]*providers.EnrichmentResult, len(configs))

	// Use the activity directly - no cloning needed since we process exactly one pipeline
	currentActivity := payload.StandardizedActivity

	// Save the original description and build enriched description separately
	// to prevent stacking across reposts
	originalDescription := currentActivity.Description
	var descriptionBuilder strings.Builder
	if originalDescription != "" {
		descriptionBuilder.WriteString(originalDescription)
	}

	for i, cfg := range configs {
		var provider providers.Provider
		var ok bool

		// Lookup by Type
		provider, ok = o.providersByType[cfg.ProviderType]
		if !ok {
			logger.Warn("Provider not found for type", "type", cfg.ProviderType)
			// Send Sentry warning - this is a configuration issue that should be investigated
			infrasentry.CaptureMessage(
				fmt.Sprintf("Enricher provider not registered: %s", cfg.ProviderType),
				"warning",
				map[string]interface{}{
					"provider_type": cfg.ProviderType.String(),
					"pipeline_id":   pipeline.ID,
					"user_id":       payload.UserId,
				},
				logger,
			)
			providerExecutions = append(providerExecutions, ProviderExecution{
				ProviderName: fmt.Sprintf("TYPE:%s", cfg.ProviderType),
				Status:       "SKIPPED",
				Error:        "provider not registered",
			})
			continue
		}

		// Skip temporarily unavailable enrichers
		if temporarilyUnavailableEnrichers[cfg.ProviderType] {
			logger.Info("Skipping temporarily unavailable enricher", "type", cfg.ProviderType, "name", provider.Name())
			providerExecutions = append(providerExecutions, ProviderExecution{
				ProviderName: provider.Name(),
				Status:       "SKIPPED",
				Error:        "temporarily unavailable",
				Metadata:     map[string]string{"skip_reason": "temporarily_unavailable"},
			})
			continue
		}

		// 3a.1 Resume Mode: Skip enrichers not in the resume list
		if isResumeMode && len(resumeOnlyEnrichers) > 0 {
			shouldRun := false
			for _, allowedName := range resumeOnlyEnrichers {
				if provider.Name() == allowedName {
					shouldRun = true
					break
				}
			}
			if !shouldRun {
				logger.Debug("Skipping enricher in resume mode", "name", provider.Name())
				providerExecutions = append(providerExecutions, ProviderExecution{
					ProviderName: provider.Name(),
					Status:       "SKIPPED",
					Metadata:     map[string]string{"skip_reason": "not_in_resume_list"},
				})
				continue
			}
		}

		startTime := time.Now()
		execID := uuid.NewString()

		pe := ProviderExecution{
			ProviderName: provider.Name(),
			ExecutionID:  execID,
			Status:       "STARTED",
		}

		// Merge pipelineExecutionID, pipelineID, and activityId into config for providers
		enricherConfig := make(map[string]string)
		for k, v := range cfg.TypedConfig {
			enricherConfig[k] = v
		}
		enricherConfig["pipeline_execution_id"] = pipelineExecutionID
		enricherConfig["pipeline_id"] = pipeline.ID
		enricherConfig["activity_id"] = activityId // For pending input linking

		// Clear stale pending inputs when re-running (not resuming)
		// This allows users to provide different input on a fresh re-run.
		if !isResumeMode {
			staleInputID := pendinginput.GenerateID(currentActivity.Source, currentActivity.ExternalId, provider.Name())
			existingInput, fetchErr := o.database.GetPendingInput(ctx, payload.UserId, staleInputID)
			if fetchErr == nil && existingInput != nil && existingInput.Status == pb.PendingInput_STATUS_WAITING {
				logger.Info("Clearing stale pending input for re-run", "provider", provider.Name(), "pending_input_id", staleInputID)
				if delErr := o.database.DeletePendingInput(ctx, payload.UserId, staleInputID); delErr != nil {
					logger.Warn("Failed to delete stale pending input", "error", delErr, "pending_input_id", staleInputID)
				}
			}
		}

		// Execute
		// TODO: Get logger from FrameworkContext when orchestrator is refactored
		providerLogger := slog.Default().With("provider", provider.Name())

		var res *providers.EnrichmentResult
		var err error

		// Resume Mode: Check if provider supports EnrichResume and we have a pending input to resolve
		if isResumeMode && payload.ResumePendingInputId != nil && *payload.ResumePendingInputId != "" {
			if resumable, ok := provider.(providers.ResumableProvider); ok {
				// Fetch the resolved pending input from database
				pendingInput, fetchErr := o.database.GetPendingInput(ctx, payload.UserId, *payload.ResumePendingInputId)
				if fetchErr != nil {
					logger.Warn("Failed to fetch pending input for resume", "error", fetchErr, "pending_input_id", *payload.ResumePendingInputId)
					// Fall back to regular Enrich
					res, err = provider.Enrich(ctx, providerLogger, currentActivity, userRec, enricherConfig, doNotRetry)
				} else if pendingInput == nil || pendingInput.Status != pb.PendingInput_STATUS_COMPLETED {
					logger.Warn("Pending input not found or not completed", "pending_input_id", *payload.ResumePendingInputId, "status", pendingInput.GetStatus())
					// Fall back to regular Enrich
					res, err = provider.Enrich(ctx, providerLogger, currentActivity, userRec, enricherConfig, doNotRetry)
				} else {
					// Call EnrichResume with the resolved pending input
					logger.Info("Calling EnrichResume with resolved pending input", "provider", provider.Name(), "pending_input_id", *payload.ResumePendingInputId)
					res, err = resumable.EnrichResume(ctx, currentActivity, userRec, pendingInput)
				}
			} else {
				// Provider doesn't support resume mode, use regular Enrich
				res, err = provider.Enrich(ctx, providerLogger, currentActivity, userRec, enricherConfig, doNotRetry)
			}
		} else {
			// Normal mode: call regular Enrich
			res, err = provider.Enrich(ctx, providerLogger, currentActivity, userRec, enricherConfig, doNotRetry)
		}
		duration := time.Since(startTime).Milliseconds()
		pe.DurationMs = duration

		if err != nil {
			// Check for expected control flow errors BEFORE logging at ERROR level
			// to prevent Sentry from capturing them as exceptions.
			if retryErr, ok := err.(*providers.RetryableError); ok {
				logger.Info(fmt.Sprintf("Provider requires retry: %v", provider.Name()), "name", provider.Name(), "reason", retryErr.Reason, "retry_after", retryErr.RetryAfter, "duration_ms", duration, "execution_id", execID)
				pe.Status = "RETRY"
				pe.Error = retryErr.Reason
				pe.Metadata = map[string]string{
					"retry_after":  retryErr.RetryAfter.String(),
					"retry_reason": retryErr.Reason,
				}
				providerExecutions = append(providerExecutions, pe)
				// Keep RUNNING status - retry is in progress, will be retried automatically
				o.updatePipelineRunStatus(ctx, logger, payload.UserId, pipelineExecutionID,
					pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING,
					fmt.Sprintf("Retry scheduled: %s", retryErr.Reason),
					providerExecutions)
				return &ProcessResult{
					Events:             []*pb.EnrichedActivityEvent{},
					ProviderExecutions: providerExecutions,
					Status:             pb.ExecutionStatus_STATUS_LAGGED_RETRY,
				}, retryErr
			}
			if waitErr, ok := err.(*user_input.WaitForInputError); ok {
				logger.Info(fmt.Sprintf("Provider waiting for user input: %v", provider.Name()), "name", provider.Name(), "activity_id", waitErr.ActivityID, "required_fields", waitErr.RequiredFields, "duration_ms", duration, "execution_id", execID)
				pe.Status = "WAITING"
				pe.Metadata = map[string]string{
					"activity_id":     waitErr.ActivityID,
					"required_fields": strings.Join(waitErr.RequiredFields, ","),
				}
				providerExecutions = append(providerExecutions, pe)
				// Update pipeline run to PENDING status - waiting for user input
				o.updatePipelineRunStatus(ctx, logger, payload.UserId, pipelineExecutionID,
					pb.PipelineRunStatus_PIPELINE_RUN_STATUS_PENDING,
					buildPendingInputStatusMessage(waitErr),
					providerExecutions)
				return o.handleWaitError(ctx, logger, payload, providerExecutions, waitErr, activityId)
			}

			// This is a genuine error - log at ERROR level for Sentry capture
			logger.Error(fmt.Sprintf("Provider failed: %v", provider.Name()), "name", provider.Name(), "error", err, "duration_ms", duration, "execution_id", execID)
			pe.Status = "FAILED"
			pe.Error = err.Error()
			providerExecutions = append(providerExecutions, pe)

			// Update pipeline run to FAILED status
			o.updatePipelineRunStatus(ctx, logger, payload.UserId, pipelineExecutionID,
				pb.PipelineRunStatus_PIPELINE_RUN_STATUS_FAILED,
				fmt.Sprintf("Enricher failed: %s - %v", provider.Name(), err),
				providerExecutions)

			// Send push notification on pipeline failure
			if o.notifications != nil {
				user, fetchErr := o.database.GetUser(ctx, payload.UserId)
				if fetchErr == nil && user != nil && len(user.FcmTokens) > 0 {
					// Check notification preferences (default to true if not set)
					prefs := user.NotificationPreferences
					shouldNotify := prefs == nil || prefs.NotifyPipelineFailure
					if shouldNotify {
						title := fmt.Sprintf("Activity Failed: %s", currentActivity.Name)
						body := fmt.Sprintf("Enricher '%s' encountered an error", provider.Name())
						data := map[string]string{
							"type":        "PIPELINE_FAILED",
							"activity_id": activityId,
							"user_id":     payload.UserId,
						}
						if notifyErr := o.notifications.SendPushNotification(ctx, payload.UserId, title, body, user.FcmTokens, data); notifyErr != nil {
							logger.Warn("Failed to send failure notification", "error", notifyErr, "user_id", payload.UserId)
						}
					}
				}
			}

			// Fail pipeline
			return &ProcessResult{
				Events:             []*pb.EnrichedActivityEvent{},
				ProviderExecutions: providerExecutions,
			}, fmt.Errorf("enricher failed: %s: %v", provider.Name(), err)
		}

		if res == nil {
			logger.Warn(fmt.Sprintf("Provider returned nil result: %v", provider.Name()), "name", provider.Name())
			pe.Status = "SKIPPED"
			pe.Error = "nil result"
			providerExecutions = append(providerExecutions, pe)
			continue
		}

		// Check if provider wants to halt the pipeline
		if res.HaltPipeline {
			logger.Info(fmt.Sprintf("Provider halted pipeline: %v", provider.Name()), "name", provider.Name(), "reason", res.HaltReason)
			pe.Status = "SKIPPED"
			pe.Metadata = res.Metadata
			if res.HaltReason != "" {
				pe.Metadata["halt_reason"] = res.HaltReason
			}
			providerExecutions = append(providerExecutions, pe)

			// Update pipeline run to SKIPPED status
			statusMsg := fmt.Sprintf("Pipeline halted by %s", provider.Name())
			if res.HaltReason != "" {
				statusMsg = fmt.Sprintf("Pipeline halted by %s: %s", provider.Name(), res.HaltReason)
			}
			o.updatePipelineRunStatus(ctx, logger, payload.UserId, pipelineExecutionID,
				pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SKIPPED,
				statusMsg,
				providerExecutions)

			// Skip remaining enrichers and don't publish events for this pipeline
			return &ProcessResult{
				Events:             []*pb.EnrichedActivityEvent{},
				ProviderExecutions: providerExecutions,
				Status:             pb.ExecutionStatus_STATUS_SKIPPED,
			}, nil
		}

		pe.Status = "SUCCESS"
		pe.Metadata = res.Metadata
		results[i] = res
		providerExecutions = append(providerExecutions, pe)

		logger.Info(fmt.Sprintf("Provider completed: %v", provider.Name()), "name", provider.Name(), "duration_ms", duration, "execution_id", execID)

		// Apply changes to currentActivity immediately so next provider sees them
		if res.Name != "" {
			currentActivity.Name = res.Name
		}
		if res.NameSuffix != "" {
			currentActivity.Name += res.NameSuffix
		}
		if res.ActivityType != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
			currentActivity.Type = res.ActivityType
		}
		if len(res.Tags) > 0 {
			currentActivity.Tags = append(currentActivity.Tags, res.Tags...)
		}

		// Apply description immediately (append with separator if not empty)
		logger.Debug(fmt.Sprintf("Applying description from provider: %v, length: %v", provider.Name(), len(res.Description)), "name", provider.Name(), "description", res.Description)
		if res.Description != "" {
			trimmed := strings.TrimSpace(res.Description)
			if trimmed != "" {
				if descriptionBuilder.Len() > 0 {
					descriptionBuilder.WriteString("\n\n")
				}
				descriptionBuilder.WriteString(trimmed)
			}
		}

		// Apply stream data immediately to currentActivity so downstream enrichers can see it
		// Ensure Laps/Records exist
		enricherSession := currentActivity.Sessions[0]
		if len(enricherSession.Laps) == 0 {
			enricherSession.Laps = append(enricherSession.Laps, &pb.Lap{
				StartTime:        enricherSession.StartTime,
				TotalElapsedTime: enricherSession.TotalElapsedTime,
				Records:          []*pb.Record{},
			})
		}

		// Check if enricher provides any stream data that needs to be applied
		hasStreamData := len(res.HeartRateStream) > 0 || len(res.PowerStream) > 0 ||
			len(res.PositionLatStream) > 0 || len(res.PositionLongStream) > 0

		// Count total existing records across ALL laps to detect multi-lap activities
		// (e.g., from FIT file uploads where records are properly distributed)
		totalExistingRecords := 0
		for _, lap := range enricherSession.Laps {
			totalExistingRecords += len(lap.Records)
		}

		// Only expand Laps[0] with placeholder records if:
		// 1. An enricher provides stream data that needs to be applied, AND
		// 2. The activity doesn't already have substantial records (less than 25% coverage)
		//
		// This protects multi-lap FIT file uploads from having their rich record data
		// destroyed by placeholder expansion, while still supporting API-sourced activities
		// (e.g., Strava) where HR/power streams need to be applied to sparse records.
		enricherDuration := int(enricherSession.TotalElapsedTime)
		// Use max(duration/4, 1) to handle short durations properly
		threshold := enricherDuration / 4
		if threshold < 1 {
			threshold = 1
		}
		needsRecordExpansion := hasStreamData && totalExistingRecords < threshold

		if needsRecordExpansion {
			enricherLap := enricherSession.Laps[0]
			enricherCurrentLen := len(enricherLap.Records)
			if enricherCurrentLen < enricherDuration {
				enricherStartTime := enricherSession.StartTime.AsTime()
				for k := enricherCurrentLen; k < enricherDuration; k++ {
					ts := timestamppb.New(enricherStartTime.Add(time.Duration(k) * time.Second))
					enricherLap.Records = append(enricherLap.Records, &pb.Record{Timestamp: ts})
				}
			}
		}

		// ALWAYS apply stream data when available - regardless of record expansion
		// For activities with existing records (like FIT files), apply to those records
		// For newly expanded activities, apply to the expanded placeholder records
		if hasStreamData {
			// Apply stream data to ALL laps' records using timestamp-based matching
			// This handles both single-lap expanded activities and multi-lap FIT activities
			activityStart := enricherSession.StartTime.AsTime()

			for _, lap := range enricherSession.Laps {
				for _, record := range lap.Records {
					if record.Timestamp == nil {
						continue
					}
					// Calculate the second offset from activity start
					offsetSec := int(record.Timestamp.AsTime().Sub(activityStart).Seconds())
					if offsetSec < 0 {
						continue
					}

					// Apply HR stream value at this offset
					if len(res.HeartRateStream) > 0 && offsetSec < len(res.HeartRateStream) {
						val := res.HeartRateStream[offsetSec]
						if val > 0 {
							record.HeartRate = int32(val)
						}
					}

					// Apply Power stream value at this offset
					if len(res.PowerStream) > 0 && offsetSec < len(res.PowerStream) {
						val := res.PowerStream[offsetSec]
						if val > 0 {
							record.Power = int32(val)
						}
					}

					// Apply GPS position streams at this offset
					if len(res.PositionLatStream) > 0 && offsetSec < len(res.PositionLatStream) {
						record.PositionLat = res.PositionLatStream[offsetSec]
					}
					if len(res.PositionLongStream) > 0 && offsetSec < len(res.PositionLongStream) {
						record.PositionLong = res.PositionLongStream[offsetSec]
					}
				}
			}
		}
	}

	// Post-enrichment: Reconcile TimeMarker labels with StrengthSet exercise names.
	// After all enrichers have run, the StrengthSets may have better names than
	// the generic FIT category-based labels on the TimeMarkers (e.g., from Hevy data).
	reconcileTimeMarkerLabels(currentActivity)

	brandingApplied := false
	// Run branding provider last (for non-paying users only)
	if brandingProvider, ok := o.providersByName["branding"]; ok && tier.ShouldShowBranding(userRec) {
		brandingLogger := slog.Default().With("provider", "branding")
		brandingRes, err := brandingProvider.Enrich(ctx, brandingLogger, currentActivity, userRec, map[string]string{}, doNotRetry)
		if err != nil {
			logger.Warn("Branding provider failed", "error", err)
		} else if brandingRes != nil && brandingRes.Description != "" {
			logger.Debug(fmt.Sprintf("Applying description from provider: %v, length: %v", brandingProvider.Name(), len(brandingRes.Description)), "name", brandingProvider.Name(), "description", brandingRes.Description)
			trimmed := strings.TrimSpace(brandingRes.Description)
			if trimmed != "" {
				if descriptionBuilder.Len() > 0 {
					descriptionBuilder.WriteString("\n\n")
				}
				descriptionBuilder.WriteString(trimmed)
				// Track that branding was applied
				brandingApplied = true
			}
		}
	}
	currentActivity.Description = descriptionBuilder.String()

	// Build final event structure (no Fan-In needed - currentActivity is already fully enriched)
	finalEvent := &pb.EnrichedActivityEvent{
		UserId:              payload.UserId,
		Source:              payload.Source,
		ActivityId:          activityId,      // Use pre-generated ID (or preserved resume ID)
		ActivityData:        currentActivity, // Already fully enriched
		ActivityType:        currentActivity.Type,
		Name:                currentActivity.Name,
		Description:         descriptionBuilder.String(),
		AppliedEnrichments:  []string{},
		EnrichmentMetadata:  make(map[string]string),
		Destinations:        pipeline.Destinations,
		PipelineId:          pipeline.ID,
		PipelineExecutionId: &pipelineExecutionID,
		StartTime:           currentActivity.Sessions[0].StartTime,
	}

	// Resume Mode: Add update metadata
	if isResumeMode {
		if useUpdateMethod {
			finalEvent.EnrichmentMetadata["use_update_method"] = "true"
		}
	}

	// Same-Source Detection: When source matches a destination, signal uploaders
	// to do a straight overwrite of title/description instead of section-based merge.
	// The activity already exists on the platform, so we just need to update metadata.
	sourceDestName := strings.ToLower(strings.TrimPrefix(pipeline.Source, "SOURCE_"))
	for _, dest := range pipeline.Destinations {
		destName := strings.ToLower(strings.TrimPrefix(dest.String(), "DESTINATION_"))
		if sourceDestName == destName {
			finalEvent.EnrichmentMetadata["same_source_destination_"+destName] = "true"
		}
	}

	// Build AppliedEnrichments list and merge metadata from results
	for i, res := range results {
		if res == nil {
			continue
		}

		cfgName := configs[i].ProviderType.String()
		finalEvent.AppliedEnrichments = append(finalEvent.AppliedEnrichments, cfgName)

		// Merge metadata
		for k, v := range res.Metadata {
			finalEvent.EnrichmentMetadata[k] = v
		}

		// Propagate section header for replaceable description sections
		if res.SectionHeader != "" {
			finalEvent.EnrichmentMetadata["section_header_"+cfgName] = res.SectionHeader
		}
	}
	// Add branding if it was applied
	if brandingApplied {
		finalEvent.AppliedEnrichments = append(finalEvent.AppliedEnrichments, "branding")
	}

	// Inject source config into metadata (with user default fallback)
	sourceConfig := pipeline.SourceConfig
	if len(sourceConfig) == 0 {
		// Fall back to user plugin default for this source
		sourcePluginId := strings.ToLower(strings.TrimPrefix(pipeline.Source, "SOURCE_"))
		if def, err := o.database.GetPluginDefault(ctx, payload.UserId, sourcePluginId); err == nil && def != nil {
			sourceConfig = def.Config
			logger.Info("Using user default for source config", "plugin", sourcePluginId)
		}
	}
	for k, v := range sourceConfig {
		finalEvent.EnrichmentMetadata[k] = v
	}

	// Inject destination configs into metadata (prefixed with destination ID)
	// For each destination, merge pipeline config with user default (pipeline wins)
	// Track which destinations have been processed via DestinationConfigs
	processedDests := make(map[string]bool)
	for destId, destCfg := range pipeline.DestinationConfigs {
		processedDests[destId] = true
		if destCfg != nil && len(destCfg.Config) > 0 {
			for k, v := range destCfg.Config {
				finalEvent.EnrichmentMetadata[destId+"_"+k] = v
			}
		} else {
			// Fall back to user plugin default for this destination
			if def, err := o.database.GetPluginDefault(ctx, payload.UserId, destId); err == nil && def != nil {
				for k, v := range def.Config {
					finalEvent.EnrichmentMetadata[destId+"_"+k] = v
				}
				logger.Info("Using user default for destination config", "destination", destId)
			}
		}
	}

	// Also check pipeline.Destinations for any destinations not in DestinationConfigs
	// These destinations have no per-pipeline config, so fall back to plugin_defaults
	for _, dest := range pipeline.Destinations {
		destId := strings.ToLower(strings.TrimPrefix(dest.String(), "DESTINATION_"))
		if processedDests[destId] {
			continue // Already handled above
		}
		// Fall back to user plugin default
		if def, err := o.database.GetPluginDefault(ctx, payload.UserId, destId); err == nil && def != nil {
			for k, v := range def.Config {
				finalEvent.EnrichmentMetadata[destId+"_"+k] = v
			}
			logger.Info("Using user default for destination config (from Destinations list)", "destination", destId)
		}
	}

	// Generate FIT file artifact
	fitBytes, err := fit.GenerateFitFile(currentActivity)
	if err != nil {
		logger.Error("Failed to generate FIT file", "error", err) // Don't fail the whole event, just log
	} else if len(fitBytes) > 0 {
		objName := fmt.Sprintf("activities/%s/%s.fit", payload.UserId, finalEvent.ActivityId)
		if err := o.storage.Write(ctx, o.bucketName, objName, fitBytes); err != nil {
			logger.Error("Failed to write FIT file artifact", "error", err)
		} else {
			finalEvent.FitFileUri = fmt.Sprintf("gs://%s/%s", o.bucketName, objName)
		}
	}

	// Finalize PipelineRun with enriched data (initial run was created at start)
	o.finalizePipelineRun(ctx, logger, payload.UserId, finalEvent, providerExecutions, originalPayloadUri)

	// Note: Success/partial notifications are now sent by destination.UpdateStatus
	// when all destinations have reported their final status (SYNCED or PARTIAL).

	return &ProcessResult{
		Events:             []*pb.EnrichedActivityEvent{finalEvent},
		ProviderExecutions: providerExecutions,
		Status:             pb.ExecutionStatus_STATUS_SUCCESS,
	}, nil
}

type configuredPipeline struct {
	ID                 string
	Source             string
	Enrichers          []configuredEnricher
	Destinations       []pb.Destination
	SourceConfig       map[string]string
	DestinationConfigs map[string]*pb.DestinationConfig
}

type configuredEnricher struct {
	ProviderType pb.EnricherProviderType
	TypedConfig  map[string]string
}

// resolvePipeline looks up a single pipeline by ID from the user's pipelines collection.
// Returns nil if the pipeline is not found or is disabled.
func (o *Orchestrator) resolvePipeline(ctx context.Context, pipelineID string, userID string, logger *slog.Logger) (*configuredPipeline, error) {
	userPipelines, err := o.database.GetUserPipelines(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user pipelines: %w", err)
	}

	for _, p := range userPipelines {
		if p.Id == pipelineID {
			if p.Disabled {
				logger.Info("Targeted pipeline is disabled", "pipeline_id", p.Id, "name", p.Name)
				return nil, nil
			}

			var enrichers []configuredEnricher
			for _, e := range p.Enrichers {
				enrichers = append(enrichers, configuredEnricher{
					ProviderType: e.ProviderType,
					TypedConfig:  e.TypedConfig,
				})
			}
			return &configuredPipeline{
				ID:                 p.Id,
				Source:             p.Source,
				Enrichers:          enrichers,
				Destinations:       p.Destinations,
				SourceConfig:       p.SourceConfig,
				DestinationConfigs: p.DestinationConfigs,
			}, nil
		}
	}

	return nil, nil // Pipeline not found
}

func (o *Orchestrator) handleWaitError(ctx context.Context, logger *slog.Logger, payload *pb.ActivityPayload, allExecs []ProviderExecution, waitErr *user_input.WaitForInputError, linkedActivityId string) (*ProcessResult, error) {
	logger.Warn("Provider requested user input", "activity_id", waitErr.ActivityID, "linked_activity_id", linkedActivityId)

	// SAFETY CHECK: Verify that we're not overwriting a completed pending input
	// This can happen when resume mode falls back to regular Enrich due to status mismatch
	existingInput, fetchErr := o.database.GetPendingInput(ctx, payload.UserId, waitErr.ActivityID)
	if fetchErr == nil && existingInput != nil && existingInput.Status == pb.PendingInput_STATUS_COMPLETED {
		logger.Warn("Pending input already exists and is completed - skipping creation to prevent overwrite",
			"activity_id", waitErr.ActivityID,
			"existing_status", existingInput.Status.String())
		// Return WAITING status to halt pipeline, but don't overwrite the completed input
		// This indicates a logic issue upstream that should be investigated
		return &ProcessResult{
			Events:             []*pb.EnrichedActivityEvent{},
			ProviderExecutions: allExecs,
			Status:             pb.ExecutionStatus_STATUS_WAITING,
		}, nil
	}

	// Upload original payload to GCS for later retrieval
	payloadUri := ""
	if o.storage != nil && o.bucketName != "" {
		payloadPath := fmt.Sprintf("payloads/%s/%s.json", payload.UserId, waitErr.ActivityID)
		payloadBytes, err := protojson.Marshal(payload)
		if err != nil {
			logger.Warn("Failed to marshal payload for GCS", "error", err)
		} else if err := o.storage.Write(ctx, o.bucketName, payloadPath, payloadBytes); err != nil {
			logger.Warn("Failed to upload payload to GCS", "error", err)
		} else {
			payloadUri = fmt.Sprintf("gs://%s/%s", o.bucketName, payloadPath)
			logger.Debug("Uploaded payload to GCS", "uri", payloadUri)
		}
	}

	// Create Pending Input in DB
	pi := &pb.PendingInput{
		ActivityId:         waitErr.ActivityID,
		UserId:             payload.UserId,
		Status:             pb.PendingInput_STATUS_WAITING,
		RequiredFields:     waitErr.RequiredFields,
		OriginalPayloadUri: payloadUri, // GCS URI for payload retrieval
		EnricherProviderId: waitErr.EnricherProviderID,
		CreatedAt:          timestamppb.Now(),
		UpdatedAt:          timestamppb.Now(),
		ProviderMetadata:   waitErr.Metadata,    // Pass provider context to UI
		LinkedActivityId:   linkedActivityId,    // Activity ID for resume mode
		PipelineId:         *payload.PipelineId, // Pipeline that created this pending input
	}
	if err := o.database.CreatePendingInput(ctx, payload.UserId, pi); err != nil {
		logger.Warn("Failed to create pending input (might already exist)", "error", err)
	}

	// Trigger Push Notification
	if o.notifications != nil {
		user, err := o.database.GetUser(ctx, payload.UserId)
		if err == nil && user != nil && len(user.FcmTokens) > 0 {
			// Check notification preferences (default to true if not set)
			prefs := user.NotificationPreferences
			shouldNotify := prefs == nil || prefs.NotifyPendingInput
			if shouldNotify {
				title := "Action Required: FitGlue"
				body := "An activity needs more information to be processed."
				data := map[string]string{
					"activity_id": waitErr.ActivityID,
					"user_id":     payload.UserId,
					"type":        "PENDING_INPUT",
				}
				if err := o.notifications.SendPushNotification(ctx, payload.UserId, title, body, user.FcmTokens, data); err != nil {
					logger.Error("Failed to send push notification", "error", err, "user_id", payload.UserId)
				}
			}
		}
	}

	return &ProcessResult{
		Events:             []*pb.EnrichedActivityEvent{},
		ProviderExecutions: allExecs,
		Status:             pb.ExecutionStatus_STATUS_WAITING,
	}, nil
}

// createInitialPipelineRun creates a minimal PipelineRun document with RUNNING status
// Called early in the pipeline execution to ensure lifecycle tracking even if pipeline fails
func (o *Orchestrator) createInitialPipelineRun(ctx context.Context, logger *slog.Logger, userId string, pipelineExecutionID string, pipelineID string, activityId string, payload *pb.ActivityPayload, destinations []pb.Destination) {
	activity := payload.GetStandardizedActivity()

	// Build destination outcomes (all pending at this point)
	destOutcomes := make([]*pb.DestinationOutcome, 0, len(destinations))
	for _, dest := range destinations {
		destOutcomes = append(destOutcomes, &pb.DestinationOutcome{
			Destination: dest,
			Status:      pb.DestinationStatus_DESTINATION_STATUS_PENDING,
		})
	}

	pipelineRun := &pb.PipelineRun{
		Id:               pipelineExecutionID,
		PipelineId:       pipelineID,
		ActivityId:       activityId,
		Source:           payload.Source.String(),
		SourceActivityId: activity.GetExternalId(),
		Title:            activity.GetName(),
		Description:      activity.GetDescription(),
		Type:             activity.GetType(),
		StartTime:        activity.GetSessions()[0].GetStartTime(),
		Status:           pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING,
		CreatedAt:        timestamppb.Now(),
		UpdatedAt:        timestamppb.Now(),
		Destinations:     destOutcomes,
	}

	if err := o.database.CreatePipelineRun(ctx, userId, pipelineRun); err != nil {
		logger.Error("Failed to create initial pipeline run", "error", err, "pipeline_run_id", pipelineRun.Id)
	} else {
		logger.Debug("Created initial pipeline run", "pipeline_run_id", pipelineRun.Id, "activity_id", activityId)

		// Also write each destination outcome to the subcollection
		// This is required for the race-condition-free UpdateStatus pattern
		for _, outcome := range destOutcomes {
			if err := o.database.SetDestinationOutcome(ctx, userId, pipelineExecutionID, outcome); err != nil {
				logger.Error("Failed to create initial destination outcome", "error", err, "destination", outcome.Destination.String())
			}
		}
	}
}

// updatePipelineRunStatus updates the pipeline run with a new status and optional message
func (o *Orchestrator) updatePipelineRunStatus(ctx context.Context, logger *slog.Logger, userId string, pipelineRunId string, status pb.PipelineRunStatus, statusMessage string, providerExecs []ProviderExecution) {
	// Convert ProviderExecutions to snake_case maps for Firestore
	boosters := boostersToFirestoreMaps(providerExecs)

	updateData := map[string]interface{}{
		"status":     int32(status),
		"updated_at": time.Now(),
		"boosters":   boosters,
	}
	if statusMessage != "" {
		updateData["status_message"] = statusMessage
	}

	if err := o.database.UpdatePipelineRun(ctx, userId, pipelineRunId, updateData); err != nil {
		logger.Error("Failed to update pipeline run status", "error", err, "pipeline_run_id", pipelineRunId, "status", status)
	} else {
		logger.Debug("Updated pipeline run status", "pipeline_run_id", pipelineRunId, "status", status, "message", statusMessage)
	}
}

// finalizePipelineRun updates the pipeline run with final enriched data on success
func (o *Orchestrator) finalizePipelineRun(ctx context.Context, logger *slog.Logger, userId string, event *pb.EnrichedActivityEvent, providerExecs []ProviderExecution, originalPayloadUri string) {
	// Convert ProviderExecutions to snake_case maps for Firestore
	boosters := boostersToFirestoreMaps(providerExecs)

	// Note: destinations are now managed via subcollection (destination_outcomes)
	// and updated atomically by each uploader via SetDestinationOutcome.
	// We no longer write the destinations array on the parent document.

	// Update pipeline run with final enriched data
	// Note: status changes from PENDING -> RUNNING, and we clear any status_message
	// (e.g., "Waiting for user input: ...") since the input has been resolved.
	// The status will transition to SYNCED/PARTIAL/FAILED once destinations are processed.
	updateData := map[string]interface{}{
		"title":                event.Name,
		"description":          event.Description,
		"type":                 int32(event.ActivityType),
		"start_time":           event.StartTime.AsTime(),
		"updated_at":           time.Now(),
		"status":               int32(pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING),
		"status_message":       nil, // Clear pending input message on successful resume
		"boosters":             boosters,
		"original_payload_uri": originalPayloadUri,
	}

	if err := o.database.UpdatePipelineRun(ctx, userId, *event.PipelineExecutionId, updateData); err != nil {
		logger.Error("Failed to finalize pipeline run", "error", err, "pipeline_run_id", *event.PipelineExecutionId)
	} else {
		logger.Debug("Finalized pipeline run", "pipeline_run_id", *event.PipelineExecutionId, "activity_id", event.ActivityId)
	}
}

// boostersToFirestoreMaps converts ProviderExecutions to snake_case maps for Firestore storage
// This ensures field names match what the web UI expects (provider_name, duration_ms, etc.)
func boostersToFirestoreMaps(providerExecs []ProviderExecution) []map[string]interface{} {
	boosters := make([]map[string]interface{}, 0, len(providerExecs))
	for _, pe := range providerExecs {
		booster := map[string]interface{}{
			"provider_name": pe.ProviderName,
			"status":        pe.Status,
			"duration_ms":   pe.DurationMs,
			"metadata":      pe.Metadata,
		}
		if pe.Error != "" {
			booster["error"] = pe.Error
		}
		boosters = append(boosters, booster)
	}
	return boosters
}

// buildPendingInputStatusMessage creates a user-friendly status message for pending input.
// It uses the display.summary from the provider metadata if available, falling back
// to display.field_labels for humanized field names, and finally to Title-Cased field names.
func buildPendingInputStatusMessage(waitErr *user_input.WaitForInputError) string {
	// Priority 1: Use display.summary if the provider set it
	if summary, ok := waitErr.Metadata["display.summary"]; ok && summary != "" {
		return fmt.Sprintf("Waiting for user input: %s", summary)
	}

	// Priority 2: Use display.field_labels for humanized names
	if labelsJSON, ok := waitErr.Metadata["display.field_labels"]; ok && labelsJSON != "" {
		var labels map[string]string
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err == nil && len(labels) > 0 {
			var friendly []string
			for _, field := range waitErr.RequiredFields {
				if label, exists := labels[field]; exists {
					friendly = append(friendly, label)
				} else {
					friendly = append(friendly, humanizeFieldName(field))
				}
			}
			return fmt.Sprintf("Waiting for user input: %s", strings.Join(friendly, ", "))
		}
	}

	// Priority 3: Humanize raw field names (e.g. fit_file_base64 -> Fit File Base64)
	var humanized []string
	for _, field := range waitErr.RequiredFields {
		humanized = append(humanized, humanizeFieldName(field))
	}
	return fmt.Sprintf("Waiting for user input: %s", strings.Join(humanized, ", "))
}

// humanizeFieldName converts snake_case to Title Case (e.g. "fit_file_base64" -> "Fit File Base64")
func humanizeFieldName(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
