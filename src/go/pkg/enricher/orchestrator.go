package enricher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	"github.com/ripixel/fitglue-server/src/go/pkg/fit"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type Orchestrator struct {
	database   shared.Database
	storage    shared.BlobStore
	bucketName string
	providers  map[string]Provider
}

func NewOrchestrator(db shared.Database, storage shared.BlobStore, bucketName string) *Orchestrator {
	return &Orchestrator{
		database:   db,
		storage:    storage,
		bucketName: bucketName,
		providers:  make(map[string]Provider),
	}
}

func (o *Orchestrator) Register(p Provider) {
	o.providers[p.Name()] = p
}

// Process executes the enrichment pipeline
func (o *Orchestrator) Process(ctx context.Context, payload *pb.ActivityPayload) (*pb.EnrichedActivityEvent, error) {
	// 1. Fetch User Config
	userDoc, err := o.database.GetUser(ctx, payload.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user config: %w", err)
	}
	userRec := o.mapUser(userDoc)

	// 2. Resolve Providers
	configs := o.resolveEnrichers(payload.Source, userDoc)
	slog.Info("Resolved enrichers", "count", len(configs), "source", payload.Source)

	// 3. Fan-Out Execution
	results := make([]*EnrichmentResult, len(configs))
	var wg sync.WaitGroup
	errs := make([]error, len(configs))

	for i, cfg := range configs {
		provider, ok := o.providers[cfg.Name]
		if !ok {
			slog.Warn("Provider not found, skipping", "name", cfg.Name)
			continue
		}

		wg.Add(1)
		go func(idx int, p Provider, inputs map[string]string) {
			defer wg.Done()
			res, err := p.Enrich(ctx, payload.StandardizedActivity, userRec, inputs)
			if err != nil {
				slog.Error("Enricher failed", "name", p.Name(), "error", err)
				errs[idx] = err
				return
			}
			results[idx] = res
		}(i, provider, cfg.Inputs)
	}
	wg.Wait()

	// 4. Merge Results & Fan-In
	finalEvent := &pb.EnrichedActivityEvent{
		UserId:       payload.UserId,
		Source:       payload.Source,
		ActivityId:   uuid.NewString(), // Generate new ID for enriched activity
		ActivityData: payload.StandardizedActivity,
		ActivityType: "WEIGHT_TRAINING", // Default, can be enriched
		Name:         "Workout",         // Default
		AppliedEnrichments: []string{},
		EnrichmentMetadata: make(map[string]string),
	}

	// Apply basic defaults from original activity if available
	if payload.StandardizedActivity != nil {
		finalEvent.Name = payload.StandardizedActivity.Name
		finalEvent.Description = payload.StandardizedActivity.Description
		finalEvent.ActivityType = payload.StandardizedActivity.Type
	}

	// Prepare Streams for Merging
	duration := 3600 // Default
	startTime, _ := time.Parse(time.RFC3339, payload.Timestamp)
	if payload.StandardizedActivity != nil && len(payload.StandardizedActivity.Sessions) > 0 {
		duration = int(payload.StandardizedActivity.Sessions[0].TotalElapsedTime)
		// Ensure non-zero
		if duration == 0 { duration = 3600 }
	}

	aggregatedHR := make([]int, duration)
	aggregatedPower := make([]int, duration)

	for i, res := range results {
		if res == nil {
			continue
		}

		cfgName := configs[i].Name
		finalEvent.AppliedEnrichments = append(finalEvent.AppliedEnrichments, cfgName)

		// 4a. Merge Metadata (Overwrite if present)
		if res.Name != "" {
			finalEvent.Name = res.Name
		}
		if res.Description != "" {
			finalEvent.Description = res.Description
		}
		if res.ActivityType != "" {
			finalEvent.ActivityType = res.ActivityType
		}

		// 4b. Merge Streams (Aggregation strategy: Non-zero values win)
		if len(res.HeartRateStream) == duration {
			for idx, val := range res.HeartRateStream {
				if val > 0 {
					aggregatedHR[idx] = val
				}
			}
		} else if len(res.HeartRateStream) > 0 {
			// Handle mismatch? Log warning? For now skip.
			slog.Warn("HeartRateStream duration mismatch", "expected", duration, "actual", len(res.HeartRateStream))
		}

		if len(res.PowerStream) == duration {
			for idx, val := range res.PowerStream {
				if val > 0 {
					aggregatedPower[idx] = val
				}
			}
		}

		// 4c. Merge Extra Metadata
		for k, v := range res.Metadata {
			finalEvent.EnrichmentMetadata[k] = v
		}
	}

	// 5. Generate Artifacts (FIT File)
	// We generate the FIT file using the MERGED streams

	// Default streams if they are empty is just 0s, which is fine.
	// But if we have HR data merged, it will be in aggregatedHR.

	fitBytes, err := fit.GenerateFitFile(startTime, duration, aggregatedPower, aggregatedHR)
	if err != nil {
		slog.Error("Failed to generate FIT file", "error", err)
		// Non-fatal? Maybe we still proceed with enriched metadata.
	} else if len(fitBytes) > 0 {
		objName := fmt.Sprintf("activities/%s/%s.fit", payload.UserId, finalEvent.ActivityId)
		if err := o.storage.Write(ctx, o.bucketName, objName, fitBytes); err != nil {
			slog.Error("Failed to write FIT file artifact", "error", err)
		} else {
			finalEvent.FitFileUri = fmt.Sprintf("gs://%s/%s", o.bucketName, objName)
		}
	}

	return finalEvent, nil
}

