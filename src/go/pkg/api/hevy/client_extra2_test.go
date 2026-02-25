package hevy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/fitglue/server/src/go/pkg/api/hevy"
)

func hevyAPIExtra2Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func hevyAPIWorkoutUUID() openapi_types.UUID {
	return openapi_types.UUID{}
}

// TestHevyAPIClientWithResponses covers ALL pkg/api/hevy ClientWithResponses methods for code coverage.
func TestHevyAPIClientWithResponses(t *testing.T) {
	srv := hevyAPIExtra2Server()
	defer srv.Close()

	c, err := hevy.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}

	ctx := context.Background()
	wid := hevyAPIWorkoutUUID()

	// Exercise History
	c.GetV1ExerciseHistoryExerciseTemplateIdWithResponse(ctx, 1, &hevy.GetV1ExerciseHistoryExerciseTemplateIdParams{}) //nolint:errcheck

	// Exercise Templates
	c.GetV1ExerciseTemplatesWithResponse(ctx, &hevy.GetV1ExerciseTemplatesParams{})                                                                      //nolint:errcheck
	c.PostV1ExerciseTemplatesWithBodyWithResponse(ctx, &hevy.PostV1ExerciseTemplatesParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.PostV1ExerciseTemplatesWithResponse(ctx, &hevy.PostV1ExerciseTemplatesParams{}, hevy.PostV1ExerciseTemplatesJSONRequestBody{})                     //nolint:errcheck
	c.GetV1ExerciseTemplatesExerciseTemplateIdWithResponse(ctx, "tmpl-1", &hevy.GetV1ExerciseTemplatesExerciseTemplateIdParams{})                        //nolint:errcheck

	// Routine Folders
	c.GetV1RoutineFoldersWithResponse(ctx, &hevy.GetV1RoutineFoldersParams{})                                                                      //nolint:errcheck
	c.PostV1RoutineFoldersWithBodyWithResponse(ctx, &hevy.PostV1RoutineFoldersParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.PostV1RoutineFoldersWithResponse(ctx, &hevy.PostV1RoutineFoldersParams{}, hevy.PostV1RoutineFoldersJSONRequestBody{})                        //nolint:errcheck
	c.GetV1RoutineFoldersFolderIdWithResponse(ctx, 1, &hevy.GetV1RoutineFoldersFolderIdParams{})                                                   //nolint:errcheck

	// Routines
	c.GetV1RoutinesWithResponse(ctx, &hevy.GetV1RoutinesParams{})                                                                                         //nolint:errcheck
	c.PostV1RoutinesWithBodyWithResponse(ctx, &hevy.PostV1RoutinesParams{}, "application/json", io.NopCloser(strings.NewReader("{}")))                    //nolint:errcheck
	c.PostV1RoutinesWithResponse(ctx, &hevy.PostV1RoutinesParams{}, hevy.PostV1RoutinesJSONRequestBody{})                                                 //nolint:errcheck
	c.GetV1RoutinesRoutineIdWithResponse(ctx, 1, &hevy.GetV1RoutinesRoutineIdParams{})                                                                    //nolint:errcheck
	c.PutV1RoutinesRoutineIdWithBodyWithResponse(ctx, 1, &hevy.PutV1RoutinesRoutineIdParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.PutV1RoutinesRoutineIdWithResponse(ctx, 1, &hevy.PutV1RoutinesRoutineIdParams{}, hevy.PutV1RoutinesRoutineIdJSONRequestBody{})                      //nolint:errcheck

	// Workouts
	c.GetV1WorkoutsWithResponse(ctx, &hevy.GetV1WorkoutsParams{})                                                                                           //nolint:errcheck
	c.PostV1WorkoutsWithBodyWithResponse(ctx, &hevy.PostV1WorkoutsParams{}, "application/json", io.NopCloser(strings.NewReader("{}")))                      //nolint:errcheck
	c.PostV1WorkoutsWithResponse(ctx, &hevy.PostV1WorkoutsParams{}, hevy.PostV1WorkoutsJSONRequestBody{})                                                   //nolint:errcheck
	c.GetV1WorkoutsCountWithResponse(ctx, &hevy.GetV1WorkoutsCountParams{})                                                                                 //nolint:errcheck
	c.GetV1WorkoutsEventsWithResponse(ctx, &hevy.GetV1WorkoutsEventsParams{})                                                                               //nolint:errcheck
	c.GetV1WorkoutsWorkoutIdWithResponse(ctx, wid, &hevy.GetV1WorkoutsWorkoutIdParams{})                                                                    //nolint:errcheck
	c.PutV1WorkoutsWorkoutIdWithBodyWithResponse(ctx, wid, &hevy.PutV1WorkoutsWorkoutIdParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.PutV1WorkoutsWorkoutIdWithResponse(ctx, wid, &hevy.PutV1WorkoutsWorkoutIdParams{}, hevy.PutV1WorkoutsWorkoutIdJSONRequestBody{})                      //nolint:errcheck

	t.Logf("All hevy api ClientWithResponses methods exercised for coverage")
}
