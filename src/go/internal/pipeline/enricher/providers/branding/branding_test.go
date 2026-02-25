package branding

import (
	"context"
	"log/slog"
	"testing"

	domainuser "github.com/fitglue/server/src/go/pkg/domain/user"
	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"
)

func TestBranding_DefaultMessage(t *testing.T) {
	p := NewBrandingProvider()
	act := &pbactivity.StandardizedActivity{Name: "Morning Run"}
	user := &domainuser.Record{UserProfile: &pbuser.UserProfile{UserId: "u1"}}

	res, err := p.Enrich(context.Background(), slog.Default(), act, user, map[string]string{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Description != "Posted via FitGlue 💪" {
		t.Errorf("expected default message, got %q", res.Description)
	}
	if res.Metadata["message"] != "Posted via FitGlue 💪" {
		t.Errorf("expected metadata message, got %q", res.Metadata["message"])
	}
}

func TestBranding_CustomMessage(t *testing.T) {
	p := NewBrandingProvider()
	act := &pbactivity.StandardizedActivity{}
	user := &domainuser.Record{UserProfile: &pbuser.UserProfile{UserId: "u1"}}
	inputs := map[string]string{"message": "Check out my workout!"}

	res, err := p.Enrich(context.Background(), slog.Default(), act, user, inputs, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Description != "Check out my workout!" {
		t.Errorf("expected custom message, got %q", res.Description)
	}
}

func TestBranding_ProviderMetadata(t *testing.T) {
	p := NewBrandingProvider()
	if p.Name() != "branding" {
		t.Errorf("expected 'branding', got %q", p.Name())
	}
}
