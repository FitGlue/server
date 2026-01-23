package providers

import (
	"context"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// EnrichmentResult represents the outcome of an enrichment provider.
type EnrichmentResult struct {
	// Metadata overrides (if empty/unspecified, original is kept)
	ActivityType pb.ActivityType
	Description  string

	Name       string
	NameSuffix string // Appended to the final name (e.g. " (#5)")
	Tags       []string

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

	// HaltPipeline signals the orchestrator to stop processing this pipeline.
	// Not a failure - the activity is intentionally skipped (e.g., filtered out).
	HaltPipeline bool
	HaltReason   string // Human-readable reason for logging/display
}

// Provider defines the interface for an enrichment service.
type Provider interface {
	// Name returns the unique identifier for the provider (e.g., "fitbit-hr", "ai-description").
	Name() string

	// ProviderType returns the protobuf enum type for this provider
	ProviderType() pb.EnricherProviderType

	// Enrich applies the logic to the activity.
	// inputConfig contains the user-specific input parameters for this provider.
	// doNotRetry indicates if the provider should return partial/success data instead of RetryableError on lag.
	Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*EnrichmentResult, error)
}
