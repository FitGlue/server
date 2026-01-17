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
	providers "github.com/fitglue/server/src/go/pkg/enricher_providers"
	"github.com/fitglue/server/src/go/pkg/enricher_providers/user_input"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
func (o *Orchestrator) Process(ctx context.Context, payload *pb.ActivityPayload, parentExecutionID string, pipelineExecutionID string, doNotRetry bool) (*ProcessResult, error) {
	// 1. Fetch User Config
	userRec, err := o.database.GetUser(ctx, payload.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user config: %w", err)
	}

	// 1.1. Check Tier Limits
	if tier.ShouldResetSyncCount(userRec) {
		// Reset monthly counter
		if err := o.database.ResetSyncCount(ctx, payload.UserId); err != nil {
			slog.Warn("Failed to reset sync count", "error", err, "userId", payload.UserId)
		}
		userRec.SyncCountThisMonth = 0
	}

	allowed, reason := tier.CanSync(userRec)
	if !allowed {
		slog.Info("Sync blocked by tier limit", "userId", payload.UserId, "reason", reason)
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
		slog.Error("Activity does not have exactly one session", "count", len(payload.StandardizedActivity.Sessions))
		return nil, fmt.Errorf("multiple sessions not supported")
	}
	if payload.StandardizedActivity.Sessions[0].TotalElapsedTime == 0 {
		slog.Error("Activity session has 0 elapsed time")
		return nil, fmt.Errorf("session total elapsed time is 0")
	}

	// 2. Resolve Pipelines
	pipelines := o.resolvePipelines(payload.Source, userRec)
	slog.Info("Resolved pipelines", "count", len(pipelines), "source", payload.Source)

	// 2.1 Handle Resume Mode
	// If is_resume=true, we're resuming after a pending input was resolved
	isResumeMode := payload.IsResume
	resumeOnlyEnrichers := payload.ResumeOnlyEnrichers
	useUpdateMethod := payload.UseUpdateMethod

	if isResumeMode {
		slog.Info("Resume mode activated",
			"resume_only_enrichers", resumeOnlyEnrichers,
			"use_update_method", useUpdateMethod,
			"resume_pending_input_id", payload.ResumePendingInputId)

		// If a specific pipeline ID is set, filter to just that pipeline
		if payload.PipelineId != nil && *payload.PipelineId != "" {
			var filteredPipelines []configuredPipeline
			for _, p := range pipelines {
				if p.ID == *payload.PipelineId {
					filteredPipelines = append(filteredPipelines, p)
					break
				}
			}
			if len(filteredPipelines) > 0 {
				pipelines = filteredPipelines
			}
		}
	}

	if len(pipelines) == 0 {

		return &ProcessResult{
			Events:             []*pb.EnrichedActivityEvent{},
			ProviderExecutions: []ProviderExecution{},
			Status:             pb.ExecutionStatus_STATUS_SKIPPED,
		}, nil
	}

	var allEvents []*pb.EnrichedActivityEvent
	var allProviderExecutions []ProviderExecution

	// 3. Execute Each Pipeline
	for _, pipeline := range pipelines {
		slog.Info("Executing pipeline", "id", pipeline.ID)

		// 3a. Execute Enrichers Sequentially
		configs := pipeline.Enrichers
		results := make([]*providers.EnrichmentResult, len(configs))
		providerExecs := []ProviderExecution{}

		// Use the standardized activity as the working state for this pipeline.
		// Note: This mutates the payload's activity in-place, allowing subsequent enrichers to see changes.
		currentActivity := payload.StandardizedActivity

		for i, cfg := range configs {
			var provider providers.Provider
			var ok bool

			// Lookup by Type
			provider, ok = o.providersByType[cfg.ProviderType]
			if !ok {
				slog.Warn("Provider not found for type", "type", cfg.ProviderType)
				providerExecs = append(providerExecs, ProviderExecution{
					ProviderName: fmt.Sprintf("TYPE:%s", cfg.ProviderType),
					Status:       "SKIPPED",
					Error:        "provider not registered",
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
					slog.Debug("Skipping enricher in resume mode", "name", provider.Name())
					providerExecs = append(providerExecs, ProviderExecution{
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

			// Execute
			res, err := provider.Enrich(ctx, currentActivity, userRec, cfg.TypedConfig, doNotRetry)
			duration := time.Since(startTime).Milliseconds()
			pe.DurationMs = duration

			if err != nil {
				slog.Error(fmt.Sprintf("Provider failed: %v", provider.Name()), "name", provider.Name(), "error", err, "duration_ms", duration, "execution_id", execID)
				// Check for retryable/wait errors
				if retryErr, ok := err.(*providers.RetryableError); ok {
					pe.Status = "RETRY"
					pe.Error = retryErr.Reason
					pe.Metadata = map[string]string{
						"retry_after":  retryErr.RetryAfter.String(),
						"retry_reason": retryErr.Reason,
					}
					providerExecs = append(providerExecs, pe)
					return &ProcessResult{
						Events:             []*pb.EnrichedActivityEvent{},
						ProviderExecutions: append(allProviderExecutions, providerExecs...),
						Status:             pb.ExecutionStatus_STATUS_LAGGED_RETRY,
					}, retryErr
				}
				if waitErr, ok := err.(*user_input.WaitForInputError); ok {
					pe.Status = "WAITING"
					pe.Metadata = map[string]string{
						"activity_id":     waitErr.ActivityID,
						"required_fields": strings.Join(waitErr.RequiredFields, ","),
					}
					providerExecs = append(providerExecs, pe)
					return o.handleWaitError(ctx, payload, append(allProviderExecutions, providerExecs...), waitErr)
				}

				pe.Status = "FAILED"
				pe.Error = err.Error()
				providerExecs = append(providerExecs, pe)

				// Fail pipeline? Yes.
				return &ProcessResult{
					Events:             []*pb.EnrichedActivityEvent{},
					ProviderExecutions: append(allProviderExecutions, providerExecs...),
				}, fmt.Errorf("enricher failed: %s: %v", provider.Name(), err)
			}

			if res == nil {
				slog.Warn(fmt.Sprintf("Provider returned nil result: %v", provider.Name()), "name", provider.Name())
				pe.Status = "SKIPPED"
				pe.Error = "nil result"
				providerExecs = append(providerExecs, pe)
				continue
			}

			// Check if provider wants to halt the pipeline
			if res.HaltPipeline {
				slog.Info(fmt.Sprintf("Provider halted pipeline: %v", provider.Name()), "name", provider.Name(), "reason", res.HaltReason)
				pe.Status = "SKIPPED"
				pe.Metadata = res.Metadata
				if res.HaltReason != "" {
					pe.Metadata["halt_reason"] = res.HaltReason
				}
				providerExecs = append(providerExecs, pe)

				// Skip remaining enrichers and don't publish events for this pipeline
				allProviderExecutions = append(allProviderExecutions, providerExecs...)
				return &ProcessResult{
					Events:             []*pb.EnrichedActivityEvent{},
					ProviderExecutions: allProviderExecutions,
					Status:             pb.ExecutionStatus_STATUS_SKIPPED,
				}, nil
			}

			pe.Status = "SUCCESS"
			pe.Metadata = res.Metadata
			results[i] = res
			providerExecs = append(providerExecs, pe)

			slog.Info(fmt.Sprintf("Provider completed: %v", provider.Name()), "name", provider.Name(), "duration_ms", duration, "execution_id", execID)

			// Apply changes to currentActivity immediately so next provider sees them
			if res.Name != "" {
				currentActivity.Name = res.Name
			}
			if res.NameSuffix != "" {
				currentActivity.Name += res.NameSuffix
			}
			// Note: Description append logic usually happens at end, but if a provider filters on description?
			// Let's update Description too.
			if res.Description != "" {
				trimmed := strings.TrimSpace(res.Description)
				if trimmed != "" {
					if currentActivity.Description != "" {
						currentActivity.Description += "\n\n"
					}
					currentActivity.Description += trimmed
				}
			}
			if res.ActivityType != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
				currentActivity.Type = res.ActivityType
			}
			// Apply Tags?
			if len(res.Tags) > 0 {
				currentActivity.Tags = append(currentActivity.Tags, res.Tags...)
			}
			// Note: We currently skip applying complex stream data (HR/Power) to currentActivity here.
			// Downstream providers typically depend only on metadata (Name/Tags) which we updated above.
			// Full stream merging happens in the final Fan-In phase.
		}

		// Append executions from this pipeline
		allProviderExecutions = append(allProviderExecutions, providerExecs...)

		// 3b. Merge Results (Fan-In)
		finalEvent := &pb.EnrichedActivityEvent{
			UserId:              payload.UserId,
			Source:              payload.Source,
			ActivityId:          uuid.NewString(),
			ActivityData:        payload.StandardizedActivity,
			ActivityType:        pb.ActivityType_ACTIVITY_TYPE_WORKOUT,
			Name:                "Workout",
			AppliedEnrichments:  []string{},
			EnrichmentMetadata:  make(map[string]string),
			Destinations:        pipeline.Destinations,
			PipelineId:          pipeline.ID,
			PipelineExecutionId: &pipelineExecutionID,
			StartTime:           payload.StandardizedActivity.Sessions[0].StartTime,
		}

		// 3b.1 Resume Mode: Add update metadata and use original activity ID
		if isResumeMode {
			if useUpdateMethod {
				finalEvent.EnrichmentMetadata["use_update_method"] = "true"
			}
			// Use the original activity ID for updates (from payload)
			if payload.ActivityId != nil && *payload.ActivityId != "" {
				finalEvent.ActivityId = *payload.ActivityId
			}
		}

		if payload.StandardizedActivity != nil {
			finalEvent.Name = payload.StandardizedActivity.Name
			finalEvent.Description = payload.StandardizedActivity.Description
			finalEvent.ActivityType = payload.StandardizedActivity.Type
		}

		// Merge Streams & Metadata
		session := payload.StandardizedActivity.Sessions[0]
		duration := int(session.TotalElapsedTime)

		// Ensure Laps/Records exist
		if len(session.Laps) == 0 {
			// Create a default lap if missing
			session.Laps = append(session.Laps, &pb.Lap{
				StartTime:        session.StartTime,
				TotalElapsedTime: session.TotalElapsedTime,
				Records:          []*pb.Record{},
			})
		}
		lap := session.Laps[0]

		// Ensure records are large enough
		currentLen := len(lap.Records)
		if currentLen < duration {
			startTime := session.StartTime.AsTime()
			// Pad with timestamp-only records
			for k := currentLen; k < duration; k++ {
				ts := timestamppb.New(startTime.Add(time.Duration(k) * time.Second))
				lap.Records = append(lap.Records, &pb.Record{Timestamp: ts})
			}
		}

		for i, res := range results {
			if res == nil {
				continue
			}
			cfgName := configs[i].ProviderType.String()
			finalEvent.AppliedEnrichments = append(finalEvent.AppliedEnrichments, cfgName)

			// Merge Data Streams into Records
			if len(res.HeartRateStream) > 0 {
				for idx, val := range res.HeartRateStream {
					if idx < len(lap.Records) && val > 0 {
						lap.Records[idx].HeartRate = int32(val)
					}
				}
			}
			if len(res.PowerStream) > 0 {
				for idx, val := range res.PowerStream {
					if idx < len(lap.Records) && val > 0 {
						lap.Records[idx].Power = int32(val)
					}
				}
			}
			if len(res.PositionLatStream) > 0 {
				for idx, val := range res.PositionLatStream {
					if idx < len(lap.Records) {
						lap.Records[idx].PositionLat = val
					}
				}
			}
			if len(res.PositionLongStream) > 0 {
				for idx, val := range res.PositionLongStream {
					if idx < len(lap.Records) {
						lap.Records[idx].PositionLong = val
					}
				}
			}

			for k, v := range res.Metadata {
				finalEvent.EnrichmentMetadata[k] = v
			}
		}

		// Always run branding provider last (unconditionally)
		if brandingProvider, ok := o.providersByName["branding"]; ok {
			// Branding provider doesn't care about retries usually, but we match signature
			brandingRes, err := brandingProvider.Enrich(ctx, payload.StandardizedActivity, userRec, map[string]string{}, doNotRetry)
			if err != nil {
				slog.Warn("Branding provider failed", "error", err)
			} else if brandingRes != nil && brandingRes.Description != "" {
				trimmed := strings.TrimSpace(brandingRes.Description)
				if trimmed != "" {
					if finalEvent.Description != "" {
						finalEvent.Description += "\n\n"
					}
					finalEvent.Description += trimmed
				}
				// Add to applied enrichments
				finalEvent.AppliedEnrichments = append(finalEvent.AppliedEnrichments, "branding")
			}
		}

		// 3c. Generate Artifacts (FIT File)
		fitBytes, err := fit.GenerateFitFile(payload.StandardizedActivity)
		if err != nil {
			slog.Error("Failed to generate FIT file", "error", err) // Don't fail the whole event, just log
		} else if len(fitBytes) > 0 {
			objName := fmt.Sprintf("activities/%s/%s.fit", payload.UserId, finalEvent.ActivityId)
			if err := o.storage.Write(ctx, o.bucketName, objName, fitBytes); err != nil {
				slog.Error("Failed to write FIT file artifact", "error", err)
			} else {
				finalEvent.FitFileUri = fmt.Sprintf("gs://%s/%s", o.bucketName, objName)
			}
		}

		allEvents = append(allEvents, finalEvent)
	}

	// Increment sync count on success
	if err := o.database.IncrementSyncCount(ctx, payload.UserId); err != nil {
		slog.Warn("Failed to increment sync count", "error", err, "userId", payload.UserId)
	}

	return &ProcessResult{
		Events:             allEvents,
		ProviderExecutions: allProviderExecutions,
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

func (o *Orchestrator) resolvePipelines(source pb.ActivitySource, userRec *pb.UserRecord) []configuredPipeline {
	var pipelines []configuredPipeline
	sourceName := source.String()

	for _, p := range userRec.Pipelines {
		// Match Source - expects canonical format like "SOURCE_HEVY" (normalized by TypeScript layer)
		if p.Source == sourceName {
			var enrichers []configuredEnricher
			for _, e := range p.Enrichers {
				enrichers = append(enrichers, configuredEnricher{
					ProviderType: e.ProviderType,
					TypedConfig:  e.TypedConfig,
				})
			}
			pipelines = append(pipelines, configuredPipeline{
				ID:           p.Id,
				Enrichers:    enrichers,
				Destinations: p.Destinations,
			})
		}
	}

	// Default/Fallback logic: If no pipelines found, create a default "Pass-through" pipeline
	// This ensures backward compatibility or "at least save it" behavior.
	if len(pipelines) == 0 {
		// Check legacy "Routes" or just default to Router handling it?
		// Router now expects destinations in the event.
		// So we MUST produce at least one event if we want anything to happen.
		// Let's create a default pipeline that has NO enrichers, but checks legacy destinations.

		// Legacy check: Strava
		var dests []pb.Destination
		if userRec.Integrations != nil && userRec.Integrations.Strava != nil && userRec.Integrations.Strava.Enabled {
			dests = append(dests, pb.Destination_DESTINATION_STRAVA)
		}

		if len(dests) > 0 {
			pipelines = append(pipelines, configuredPipeline{
				ID:           "default-legacy",
				Destinations: dests,
				Enrichers:    []configuredEnricher{}, // No enrichers
			})
		}
	}

	return pipelines
}

func (o *Orchestrator) handleWaitError(ctx context.Context, payload *pb.ActivityPayload, allExecs []ProviderExecution, waitErr *user_input.WaitForInputError) (*ProcessResult, error) {
	slog.Warn("Provider requested user input", "activity_id", waitErr.ActivityID)
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
		slog.Warn("Failed to create pending input (might already exist)", "error", err)
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
				slog.Error("Failed to send push notification", "error", err, "user_id", payload.UserId)
			}
		}
	}

	return &ProcessResult{
		Events:             []*pb.EnrichedActivityEvent{},
		ProviderExecutions: allExecs,
		Status:             pb.ExecutionStatus_STATUS_WAITING,
	}, nil
}
