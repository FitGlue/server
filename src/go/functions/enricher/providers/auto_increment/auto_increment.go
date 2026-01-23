package auto_increment

import (
	"context"
	"fmt"
	"strings"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/functions/enricher/providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AutoIncrementProvider struct {
	service *bootstrap.Service
}

func init() {
	providers.Register(&AutoIncrementProvider{})
}

func (p *AutoIncrementProvider) SetService(s *bootstrap.Service) {
	p.service = s
}

func (p *AutoIncrementProvider) Name() string {
	return "auto_increment"
}

func (p *AutoIncrementProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_AUTO_INCREMENT
}

func (p *AutoIncrementProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// 1. Validation
	key := inputs["counter_key"]
	if key == "" {
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"auto_increment_applied": "false",
				"reason":                 "Misconfigured",
			},
		}, nil
	}

	// 2. Title Filter (Optional)
	if filter, ok := inputs["title_contains"]; ok && filter != "" {
		if !strings.Contains(strings.ToLower(activity.Name), strings.ToLower(filter)) {
			return &providers.EnrichmentResult{
				Metadata: map[string]string{
					"auto_increment_applied": "false",
					"reason":                 "Title does not contain filter",
				},
			}, nil
		}
	}

	if p.service == nil {
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"auto_increment_applied": "false",
			},
		}, fmt.Errorf("service not initialized")
	}

	// 3. Get/Increment Counter
	counter, err := p.service.DB.GetCounter(ctx, user.UserId, key)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			counter = nil // Treat as missing -> initialize below
		} else {
			// Real error from DB
			return &providers.EnrichmentResult{
				Metadata: map[string]string{
					"auto_increment_applied": "false",
				},
			}, fmt.Errorf("failed to get counter: %v", err)
		}
	}

	if counter == nil {
		// Not found - initialize
		var currentCount int64 = 0
		if initialValStr, ok := inputs["initial_value"]; ok && initialValStr != "" {
			var initialVal int64
			if _, err := fmt.Sscanf(initialValStr, "%d", &initialVal); err == nil {
				// We want the *next* increment to result in `initialVal`.
				// So we start at `initialVal - 1`.
				currentCount = initialVal - 1
			}
		}

		counter = &pb.Counter{
			Id:    key,
			Count: currentCount,
		}
	}

	newCount := counter.Count + 1
	counter.Count = newCount
	counter.LastUpdated = timestamppb.Now()

	// Persist
	if err := p.service.DB.SetCounter(ctx, user.UserId, counter); err != nil {
		return nil, fmt.Errorf("failed to update counter: %v", err)
	}

	return &providers.EnrichmentResult{
		NameSuffix: fmt.Sprintf(" (#%d)", newCount),
		Metadata: map[string]string{
			"auto_increment_applied": "true",
			"auto_increment_key":     key,
			"auto_increment_val":     fmt.Sprintf("%d", newCount),
		},
	}, nil
}
