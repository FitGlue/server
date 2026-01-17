package enricher_providers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/fitglue/server/src/go/pkg/domain/activity"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type TypeMapperProvider struct{}

func init() {
	Register(NewTypeMapperProvider())
}

func NewTypeMapperProvider() *TypeMapperProvider {
	return &TypeMapperProvider{}
}

func (p *TypeMapperProvider) Name() string {
	return "type-mapper"
}

func (p *TypeMapperProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_TYPE_MAPPER
}

// TypeMapperRule represents a single rule for mapping activity types based on title
type TypeMapperRule struct {
	Substring  string `json:"substring"`
	TargetType string `json:"target_type"`
}

func (p *TypeMapperProvider) Enrich(ctx context.Context, act *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*EnrichmentResult, error) {
	var rules []TypeMapperRule

	// Check for type_rules JSON map (from web UI: {"title substring": "ACTIVITY_TYPE_..."})
	typeRulesJson, hasTypeRules := inputConfig["type_rules"]
	if hasTypeRules && typeRulesJson != "" {
		var typeRulesMap map[string]string
		if err := json.Unmarshal([]byte(typeRulesJson), &typeRulesMap); err == nil {
			for substring, targetType := range typeRulesMap {
				if substring != "" && targetType != "" {
					rules = append(rules, TypeMapperRule{
						Substring:  substring,
						TargetType: targetType,
					})
				}
			}
		}
	}

	// Also check for rules JSON array (from admin-cli)
	rulesJson, ok := inputConfig["rules"]
	if ok && rulesJson != "" {
		var jsonRules []TypeMapperRule
		if err := json.Unmarshal([]byte(rulesJson), &jsonRules); err == nil {
			rules = append(rules, jsonRules...)
		}
	}

	// No rules configured, nothing to do
	if len(rules) == 0 {
		return &EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "no_rules_configured"},
		}, nil
	}

	// Get the current activity title
	activityTitle := act.Name
	if activityTitle == "" {
		return &EnrichmentResult{
			Metadata: map[string]string{"status": "skipped", "reason": "no_activity_title"},
		}, nil
	}

	// Get original type for metadata
	originalType := act.Type
	originalTypeName := activity.GetStravaActivityType(originalType)

	// Check each rule - first match wins
	for _, rule := range rules {
		if rule.Substring == "" {
			continue
		}
		// Match case-insensitively against the activity title
		if strings.Contains(strings.ToLower(activityTitle), strings.ToLower(rule.Substring)) {
			// Parse the target type
			newType := activity.ParseActivityTypeFromString(rule.TargetType)
			if newType != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
				act.Type = newType
				return &EnrichmentResult{
					Metadata: map[string]string{
						"original_type":   originalTypeName,
						"new_type":        activity.GetStravaActivityType(newType),
						"matched_title":   activityTitle,
						"matched_pattern": rule.Substring,
					},
				}, nil
			}
		}
	}

	// No matching rule found
	return &EnrichmentResult{
		Metadata: map[string]string{"status": "skipped", "reason": "no_matching_rule", "title": activityTitle},
	}, nil
}
