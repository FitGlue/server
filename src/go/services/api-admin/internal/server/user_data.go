package server

import (
	"context"
	"fmt"
	"net/http"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"
	"google.golang.org/api/iterator"

	pipelinepb "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
)

func (s *APIServer) handleDeleteUserData(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	dataType := chi.URLParam(r, "dataType")

	if userID == "" || dataType == "" {
		WriteError(w, statusError(http.StatusBadRequest, "missing user id or data type"))
		return
	}

	ctx := r.Context()
	var err error

	switch dataType {
	case "integrations":
		err = s.deleteUserIntegrations(ctx, userID)
	case "pipelines":
		err = s.deleteUserPipelines(ctx, userID)
	case "activities":
		err = s.deleteUserActivities(ctx, userID)
	case "pending-inputs":
		err = s.deleteUserPendingInputs(ctx, userID)
	default:
		WriteError(w, statusError(http.StatusBadRequest, fmt.Sprintf("unknown data type: %s", dataType)))
		return
	}

	if err != nil {
		s.logger.Error(ctx, "failed to delete user data", "userId", userID, "dataType", dataType, "error", err)
		WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteUserIntegrations clears the integrations document for a user via Firestore
func (s *APIServer) deleteUserIntegrations(ctx context.Context, userID string) error {
	// The integrations are stored as a single document — delete it directly
	_, err := s.firestoreClient.Collection("users").Doc(userID).Collection("integrations").Doc("providers").Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete integrations: %w", err)
	}

	// Also clear the integrations field on the user document
	_, err = s.firestoreClient.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "integrations", Value: nil},
	})
	// Non-fatal if this fails (field may not exist)
	return nil
}

// deleteUserPipelines deletes all pipelines for a user via the pipeline domain service
func (s *APIServer) deleteUserPipelines(ctx context.Context, userID string) error {
	res, err := s.pipelineSvc.ListPipelines(ctx, &pipelinepb.ListPipelinesRequest{
		UserId: userID,
	})
	if err != nil {
		return err
	}

	for _, pipeline := range res.GetPipelines() {
		_, err := s.pipelineSvc.DeletePipeline(ctx, &pipelinepb.DeletePipelineRequest{
			UserId:     userID,
			PipelineId: pipeline.GetId(),
		})
		if err != nil {
			return fmt.Errorf("failed to delete pipeline %s: %w", pipeline.GetId(), err)
		}
	}
	return nil
}

// deleteUserActivities deletes all activities for a user via Firestore
func (s *APIServer) deleteUserActivities(ctx context.Context, userID string) error {
	iter := s.firestoreClient.Collection("users").Doc(userID).Collection("activities").Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list activities: %w", err)
		}

		if _, err := doc.Ref.Delete(ctx); err != nil {
			return fmt.Errorf("failed to delete activity %s: %w", doc.Ref.ID, err)
		}
	}
	return nil
}

// deleteUserPendingInputs resolves all pending inputs for a user via the pipeline service
func (s *APIServer) deleteUserPendingInputs(ctx context.Context, userID string) error {
	res, err := s.pipelineSvc.ListPendingInputs(ctx, &pipelinepb.ListPendingInputsRequest{
		UserId: userID,
	})
	if err != nil {
		return err
	}

	for _, input := range res.GetInputs() {
		_, err := s.pipelineSvc.ResolvePendingInput(ctx, &pipelinepb.ResolvePendingInputRequest{
			UserId:         userID,
			PendingInputId: input.GetActivityId(),
		})
		if err != nil {
			return fmt.Errorf("failed to resolve pending input %s: %w", input.GetActivityId(), err)
		}
	}
	return nil
}
