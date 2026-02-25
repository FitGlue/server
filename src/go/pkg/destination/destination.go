// Package destination defines the standard interface all activity destinations must implement.
package destination

import (
	"context"
	"github.com/fitglue/server/src/go/pkg/domain/user"

	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
)

// Destination defines the interface all activity destinations must implement.
// Each destination supports both Create (new activity) and Update (modify existing).
type Destination interface {
	// Create uploads a new activity to the destination.
	// Returns the destination-specific activity ID (e.g., Strava activity ID).
	Create(ctx context.Context, payload *pbevents.ActivityPayload, user *user.Record) (string, error)

	// Update modifies an existing activity on the destination.
	// Uses PipelineRun to find the previously created activity via destinations[].external_id.
	// Should fetch existing activity, merge new data, and PUT.
	Update(ctx context.Context, payload *pbevents.ActivityPayload, user *user.Record, pipelineRun *pbpipeline.PipelineRun) error

	// Name returns the destination identifier (e.g., "strava", "mock").
	Name() string
}
