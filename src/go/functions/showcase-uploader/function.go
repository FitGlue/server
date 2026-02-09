package showcaseuploader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/destination"
	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/framework"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

const (
	// Hobbyist retention: 30 days
	HobbyistRetentionDays = 30
)

func init() {
	functions.CloudEvent("ShowcaseUpload", ShowcaseUpload)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		baseSvc, err := bootstrap.NewService(ctx)
		if err != nil {
			// Error returned to caller
			svcErr = err
			return
		}
		svc = baseSvc
	})
	return svc, svcErr
}

// ShowcaseUpload is the entry point for the showcase destination.
// It creates a public, shareable activity page and persists a SynchronizedActivity record.
func ShowcaseUpload(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("showcase-uploader", svc, showcaseHandler())(ctx, e)
}

// slugify converts a string to a URL-safe slug
func slugify(s string) string {
	// Lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	s = reg.ReplaceAllString(s, "")

	// Collapse multiple hyphens
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	// Trim hyphens from edges
	s = strings.Trim(s, "-")

	// Truncate to 30 characters
	if len(s) > 30 {
		s = s[:30]
		// Don't end with a hyphen after truncation
		s = strings.TrimRight(s, "-")
	}

	return s
}

// generateRandomSuffix creates a 6-character random alphanumeric suffix
func generateRandomSuffix() string {
	b := make([]byte, 3)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateShowcaseID creates a human-readable, unique showcase ID
// Format: {slugified-title}-{date} or {slugified-title}-{date}-{random6} if collision
func generateShowcaseID(ctx context.Context, svc *bootstrap.Service, title string, startTime time.Time) (string, error) {
	slug := slugify(title)
	if slug == "" {
		slug = "activity"
	}

	dateStr := startTime.Format("2006-01-02")
	baseID := fmt.Sprintf("%s-%s", slug, dateStr)

	// Check if base ID exists
	exists, err := svc.DB.ShowcaseActivityExists(ctx, baseID)
	if err != nil {
		return "", fmt.Errorf("failed to check showcase ID existence: %w", err)
	}

	if !exists {
		return baseID, nil
	}

	// Base ID exists, append random suffix
	// Try up to 5 times with different suffixes
	for i := 0; i < 5; i++ {
		suffix := generateRandomSuffix()
		candidateID := fmt.Sprintf("%s-%s", baseID, suffix)

		exists, err = svc.DB.ShowcaseActivityExists(ctx, candidateID)
		if err != nil {
			return "", fmt.Errorf("failed to check showcase ID existence: %w", err)
		}

		if !exists {
			return candidateID, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique showcase ID after 5 attempts")
}

func calculateExpiration(tier pb.UserTier, createdAt time.Time) *time.Time {
	if tier == pb.UserTier_USER_TIER_ATHLETE {
		// Athlete tier: never expires
		return nil
	}

	// Hobbyist tier: expires after 30 days
	expiry := createdAt.AddDate(0, 0, HobbyistRetentionDays)
	return &expiry
}

func showcaseHandler() framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		var eventPayload pb.EnrichedActivityEvent

		unmarshaler := protojson.UnmarshalOptions{
			DiscardUnknown: true,
			AllowPartial:   true,
		}
		if err := unmarshaler.Unmarshal(e.Data(), &eventPayload); err != nil {
			return nil, fmt.Errorf("protojson.Unmarshal: %w", err)
		}

		// Log initial state for debugging data flow issues
		fwCtx.Logger.Debug("Event payload after unmarshal",
			"has_activity_data", eventPayload.ActivityData != nil,
			"activity_data_uri", eventPayload.ActivityDataUri,
		)

		// Resolve activity data from GCS if needed (for large payloads offloaded by enricher)
		if err := activity.ResolveEnrichedEvent(ctx, &eventPayload, fwCtx.Service.Store); err != nil {
			fwCtx.Logger.Error("Failed to resolve activity data from GCS - showcase will be missing rich data",
				"error", err,
				"activity_data_uri", eventPayload.ActivityDataUri,
			)
		}

		// Log state after resolution attempt
		if eventPayload.ActivityData == nil {
			fwCtx.Logger.Warn("Activity data is nil after resolution - showcase will be missing rich data",
				"activity_data_uri", eventPayload.ActivityDataUri,
				"activity_id", eventPayload.ActivityId,
			)
		}

		fwCtx.Logger.Info("Showcase upload received",
			"activity_id", eventPayload.ActivityId,
			"pipeline_id", eventPayload.PipelineId,
			"user_id", eventPayload.UserId,
			"name", eventPayload.Name,
			"type", eventPayload.ActivityType,
			"source", eventPayload.Source,
		)

		// Get user to determine tier for expiration
		user, err := svc.DB.GetUser(ctx, eventPayload.UserId)
		if err != nil {
			fwCtx.Logger.Warn("Failed to get user for tier lookup, defaulting to hobbyist tier", "error", err, "userId", eventPayload.UserId)
			user = &pb.UserRecord{Tier: pb.UserTier_USER_TIER_HOBBYIST}
		}

		// Determine start time
		var startTime time.Time
		if eventPayload.StartTime != nil {
			startTime = eventPayload.StartTime.AsTime()
		} else {
			startTime = time.Now()
		}

		// Check if this activity already has a showcase ID (resume/update scenario)
		var showcaseID string
		existingPipelineRun, _ := svc.DB.GetPipelineRunByActivityId(ctx, eventPayload.UserId, eventPayload.ActivityId)
		if existingPipelineRun != nil && existingPipelineRun.Destinations != nil {
			for _, dest := range existingPipelineRun.Destinations {
				if dest.Destination == pb.Destination_DESTINATION_SHOWCASE && dest.ExternalId != nil && *dest.ExternalId != "" {
					showcaseID = *dest.ExternalId
					fwCtx.Logger.Info("Reusing existing showcase ID for update", "showcase_id", showcaseID)
					break
				}
			}
		}

		// Only generate new ID if we don't have an existing one
		if showcaseID == "" {
			var err error
			showcaseID, err = generateShowcaseID(ctx, svc, eventPayload.Name, startTime)
			if err != nil {
				destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_SHOWCASE, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("failed to generate ID: %s", err), eventPayload.Name, fwCtx.Logger)
				return nil, fmt.Errorf("failed to generate showcase ID: %w", err)
			}
		}

		// Calculate expiration
		createdAt := time.Now()
		expiresAt := calculateExpiration(user.Tier, createdAt)

		// Create the showcased activity document
		showcasedActivity := &pb.ShowcasedActivity{
			ShowcaseId:   showcaseID,
			ActivityId:   eventPayload.ActivityId,
			UserId:       eventPayload.UserId,
			Title:        eventPayload.Name,
			Description:  eventPayload.Description,
			ActivityType: eventPayload.ActivityType,
			Source:       eventPayload.Source,
			StartTime:    eventPayload.StartTime,
			// ActivityData is stored in GCS, not inline (avoids Firestore 1MB limit)
			ActivityData:        nil,
			FitFileUri:          eventPayload.FitFileUri,
			AppliedEnrichments:  eventPayload.AppliedEnrichments,
			EnrichmentMetadata:  eventPayload.EnrichmentMetadata,
			Tags:                eventPayload.Tags,
			PipelineExecutionId: eventPayload.PipelineExecutionId,
			CreatedAt:           timestamppb.New(createdAt),
			ExpiresAt:           nil, // Will be set below
		}

		// Store the enriched event URI directly (points to enriched_events/{userId}/{pipelineExecutionId}.json)
		// The enricher/router guarantees this file exists in GCS with the full EnrichedActivityEvent
		if eventPayload.ActivityDataUri != "" {
			showcasedActivity.ActivityDataUri = eventPayload.ActivityDataUri
			fwCtx.Logger.Info("Stored enriched event URI",
				"uri", eventPayload.ActivityDataUri,
				"showcase_id", showcaseID,
			)
		} else {
			fwCtx.Logger.Warn("No ActivityDataUri available - showcase will be missing activity data",
				"showcase_id", showcaseID,
				"activity_id", eventPayload.ActivityId,
			)
		}

		if expiresAt != nil {
			showcasedActivity.ExpiresAt = timestamppb.New(*expiresAt)
		}

		// Fetch user display name from Firebase Auth for public attribution
		if svc.Auth != nil {
			authUser, err := svc.Auth.GetUser(ctx, eventPayload.UserId)
			if err != nil {
				fwCtx.Logger.Warn("Failed to fetch user from Firebase Auth for display name", "error", err, "userId", eventPayload.UserId)
			} else if authUser != nil {
				if authUser.DisplayName != "" {
					showcasedActivity.OwnerDisplayName = authUser.DisplayName
					fwCtx.Logger.Info("Set owner display name from Firebase Auth DisplayName", "displayName", authUser.DisplayName, "userId", eventPayload.UserId)
				} else if authUser.Email != "" {
					// Fallback: use email prefix (part before @) as display name
					emailPrefix := authUser.Email
					if atIdx := strings.Index(authUser.Email, "@"); atIdx > 0 {
						emailPrefix = authUser.Email[:atIdx]
					}
					showcasedActivity.OwnerDisplayName = emailPrefix
					fwCtx.Logger.Info("Set owner display name from email prefix", "displayName", emailPrefix, "userId", eventPayload.UserId)
				} else {
					fwCtx.Logger.Warn("Firebase Auth user has no DisplayName or Email set", "userId", eventPayload.UserId)
				}
			}
		} else {
			fwCtx.Logger.Warn("svc.Auth is nil, cannot fetch display name from Firebase Auth", "userId", eventPayload.UserId)
		}

		// Persist to Firestore
		if err := svc.DB.SetShowcasedActivity(ctx, showcasedActivity); err != nil {
			fwCtx.Logger.Error("Failed to persist showcased activity", "error", err)
			destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_SHOWCASE, pb.DestinationStatus_DESTINATION_STATUS_FAILED, "", fmt.Sprintf("persist failed: %s", err), eventPayload.Name, fwCtx.Logger)
			return nil, fmt.Errorf("failed to persist showcased activity: %w", err)
		}

		// Note: synchronized_activities is deprecated - pipeline_runs is now the source of truth
		// The destination.UpdateStatus call at the end of this function updates pipeline_runs with the externalId

		// Increment sync count for billing (per successful destination sync)
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		fwCtx.Logger.Info("Showcase upload complete",
			"activity_id", eventPayload.ActivityId,
			"showcase_id", showcaseID,
			"expires_at", expiresAt,
		)

		// Update PipelineRun destination as synced
		destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId, pb.Destination_DESTINATION_SHOWCASE, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS, showcaseID, "", eventPayload.Name, fwCtx.Logger)

		return map[string]interface{}{
			"status":        "SUCCESS",
			"showcase_id":   showcaseID,
			"activity_id":   eventPayload.ActivityId,
			"pipeline_id":   eventPayload.PipelineId,
			"activity_name": eventPayload.Name,
			"activity_type": eventPayload.ActivityType.String(),
			"description":   eventPayload.Description,
		}, nil
	}
}
