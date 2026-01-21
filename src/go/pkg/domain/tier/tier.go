package tier

import (
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

const (
	FreeTierSyncsPerMonth  = 25
	FreeTierMaxConnections = 2
)

type EffectiveTier string

const (
	TierFree EffectiveTier = "free"
	TierPro  EffectiveTier = "pro"
)

// GetEffectiveTier determines the user's effective tier based on admin status,
// trial period, and stored tier.
func GetEffectiveTier(user *pb.UserRecord) EffectiveTier {
	// Admin override always grants Pro
	if user.IsAdmin {
		return TierPro
	}

	// Active trial grants Pro
	if user.TrialEndsAt != nil && user.TrialEndsAt.AsTime().After(time.Now()) {
		return TierPro
	}

	// Fall back to stored tier (default: free)
	if user.Tier == "pro" || user.Tier == "athlete" {
		return TierPro
	}

	return TierFree
}

// CanSync checks if user can perform a sync within their tier limits.
func CanSync(user *pb.UserRecord) (allowed bool, reason string) {
	tier := GetEffectiveTier(user)

	if tier == TierPro {
		return true, ""
	}

	// Check monthly limit for free tier
	if user.SyncCountThisMonth >= FreeTierSyncsPerMonth {
		return false, "Free tier limit reached (25/month). Upgrade to Pro for unlimited syncs."
	}

	return true, ""
}

// CanAddConnection checks if user can add a new connection within their tier limits.
func CanAddConnection(user *pb.UserRecord, currentCount int) (allowed bool, reason string) {
	tier := GetEffectiveTier(user)

	if tier == TierPro {
		return true, ""
	}

	if currentCount >= FreeTierMaxConnections {
		return false, "Free tier limited to 2 connections. Upgrade to Pro for unlimited."
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
