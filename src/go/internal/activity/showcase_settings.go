package activity

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbsvc "github.com/fitglue/server/src/go/pkg/types/pb/services/activity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Ensure unused imports are referenced
var _ = time.Minute

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,38}[a-z0-9]$`)

// GetShowcaseSettings returns the user's showcase profile settings along with their showcased activities.
func (s *Service) GetShowcaseSettings(ctx context.Context, req *pbsvc.GetShowcaseSettingsRequest) (*pbsvc.GetShowcaseSettingsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	profile, err := s.store.GetShowcasePreferences(ctx, req.UserId)
	if err != nil {
		s.logger.Error(ctx, "failed to get showcase profile", "error", err)
		return nil, status.Error(codes.Internal, "failed to get showcase settings")
	}

	if profile == nil {
		profile = &pbactivity.ShowcaseProfile{
			UserId: req.UserId,
		}
	}

	// Fetch showcased activities to build the entries list
	showcases, err := s.store.ListShowcases(ctx, req.UserId)
	if err != nil {
		s.logger.Error(ctx, "failed to list showcases for settings", "error", err)
		return nil, status.Error(codes.Internal, "failed to list showcases")
	}

	// Build ShowcaseActivityEntry list with in_profile flags
	profileEntryIDs := make(map[string]bool)
	for _, entry := range profile.Entries {
		profileEntryIDs[entry.ShowcaseId] = true
	}

	var activities []*pbsvc.ShowcaseActivityEntry
	for _, sc := range showcases {
		activities = append(activities, &pbsvc.ShowcaseActivityEntry{
			ShowcaseId:   sc.ShowcaseId,
			Title:        sc.Title,
			ActivityType: sc.ActivityType.String(),
			Source:       sc.Source.String(),
			InProfile:    profileEntryIDs[sc.ShowcaseId],
		})
	}

	return &pbsvc.GetShowcaseSettingsResponse{
		Profile:    profile,
		Activities: activities,
	}, nil
}

// UpdateShowcaseSettings updates the user's showcase profile settings (display name, bio, theme, etc.)
func (s *Service) UpdateShowcaseSettings(ctx context.Context, req *pbsvc.UpdateShowcaseSettingsRequest) (*pbactivity.ShowcaseProfile, error) {
	if req.UserId == "" || req.Settings == nil {
		return nil, status.Error(codes.InvalidArgument, "user_id and settings are required")
	}

	// Ensure the user ID is set on the settings
	req.Settings.UserId = req.UserId

	updated, err := s.store.UpdateShowcasePreferences(ctx, req.UserId, req.Settings)
	if err != nil {
		s.logger.Error(ctx, "failed to update showcase settings", "error", err)
		return nil, status.Error(codes.Internal, "failed to update showcase settings")
	}

	return updated, nil
}

// UpdateShowcaseSlug updates the user's showcase profile slug (URL-friendly unique identifier).
func (s *Service) UpdateShowcaseSlug(ctx context.Context, req *pbsvc.UpdateShowcaseSlugRequest) (*pbsvc.UpdateShowcaseSlugResponse, error) {
	if req.UserId == "" || req.Slug == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and slug are required")
	}

	// Normalize slug
	slug := strings.ToLower(strings.TrimSpace(req.Slug))

	// Validate slug format
	if !slugRegex.MatchString(slug) {
		return nil, status.Error(codes.InvalidArgument, "slug must be 4-40 characters, lowercase alphanumeric and hyphens only, cannot start or end with a hyphen")
	}

	if err := s.store.UpdateShowcaseSlug(ctx, req.UserId, slug); err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return nil, err
		}
		s.logger.Error(ctx, "failed to update showcase slug", "error", err)
		return nil, status.Error(codes.Internal, "failed to update showcase slug")
	}

	return &pbsvc.UpdateShowcaseSlugResponse{
		Slug: slug,
	}, nil
}

// AddShowcaseEntry adds a showcased activity to the user's showcase profile.
func (s *Service) AddShowcaseEntry(ctx context.Context, req *pbsvc.AddShowcaseEntryRequest) (*emptypb.Empty, error) {
	if req.UserId == "" || req.ShowcaseId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and showcase_id are required")
	}

	// Verify the showcase exists and belongs to the user
	showcase, err := s.store.GetShowcase(ctx, req.UserId, req.ShowcaseId)
	if err != nil {
		s.logger.Error(ctx, "failed to verify showcase ownership", "error", err)
		return nil, status.Error(codes.Internal, "failed to verify showcase")
	}
	if showcase == nil {
		return nil, status.Error(codes.NotFound, "showcase not found")
	}

	// Get current profile
	profile, err := s.store.GetShowcasePreferences(ctx, req.UserId)
	if err != nil {
		s.logger.Error(ctx, "failed to get showcase profile", "error", err)
		return nil, status.Error(codes.Internal, "failed to read profile")
	}

	if profile == nil {
		profile = &pbactivity.ShowcaseProfile{
			UserId: req.UserId,
		}
	}

	// Check if already in entries
	for _, entry := range profile.Entries {
		if entry.ShowcaseId == req.ShowcaseId {
			return &emptypb.Empty{}, nil // Already added, idempotent
		}
	}

	// Build the new entry from the showcase metadata
	newEntry := &pbactivity.ShowcaseProfileEntry{
		ShowcaseId:   req.ShowcaseId,
		Title:        showcase.Title,
		ActivityType: showcase.ActivityType,
		Source:       showcase.Source,
		StartTime:    showcase.StartTime,
	}

	profile.Entries = append(profile.Entries, newEntry)

	if _, err := s.store.UpdateShowcasePreferences(ctx, req.UserId, profile); err != nil {
		s.logger.Error(ctx, "failed to add showcase entry", "error", err)
		return nil, status.Error(codes.Internal, "failed to add entry")
	}

	return &emptypb.Empty{}, nil
}

// RemoveShowcaseEntry removes a showcased activity from the user's showcase profile.
func (s *Service) RemoveShowcaseEntry(ctx context.Context, req *pbsvc.RemoveShowcaseEntryRequest) (*emptypb.Empty, error) {
	if req.UserId == "" || req.ShowcaseId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and showcase_id are required")
	}

	// Get current profile
	profile, err := s.store.GetShowcasePreferences(ctx, req.UserId)
	if err != nil {
		s.logger.Error(ctx, "failed to get showcase profile", "error", err)
		return nil, status.Error(codes.Internal, "failed to read profile")
	}

	if profile == nil {
		return &emptypb.Empty{}, nil // No profile, nothing to remove
	}

	// Filter out the entry
	var filtered []*pbactivity.ShowcaseProfileEntry
	for _, entry := range profile.Entries {
		if entry.ShowcaseId != req.ShowcaseId {
			filtered = append(filtered, entry)
		}
	}

	profile.Entries = filtered

	if _, err := s.store.UpdateShowcasePreferences(ctx, req.UserId, profile); err != nil {
		s.logger.Error(ctx, "failed to remove showcase entry", "error", err)
		return nil, status.Error(codes.Internal, "failed to remove entry")
	}

	return &emptypb.Empty{}, nil
}

// GetShowcaseProfilePictureUploadUrl generates a signed URL for uploading a showcase profile picture.
func (s *Service) GetShowcaseProfilePictureUploadUrl(ctx context.Context, req *pbsvc.GetShowcaseProfilePictureUploadUrlRequest) (*pbsvc.GetShowcaseProfilePictureUploadUrlResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Validate content type
	validTypes := map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/webp": "webp",
	}
	ext, ok := validTypes[contentType]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "content_type must be image/jpeg, image/png, or image/webp")
	}

	path := fmt.Sprintf("showcase_pictures/%s/profile.%s", req.UserId, ext)
	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", s.bucketName, path)

	uploadURL, err := s.blobStore.SignedURL(ctx, s.bucketName, path, contentType, 5*1024*1024, 15*time.Minute)
	if err != nil {
		s.logger.Error(ctx, "failed to generate signed upload URL", "error", err)
		return nil, status.Error(codes.Internal, "failed to generate upload URL")
	}

	return &pbsvc.GetShowcaseProfilePictureUploadUrlResponse{
		UploadUrl:    uploadURL,
		PublicUrl:    publicURL,
		ContentType:  contentType,
		MaxSizeBytes: 5 * 1024 * 1024, // 5MB limit
	}, nil
}

// GetPublicShowcaseProfile returns a public showcase profile page by slug, with paginated activities.
func (s *Service) GetPublicShowcaseProfile(ctx context.Context, req *pbsvc.GetPublicShowcaseProfileRequest) (*pbsvc.GetPublicShowcaseProfileResponse, error) {
	if req.Slug == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}

	profile, err := s.store.GetShowcaseProfileBySlug(ctx, req.Slug)
	if err != nil {
		s.logger.Error(ctx, "failed to get showcase profile by slug", "error", err)
		return nil, status.Error(codes.Internal, "failed to read showcase profile")
	}
	if profile == nil {
		return nil, status.Error(codes.NotFound, "showcase profile not found")
	}

	// Check visibility
	if !profile.Visible {
		return nil, status.Error(codes.NotFound, "showcase profile not found")
	}

	// Paginated showcased activities
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := int32(12)
	offset := (page - 1) * pageSize

	activities, _, err := s.store.ListShowcasedActivitiesByUser(ctx, profile.UserId, pageSize, offset)
	if err != nil {
		s.logger.Error(ctx, "failed to list showcased activities for profile", "error", err)
		return nil, status.Error(codes.Internal, "failed to list activities")
	}

	// Calculate total pages
	totalShowcases, _ := s.store.CountShowcasedActivities(ctx, profile.UserId)
	totalPages := (totalShowcases + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	return &pbsvc.GetPublicShowcaseProfileResponse{
		Profile:     profile,
		Showcases:   activities,
		TotalPages:  totalPages,
		CurrentPage: page,
	}, nil
}

// GetActivityStats returns aggregated activity statistics for a user (pipeline run counts, showcase counts).
func (s *Service) GetActivityStats(ctx context.Context, req *pbsvc.GetActivityStatsRequest) (*pbsvc.GetActivityStatsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Count total pipeline runs (all statuses)
	totalActivities, err := s.store.CountPipelineRunsByStatus(ctx, req.UserId, "")
	if err != nil {
		s.logger.Error(ctx, "failed to count total pipeline runs", "error", err)
		totalActivities = 0
	}

	// Count showcased activities
	totalShowcases, err := s.store.CountShowcasedActivities(ctx, req.UserId)
	if err != nil {
		s.logger.Error(ctx, "failed to count showcased activities", "error", err)
		totalShowcases = 0
	}

	return &pbsvc.GetActivityStatsResponse{
		TotalActivities: totalActivities,
		TotalShowcases:  totalShowcases,
	}, nil
}
