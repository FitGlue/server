// Package loopprevention provides source-level loop detection for FitGlue activity pipelines.
//
// When activities are uploaded to destinations (Strava, Hevy, etc.), those platforms send
// webhooks back which appear as new activities. This package prevents infinite loops by:
// 1. Tracking uploaded activities in Firestore keyed by {destination}:{destination_id}
// 2. Checking if incoming webhook activities are "bouncebacks" from our own uploads
//
// Key insight: When Hevy sends a webhook, the external ID is Hevy's workout ID, not
// the original source's ID. So we must store records using the destination's ID.
package loopprevention

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// SourceToDestinationMap maps ActivitySource enums to their corresponding Destination enums.
// Sources without destinations (e.g., FILE_UPLOAD) are not included.
var SourceToDestinationMap = map[pb.ActivitySource]pb.Destination{
	pb.ActivitySource_SOURCE_HEVY:   pb.Destination_DESTINATION_HEVY,
	pb.ActivitySource_SOURCE_STRAVA: pb.Destination_DESTINATION_STRAVA,
	pb.ActivitySource_SOURCE_FITBIT: pb.Destination_DESTINATION_UNSPECIFIED, // Future: DESTINATION_FITBIT
	// Note: SOURCE_FILE_UPLOAD, SOURCE_PARKRUN_RESULTS, etc. are source-only
}

// GetCorrespondingDestination returns the destination that corresponds to the given source.
// Returns DESTINATION_UNSPECIFIED if the source has no corresponding destination.
func GetCorrespondingDestination(source pb.ActivitySource) pb.Destination {
	if dest, ok := SourceToDestinationMap[source]; ok {
		return dest
	}
	return pb.Destination_DESTINATION_UNSPECIFIED
}

// UploadedActivityStore defines the interface for persisting uploaded activity records.
type UploadedActivityStore interface {
	// SetUploadedActivity records that an activity was uploaded to a destination.
	SetUploadedActivity(ctx context.Context, userId string, record *pb.UploadedActivityRecord) error

	// GetUploadedActivity retrieves an uploaded activity record by destination and destination ID.
	// This matches how webhooks arrive: the destination (e.g., Hevy) sends its own ID.
	GetUploadedActivity(ctx context.Context, userId string, destination pb.Destination, destinationId string) (*pb.UploadedActivityRecord, error)
}

// IsBounceback checks if an incoming activity from a webhook is a "bounceback" from
// our own upload. Returns true if we have a record of uploading this activity.
//
// The check works by:
// 1. Getting the destination that corresponds to the webhook source (e.g., SOURCE_HEVY -> DESTINATION_HEVY)
// 2. Looking up if we have a record of uploading to that destination with this external ID
func IsBounceback(
	ctx context.Context,
	store UploadedActivityStore,
	userId string,
	source pb.ActivitySource,
	externalId string,
) (bool, error) {
	correspondingDest := GetCorrespondingDestination(source)
	if correspondingDest == pb.Destination_DESTINATION_UNSPECIFIED {
		// Source has no destination counterpart, not a bounceback scenario
		return false, nil
	}

	// When a webhook arrives from Hevy with hevy_workout_123, we look for
	// a record stored as DESTINATION_HEVY:hevy_workout_123
	record, err := store.GetUploadedActivity(ctx, userId, correspondingDest, externalId)
	if err != nil {
		// Log error but don't block processing - fail open
		return false, fmt.Errorf("failed to check uploaded activity: %w", err)
	}

	return record != nil, nil
}

// BuildUploadedActivityID creates a composite ID for an uploaded activity record.
// Format: "{destination}:{destination_id}" for efficient lookup.
//
// This is the KEY insight: when Hevy sends a webhook, the external ID will be
// Hevy's workout ID. So we store records using the destination's ID, not the source's.
func BuildUploadedActivityID(destination pb.Destination, destinationId string) string {
	// Use lowercase destination name without prefix
	destName := strings.TrimPrefix(destination.String(), "DESTINATION_")
	return fmt.Sprintf("%s:%s", strings.ToLower(destName), destinationId)
}
