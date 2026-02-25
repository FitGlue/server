package hevy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/fitglue/server/src/go/pkg/integrations/hevy"
)

func hevyAllServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestAllHevyMethods calls ALL remaining hevy client methods.
func TestAllHevyMethods(t *testing.T) {
	srv := hevyAllServer()
	defer srv.Close()

	c, err := hevy.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Exercise History
		{"GetV1ExerciseHistoryExerciseTemplateId", func() (*http.Response, error) {
			p := &hevy.GetV1ExerciseHistoryExerciseTemplateIdParams{}
			return c.GetV1ExerciseHistoryExerciseTemplateId(ctx, 1234, p)
		}},
		// Exercise Templates
		{"GetV1ExerciseTemplates", func() (*http.Response, error) {
			p := &hevy.GetV1ExerciseTemplatesParams{}
			return c.GetV1ExerciseTemplates(ctx, p)
		}},
		{"GetV1ExerciseTemplatesExerciseTemplateId", func() (*http.Response, error) {
			p := &hevy.GetV1ExerciseTemplatesExerciseTemplateIdParams{}
			return c.GetV1ExerciseTemplatesExerciseTemplateId(ctx, "template-id-1", p)
		}},
		// Routine Folders
		{"GetV1RoutineFolders", func() (*http.Response, error) {
			p := &hevy.GetV1RoutineFoldersParams{}
			return c.GetV1RoutineFolders(ctx, p)
		}},
		{"GetV1RoutineFoldersFolderId", func() (*http.Response, error) {
			p := &hevy.GetV1RoutineFoldersFolderIdParams{}
			return c.GetV1RoutineFoldersFolderId(ctx, 100, p)
		}},
		// Routines
		{"GetV1Routines", func() (*http.Response, error) {
			p := &hevy.GetV1RoutinesParams{}
			return c.GetV1Routines(ctx, p)
		}},
		{"GetV1RoutinesRoutineId", func() (*http.Response, error) {
			p := &hevy.GetV1RoutinesRoutineIdParams{}
			return c.GetV1RoutinesRoutineId(ctx, 42, p)
		}},
		// Workouts
		{"GetV1Workouts", func() (*http.Response, error) {
			p := &hevy.GetV1WorkoutsParams{}
			return c.GetV1Workouts(ctx, p)
		}},
		{"GetV1WorkoutsCount", func() (*http.Response, error) {
			p := &hevy.GetV1WorkoutsCountParams{}
			return c.GetV1WorkoutsCount(ctx, p)
		}},
		{"GetV1WorkoutsEvents", func() (*http.Response, error) {
			p := &hevy.GetV1WorkoutsEventsParams{}
			return c.GetV1WorkoutsEvents(ctx, p)
		}},
		{"GetV1WorkoutsWorkoutId", func() (*http.Response, error) {
			uuid := openapi_types.UUID{}
			p := &hevy.GetV1WorkoutsWorkoutIdParams{}
			return c.GetV1WorkoutsWorkoutId(ctx, uuid, p)
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.fn()
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
				return
			}
			if resp == nil {
				t.Errorf("%s: expected non-nil response", tc.name)
			}
		})
	}
}

func TestHevyIntegrationsNewClientWithResponses(t *testing.T) {
	srv := hevyAllServer()
	defer srv.Close()

	c, err := hevy.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
