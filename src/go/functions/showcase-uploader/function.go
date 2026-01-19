package showcaseuploader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/framework"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

// Tier constants for expiration calculation
const (
	TierFree = "free"
	TierPro  = "pro"

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
			slog.Error("Failed to initialize service", "error", err)
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

// calculateExpiration determines when a showcase activity should expire based on user tier
func calculateExpiration(tier string, createdAt time.Time) *time.Time {
	if tier == TierPro {
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
			fwCtx.Logger.Warn("Failed to get user for tier lookup, defaulting to free tier", "error", err, "userId", eventPayload.UserId)
			user = &pb.UserRecord{Tier: TierFree}
		}

		// Determine start time
		var startTime time.Time
		if eventPayload.StartTime != nil {
			startTime = eventPayload.StartTime.AsTime()
		} else {
			startTime = time.Now()
		}

		// Generate human-readable showcase ID
		showcaseID, err := generateShowcaseID(ctx, svc, eventPayload.Name, startTime)
		if err != nil {
			return nil, fmt.Errorf("failed to generate showcase ID: %w", err)
		}

		// Calculate expiration
		createdAt := time.Now()
		expiration := calculateExpiration(user.Tier, createdAt)

		// Create the showcased activity document
		showcasedActivity := &pb.ShowcasedActivity{
			ShowcaseId:          showcaseID,
			ActivityId:          eventPayload.ActivityId,
			UserId:              eventPayload.UserId,
			Title:               eventPayload.Name,
			Description:         eventPayload.Description,
			ActivityType:        eventPayload.ActivityType,
			Source:              eventPayload.Source,
			StartTime:           eventPayload.StartTime,
			ActivityData:        eventPayload.ActivityData,
			FitFileUri:          eventPayload.FitFileUri,
			AppliedEnrichments:  eventPayload.AppliedEnrichments,
			EnrichmentMetadata:  eventPayload.EnrichmentMetadata,
			Tags:                eventPayload.Tags,
			PipelineExecutionId: eventPayload.PipelineExecutionId,
			CreatedAt:           timestamppb.New(createdAt),
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

		if expiration != nil {
			showcasedActivity.ExpiresAt = timestamppb.New(*expiration)
		}

		// Persist to Firestore
		if err := svc.DB.SetShowcasedActivity(ctx, showcasedActivity); err != nil {
			fwCtx.Logger.Error("Failed to persist showcased activity", "error", err)
			return nil, fmt.Errorf("failed to persist showcased activity: %w", err)
		}

		// Persist SynchronizedActivity record
		syncedActivity := &pb.SynchronizedActivity{
			ActivityId:          eventPayload.ActivityId,
			Title:               eventPayload.Name,
			Description:         eventPayload.Description,
			Type:                eventPayload.ActivityType,
			Source:              eventPayload.Source.String(),
			StartTime:           eventPayload.StartTime,
			SyncedAt:            timestamppb.Now(),
			PipelineId:          eventPayload.PipelineId,
			PipelineExecutionId: fwCtx.PipelineExecutionId,
			Destinations: map[string]string{
				"showcase": showcaseID,
			},
		}

		if err := svc.DB.SetSynchronizedActivity(ctx, eventPayload.UserId, syncedActivity); err != nil {
			fwCtx.Logger.Error("Failed to persist synchronized activity", "error", err)
			return nil, fmt.Errorf("failed to persist synchronized activity: %w", err)
		}

		// Increment sync count for billing (per successful destination sync)
		if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
			fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
		}

		fwCtx.Logger.Info("Showcase upload complete",
			"activity_id", eventPayload.ActivityId,
			"showcase_id", showcaseID,
			"expires_at", expiration,
		)

		return map[string]interface{}{
			"status":      "SUCCESS",
			"showcase_id": showcaseID,
			"activity_id": eventPayload.ActivityId,
			"pipeline_id": eventPayload.PipelineId,
		}, nil
	}
}
