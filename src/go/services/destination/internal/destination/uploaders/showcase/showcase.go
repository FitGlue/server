package showcase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/domain/tier"
	"github.com/fitglue/server/src/go/pkg/domain/user"
	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	HobbyistRetentionDays = 30
)

// Uploader implements destination.Destination for Showcase
type Uploader struct {
	svc *bootstrap.Service
}

// New returns a new Showcase Uploader initialized with dependencies.
func New(svc *bootstrap.Service) *Uploader {
	return &Uploader{
		svc: svc,
	}
}

// Name returns the identifier for this uploader
func (u *Uploader) Name() string {
	return "showcase"
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	s = reg.ReplaceAllString(s, "")
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 30 {
		s = s[:30]
		s = strings.TrimRight(s, "-")
	}
	return s
}

func generateRandomSuffix() string {
	b := make([]byte, 3)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (u *Uploader) generateShowcaseID(ctx context.Context, title string, startTime time.Time) (string, error) {
	slug := slugify(title)
	if slug == "" {
		slug = "activity"
	}

	dateStr := startTime.Format("2006-01-02")
	baseID := fmt.Sprintf("%s-%s", slug, dateStr)

	exists, err := u.svc.DB.ShowcaseActivityExists(ctx, baseID)
	if err != nil {
		return "", fmt.Errorf("failed to check showcase ID existence: %w", err)
	}

	if !exists {
		return baseID, nil
	}

	for i := 0; i < 5; i++ {
		suffix := generateRandomSuffix()
		candidateID := fmt.Sprintf("%s-%s", baseID, suffix)

		exists, err = u.svc.DB.ShowcaseActivityExists(ctx, candidateID)
		if err != nil {
			return "", fmt.Errorf("failed to check showcase ID existence: %w", err)
		}

		if !exists {
			return candidateID, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique showcase ID after 5 attempts")
}

func calculateExpiration(userRec *user.Record, createdAt time.Time) *time.Time {
	if tier.GetEffectiveTier(userRec) == tier.TierAthlete {
		return nil
	}
	expiry := createdAt.AddDate(0, 0, HobbyistRetentionDays)
	return &expiry
}

// Create uploads a new activity to Showcase
func (u *Uploader) Create(ctx context.Context, payload *pbevents.ActivityPayload, userRec *user.Record) (string, error) {
	logger := slog.Default()

	var startTime time.Time
	if payload.Timestamp != nil {
		startTime = payload.Timestamp.AsTime()
	} else {
		startTime = time.Now()
	}

	activityName := payload.Metadata["activity_name"]
	if activityName == "" {
		activityName = "Activity"
	}

	showcaseID, err := u.generateShowcaseID(ctx, activityName, startTime)
	if err != nil {
		return "", fmt.Errorf("failed to generate showcase ID: %w", err)
	}

	createdAt := time.Now()
	expiresAt := calculateExpiration(userRec, createdAt)

	activityTypeVal, ok := pbactivity.ActivityType_value[payload.Metadata["activity_type"]]
	activityType := pbactivity.ActivityType_ACTIVITY_TYPE_UNSPECIFIED
	if ok {
		activityType = pbactivity.ActivityType(activityTypeVal)
	}

	appliedEnrichmentsStr := payload.Metadata["applied_enrichments"]
	var appliedEnrichments []string
	if appliedEnrichmentsStr != "" {
		appliedEnrichments = strings.Split(appliedEnrichmentsStr, ",")
	}

	tagsStr := payload.Metadata["tags"]
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	showcasedActivity := &pbactivity.ShowcasedActivity{
		ShowcaseId:          showcaseID,
		ActivityId:          payload.GetActivityId(),
		UserId:              payload.UserId,
		Title:               activityName,
		Description:         payload.Metadata["description"],
		ActivityType:        activityType,
		Source:              payload.Source,
		StartTime:           payload.Timestamp,
		ActivityData:        nil,
		FitFileUri:          payload.Metadata["fit_file_uri"],
		AppliedEnrichments:  appliedEnrichments,
		EnrichmentMetadata:  payload.Metadata,
		Tags:                tags,
		PipelineExecutionId: payload.PipelineExecutionId,
		CreatedAt:           timestamppb.New(createdAt),
		ExpiresAt:           nil,
	}

	if uri, ok := payload.Metadata["activity_data_uri"]; ok && uri != "" {
		showcasedActivity.ActivityDataUri = uri
	} else {
		logger.Warn("No ActivityDataUri available - showcase will be missing activity data",
			"showcase_id", showcaseID,
			"activity_id", payload.ActivityId,
		)
	}

	if expiresAt != nil {
		showcasedActivity.ExpiresAt = timestamppb.New(*expiresAt)
	}

	if u.svc.Auth != nil {
		authUser, err := u.svc.Auth.GetUser(ctx, payload.UserId)
		if err != nil {
			logger.Warn("Failed to fetch user from Firebase Auth for display name", "error", err, "userId", payload.UserId)
		} else if authUser != nil {
			if authUser.DisplayName != "" {
				showcasedActivity.OwnerDisplayName = authUser.DisplayName
			} else if authUser.Email != "" {
				emailPrefix := authUser.Email
				if atIdx := strings.Index(authUser.Email, "@"); atIdx > 0 {
					emailPrefix = authUser.Email[:atIdx]
				}
				showcasedActivity.OwnerDisplayName = emailPrefix
			}
		}
	}

	if err := u.svc.DB.SetShowcasedActivity(ctx, showcasedActivity); err != nil {
		return "", fmt.Errorf("failed to persist showcased activity: %w", err)
	}

	if tier.GetEffectiveTier(userRec) == tier.TierAthlete {
		if err := u.updateShowcaseProfile(ctx, payload, userRec, showcasedActivity, createdAt, logger); err != nil {
			logger.Warn("Failed to update showcase profile", "error", err)
		}
	}

	_ = u.svc.DB.IncrementSyncCount(ctx, payload.UserId)

	return showcaseID, nil
}

// Update modifies an existing Showcase activity and profile entry
func (u *Uploader) Update(ctx context.Context, payload *pbevents.ActivityPayload, userRec *user.Record, pipelineRun *pbpipeline.PipelineRun) error {
	logger := slog.Default()

	var showcaseID string
	if pipelineRun != nil {
		for _, dest := range pipelineRun.Destinations {
			if dest.Destination == pbplugin.DestinationType_DESTINATION_SHOWCASE && dest.ExternalId != nil && *dest.ExternalId != "" {
				showcaseID = *dest.ExternalId
				break
			}
		}
	}

	if showcaseID == "" {
		return fmt.Errorf("no Showcase destination found in pipeline run")
	}

	createdAt := time.Now()
	expiresAt := calculateExpiration(userRec, createdAt)

	activityName := payload.Metadata["activity_name"]
	if activityName == "" {
		activityName = "Activity"
	}

	activityTypeVal, ok := pbactivity.ActivityType_value[payload.Metadata["activity_type"]]
	activityType := pbactivity.ActivityType_ACTIVITY_TYPE_UNSPECIFIED
	if ok {
		activityType = pbactivity.ActivityType(activityTypeVal)
	}

	appliedEnrichmentsStr := payload.Metadata["applied_enrichments"]
	var appliedEnrichments []string
	if appliedEnrichmentsStr != "" {
		appliedEnrichments = strings.Split(appliedEnrichmentsStr, ",")
	}

	tagsStr := payload.Metadata["tags"]
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	showcasedActivity := &pbactivity.ShowcasedActivity{
		ShowcaseId:          showcaseID,
		ActivityId:          payload.GetActivityId(),
		UserId:              payload.UserId,
		Title:               activityName,
		Description:         payload.Metadata["description"],
		ActivityType:        activityType,
		Source:              payload.Source,
		StartTime:           payload.Timestamp,
		ActivityData:        nil,
		FitFileUri:          payload.Metadata["fit_file_uri"],
		AppliedEnrichments:  appliedEnrichments,
		EnrichmentMetadata:  payload.Metadata,
		Tags:                tags,
		PipelineExecutionId: payload.PipelineExecutionId,
		CreatedAt:           timestamppb.New(createdAt),
	}

	if uri, ok := payload.Metadata["activity_data_uri"]; ok && uri != "" {
		showcasedActivity.ActivityDataUri = uri
	}

	if expiresAt != nil {
		showcasedActivity.ExpiresAt = timestamppb.New(*expiresAt)
	}

	if u.svc.Auth != nil {
		authUser, err := u.svc.Auth.GetUser(ctx, payload.UserId)
		if err == nil && authUser != nil {
			if authUser.DisplayName != "" {
				showcasedActivity.OwnerDisplayName = authUser.DisplayName
			} else if authUser.Email != "" {
				emailPrefix := authUser.Email
				if atIdx := strings.Index(authUser.Email, "@"); atIdx > 0 {
					emailPrefix = authUser.Email[:atIdx]
				}
				showcasedActivity.OwnerDisplayName = emailPrefix
			}
		}
	}

	if err := u.svc.DB.SetShowcasedActivity(ctx, showcasedActivity); err != nil {
		return fmt.Errorf("failed to persist updated showcased activity: %w", err)
	}

	if tier.GetEffectiveTier(userRec) == tier.TierAthlete {
		if err := u.updateShowcaseProfile(ctx, payload, userRec, showcasedActivity, createdAt, logger); err != nil {
			logger.Warn("Failed to update showcase profile", "error", err)
		}
	}

	_ = u.svc.DB.IncrementSyncCount(ctx, payload.UserId)

	return nil
}

func (u *Uploader) updateShowcaseProfile(ctx context.Context, payload *pbevents.ActivityPayload, userRec *user.Record, showcasedActivity *pbactivity.ShowcasedActivity, actionTime time.Time, logger *slog.Logger) error {
	existingProfile, _ := u.svc.DB.GetShowcaseProfileByUserId(ctx, payload.UserId)
	profileSlug := ""
	if existingProfile != nil && existingProfile.Slug != "" {
		profileSlug = existingProfile.Slug
	} else {
		profileSlug = slugify(showcasedActivity.OwnerDisplayName)
	}

	if profileSlug == "" {
		return nil
	}

	var activityDistance float64
	var activityDuration float64
	var activitySets int32
	var activityReps int32
	var activityWeightKg float64

	if payload.StandardizedActivity != nil {
		for _, session := range payload.StandardizedActivity.Sessions {
			activityDistance += session.TotalDistance
			activityDuration += float64(session.TotalElapsedTime)
			for _, set := range session.StrengthSets {
				activitySets++
				activityReps += set.Reps
				activityWeightKg += set.WeightKg * float64(set.Reps)
			}
		}
	}

	entry := &pbactivity.ShowcaseProfileEntry{
		ShowcaseId:      showcasedActivity.ShowcaseId,
		Title:           showcasedActivity.Title,
		ActivityType:    showcasedActivity.ActivityType,
		Source:          showcasedActivity.Source,
		StartTime:       showcasedActivity.StartTime,
		DistanceMeters:  activityDistance,
		DurationSeconds: activityDuration,
		TotalSets:       activitySets,
		TotalReps:       activityReps,
		TotalWeightKg:   activityWeightKg,
	}

	if thumb, ok := payload.Metadata["route_thumbnail_url"]; ok {
		entry.RouteThumbnailUrl = thumb
	}

	// Write entry to the sub-collection
	if err := u.svc.DB.SetShowcaseProfileEntry(ctx, payload.UserId, entry); err != nil {
		return fmt.Errorf("failed to save profile entry: %w", err)
	}

	// Build/update the profile with delta stats (entry is idempotent via MergeAll,
	// so for re-runs we accept a small drift in stats — acceptable trade-off)
	var profile *pbactivity.ShowcaseProfile
	if existingProfile != nil {
		profile = existingProfile
		profile.DisplayName = showcasedActivity.OwnerDisplayName
	} else {
		profile = &pbactivity.ShowcaseProfile{
			Slug:        profileSlug,
			UserId:      payload.UserId,
			DisplayName: showcasedActivity.OwnerDisplayName,
			CreatedAt:   timestamppb.New(actionTime),
		}
	}

	profile.TotalActivities++
	profile.TotalDistanceMeters += activityDistance
	profile.TotalDurationSeconds += activityDuration
	profile.TotalSets += activitySets
	profile.TotalReps += activityReps
	profile.TotalWeightKg += activityWeightKg

	if entry.StartTime != nil && (profile.LatestActivityAt == nil || entry.StartTime.AsTime().After(profile.LatestActivityAt.AsTime())) {
		profile.LatestActivityAt = entry.StartTime
	}
	profile.UpdatedAt = timestamppb.New(actionTime)

	if err := u.svc.DB.SetShowcaseProfile(ctx, profile); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	return nil
}
