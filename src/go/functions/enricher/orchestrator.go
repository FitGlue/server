package enricher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ripixel/fitglue-server/src/go/functions/enricher/providers"
	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	fit "github.com/ripixel/fitglue-server/src/go/pkg/domain/file_generators"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type Orchestrator struct {
	database   shared.Database
	storage    shared.BlobStore
	bucketName string
	providers  map[string]providers.Provider
}

func NewOrchestrator(db shared.Database, storage shared.BlobStore, bucketName string) *Orchestrator {
	return &Orchestrator{
		database:   db,
		storage:    storage,
		bucketName: bucketName,
		providers:  make(map[string]providers.Provider),
	}
}

func (o *Orchestrator) Register(p providers.Provider) {
	o.providers[p.Name()] = p
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
}

// Process executes the enrichment pipelines for the activity
func (o *Orchestrator) Process(ctx context.Context, payload *pb.ActivityPayload, parentExecutionID string) (*ProcessResult, error) {
	// 1. Fetch User Config
	userDoc, err := o.database.GetUser(ctx, payload.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user config: %w", err)
	}
	userRec := o.mapUser(userDoc)

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
			provider, ok := o.providers[cfg.Name]
			if !ok {
				slog.Warn("Provider not found, skipping", "name", cfg.Name)
				providerExecs[i] = ProviderExecution{
					ProviderName: cfg.Name,
					Status:       "SKIPPED",
					Error:        "provider not registered",
				}
				continue
			}

			wg.Add(1)
			go func(idx int, p providers.Provider, inputs map[string]string) {
				defer wg.Done()

				startTime := time.Now()

				// Log child execution start (non-blocking, best effort)
				// We don't fail the enrichment if logging fails
				providerExecs[idx].ProviderName = p.Name()
				providerExecs[idx].Status = "STARTED"

				res, err := p.Enrich(ctx, payload.StandardizedActivity, userRec, inputs)
				duration := time.Since(startTime).Milliseconds()
				providerExecs[idx].DurationMs = duration

				if err != nil {
					slog.Error("Enricher failed", "name", p.Name(), "error", err, "duration_ms", duration)
					errs[idx] = err
					providerExecs[idx].Status = "FAILED"
					providerExecs[idx].Error = err.Error()
					return
				}

				providerExecs[idx].Status = "SUCCESS"
				results[idx] = res
				slog.Info("Enricher completed", "name", p.Name(), "duration_ms", duration)
			}(i, provider, cfg.Inputs)
		}
		wg.Wait()

		// Collect provider executions
		for _, pe := range providerExecs {
			if pe.ProviderName != "" {
				allProviderExecutions = append(allProviderExecutions, pe)
			}
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
		duration := 3600
		startTime, _ := time.Parse(time.RFC3339, payload.Timestamp)
		if payload.StandardizedActivity != nil && len(payload.StandardizedActivity.Sessions) > 0 {
			duration = int(payload.StandardizedActivity.Sessions[0].TotalElapsedTime)
			if duration == 0 {
				duration = 3600
			}
		}

		aggregatedHR := make([]int, duration)
		aggregatedPower := make([]int, duration)

		for i, res := range results {
			if res == nil {
				continue
			}
			cfgName := configs[i].Name
			finalEvent.AppliedEnrichments = append(finalEvent.AppliedEnrichments, cfgName)

			if res.Name != "" {
				finalEvent.Name = res.Name
			}
			if res.Description != "" {
				finalEvent.Description = res.Description
			}
			if res.ActivityType != "" {
				finalEvent.ActivityType = res.ActivityType
			}

			if len(res.HeartRateStream) == duration {
				for idx, val := range res.HeartRateStream {
					if val > 0 {
						aggregatedHR[idx] = val
					}
				}
			}
			if len(res.PowerStream) == duration {
				for idx, val := range res.PowerStream {
					if val > 0 {
						aggregatedPower[idx] = val
					}
				}
			}
			for k, v := range res.Metadata {
				finalEvent.EnrichmentMetadata[k] = v
			}
		}

		// 3c. Generate Artifacts (FIT File)
		fitBytes, err := fit.GenerateFitFile(startTime, duration, aggregatedPower, aggregatedHR)
		if err == nil && len(fitBytes) > 0 {
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
	Name   string
	Inputs map[string]string
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
					Name:   e.Name,
					Inputs: e.Inputs,
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

func (o *Orchestrator) mapUser(data map[string]interface{}) *pb.UserRecord {
	// Re-using the JSON round-trip hack because manual mapping of deeply nested structures like Pipelines
	// is error-prone and verbose in Go without a dedicated library.
	// Since performance is not critical here (per-activity), this is acceptable.

	// Create a temporary struct to handle Firestore -> JSON -> Proto
	// Ideally we'd map manually, but "pipelines" is a list of complex objects.
	// We'll rely on the existing manual map for Integrations to keep code stable,
	// but add Pipelines parsing.

	rec := &pb.UserRecord{
		UserId:       fmt.Sprintf("%v", data["user_id"]),
		Integrations: &pb.UserIntegrations{},
		Pipelines:    []*pb.PipelineConfig{},
	}

	if integrations, ok := data["integrations"].(map[string]interface{}); ok {
		if fitbit, ok := integrations["fitbit"].(map[string]interface{}); ok {
			rec.Integrations.Fitbit = &pb.FitbitIntegration{
				Enabled:      fitbit["enabled"] == true,
				AccessToken:  fmt.Sprintf("%v", fitbit["access_token"]),
				RefreshToken: fmt.Sprintf("%v", fitbit["refresh_token"]),
				FitbitUserId: fmt.Sprintf("%v", fitbit["fitbit_user_id"]),
			}
		}
		if strava, ok := integrations["strava"].(map[string]interface{}); ok {
			rec.Integrations.Strava = &pb.StravaIntegration{
				Enabled: strava["enabled"] == true,
			}
		}
	}

	if pipelines, ok := data["pipelines"].([]interface{}); ok {
		for _, p := range pipelines {
			if pMap, ok := p.(map[string]interface{}); ok {
				pc := &pb.PipelineConfig{
					Id:     fmt.Sprintf("%v", pMap["id"]),
					Source: fmt.Sprintf("%v", pMap["source"]),
				}

				// Dests
				if dests, ok := pMap["destinations"].([]interface{}); ok {
					for _, d := range dests {
						pc.Destinations = append(pc.Destinations, fmt.Sprintf("%v", d))
					}
				}

				// Enrichers
				if enrichers, ok := pMap["enrichers"].([]interface{}); ok {
					for _, e := range enrichers {
						if eMap, ok := e.(map[string]interface{}); ok {
							ec := &pb.EnricherConfig{
								Name:   fmt.Sprintf("%v", eMap["name"]),
								Inputs: make(map[string]string),
							}
							if inputs, ok := eMap["inputs"].(map[string]interface{}); ok {
								for k, v := range inputs {
									ec.Inputs[k] = fmt.Sprintf("%v", v)
								}
							}
							pc.Enrichers = append(pc.Enrichers, ec)
						}
					}
				}
				rec.Pipelines = append(rec.Pipelines, pc)
			}
		}
	}

	return rec
}
