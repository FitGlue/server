package enricher

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	shared "github.com/fitglue/server/src/go/pkg"
	fit "github.com/fitglue/server/src/go/pkg/domain/file_generators"
	"github.com/fitglue/server/src/go/pkg/domain/tier"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/functions/enricher/providers/user_input"
	"github.com/google/uuid"
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
		return &ProcessResult{
			Events:             []*pb.EnrichedActivityEvent{},
			ProviderExecutions: []ProviderExecution{},
			Status:             pb.ExecutionStatus_STATUS_SKIPPED,
		}, fmt.Errorf("tier limit: %s", reason)
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
	pipelineExecutionID := fmt.Sprintf("%s-%s", basePipelineExecutionID, pipeline.ID)
	logger.Info("Executing pipeline", "id", pipeline.ID, "pipelineExecutionId", pipelineExecutionID)

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

		// Merge pipelineExecutionID and pipelineID into config for asset-generating providers
		enricherConfig := make(map[string]string)
		for k, v := range cfg.TypedConfig {
			enricherConfig[k] = v
		}
		enricherConfig["pipeline_execution_id"] = pipelineExecutionID
		enricherConfig["pipeline_id"] = pipeline.ID

		// Execute
		// TODO: Get logger from FrameworkContext when orchestrator is refactored
		providerLogger := slog.Default().With("provider", provider.Name())
		res, err := provider.Enrich(ctx, providerLogger, currentActivity, userRec, enricherConfig, doNotRetry)
		duration := time.Since(startTime).Milliseconds()
		pe.DurationMs = duration

		if err != nil {
			logger.Error(fmt.Sprintf("Provider failed: %v", provider.Name()), "name", provider.Name(), "error", err, "duration_ms", duration, "execution_id", execID)
			// Check for retryable/wait errors
			if retryErr, ok := err.(*providers.RetryableError); ok {
				pe.Status = "RETRY"
				pe.Error = retryErr.Reason
				pe.Metadata = map[string]string{
					"retry_after":  retryErr.RetryAfter.String(),
					"retry_reason": retryErr.Reason,
				}
				providerExecutions = append(providerExecutions, pe)
				return &ProcessResult{
					Events:             []*pb.EnrichedActivityEvent{},
					ProviderExecutions: providerExecutions,
					Status:             pb.ExecutionStatus_STATUS_LAGGED_RETRY,
				}, retryErr
			}
			if waitErr, ok := err.(*user_input.WaitForInputError); ok {
				pe.Status = "WAITING"
				pe.Metadata = map[string]string{
					"activity_id":     waitErr.ActivityID,
					"required_fields": strings.Join(waitErr.RequiredFields, ","),
				}
				providerExecutions = append(providerExecutions, pe)
				return o.handleWaitError(ctx, logger, payload, providerExecutions, waitErr)
			}

			pe.Status = "FAILED"
			pe.Error = err.Error()
			providerExecutions = append(providerExecutions, pe)

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
		enricherLap := enricherSession.Laps[0]

		// Ensure records are large enough for stream data
		enricherDuration := int(enricherSession.TotalElapsedTime)
		enricherCurrentLen := len(enricherLap.Records)
		if enricherCurrentLen < enricherDuration {
			enricherStartTime := enricherSession.StartTime.AsTime()
			for k := enricherCurrentLen; k < enricherDuration; k++ {
				ts := timestamppb.New(enricherStartTime.Add(time.Duration(k) * time.Second))
				enricherLap.Records = append(enricherLap.Records, &pb.Record{Timestamp: ts})
			}
		}

		// Apply HR stream
		if len(res.HeartRateStream) > 0 {
			for idx, val := range res.HeartRateStream {
				if idx < len(enricherLap.Records) && val > 0 {
					enricherLap.Records[idx].HeartRate = int32(val)
				}
			}
		}

		// Apply Power stream
		if len(res.PowerStream) > 0 {
			for idx, val := range res.PowerStream {
				if idx < len(enricherLap.Records) && val > 0 {
					enricherLap.Records[idx].Power = int32(val)
				}
			}
		}

		// Apply GPS position streams
		if len(res.PositionLatStream) > 0 {
			for idx, val := range res.PositionLatStream {
				if idx < len(enricherLap.Records) {
					enricherLap.Records[idx].PositionLat = val
				}
			}
		}
		if len(res.PositionLongStream) > 0 {
			for idx, val := range res.PositionLongStream {
				if idx < len(enricherLap.Records) {
					enricherLap.Records[idx].PositionLong = val
				}
			}
		}
	}

	brandingApplied := false
	// Run branding provider last (only for Hobbyist tier)
	if brandingProvider, ok := o.providersByName["branding"]; ok && tier.GetEffectiveTier(userRec) == tier.TierHobbyist {
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
		ActivityId:          uuid.NewString(),
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

	// Resume Mode: Add update metadata and use original activity ID
	if isResumeMode {
		if useUpdateMethod {
			finalEvent.EnrichmentMetadata["use_update_method"] = "true"
		}
		if payload.ActivityId != nil && *payload.ActivityId != "" {
			finalEvent.ActivityId = *payload.ActivityId
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

	return &ProcessResult{
		Events:             []*pb.EnrichedActivityEvent{finalEvent},
		ProviderExecutions: providerExecutions,
		Status:             pb.ExecutionStatus_STATUS_SUCCESS,
	}, nil
}

type configuredPipeline struct {
	ID           string
	Enrichers    []configuredEnricher
	Destinations []pb.Destination
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
				ID:           p.Id,
				Enrichers:    enrichers,
				Destinations: p.Destinations,
			}, nil
		}
	}

	return nil, nil // Pipeline not found
}

func (o *Orchestrator) handleWaitError(ctx context.Context, logger *slog.Logger, payload *pb.ActivityPayload, allExecs []ProviderExecution, waitErr *user_input.WaitForInputError) (*ProcessResult, error) {
	logger.Warn("Provider requested user input", "activity_id", waitErr.ActivityID)
	// Create Pending Input in DB
	pi := &pb.PendingInput{
		ActivityId:      waitErr.ActivityID,
		UserId:          payload.UserId,
		Status:          pb.PendingInput_STATUS_WAITING,
		RequiredFields:  waitErr.RequiredFields,
		OriginalPayload: payload, // Full payload for re-publish
		CreatedAt:       timestamppb.Now(),
		UpdatedAt:       timestamppb.Now(),
	}
	if err := o.database.CreatePendingInput(ctx, pi); err != nil {
		logger.Warn("Failed to create pending input (might already exist)", "error", err)
	}

	// Trigger Push Notification
	if o.notifications != nil {
		user, err := o.database.GetUser(ctx, payload.UserId)
		if err == nil && user != nil && len(user.FcmTokens) > 0 {
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

	return &ProcessResult{
		Events:             []*pb.EnrichedActivityEvent{},
		ProviderExecutions: allExecs,
		Status:             pb.ExecutionStatus_STATUS_WAITING,
	}, nil
}
