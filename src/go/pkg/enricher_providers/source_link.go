package enricher_providers

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// SourceLinkProvider appends a link to the original activity in the description.
type SourceLinkProvider struct{}

func init() {
	Register(NewSourceLinkProvider())
}

func NewSourceLinkProvider() *SourceLinkProvider {
	return &SourceLinkProvider{}
}

func (p *SourceLinkProvider) Name() string {
	return "source-link"
}

func (p *SourceLinkProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_SOURCE_LINK
}

func (p *SourceLinkProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*EnrichmentResult, error) {
	if activity.ExternalId == "" {
		return &EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "no_external_id"},
		}, nil
	}

	var link string
	sourceLower := strings.ToLower(activity.Source)

	// Define URL templates (Move to config/map if this grows)
	switch sourceLower {
	case "hevy", "source_hevy":
		link = fmt.Sprintf("https://hevy.com/workout/%s", activity.ExternalId)
	case "strava", "source_strava":
		link = fmt.Sprintf("https://www.strava.com/activities/%s", activity.ExternalId)
	default:
		// If unknown source, don't generate a link
		return &EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "unknown_source", "source": sourceLower},
		}, nil
	}

	// Format: "View on [Source]: [URL]"
	// We can allow customization via inputConfig if needed later
	sourceDisplay := strings.Title(strings.TrimPrefix(sourceLower, "source_"))
	desc := fmt.Sprintf("View on %s: %s", sourceDisplay, link)

	return &EnrichmentResult{
		Description: desc,
		Metadata: map[string]string{
			"source": sourceDisplay,
			"link":   link,
		},
	}, nil
}
