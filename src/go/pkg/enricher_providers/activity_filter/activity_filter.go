package activity_filter

import (
	"context"
	"fmt"
	"strings"

	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	"github.com/fitglue/server/src/go/pkg/plugin"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// ActivityFilterProvider filters activities based on type, title, or description patterns.
// When conditions match, it halts the pipeline (activity is skipped, not failed).
type ActivityFilterProvider struct{}

func init() {
	enricher_providers.Register(NewActivityFilterProvider())

	plugin.RegisterEnricher(pb.EnricherProviderType_ENRICHER_PROVIDER_ACTIVITY_FILTER, &pb.PluginManifest{
		Id:          "activity-filter",
		Type:        pb.PluginType_PLUGIN_TYPE_ENRICHER,
		Name:        "Activity Filter",
		Description: "Skips activities that match exclude patterns or don't match include patterns",
		Icon:        "🚫",
		Enabled:     true,
		ConfigSchema: []*pb.ConfigFieldSchema{
			{
				Key:         "exclude_activity_types",
				Label:       "Exclude Activity Types",
				Description: "Comma-separated activity types to exclude (e.g., Walk,Yoga)",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "exclude_title_contains",
				Label:       "Exclude Titles Containing",
				Description: "Comma-separated patterns to exclude (e.g., test,morning)",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "exclude_description_contains",
				Label:       "Exclude Descriptions Containing",
				Description: "Comma-separated patterns to exclude from description",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "include_activity_types",
				Label:       "Include Only Activity Types",
				Description: "Comma-separated activity types to include (all others excluded)",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
			{
				Key:         "include_title_contains",
				Label:       "Include Only Titles Containing",
				Description: "Activity must contain one of these patterns",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
				Required:    false,
			},
		},
	})
}

func NewActivityFilterProvider() *ActivityFilterProvider {
	return &ActivityFilterProvider{}
}

func (p *ActivityFilterProvider) Name() string {
	return "activity_filter"
}

func (p *ActivityFilterProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_ACTIVITY_FILTER
}

func (p *ActivityFilterProvider) Enrich(ctx context.Context, act *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	// Check exclude_activity_types
	if excludeTypes := inputs["exclude_activity_types"]; excludeTypes != "" {
		actType := act.Type.String()
		for _, t := range strings.Split(excludeTypes, ",") {
			t = strings.TrimSpace(t)
			// Match by name (e.g., "WALK", "YOGA") or full enum name
			if strings.EqualFold(t, actType) ||
				strings.EqualFold("ACTIVITY_TYPE_"+t, actType) {
				return &enricher_providers.EnrichmentResult{
					HaltPipeline: true,
					HaltReason:   fmt.Sprintf("Activity type %s is excluded", actType),
					Metadata: map[string]string{
						"filter_applied": "true",
						"filter_reason":  "activity_type_excluded",
						"excluded_type":  t,
					},
				}, nil
			}
		}
	}

	// Check exclude_title_contains
	if excludeTitle := inputs["exclude_title_contains"]; excludeTitle != "" {
		titleLower := strings.ToLower(act.Name)
		for _, pattern := range strings.Split(excludeTitle, ",") {
			pattern = strings.TrimSpace(strings.ToLower(pattern))
			if pattern != "" && strings.Contains(titleLower, pattern) {
				return &enricher_providers.EnrichmentResult{
					HaltPipeline: true,
					HaltReason:   fmt.Sprintf("Title contains excluded pattern: %s", pattern),
					Metadata: map[string]string{
						"filter_applied":   "true",
						"filter_reason":    "title_pattern_excluded",
						"excluded_pattern": pattern,
					},
				}, nil
			}
		}
	}

	// Check exclude_description_contains
	if excludeDesc := inputs["exclude_description_contains"]; excludeDesc != "" {
		descLower := strings.ToLower(act.Description)
		for _, pattern := range strings.Split(excludeDesc, ",") {
			pattern = strings.TrimSpace(strings.ToLower(pattern))
			if pattern != "" && strings.Contains(descLower, pattern) {
				return &enricher_providers.EnrichmentResult{
					HaltPipeline: true,
					HaltReason:   fmt.Sprintf("Description contains excluded pattern: %s", pattern),
					Metadata: map[string]string{
						"filter_applied":   "true",
						"filter_reason":    "description_pattern_excluded",
						"excluded_pattern": pattern,
					},
				}, nil
			}
		}
	}

	// --- INCLUSION LOGIC (Allow-list) ---
	// If any include_* rules are present, the activity MUST match at least one of them to proceed.

	incTypes := inputs["include_activity_types"]
	incTitle := inputs["include_title_contains"]
	incDesc := inputs["include_description_contains"]

	hasInclusionRules := incTypes != "" || incTitle != "" || incDesc != ""

	if hasInclusionRules {
		matched := false

		// Check include_activity_types
		if incTypes != "" {
			actType := act.Type.String()
			for _, t := range strings.Split(incTypes, ",") {
				t = strings.TrimSpace(t)
				if strings.EqualFold(t, actType) || strings.EqualFold("ACTIVITY_TYPE_"+t, actType) {
					matched = true
					break
				}
			}
		}

		// Check include_title_contains
		if !matched && incTitle != "" {
			titleLower := strings.ToLower(act.Name)
			for _, pattern := range strings.Split(incTitle, ",") {
				pattern = strings.TrimSpace(strings.ToLower(pattern))
				if pattern != "" && strings.Contains(titleLower, pattern) {
					matched = true
					break
				}
			}
		}

		// Check include_description_contains
		if !matched && incDesc != "" {
			descLower := strings.ToLower(act.Description)
			for _, pattern := range strings.Split(incDesc, ",") {
				pattern = strings.TrimSpace(strings.ToLower(pattern))
				if pattern != "" && strings.Contains(descLower, pattern) {
					matched = true
					break
				}
			}
		}

		if !matched {
			return &enricher_providers.EnrichmentResult{
				HaltPipeline: true,
				HaltReason:   "Activity did not match any inclusion criteria",
				Metadata: map[string]string{
					"filter_applied": "true",
					"filter_reason":  "not_included",
				},
			}, nil
		}
	}

	// No filter matched - activity passes through
	return &enricher_providers.EnrichmentResult{
		Metadata: map[string]string{
			"filter_applied": "false",
		},
	}, nil
}
