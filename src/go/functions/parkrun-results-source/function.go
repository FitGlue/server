package parkrun_results_source

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	shared "github.com/fitglue/server/src/go/pkg"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/framework"
	infrapubsub "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	parkrunutil "github.com/fitglue/server/src/go/pkg/parkrun"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("PollParkrunResults", PollParkrunResults)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		svc, svcErr = bootstrap.NewService(ctx)
	})
	return svc, svcErr
}

// PollParkrunResults is triggered by Cloud Scheduler to check for pending Parkrun results
func PollParkrunResults(ctx context.Context, e cloudevents.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("parkrun-results-source", svc, pollHandler(nil))(ctx, e)
}

// pollHandler contains the business logic
// Uses the Pipeline Resume Pattern - queries auto-populated pending inputs and triggers enricher resume
func pollHandler(httpClient *http.Client) framework.HandlerFunc {
	return func(ctx context.Context, e cloudevents.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		fwCtx.Logger.Info("Starting Parkrun results poll")

		// Use plain HTTP client - matches successful local testing configuration
		// The oauth wrapper transports may interfere with headers in ways that trigger bot protection
		if httpClient == nil {
			httpClient = &http.Client{
				Timeout: 30 * time.Second,
			}
		}

		// Query for auto-populated pending inputs from the Parkrun enricher
		pendingInputs, err := fwCtx.Service.DB.ListPendingInputsByEnricher(ctx, "parkrun", pb.PendingInput_STATUS_WAITING)
		if err != nil {
			fwCtx.Logger.Error("Failed to query pending inputs", "error", err)
			return nil, fmt.Errorf("query pending inputs: %w", err)
		}

		if len(pendingInputs) == 0 {
			fwCtx.Logger.Info("No pending Parkrun inputs found")
			return map[string]interface{}{
				"status":    "SUCCESS",
				"processed": 0,
				"updated":   0,
				"message":   "No pending Parkrun results to process",
			}, nil
		}

		fwCtx.Logger.Info("Found pending Parkrun inputs", "count", len(pendingInputs))

		var processed, updated, failed int
		for _, input := range pendingInputs {
			processed++

			// Only process auto-populated inputs that continued without resolution
			if !input.AutoPopulated || !input.ContinuedWithoutResolution {
				fwCtx.Logger.Debug("Skipping non-auto-populated input", "input_id", input.ActivityId)
				continue
			}

			// Get user for Parkrun integration credentials
			user, err := fwCtx.Service.DB.GetUser(ctx, input.UserId)
			if err != nil || user == nil {
				fwCtx.Logger.Warn("Failed to get user", "user_id", input.UserId, "error", err)
				continue
			}

			if user.Integrations == nil || user.Integrations.Parkrun == nil || !user.Integrations.Parkrun.Enabled {
				fwCtx.Logger.Debug("User has no Parkrun integration", "user_id", input.UserId)
				continue
			}

			// Extract event info from pending input metadata
			eventSlug := ""
			eventName := ""
			if input.OriginalPayload != nil && input.OriginalPayload.Metadata != nil {
				eventSlug = input.OriginalPayload.Metadata["parkrun_event_slug"]
				eventName = input.OriginalPayload.Metadata["parkrun_event_name"]
			} else {
				// This is an error - the payload should always have metadata
				fwCtx.Logger.Error("Pending input missing OriginalPayload or Metadata",
					"input_id", input.ActivityId,
					"has_original_payload", input.OriginalPayload != nil)
				continue
			}

			if eventSlug == "" {
				fwCtx.Logger.Error("Required field missing: parkrun_event_slug is empty",
					"input_id", input.ActivityId,
					"event_name", eventName)
				continue
			}

			// Fetch results from Parkrun using shared utility
			integration := user.Integrations.Parkrun
			results, err := parkrunutil.FetchResultsForAthlete(
				ctx, fwCtx.Logger,
				integration.AthleteId,
				integration.CountryUrl,
				eventSlug,
			)
			if err != nil {
				fwCtx.Logger.Warn("Failed to fetch results (may not be published yet)",
					"activity_id", input.ActivityId,
					"event_slug", eventSlug,
					"error", err)
				continue
			}

			if results == nil {
				fwCtx.Logger.Info("Results not yet available",
					"activity_id", input.ActivityId,
					"event_slug", eventSlug)
				continue
			}

			// Update the pending input with the resolved data
			resultDescription := parkrunutil.FormatResultsDescription(results, eventName)
			err = fwCtx.Service.DB.UpdatePendingInput(ctx, input.ActivityId, map[string]interface{}{
				"status":       int32(pb.PendingInput_STATUS_COMPLETED),
				"completed_at": timestamppb.Now(),
				"input_data": map[string]string{
					"description": resultDescription,
					"position":    fmt.Sprintf("%d", results.Position),
					"time":        results.Time,
					"age_grade":   results.AgeGrade,
				},
			})
			if err != nil {
				fwCtx.Logger.Error("Failed to update pending input", "error", err)
				failed++
				continue
			}

			// Trigger pipeline resume by using the original payload with resume flags
			// The OriginalPayload contains the full ActivityPayload including StandardizedActivity
			// which the enricher requires to function
			resumePayload := input.OriginalPayload
			if resumePayload == nil {
				fwCtx.Logger.Error("No OriginalPayload in pending input", "activity_id", input.ActivityId)
				failed++
				continue
			}
			// Add resume flags to the original payload
			resumePayload.IsResume = true
			resumePayload.ResumeOnlyEnrichers = []string{"parkrun"}
			resumePayload.UseUpdateMethod = true
			resumePayload.ResumePendingInputId = &input.ActivityId
			if input.LinkedActivityId != "" {
				resumePayload.ActivityId = &input.LinkedActivityId
			}

			// Debug: log pipelineId from pending input
			fwCtx.Logger.Info("Resume payload pipelineId check",
				"input_pipeline_id", input.PipelineId,
				"activity_id", input.ActivityId)

			if input.PipelineId != "" {
				resumePayload.PipelineId = &input.PipelineId
				fwCtx.Logger.Info("Set pipelineId on resume payload", "pipeline_id", input.PipelineId)
			}

			// Generate a fresh pipeline execution ID for this resume flow
			newPipelineExecID := fmt.Sprintf("parkrun-results-%s", uuid.NewString())
			resumePayload.PipelineExecutionId = &newPipelineExecID

			eventData, err := protojson.Marshal(resumePayload)
			if err != nil {
				fwCtx.Logger.Error("Failed to marshal resume payload", "error", err)
				failed++
				continue
			}

			ceEvent, err := infrapubsub.NewCloudEvent(
				"/integrations/parkrun/results",
				"com.fitglue.activity.enriched", // Re-trigger enricher with resume mode
				eventData,
			)
			if err != nil {
				fwCtx.Logger.Error("Failed to create CloudEvent", "error", err)
				failed++
				continue
			}

			// Publish to PIPELINE_ACTIVITY since we already have the pipelineId (bypasses splitter)
			_, err = fwCtx.Service.Pub.PublishCloudEvent(ctx, shared.TopicPipelineActivity, ceEvent)
			if err != nil {
				fwCtx.Logger.Error("Failed to publish resume event", "error", err)
				failed++
				continue
			}

			updated++
			fwCtx.Logger.Info("Published pipeline resume for Parkrun results",
				"activity_id", input.LinkedActivityId,
				"position", results.Position,
				"time", results.Time)
		}

		return map[string]interface{}{
			"status":    "SUCCESS",
			"processed": processed,
			"updated":   updated,
			"failed":    failed,
		}, nil
	}
}
