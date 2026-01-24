// Package loopprevention provides source-level loop detection for FitGlue activity pipelines.
//
// When activities are uploaded to destinations (Strava, Hevy, etc.), those platforms send
// webhooks back which appear as new activities. This package prevents infinite loops by:
// 1. Tracking uploaded activities in Firestore
// 2. Checking if incoming webhook activities are "bouncebacks" from our own uploads
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

	// GetUploadedActivity retrieves an uploaded activity record by source and external ID.
	GetUploadedActivity(ctx context.Context, userId string, source pb.ActivitySource, externalId string) (*pb.UploadedActivityRecord, error)
}

// IsBounceback checks if an incoming activity from a webhook is a "bounceback" from
// our own upload. Returns true if:
// 1. The source has a corresponding destination
// 2. We have a record of uploading this activity to that destination
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

	record, err := store.GetUploadedActivity(ctx, userId, source, externalId)
	if err != nil {
		// Log error but don't block processing - fail open
		return false, fmt.Errorf("failed to check uploaded activity: %w", err)
	}

	if record == nil {
		return false, nil
	}

	// Check if we uploaded to the corresponding destination
	return record.Destination == correspondingDest, nil
}

// BuildUploadedActivityID creates a composite ID for an uploaded activity record.
// Format: "{source}:{external_id}" for efficient lookup.
func BuildUploadedActivityID(source pb.ActivitySource, externalId string) string {
	// Use lowercase source name without prefix
	sourceName := strings.TrimPrefix(source.String(), "SOURCE_")
	return fmt.Sprintf("%s:%s", strings.ToLower(sourceName), externalId)
}
