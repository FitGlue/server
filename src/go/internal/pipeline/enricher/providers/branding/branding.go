package branding

import (
	"context"
	"github.com/fitglue/server/src/go/pkg/domain/user"
	"log/slog"

	"github.com/fitglue/server/src/go/internal/pipeline/enricher/providers"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"

	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
)

// BrandingProvider adds a footer to the activity description
type BrandingProvider struct{}

func init() {
	providers.Register(NewBrandingProvider())
}

func NewBrandingProvider() *BrandingProvider {
	return &BrandingProvider{}
}

func (p *BrandingProvider) Name() string {
	return "branding"
}

func (p *BrandingProvider) ProviderType() pbplugin.EnricherProviderType {
	return pbplugin.EnricherProviderType_ENRICHER_PROVIDER_UNSPECIFIED
}

func (p *BrandingProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pbactivity.StandardizedActivity, user *user.Record, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// Get custom message from config, or use default
	message := inputConfig["message"]
	if message == "" {
		message = "Posted via FitGlue 💪"
	}

	logger.Debug("branding: applying footer",
		"message", message,
		"custom", inputConfig["message"] != "",
	)

	return &providers.EnrichmentResult{
		Description: message,
		Metadata: map[string]string{
			"message": message,
		},
	}, nil
}
