package source_link

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	domainuser "github.com/fitglue/server/src/go/pkg/domain/user"
	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbuser "github.com/fitglue/server/src/go/pkg/types/pb/models/user"
)

func newUser() *domainuser.Record {
	return &domainuser.Record{UserProfile: &pbuser.UserProfile{UserId: "u1"}}
}

func TestSourceLink_NoExternalID(t *testing.T) {
	p := NewSourceLinkProvider()
	act := &pbactivity.StandardizedActivity{Source: pbactivity.ActivitySource_SOURCE_STRAVA}
	res, err := p.Enrich(context.Background(), slog.Default(), act, newUser(), nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["status"] != "skipped" || res.Metadata["reason"] != "no_external_id" {
		t.Errorf("expected skipped (no external_id), got %v", res.Metadata)
	}
}

func TestSourceLink_Strava(t *testing.T) {
	p := NewSourceLinkProvider()
	act := &pbactivity.StandardizedActivity{
		Source:     pbactivity.ActivitySource_SOURCE_STRAVA,
		ExternalId: "12345",
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, newUser(), nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "strava.com") {
		t.Errorf("expected strava.com link in description, got %q", res.Description)
	}
	if !strings.Contains(res.Description, "12345") {
		t.Errorf("expected activity ID in link, got %q", res.Description)
	}
}

func TestSourceLink_Hevy(t *testing.T) {
	p := NewSourceLinkProvider()
	act := &pbactivity.StandardizedActivity{
		Source:     pbactivity.ActivitySource_SOURCE_HEVY,
		ExternalId: "abc123",
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, newUser(), nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Description, "hevy.com") {
		t.Errorf("expected hevy.com link in description, got %q", res.Description)
	}
}

func TestSourceLink_UnknownSource(t *testing.T) {
	p := NewSourceLinkProvider()
	act := &pbactivity.StandardizedActivity{
		Source:     pbactivity.ActivitySource_SOURCE_FITBIT,
		ExternalId: "xyz",
	}
	res, err := p.Enrich(context.Background(), slog.Default(), act, newUser(), nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metadata["status"] != "skipped" || res.Metadata["reason"] != "unknown_source" {
		t.Errorf("expected skipped (unknown source), got %v", res.Metadata)
	}
}

func TestSourceLink_ProviderMetadata(t *testing.T) {
	p := NewSourceLinkProvider()
	if p.Name() != "source-link" {
		t.Errorf("expected 'source-link', got %q", p.Name())
	}
}
