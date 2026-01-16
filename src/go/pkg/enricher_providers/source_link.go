package enricher_providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/fitglue/server/src/go/pkg/plugin"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// SourceLinkProvider appends a link to the original activity in the description.
type SourceLinkProvider struct{}

func init() {
	Register(NewSourceLinkProvider())

	plugin.RegisterEnricher(pb.EnricherProviderType_ENRICHER_PROVIDER_SOURCE_LINK, &pb.PluginManifest{
		Id:           "source-link",
		Type:         pb.PluginType_PLUGIN_TYPE_ENRICHER,
		Name:         "Source Link",
		Description:  "Appends a link to the original activity in the description",
		Icon:         "🔗",
		Enabled:      true,
		ConfigSchema: []*pb.ConfigFieldSchema{}, // No config needed
	})
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
		return &EnrichmentResult{}, nil
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
		return &EnrichmentResult{}, nil
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
