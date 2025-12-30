package enricher_providers

import (
	"context"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// BrandingProvider adds a footer to the activity description
type BrandingProvider struct{}

func NewBrandingProvider() *BrandingProvider {
	return &BrandingProvider{}
}

func (p *BrandingProvider) Name() string {
	return "branding"
}

func (p *BrandingProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*EnrichmentResult, error) {
	// Get custom message from config, or use default
	message := inputConfig["message"]
	if message == "" {
		message = "Posted via fitglue.tech 💪"
	}

	return &EnrichmentResult{
		Description: "\n\n" + message,
	}, nil
}
