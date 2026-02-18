package providers

import (
	"context"
	"log/slog"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// EnrichmentResult represents the outcome of an enrichment provider.
type EnrichmentResult struct {
	// Metadata overrides (if empty/unspecified, original is kept)
	ActivityType pb.ActivityType
	Description  string

	// SectionHeader identifies this description as a replaceable section.
	// If set, uploaders in UPDATE mode will replace existing content
	// matching this header instead of appending.
	// Example: "🏃 Parkrun Results:"
	SectionHeader string

	Name       string
	NameSuffix string // Appended to the final name (e.g. " (#5)")
	Tags       []string

	// Raw Data Streams (for merging)
	HeartRateStream    []int
	PowerStream        []int
	PositionLatStream  []float64
	PositionLongStream []float64

	// TimeMarkers from enricher (e.g., exercise transitions from FIT file uploads)
	TimeMarkers []*pb.TimeMarker

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
	// logger is the structured logger from FrameworkContext for debug/info logging.
	// inputConfig contains the user-specific input parameters for this provider.
	// doNotRetry indicates if the provider should return partial/success data instead of RetryableError on lag.
	Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*EnrichmentResult, error)
}

// ResumableProvider is an optional interface for providers that support resume mode.
// When the orchestrator is in resume mode and the provider is in the resume_only_enrichers list,
// if the provider implements this interface, EnrichResume will be called instead of Enrich.
// This allows providers to apply resolved pending input data directly.
type ResumableProvider interface {
	Provider
	// EnrichResume is called during resume mode to apply resolved pending input data.
	// The pendingInput contains the resolved InputData from the background polling service.
	EnrichResume(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, pendingInput *pb.PendingInput) (*EnrichmentResult, error)
}

// DeferrableProvider is an optional interface for providers that benefit from
// running after all other enrichers have completed (e.g., AI providers).
// The orchestrator defers their execution to Phase 2 but preserves their
// pipeline position for description ordering. Deferred providers receive
// an "enriched_description" key in their config containing the accumulated
// description from all non-deferred enrichers.
type DeferrableProvider interface {
	Provider
	// ShouldDefer returns true if this provider should be deferred to Phase 2.
	ShouldDefer() bool
}