type configuredEnricher struct {
	Name   string
	Inputs map[string]string
}

func (o *Orchestrator) resolveEnrichers(source pb.ActivitySource, userDoc map[string]interface{}) []configuredEnricher {
	enrichments, ok := userDoc["enrichments"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Try strict match "SOURCE_HEVY" then flexible "HEVY"
	sourceName := source.String()

	var sourceConfig map[string]interface{}
	if val, found := enrichments[sourceName]; found {
		sourceConfig, _ = val.(map[string]interface{})
	} else if val, found := enrichments["HEVY"]; found && source == pb.ActivitySource_SOURCE_HEVY {
		sourceConfig, _ = val.(map[string]interface{})
	}

	if sourceConfig == nil {
		return nil
	}

	enrichersRaw, ok := sourceConfig["enrichers"].([]interface{})
	if !ok {
		return nil
	}

	var results []configuredEnricher
	for _, item := range enrichersRaw {
		cfgMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := cfgMap["name"].(string)
		inputsRaw, _ := cfgMap["inputs"].(map[string]interface{})

		inputs := make(map[string]string)
		for k, v := range inputsRaw {
			inputs[k] = fmt.Sprintf("%v", v)
		}

		if name != "" {
			results = append(results, configuredEnricher{Name: name, Inputs: inputs})
		}
	}
	return results
}

func (o *Orchestrator) mapUser(data map[string]interface{}) *pb.UserRecord {
	rec := &pb.UserRecord{
		UserId:       fmt.Sprintf("%v", data["user_id"]),
		Integrations: &pb.UserIntegrations{},
	}

	if integrations, ok := data["integrations"].(map[string]interface{}); ok {
		if fitbit, ok := integrations["fitbit"].(map[string]interface{}); ok {
			rec.Integrations.Fitbit = &pb.FitbitIntegration{
				Enabled:        fitbit["enabled"] == true,
				AccessToken:    fmt.Sprintf("%v", fitbit["access_token"]),
				RefreshToken:   fmt.Sprintf("%v", fitbit["refresh_token"]),
				FitbitUserId:   fmt.Sprintf("%v", fitbit["fitbit_user_id"]),
			}
		}
		// Add Hevy mapping if needed
		if hevy, ok := integrations["hevy"].(map[string]interface{}); ok {
			rec.Integrations.Hevy = &pb.HevyIntegration{
				Enabled: hevy["enabled"] == true,
				ApiKey:  fmt.Sprintf("%v", hevy["api_key"]),
				UserId:  fmt.Sprintf("%v", hevy["user_id"]),
			}
		}
	}
	return rec
}
