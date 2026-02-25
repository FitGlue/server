// nolint:proto-json
package destination

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/fitglue/server/src/go/internal/infra"
	shared "github.com/fitglue/server/src/go/pkg"
	"github.com/fitglue/server/src/go/pkg/destination"
	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/domain/user"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	activitypb "github.com/fitglue/server/src/go/pkg/types/pb/services/activity"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"google.golang.org/protobuf/encoding/protojson"
)

// UploadExecutor handles the consumption of Pub/Sub messages
// and orchestrating the destination interface lifecycle.
type UploadExecutor struct {
	registry       *Registry
	userClient     userpb.UserServiceClient
	activityClient activitypb.ActivityServiceClient
	db             shared.Database
	notifications  shared.NotificationService
	logger         infra.Logger
}

// NewUploadExecutor creates an orchestrator initialized with dependencies.
func NewUploadExecutor(
	registry *Registry,
	userClient userpb.UserServiceClient,
	activityClient activitypb.ActivityServiceClient,
	db shared.Database,
	notifications shared.NotificationService,
	logger infra.Logger,
) *UploadExecutor {
	return &UploadExecutor{
		registry:       registry,
		userClient:     userClient,
		activityClient: activityClient,
		db:             db,
		notifications:  notifications,
		logger:         logger,
	}
}

// HandlePubSubPush parses an HTTP request sent via Pub/Sub Push Subscription.
func (e *UploadExecutor) HandlePubSubPush(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		e.logger.Error(ctx, "Failed to read request body", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the Pub/Sub Push message envelope
	var msg struct {
		Message struct {
			Data []byte `json:"data"`
			ID   string `json:"messageId"`
		} `json:"message"`
		Subscription string `json:"subscription"`
	}

	if err := json.Unmarshal(body, &msg); err != nil {
		e.logger.Error(ctx, "Failed to unmarshal pub/sub envelope", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Unmarshal the inner payload as a CloudEvent
	var ce event.Event
	if err := json.Unmarshal(msg.Message.Data, &ce); err != nil {
		e.logger.Error(ctx, "Failed to unmarshal inner CloudEvent", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := e.Process(ctx, &ce); err != nil {
		e.logger.Error(ctx, "Failed to process destination upload event", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

// Process unmarshals the event to an EnrichedActivityEvent and distributes it to the correct Destinations
func (e *UploadExecutor) Process(ctx context.Context, ce *event.Event) error {
	var payload pbevents.EnrichedActivityEvent

	// Use protojson to unmarshal to handle enum strings correctly
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
		AllowPartial:   true,
	}
	if err := unmarshaler.Unmarshal(ce.Data(), &payload); err != nil {
		e.logger.Error(ctx, "Failed to unmarshal EnrichedActivityEvent via protojson", "error", err)
		return nil // Return nil so pubsub ack's it as a bad payload
	}

	e.logger.Info(ctx, "Processing upload for activity", "activity_id", payload.ActivityId, "user_id", payload.UserId, "destinations_count", len(payload.Destinations))

	if len(payload.Destinations) == 0 {
		return nil
	}

	// Fetch User Record
	profileResp, err := e.userClient.GetProfile(ctx, &userpb.GetProfileRequest{UserId: payload.UserId})
	if err != nil {
		e.logger.Error(ctx, "Failed to fetch user profile", "error", err)
		return fmt.Errorf("getting user profile: %w", err)
	}

	integrationsResp, err := e.userClient.ListIntegrations(ctx, &userpb.ListIntegrationsRequest{UserId: payload.UserId})
	if err != nil {
		e.logger.Error(ctx, "Failed to fetch user integrations", "error", err)
		return fmt.Errorf("getting user integrations: %w", err)
	}

	userRecord := &user.Record{
		UserProfile:  profileResp,
		Integrations: integrationsResp,
	}

	// Merge EnrichmentMetadata into a new Metadata map
	metadata := make(map[string]string)
	if payload.EnrichmentMetadata != nil {
		for k, v := range payload.EnrichmentMetadata {
			metadata[k] = v
		}
	}

	// Inject fields required by uploaders that aren't native to ActivityPayload
	metadata["fit_file_uri"] = payload.FitFileUri
	metadata["activity_name"] = payload.Name
	metadata["description"] = payload.Description
	metadata["strava_sport_type"] = activity.GetStravaActivityType(payload.ActivityType)
	metadata["activity_type"] = payload.ActivityType.String()

	// Construct generic ActivityPayload for Destination Uploaders
	activityPayload := &pbevents.ActivityPayload{
		Source:               payload.Source,
		UserId:               payload.UserId,
		ActivityId:           &payload.ActivityId,
		StandardizedActivity: payload.ActivityData, // Map ActivityData -> StandardizedActivity
		OriginalPayloadJson:  "",
		Metadata:             metadata, // Injected Metadata
	}

	isUpdate := false
	if useUpdate, ok := payload.EnrichmentMetadata["use_update_method"]; ok && useUpdate == "true" {
		isUpdate = true
	}

	// Fetch the parent PipelineRun
	var pr *pbpipeline.PipelineRun
	var pipelineRunId string
	if payload.PipelineExecutionId != nil {
		pipelineRunId = *payload.PipelineExecutionId
		// We could fetch the PR here from e.db if needed by uploaders (e.g. for Update logic)
		// For now, we mainly need the ID for UpdateStatus
	}

	for _, destEnum := range payload.Destinations {
		if destEnum == pbplugin.DestinationType_DESTINATION_UNSPECIFIED {
			continue
		}

		uploader, ok := e.registry.Get(destEnum)
		if !ok {
			e.logger.Warn(ctx, "No uploader registered for destination", "destination", destEnum.String())
			if pipelineRunId != "" {
				destination.UpdateStatus(ctx, e.db, e.notifications, payload.UserId, pipelineRunId, destEnum, pbpipeline.DestinationStatus_DESTINATION_STATUS_FAILED, "", "Uploader not registered", payload.Name, payload.ActivityId, slog.Default())
			}
			continue
		}

		e.logger.Info(ctx, "Triggering destination uploader", "destination", destEnum.String(), "is_update", isUpdate)

		var externalId string
		var uploadErr error

		// Create or Update
		if isUpdate {
			uploadErr = uploader.Update(ctx, activityPayload, userRecord, pr)
		} else {
			externalId, uploadErr = uploader.Create(ctx, activityPayload, userRecord)
		}

		if uploadErr != nil {
			e.logger.Error(ctx, "Destination uploader failed", "destination", destEnum.String(), "error", uploadErr)
			if pipelineRunId != "" {
				destination.UpdateStatus(ctx, e.db, e.notifications, payload.UserId, pipelineRunId, destEnum, pbpipeline.DestinationStatus_DESTINATION_STATUS_FAILED, externalId, uploadErr.Error(), payload.Name, payload.ActivityId, slog.Default())
			}
			continue
		}

		// Success
		if pipelineRunId != "" {
			destination.UpdateStatus(ctx, e.db, e.notifications, payload.UserId, pipelineRunId, destEnum, pbpipeline.DestinationStatus_DESTINATION_STATUS_SUCCESS, externalId, "", payload.Name, payload.ActivityId, slog.Default())
		}

		e.logger.Info(ctx, "Destination uploader completed successfully", "destination", destEnum.String())
	}

	return nil
}
