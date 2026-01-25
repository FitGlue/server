package branding

import (
	"context"
	"log/slog"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
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

func (p *BrandingProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_UNSPECIFIED
}

func (p *BrandingProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
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
