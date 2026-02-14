package auto_increment

import (
	"context"
	"encoding/json"
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
		"has_counter_rules", inputs["counter_rules"] != "",
		"counter_key", inputs["counter_key"],
		"initial_value", inputs["initial_value"],
	)

	// 1. Resolve counter key — new counter_rules map or legacy counter_key field
	key := p.resolveCounterKey(logger, activity.Name, inputs)
	if key == "" {
		logger.Debug("auto_increment: skipping - no matching counter key")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"auto_increment_applied": "false",
				"reason":                 "No matching rule",
			},
		}, nil
	}

	if p.service == nil {
		logger.Debug("auto_increment: error - service not initialized")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"auto_increment_applied": "false",
			},
		}, fmt.Errorf("service not initialized")
	}

	// 2. Get/Increment Counter
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
		// Not found - initialize at 0 so first increment yields 1
		counter = &pb.Counter{
			Id:    key,
			Count: 0,
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

// resolveCounterKey determines the counter key to use based on inputs.
// New format: counter_rules JSON map {"title substring": "counter_key"} — first match wins.
// Legacy format: counter_key + optional title_contains filter.
func (p *AutoIncrementProvider) resolveCounterKey(logger *slog.Logger, activityName string, inputs map[string]string) string {
	// New format: counter_rules JSON map
	if rulesJSON, ok := inputs["counter_rules"]; ok && rulesJSON != "" {
		var rules map[string]string
		if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
			logger.Debug("auto_increment: failed to parse counter_rules JSON",
				"error", err.Error(),
			)
			return ""
		}

		for substring, counterKey := range rules {
			if substring == "" || counterKey == "" {
				continue
			}
			if strings.Contains(strings.ToLower(activityName), strings.ToLower(substring)) {
				logger.Debug("auto_increment: counter_rules matched",
					"matched_substring", substring,
					"counter_key", counterKey,
				)
				return counterKey
			}
		}

		logger.Debug("auto_increment: no counter_rules matched",
			"activity_name", activityName,
			"rule_count", len(rules),
		)
		return ""
	}

	// Legacy format: counter_key + optional title_contains
	key := inputs["counter_key"]
	if key == "" {
		logger.Debug("auto_increment: skipping - no counter_key configured")
		return ""
	}

	if filter, ok := inputs["title_contains"]; ok && filter != "" {
		if !strings.Contains(strings.ToLower(activityName), strings.ToLower(filter)) {
			logger.Debug("auto_increment: skipping - title does not match filter",
				"filter", filter,
				"activity_name", activityName,
			)
			return ""
		}
		logger.Debug("auto_increment: title filter matched",
			"filter", filter,
		)
	}

	return key
}
