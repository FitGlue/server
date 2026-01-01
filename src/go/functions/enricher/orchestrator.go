package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	fit "github.com/ripixel/fitglue-server/src/go/pkg/domain/file_generators"
	providers "github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/encoding/protojson"
)

type Orchestrator struct {
	database        shared.Database
	storage         shared.BlobStore
	bucketName      string
	providersByName map[string]providers.Provider
	providersByType map[pb.EnricherProviderType]providers.Provider
}

func NewOrchestrator(db shared.Database, storage shared.BlobStore, bucketName string) *Orchestrator {
	return &Orchestrator{
		database:        db,
		storage:         storage,
		bucketName:      bucketName,
		providersByName: make(map[string]providers.Provider),
		providersByType: make(map[pb.EnricherProviderType]providers.Provider),
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
func (o *Orchestrator) Process(ctx context.Context, payload *pb.ActivityPayload, parentExecutionID string, doNotRetry bool) (*ProcessResult, error) {
	// 1. Fetch User Config
	userDoc, err := o.database.GetUser(ctx, payload.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user config: %w", err)
	}
	userRec, err := o.mapUser(payload.UserId, userDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to map user record: %w", err)
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

	if len(pipelines) == 0 {
		return &ProcessResult{Events: []*pb.EnrichedActivityEvent{}, ProviderExecutions: []ProviderExecution{}}, nil
	}

	var allEvents []*pb.EnrichedActivityEvent
	var allProviderExecutions []ProviderExecution

	// 3. Execute Each Pipeline
	for _, pipeline := range pipelines {
		slog.Info("Executing pipeline", "id", pipeline.ID)

		// 3a. Fan-Out Enrichers for this Pipeline
		configs := pipeline.Enrichers
		results := make([]*providers.EnrichmentResult, len(configs))
		providerExecs := make([]ProviderExecution, len(configs))
		var wg sync.WaitGroup
		errs := make([]error, len(configs))

		for i, cfg := range configs {
			var provider providers.Provider
			var ok bool

			// Lookup by Type
			provider, ok = o.providersByType[cfg.ProviderType]
			if !ok {
				// Fallback or skip
				slog.Warn("Provider not found for type", "type", cfg.ProviderType)
				providerExecs[i] = ProviderExecution{
					ProviderName: fmt.Sprintf("TYPE:%s", cfg.ProviderType),
					Status:       "SKIPPED",
					Error:        "provider not registered",
				}
				continue
			}

			wg.Add(1)
			go func(idx int, p providers.Provider, inputs map[string]string) {
				defer wg.Done()

				startTime := time.Now()

				// Generate ExecutionID for tracking
				execID := uuid.NewString()

				// Log child execution start (non-blocking, best effort)
				// We don't fail the enrichment if logging fails
				providerExecs[idx].ProviderName = p.Name()
				providerExecs[idx].ExecutionID = execID
				providerExecs[idx].Status = "STARTED"

				res, err := p.Enrich(ctx, payload.StandardizedActivity, userRec, inputs, doNotRetry)
				duration := time.Since(startTime).Milliseconds()
				providerExecs[idx].DurationMs = duration

				if err != nil {
					slog.Error("Enricher failed", "name", p.Name(), "error", err, "duration_ms", duration, "execution_id", execID)
					errs[idx] = err
					providerExecs[idx].Status = "FAILED"
					providerExecs[idx].Error = err.Error()
					return
				}

				providerExecs[idx].Status = "SUCCESS"
				providerExecs[idx].Metadata = res.Metadata
				results[idx] = res
				slog.Info("Enricher completed", "name", p.Name(), "duration_ms", duration, "execution_id", execID)
			}(i, provider, cfg.Inputs)
		}
		wg.Wait()

		// Collect provider executions
		for _, pe := range providerExecs {
			if pe.ProviderName != "" {
				allProviderExecutions = append(allProviderExecutions, pe)
			}
		}

		// Check if any configured enrichers failed
		var failedEnrichers []string
		for i, cfg := range configs {
			if errs[i] != nil {
				// Check if this is a RetryableError
				if retryErr, ok := errs[i].(*providers.RetryableError); ok {
					slog.Warn("Provider requested retry", "provider", cfg.ProviderType, "reason", retryErr.Reason, "retry_after", retryErr.RetryAfter)
					// Fail the entire pipeline immediately with the retryable error
					// NOTE: Even if 'doNotRetry' was true, if the provider returned RetryableError,
					// it means it chose NOT to return partial data (or couldn't). So we respect the error.
					return &ProcessResult{
						Events:             []*pb.EnrichedActivityEvent{},
						ProviderExecutions: allProviderExecutions,
					}, retryErr
				}

				failedEnrichers = append(failedEnrichers, fmt.Sprintf("%s: %v", cfg.ProviderType, errs[i]))
			}
		}
		if len(failedEnrichers) > 0 {
			return &ProcessResult{
				Events:             []*pb.EnrichedActivityEvent{},
				ProviderExecutions: allProviderExecutions,
			}, fmt.Errorf("enricher(s) failed: %s", strings.Join(failedEnrichers, "; "))
		}

		// 3b. Merge Results (Fan-In)
		finalEvent := &pb.EnrichedActivityEvent{
			UserId:             payload.UserId,
			Source:             payload.Source,
			ActivityId:         uuid.NewString(),
			ActivityData:       payload.StandardizedActivity,
			ActivityType:       "WEIGHT_TRAINING",
			Name:               "Workout",
			AppliedEnrichments: []string{},
			EnrichmentMetadata: make(map[string]string),
			Destinations:       pipeline.Destinations,
			PipelineId:         pipeline.ID,
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
			startTime, _ := time.Parse(time.RFC3339, session.StartTime)
			// Pad with timestamp-only records
			for k := currentLen; k < duration; k++ {
				ts := startTime.Add(time.Duration(k) * time.Second).Format(time.RFC3339)
				lap.Records = append(lap.Records, &pb.Record{Timestamp: ts})
			}
		}

		for i, res := range results {
			if res == nil {
				continue
			}
			cfgName := configs[i].ProviderType.String()
			finalEvent.AppliedEnrichments = append(finalEvent.AppliedEnrichments, cfgName)

			if res.Name != "" {
				finalEvent.Name = res.Name
			}
			if res.Description != "" {
				trimmed := strings.TrimSpace(res.Description)
				if trimmed != "" {
					if finalEvent.Description != "" {
						finalEvent.Description += "\n\n"
					}
					finalEvent.Description += trimmed
				}
			}
			if res.ActivityType != "" {
				finalEvent.ActivityType = res.ActivityType
			}
			if len(res.Tags) > 0 {
				finalEvent.Tags = append(finalEvent.Tags, res.Tags...)
				finalEvent.ActivityData.Tags = append(finalEvent.ActivityData.Tags, res.Tags...)
			}

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

	return &ProcessResult{
		Events:             allEvents,
		ProviderExecutions: allProviderExecutions,
	}, nil
}

type configuredPipeline struct {
	ID           string
	Enrichers    []configuredEnricher
	Destinations []string
}

type configuredEnricher struct {
	ProviderType pb.EnricherProviderType
	Inputs       map[string]string
}

func (o *Orchestrator) resolvePipelines(source pb.ActivitySource, userRec *pb.UserRecord) []configuredPipeline {
	var pipelines []configuredPipeline
	sourceName := source.String()

	for _, p := range userRec.Pipelines {
		// Match Source
		if p.Source == sourceName {
			var enrichers []configuredEnricher
			for _, e := range p.Enrichers {
				enrichers = append(enrichers, configuredEnricher{
					ProviderType: e.ProviderType,
					Inputs:       e.Inputs,
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
		dests := []string{}
		if userRec.Integrations != nil && userRec.Integrations.Strava != nil && userRec.Integrations.Strava.Enabled {
			dests = append(dests, "strava")
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

func (o *Orchestrator) mapUser(userId string, data map[string]interface{}) (*pb.UserRecord, error) {
	// Convert Firestore map to JSON, then unmarshal using protojson
	// This is type-safe and automatically handles all nested structures
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user data: %w", err)
	}

	var rec pb.UserRecord
	unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOpts.Unmarshal(jsonBytes, &rec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user record: %w", err)
	}

	// Set the user ID (not stored in Firestore document, only as document ID)
	rec.UserId = userId

	return &rec, nil
}
