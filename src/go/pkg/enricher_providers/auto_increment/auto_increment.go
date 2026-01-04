package auto_increment

import (
	"context"
	"fmt"
	"strings"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/enricher_providers"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AutoIncrementProvider struct {
	service *bootstrap.Service
}

func init() {
	enricher_providers.Register(&AutoIncrementProvider{})
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

func (p *AutoIncrementProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	// 1. Validation
	key := inputs["counter_key"]
	if key == "" {
		return nil, nil // Misconfigured
	}

	// 2. Title Filter (Optional)
	// NOTE: Since we run in parallel with other providers, this checks the ORIGINAL activity Name.
	// If the user relies on ConditionMatcher to set the name `Parkrun`, AND this filter to check for `Parkrun`,
	// it will FAIL because ConditionMatcher hasn't finished yet (or runs in parallel).
	// This filter is only useful for filtering based on SOURCE name.
	if filter, ok := inputs["title_contains"]; ok && filter != "" {
		if !strings.Contains(activity.Name, filter) {
			return nil, nil
		}
	}

	if p.service == nil {
		return nil, fmt.Errorf("service not initialized")
	}

	// 3. Get/Increment Counter
	// We use atomic increment if possible, or Get+Set.
	// Since we are inside an enricher, we should treat this carefully.
	// If we just Get+Set, race conditions might occur if user uploads 2 activities at once.
	// Firestore supports transactions, but our Database interface abstracts it.
	// For now, Get + Set (simple).
	// Ideally we'd use `FieldValue.increment` but our Store interface expects full object updates for `SetCounter`.
	// Let's rely on standard Get+Set logic for MVP.

	counter, err := p.service.DB.GetCounter(ctx, user.UserId, key)
	if err != nil {
		// Real error from DB
		return nil, fmt.Errorf("failed to get counter: %v", err)
	}

	if counter == nil {
		// Not found - initialize
		counter = &pb.Counter{
			Id:    key,
			Count: 0,
		}
	}

	newCount := counter.Count + 1
	counter.Count = newCount
	counter.LastUpdated = timestamppb.Now()

	// Persist
	if err := p.service.DB.SetCounter(ctx, user.UserId, counter); err != nil {
		return nil, fmt.Errorf("failed to update counter: %v", err)
	}

	return &enricher_providers.EnrichmentResult{
		NameSuffix: fmt.Sprintf(" (#%d)", newCount),
		Metadata: map[string]string{
			"auto_increment_key": key,
			"auto_increment_val": fmt.Sprintf("%d", newCount),
		},
	}, nil
}
