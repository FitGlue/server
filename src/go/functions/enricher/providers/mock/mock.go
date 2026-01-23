package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// MockProvider is a configurable enricher provider for testing the full pipeline.
// It simulates different behaviors based on the "behavior" input:
//   - "success": Returns success with optional description
//   - "lag": Returns a RetryableError to trigger LAG_RETRY
//   - "fail": Returns a hard error
type MockProvider struct{}

func init() {
	providers.Register(NewMockProvider())
}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Name() string {
	return "mock"
}

func (p *MockProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK
}

func (p *MockProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	behavior := inputs["behavior"]
	if behavior == "" {
		behavior = "success"
	}

	switch behavior {
	case "success":
		return p.handleSuccess(inputs)

	case "lag":
		return p.handleLag(inputs, doNotRetry)

	case "fail":
		return p.handleFail(inputs)

	default:
		return nil, fmt.Errorf("unknown mock behavior: %s", behavior)
	}
}

func (p *MockProvider) handleSuccess(inputs map[string]string) (*providers.EnrichmentResult, error) {
	result := &providers.EnrichmentResult{
		Name:        inputs["name"],
		Description: inputs["description"],
		Metadata: map[string]string{
			"mock_provider": "true",
			"behavior":      "success",
		},
	}

	if result.Name == "" {
		result.Name = "Mock Activity"
	}
	if result.Description == "" {
		result.Description = "This activity was enriched by the mock provider"
	}

	return result, nil
}

func (p *MockProvider) handleLag(_ map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// If doNotRetry is true (lag exhausted), return success instead
	if doNotRetry {
		result := &providers.EnrichmentResult{
			Name:        "Mock Activity (Lag Exhausted)",
			Description: "This activity was enriched after lag retry was exhausted",
			Metadata: map[string]string{
				"mock_provider":  "true",
				"behavior":       "lag",
				"lag_exhausted":  "true",
				"forced_success": "true",
			},
		}
		return result, nil
	}

	// Return a retryable error to trigger LAG_RETRY
	return nil, providers.NewRetryableError(
		fmt.Errorf("mock provider simulating incomplete data"),
		1*time.Minute,
		"mock lag delay",
	)
}

func (p *MockProvider) handleFail(inputs map[string]string) (*providers.EnrichmentResult, error) {
	message := inputs["error_message"]
	if message == "" {
		message = "mock provider hard failure"
	}
	return nil, fmt.Errorf("%s", message)
}
