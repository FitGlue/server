// Package destination defines the standard interface all activity destinations must implement.
package destination

import (
	"context"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// Destination defines the interface all activity destinations must implement.
// Each destination supports both Create (new activity) and Update (modify existing).
type Destination interface {
	// Create uploads a new activity to the destination.
	// Returns the destination-specific activity ID (e.g., Strava activity ID).
	Create(ctx context.Context, payload *pb.ActivityPayload, user *pb.UserRecord) (string, error)

	// Update modifies an existing activity on the destination.
	// Uses SynchronizedActivity to find the previously created activity.
	// Should fetch existing activity, merge new data, and PUT.
	Update(ctx context.Context, payload *pb.ActivityPayload, user *pb.UserRecord, syncActivity *pb.SynchronizedActivity) error

	// Name returns the destination identifier (e.g., "strava", "mock").
	Name() string
}
