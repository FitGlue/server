package tier

import (
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGetEffectiveTier(t *testing.T) {
	tests := []struct {
		name     string
		user     *pb.UserRecord
		expected EffectiveTier
	}{
		{
			name: "Admin gets Athlete",
			user: &pb.UserRecord{
				IsAdmin: true,
				Tier:    pb.UserTier_USER_TIER_HOBBYIST,
			},
			expected: TierAthlete,
		},
		{
			name: "Active trial gets Athlete",
			user: &pb.UserRecord{
				TrialEndsAt: timestamppb.New(time.Now().Add(time.Hour)),
				Tier:        pb.UserTier_USER_TIER_HOBBYIST,
			},
			expected: TierAthlete,
		},
		{
			name: "Stored hobbyist tier gets Hobbyist",
			user: &pb.UserRecord{
				Tier: pb.UserTier_USER_TIER_HOBBYIST,
			},
			expected: TierHobbyist,
		},
		{
			name: "Unspecified tier gets Hobbyist",
			user: &pb.UserRecord{
				Tier: pb.UserTier_USER_TIER_UNSPECIFIED,
			},
			expected: TierHobbyist,
		},
		{
			name: "Stored athlete tier gets Athlete",
			user: &pb.UserRecord{
				Tier: pb.UserTier_USER_TIER_ATHLETE,
			},
			expected: TierAthlete,
		},
		{
			name: "Expired trial with hobbyist tier gets Hobbyist",
			user: &pb.UserRecord{
				TrialEndsAt: timestamppb.New(time.Now().Add(-time.Hour)),
				Tier:        pb.UserTier_USER_TIER_HOBBYIST,
			},
			expected: TierHobbyist,
		},
		{
			name: "Expired trial with athlete tier gets Athlete",
			user: &pb.UserRecord{
				TrialEndsAt: timestamppb.New(time.Now().Add(-time.Hour)),
				Tier:        pb.UserTier_USER_TIER_ATHLETE,
			},
			expected: TierAthlete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetEffectiveTier(tt.user); got != tt.expected {
				t.Errorf("%s: GetEffectiveTier() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestCanSync(t *testing.T) {
	tests := []struct {
		name    string
		user    *pb.UserRecord
		allowed bool
	}{
		{
			name: "Athlete can always sync",
			user: &pb.UserRecord{
				Tier:               pb.UserTier_USER_TIER_ATHLETE,
				SyncCountThisMonth: 1000,
			},
			allowed: true,
		},
		{
			name: "Hobbyist under limit can sync",
			user: &pb.UserRecord{
				Tier:               pb.UserTier_USER_TIER_HOBBYIST,
				SyncCountThisMonth: 10,
			},
			allowed: true,
		},
		{
			name: "Hobbyist at limit cannot sync",
			user: &pb.UserRecord{
				Tier:               pb.UserTier_USER_TIER_HOBBYIST,
				SyncCountThisMonth: 25,
			},
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := CanSync(tt.user)
			if got != tt.allowed {
				t.Errorf("%s: CanSync() = %v, want %v", tt.name, got, tt.allowed)
			}
		})
	}
}

func TestShouldResetSyncCount(t *testing.T) {
	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)

	tests := []struct {
		name     string
		user     *pb.UserRecord
		expected bool
	}{
		{
			name:     "Nil reset date should reset",
			user:     &pb.UserRecord{SyncCountResetAt: nil},
			expected: true,
		},
		{
			name:     "Same month should NOT reset",
			user:     &pb.UserRecord{SyncCountResetAt: timestamppb.New(now)},
			expected: false,
		},
		{
			name:     "Different month should reset",
			user:     &pb.UserRecord{SyncCountResetAt: timestamppb.New(lastMonth)},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldResetSyncCount(tt.user); got != tt.expected {
				t.Errorf("%s: ShouldResetSyncCount() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestGetTrialDaysRemaining(t *testing.T) {
	now := time.Now()
	future := now.Add(10 * 24 * time.Hour)
	past := now.Add(-10 * 24 * time.Hour)

	tests := []struct {
		name     string
		user     *pb.UserRecord
		expected int
	}{
		{
			name:     "No trial",
			user:     &pb.UserRecord{TrialEndsAt: nil},
			expected: -1,
		},
		{
			name:     "Active trial",
			user:     &pb.UserRecord{TrialEndsAt: timestamppb.New(future)},
			expected: 10, // Matches implementation: Sub is 9.something days -> 9 + 1 = 10
		},
		{
			name:     "Expired trial",
			user:     &pb.UserRecord{TrialEndsAt: timestamppb.New(past)},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTrialDaysRemaining(tt.user); got != tt.expected {
				t.Errorf("%s: GetTrialDaysRemaining() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}
