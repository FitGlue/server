package tier

import (
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGetEffectiveTier_Admin(t *testing.T) {
	user := &pb.UserRecord{
		IsAdmin: true,
		Tier:    "free",
	}

	tier := GetEffectiveTier(user)
	if tier != TierPro {
		t.Errorf("Expected TierPro for admin, got %v", tier)
	}
}

func TestGetEffectiveTier_ActiveTrial(t *testing.T) {
	futureDate := time.Now().Add(10 * 24 * time.Hour)
	user := &pb.UserRecord{
		IsAdmin:     false,
		Tier:        "free",
		TrialEndsAt: timestamppb.New(futureDate),
	}

	tier := GetEffectiveTier(user)
	if tier != TierPro {
		t.Errorf("Expected TierPro for active trial, got %v", tier)
	}
}

func TestGetEffectiveTier_ExpiredTrial(t *testing.T) {
	pastDate := time.Now().Add(-10 * 24 * time.Hour)
	user := &pb.UserRecord{
		IsAdmin:     false,
		Tier:        "free",
		TrialEndsAt: timestamppb.New(pastDate),
	}

	tier := GetEffectiveTier(user)
	if tier != TierFree {
		t.Errorf("Expected TierFree for expired trial, got %v", tier)
	}
}

func TestGetEffectiveTier_ProSubscriber(t *testing.T) {
	user := &pb.UserRecord{
		IsAdmin: false,
		Tier:    "pro",
	}

	tier := GetEffectiveTier(user)
	if tier != TierPro {
		t.Errorf("Expected TierPro for pro subscriber, got %v", tier)
	}
}

func TestGetEffectiveTier_AthleteSubscriber(t *testing.T) {
	user := &pb.UserRecord{
		IsAdmin: false,
		Tier:    "athlete",
	}

	tier := GetEffectiveTier(user)
	if tier != TierPro {
		t.Errorf("Expected TierPro for athlete subscriber, got %v", tier)
	}
}

func TestGetEffectiveTier_FreeUser(t *testing.T) {
	user := &pb.UserRecord{
		IsAdmin: false,
		Tier:    "free",
	}

	tier := GetEffectiveTier(user)
	if tier != TierFree {
		t.Errorf("Expected TierFree for free user, got %v", tier)
	}
}

func TestCanSync_ProUser(t *testing.T) {
	user := &pb.UserRecord{
		Tier:               "pro",
		SyncCountThisMonth: 100,
	}

	allowed, _ := CanSync(user)
	if !allowed {
		t.Error("Pro user should be allowed to sync regardless of count")
	}
}

func TestCanSync_FreeBelowLimit(t *testing.T) {
	user := &pb.UserRecord{
		Tier:               "free",
		SyncCountThisMonth: 10,
	}

	allowed, _ := CanSync(user)
	if !allowed {
		t.Error("Free user below limit should be allowed to sync")
	}
}

func TestCanSync_FreeAtLimit(t *testing.T) {
	user := &pb.UserRecord{
		Tier:               "free",
		SyncCountThisMonth: 25,
	}

	allowed, reason := CanSync(user)
	if allowed {
		t.Error("Free user at limit should NOT be allowed to sync")
	}
	if reason == "" {
		t.Error("Expected reason for denial")
	}
}

func TestShouldResetSyncCount_NilResetAt(t *testing.T) {
	user := &pb.UserRecord{}

	if !ShouldResetSyncCount(user) {
		t.Error("Should reset if SyncCountResetAt is nil")
	}
}

func TestShouldResetSyncCount_SameMonth(t *testing.T) {
	now := time.Now()
	user := &pb.UserRecord{
		SyncCountResetAt: timestamppb.New(now),
	}

	if ShouldResetSyncCount(user) {
		t.Error("Should NOT reset if same month")
	}
}

func TestShouldResetSyncCount_DifferentMonth(t *testing.T) {
	lastMonth := time.Now().AddDate(0, -1, 0)
	user := &pb.UserRecord{
		SyncCountResetAt: timestamppb.New(lastMonth),
	}

	if !ShouldResetSyncCount(user) {
		t.Error("Should reset if different month")
	}
}

func TestGetTrialDaysRemaining_NoTrial(t *testing.T) {
	user := &pb.UserRecord{}

	days := GetTrialDaysRemaining(user)
	if days != -1 {
		t.Errorf("Expected -1 for no trial, got %d", days)
	}
}

func TestGetTrialDaysRemaining_ActiveTrial(t *testing.T) {
	futureDate := time.Now().Add(10 * 24 * time.Hour)
	user := &pb.UserRecord{
		TrialEndsAt: timestamppb.New(futureDate),
	}

	days := GetTrialDaysRemaining(user)
	if days < 10 || days > 11 {
		t.Errorf("Expected ~10-11 days remaining, got %d", days)
	}
}

func TestGetTrialDaysRemaining_ExpiredTrial(t *testing.T) {
	pastDate := time.Now().Add(-10 * 24 * time.Hour)
	user := &pb.UserRecord{
		TrialEndsAt: timestamppb.New(pastDate),
	}

	days := GetTrialDaysRemaining(user)
	if days != 0 {
		t.Errorf("Expected 0 for expired trial, got %d", days)
	}
}
