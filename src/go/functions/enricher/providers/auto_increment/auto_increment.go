package auto_increment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
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

func (p *AutoIncrementProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("auto_increment: starting",
		"activity_name", activity.Name,
		"counter_key", inputs["counter_key"],
		"title_filter", inputs["title_contains"],
		"initial_value", inputs["initial_value"],
	)

	// 1. Validation
	key := inputs["counter_key"]
	if key == "" {
		logger.Debug("auto_increment: skipping - no counter_key configured")
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
			logger.Debug("auto_increment: skipping - title does not match filter",
				"filter", filter,
				"activity_name", activity.Name,
			)
			return &providers.EnrichmentResult{
				Metadata: map[string]string{
					"auto_increment_applied": "false",
					"reason":                 "Title does not contain filter",
				},
			}, nil
		}
		logger.Debug("auto_increment: title filter matched",
			"filter", filter,
		)
	}

	if p.service == nil {
		logger.Debug("auto_increment: error - service not initialized")
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
			logger.Debug("auto_increment: counter not found, will initialize",
				"key", key,
			)
			counter = nil // Treat as missing -> initialize below
		} else {
			// Real error from DB
			logger.Debug("auto_increment: error getting counter",
				"error", err.Error(),
			)
			return &providers.EnrichmentResult{
				Metadata: map[string]string{
					"auto_increment_applied": "false",
				},
			}, fmt.Errorf("failed to get counter: %w", err)
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
				logger.Debug("auto_increment: using custom initial value",
					"initial_value", initialVal,
					"starting_count", currentCount,
				)
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

	logger.Debug("auto_increment: incrementing counter",
		"key", key,
		"previous_count", counter.Count-1,
		"new_count", newCount,
	)

	// Persist
	if err := p.service.DB.SetCounter(ctx, user.UserId, counter); err != nil {
		logger.Debug("auto_increment: error persisting counter",
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to update counter: %w", err)
	}

	logger.Debug("auto_increment: successfully applied",
		"key", key,
		"new_count", newCount,
		"suffix", fmt.Sprintf(" (#%d)", newCount),
	)

	return &providers.EnrichmentResult{
		NameSuffix: fmt.Sprintf(" (#%d)", newCount),
		Metadata: map[string]string{
			"auto_increment_applied": "true",
			"auto_increment_key":     key,
			"auto_increment_val":     fmt.Sprintf("%d", newCount),
		},
	}, nil
}
