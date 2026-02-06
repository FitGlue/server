package source_link

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// SourceLinkProvider appends a link to the original activity in the description.
type SourceLinkProvider struct{}

func init() {
	providers.Register(NewSourceLinkProvider())
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

func (p *SourceLinkProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("source_link: starting",
		"source", activity.Source,
		"external_id", activity.ExternalId,
	)

	if activity.ExternalId == "" {
		logger.Debug("source_link: skipping - no external_id")
		return &providers.EnrichmentResult{
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
		logger.Debug("source_link: skipping - unknown source",
			"source", sourceLower,
		)
		return &providers.EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "unknown_source", "source": sourceLower},
		}, nil
	}

	// Format: "View on [Source]: [URL]"
	// We can allow customization via inputConfig if needed later
	sourceName := strings.TrimPrefix(sourceLower, "source_")
	sourceDisplay := sourceName
	if len(sourceName) > 0 {
		sourceDisplay = strings.ToUpper(sourceName[:1]) + sourceName[1:]
	}
	desc := fmt.Sprintf("View on %s: %s", sourceDisplay, link)

	logger.Debug("source_link: generated link",
		"source_display", sourceDisplay,
		"link", link,
	)

	return &providers.EnrichmentResult{
		Description: desc,
		Metadata: map[string]string{
			"source": sourceDisplay,
			"link":   link,
		},
	}, nil
}
