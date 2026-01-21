package tier

import (
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

const (
	HobbyistTierSyncsPerMonth  = 25
	HobbyistTierMaxConnections = 2
)

// Effective tier is used for internal logic
type EffectiveTier string

const (
	TierHobbyist EffectiveTier = "hobbyist"
	TierAthlete  EffectiveTier = "athlete"
)

// GetEffectiveTier determines the user's effective tier based on admin status,
// trial period, and stored tier.
func GetEffectiveTier(user *pb.UserRecord) EffectiveTier {
	// Admin override always grants Athlete
	if user.IsAdmin {
		return TierAthlete
	}

	// Active trial grants Athlete
	if user.TrialEndsAt != nil && user.TrialEndsAt.AsTime().After(time.Now()) {
		return TierAthlete
	}

	// Fall back to stored tier (default: hobbyist)
	if user.Tier == pb.UserTier_USER_TIER_ATHLETE {
		return TierAthlete
	}

	return TierHobbyist
}

// CanSync checks if user can perform a sync within their tier limits.
func CanSync(user *pb.UserRecord) (allowed bool, reason string) {
	tier := GetEffectiveTier(user)

	if tier == TierAthlete {
		return true, ""
	}

	// Check monthly limit for hobbyist tier
	if user.SyncCountThisMonth >= HobbyistTierSyncsPerMonth {
		return false, "Hobbyist tier limit reached (25/month). Upgrade to Athlete for unlimited syncs."
	}

	return true, ""
}

// CanAddConnection checks if user can add a new connection within their tier limits.
func CanAddConnection(user *pb.UserRecord, currentCount int) (allowed bool, reason string) {
	tier := GetEffectiveTier(user)

	if tier == TierAthlete {
		return true, ""
	}

	if currentCount >= HobbyistTierMaxConnections {
		return false, "Hobbyist tier limited to 2 connections. Upgrade to Athlete for unlimited."
	}

	return true, ""
}

// ShouldResetSyncCount checks if the sync counter should be reset (monthly)
func ShouldResetSyncCount(user *pb.UserRecord) bool {
	if user.SyncCountResetAt == nil {
		return true
	}

	resetTime := user.SyncCountResetAt.AsTime()
	now := time.Now()

	// Reset if the reset date is in a different month
	return resetTime.Year() != now.Year() || resetTime.Month() != now.Month()
}

// GetTrialDaysRemaining returns the number of days left in trial, or -1 if not on trial
func GetTrialDaysRemaining(user *pb.UserRecord) int {
	if user.TrialEndsAt == nil {
		return -1
	}

	now := time.Now()
	trialEnd := user.TrialEndsAt.AsTime()

	if trialEnd.Before(now) || trialEnd.Equal(now) {
		return 0
	}

	return int(trialEnd.Sub(now).Hours()/24) + 1
}
