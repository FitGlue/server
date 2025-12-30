package enricher_providers

import (
	"context"
	"encoding/json"
	"strings"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type TypeMappingRule struct {
	Substring  string `json:"substring"`
	TargetType string `json:"target_type"`
}

type TypeMapperProvider struct{}

func NewTypeMapperProvider() *TypeMapperProvider {
	return &TypeMapperProvider{}
}

func (p *TypeMapperProvider) Name() string {
	return "type-mapper"
}

func (p *TypeMapperProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string) (*EnrichmentResult, error) {
	rulesJson, ok := inputConfig["rules"]
	if !ok || rulesJson == "" {
		// No rules configured, nothing to do
		return &EnrichmentResult{}, nil
	}

	var rules []TypeMappingRule
	if err := json.Unmarshal([]byte(rulesJson), &rules); err != nil {
		// If JSON is invalid, log error implicitly by returning it?
		// Or just skip? Better to bubble up for visibility, though enrichers usually "best effort".
		// We'll return empty result but maybe log if we had a logger.
		// For now, silent failure on invalid config is safer than crashing pipeline.
		return &EnrichmentResult{}, nil
	}

	activityName := strings.ToLower(activity.Name)
	originalType := activity.Type
	newType := ""

	for _, rule := range rules {
		if rule.Substring == "" || rule.TargetType == "" {
			continue
		}
		if strings.Contains(activityName, strings.ToLower(rule.Substring)) {
			activity.Type = rule.TargetType
			newType = rule.TargetType
			break // First match wins
		}
	}

	if newType != "" {
		return &EnrichmentResult{
			Metadata: map[string]string{
				"original_type": originalType,
				"new_type":      newType,
				"rule_matched":  "true",
			},
		}, nil
	}

	return &EnrichmentResult{}, nil
}
