package enricher_providers

import (
	"context"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// EnrichmentResult represents the outcome of an enrichment provider.
type EnrichmentResult struct {
	// Metadata overrides (if empty, original is kept)
	ActivityType string
	Description  string
	Name         string

	// Raw Data Streams (for merging)
	HeartRateStream    []int
	PowerStream        []int
	PositionLatStream  []float64
	PositionLongStream []float64

	// Artifacts (Providers can still generate specific artifacts if independent)
	// But main FIT generation should normally happen in Orchestrator fan-in.
	FitFileContent []byte

	// Extra metadata to append
	Metadata map[string]string
}

// Provider defines the interface for an enrichment service.
type Provider interface {
	// Name returns the unique identifier for the provider (e.g., "fitbit-hr", "ai-description").
	Name() string

	// ProviderType returns the protobuf enum type for this provider
	ProviderType() pb.EnricherProviderType

	// Enrich applies the logic to the activity.
	// inputConfig contains the user-specific input parameters for this provider.
	Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*EnrichmentResult, error)
}
