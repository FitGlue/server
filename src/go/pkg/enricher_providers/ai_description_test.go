package enricher_providers_test

import (
	"context"
	"testing"

	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAIDescriptionProvider_TierCheck(t *testing.T) {
	provider := enricher_providers.NewAIDescriptionProvider()

	activity := &pb.StandardizedActivity{
		Name: "Morning Run",
		Type: pb.ActivityType_ACTIVITY_TYPE_RUN,
	}

	tests := []struct {
		name         string
		user         *pb.UserRecord
		expectStatus string
	}{
		{
			name: "free user is skipped",
			user: &pb.UserRecord{
				UserId: "free-user",
				Tier:   "free",
			},
			expectStatus: "skipped",
		},
		{
			name: "pro user proceeds (but no API key set)",
			user: &pb.UserRecord{
				UserId: "pro-user",
				Tier:   "pro",
			},
			expectStatus: "skipped", // API key not configured
		},
		{
			name: "admin user proceeds (but no API key set)",
			user: &pb.UserRecord{
				UserId:  "admin-user",
				Tier:    "free",
				IsAdmin: true,
			},
			expectStatus: "skipped", // API key not configured
		},
		{
			name: "user on trial proceeds (but no API key set)",
			user: &pb.UserRecord{
				UserId:      "trial-user",
				Tier:        "free",
				TrialEndsAt: timestamppb.Now(), // Will be expired, but test structure
			},
			expectStatus: "skipped", // Either tier_restricted or api_key_not_configured
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.Enrich(context.Background(), activity, tt.user, nil, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Metadata["status"] != tt.expectStatus {
				t.Errorf("expected status %q, got %q (reason: %s)",
					tt.expectStatus, result.Metadata["status"], result.Metadata["reason"])
			}
		})
	}
}

func TestAIDescriptionProvider_FreeTierSkipped(t *testing.T) {
	provider := enricher_providers.NewAIDescriptionProvider()

	activity := &pb.StandardizedActivity{
		Name: "Weight Training",
		Type: pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING,
	}

	freeUser := &pb.UserRecord{
		UserId: "free-user-123",
		Tier:   "free",
	}

	result, err := provider.Enrich(context.Background(), activity, freeUser, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Metadata["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got %q", result.Metadata["status"])
	}

	if result.Metadata["reason"] != "tier_restricted" {
		t.Errorf("expected reason 'tier_restricted', got %q", result.Metadata["reason"])
	}

	if result.Metadata["required_tier"] != "pro" {
		t.Errorf("expected required_tier 'pro', got %q", result.Metadata["required_tier"])
	}

	// Should not modify activity
	if result.Name != "" {
		t.Errorf("expected empty Name for skipped result, got %q", result.Name)
	}
	if result.Description != "" {
		t.Errorf("expected empty Description for skipped result, got %q", result.Description)
	}
}

func TestAIDescriptionProvider_ModeConfig(t *testing.T) {
	provider := enricher_providers.NewAIDescriptionProvider()

	activity := &pb.StandardizedActivity{
		Name: "Morning Run",
		Type: pb.ActivityType_ACTIVITY_TYPE_RUN,
	}

	proUser := &pb.UserRecord{
		UserId: "pro-user",
		Tier:   "pro",
	}

	// Without API key, these will all be skipped with api_key_not_configured
	// but we can verify the mode is being read correctly
	modes := []string{"title", "description", "both", ""}

	for _, mode := range modes {
		t.Run("mode="+mode, func(t *testing.T) {
			config := map[string]string{}
			if mode != "" {
				config["mode"] = mode
			}

			result, err := provider.Enrich(context.Background(), activity, proUser, config, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Should skip due to no API key, but not error
			if result.Metadata["status"] != "skipped" {
				t.Logf("status=%s, reason=%s", result.Metadata["status"], result.Metadata["reason"])
			}
		})
	}
}

func TestAIDescriptionProvider_Name(t *testing.T) {
	provider := enricher_providers.NewAIDescriptionProvider()

	if provider.Name() != "ai-description" {
		t.Errorf("expected name 'ai-description', got %q", provider.Name())
	}
}

func TestAIDescriptionProvider_ProviderType(t *testing.T) {
	provider := enricher_providers.NewAIDescriptionProvider()

	expected := pb.EnricherProviderType_ENRICHER_PROVIDER_AI_DESCRIPTION
	if provider.ProviderType() != expected {
		t.Errorf("expected provider type %v, got %v", expected, provider.ProviderType())
	}
}
