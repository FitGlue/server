package ai_banner

import (
	"context"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAIBanner_Name(t *testing.T) {
	provider := NewAIBannerProvider()
	expected := "ai-banner"
	if provider.Name() != expected {
		t.Errorf("Expected provider name %q, got %q", expected, provider.Name())
	}
}

func TestAIBanner_ProviderType(t *testing.T) {
	provider := NewAIBannerProvider()
	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_AI_BANNER
	if provider.ProviderType() != expected {
		t.Errorf("Expected provider type %v, got %v", expected, provider.ProviderType())
	}
}

func TestAIBanner_Enrich_TierRestriction(t *testing.T) {
	provider := NewAIBannerProvider()
	provider.Service = &bootstrap.Service{}

	// Create activity
	activity := &pb.StandardizedActivity{
		ExternalId: "test-activity-123",
		StartTime:  timestamppb.New(time.Now()),
		Type:       pb.ActivityType_ACTIVITY_TYPE_RUN,
	}

	// Hobbyist tier user (should be skipped)
	user := &pb.UserRecord{
		UserId: "test-user",
		Tier:   pb.UserTier_USER_TIER_HOBBYIST,
	}

	result, err := provider.Enrich(context.Background(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify tier restriction
	if result.Metadata["status"] != "skipped" {
		t.Errorf("Expected status=skipped, got %s", result.Metadata["status"])
	}
	if result.Metadata["reason"] != "tier_restricted" {
		t.Errorf("Expected reason=tier_restricted, got %s", result.Metadata["reason"])
	}
}

func TestAIBanner_Enrich_NoActivityID(t *testing.T) {
	provider := NewAIBannerProvider()
	provider.Service = &bootstrap.Service{}

	// Create activity without ID
	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Type:      pb.ActivityType_ACTIVITY_TYPE_RUN,
	}

	// Athlete tier user
	user := &pb.UserRecord{
		UserId: "test-user",
		Tier:   pb.UserTier_USER_TIER_ATHLETE,
	}

	result, err := provider.Enrich(context.Background(), activity, user, nil, false)
	if err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify skipped due to no activity ID
	if result.Metadata["status"] != "skipped" {
		t.Errorf("Expected status=skipped, got %s", result.Metadata["status"])
	}
	if result.Metadata["reason"] != "no_activity_id" {
		t.Errorf("Expected reason=no_activity_id, got %s", result.Metadata["reason"])
	}
}

func TestBuildImagePrompt(t *testing.T) {
	tests := []struct {
		name     string
		activity *pb.StandardizedActivity
		style    string
		contains []string
	}{
		{
			name: "morning run vibrant",
			activity: &pb.StandardizedActivity{
				Type:      pb.ActivityType_ACTIVITY_TYPE_RUN,
				StartTime: timestamppb.New(time.Date(2026, 1, 21, 7, 30, 0, 0, time.UTC)),
			},
			style: "vibrant",
			contains: []string{
				"banner image",
				"run",
				"early morning",
				"vibrant",
			},
		},
		{
			name: "afternoon ride minimal",
			activity: &pb.StandardizedActivity{
				Type:      pb.ActivityType_ACTIVITY_TYPE_RIDE,
				StartTime: timestamppb.New(time.Date(2026, 1, 21, 14, 0, 0, 0, time.UTC)),
			},
			style: "minimal",
			contains: []string{
				"banner image",
				"ride",
				"afternoon",
				"minimalist",
			},
		},
		{
			name: "evening strength dramatic",
			activity: &pb.StandardizedActivity{
				Type:      pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
				StartTime: timestamppb.New(time.Date(2026, 1, 21, 18, 30, 0, 0, time.UTC)),
			},
			style: "dramatic",
			contains: []string{
				"banner image",
				"weight training",
				"evening",
				"dramatic",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := buildImagePrompt(tt.activity, tt.style)
			for _, contains := range tt.contains {
				if !containsIgnoreCase(prompt, contains) {
					t.Errorf("Expected prompt to contain %q, got: %s", contains, prompt)
				}
			}
		})
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && containsIgnoreCase(s[1:], substr) ||
			(len(s) >= len(substr) && equalFoldPrefix(s, substr)))
}

func equalFoldPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c1 := s[i]
		c2 := prefix[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}
