package enricher_providers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ripixel/fitglue-server/src/go/pkg/domain/activity"
	"github.com/ripixel/fitglue-server/src/go/pkg/plugin"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type TypeMapperProvider struct{}

func init() {
	Register(NewTypeMapperProvider())

	plugin.RegisterEnricher(pb.EnricherProviderType_ENRICHER_PROVIDER_TYPE_MAPPER, &pb.PluginManifest{
		Id:          "type-mapper",
		Type:        pb.PluginType_PLUGIN_TYPE_ENRICHER,
		Name:        "Type Mapper",
		Description: "Maps activity types from one type to another (e.g., Ride → Virtual Ride)",
		Icon:        "🏷️",
		Enabled:     true,
		ConfigSchema: []*pb.ConfigFieldSchema{
			{
				Key:         "type_mappings",
				Label:       "Type Mappings",
				Description: "Map original activity types to desired types",
				FieldType:   pb.ConfigFieldType_CONFIG_FIELD_TYPE_KEY_VALUE_MAP,
				Required:    true,
			},
		},
	})
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

func (p *TypeMapperProvider) Enrich(ctx context.Context, act *pb.StandardizedActivity, user *pb.UserRecord, inputConfig map[string]string, doNotRetry bool) (*EnrichmentResult, error) {
	// Get type mappings config (stored as JSON object: {"OriginalType": "DesiredType"})
	mappingsJson, ok := inputConfig["type_mappings"]
	if !ok || mappingsJson == "" {
		// No mappings configured, nothing to do
		return &EnrichmentResult{}, nil
	}

	// Parse the JSON map
	var mappings map[string]string
	if err := json.Unmarshal([]byte(mappingsJson), &mappings); err != nil {
		// Silent failure on invalid config is safer than crashing pipeline
		return &EnrichmentResult{}, nil
	}

	// Get the current activity type as a string (friendly name)
	originalType := act.Type
	originalTypeName := activity.GetStravaActivityType(originalType)

	// Check if there's a mapping for this type
	for fromType, toType := range mappings {
		// Match case-insensitively
		if strings.EqualFold(fromType, originalTypeName) {
			// Parse the target type
			newType := activity.ParseActivityTypeFromString(toType)
			if newType != pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
				act.Type = newType
				return &EnrichmentResult{
					Metadata: map[string]string{
						"original_type": originalTypeName,
						"new_type":      activity.GetStravaActivityType(newType),
						"mapping_used":  fromType + " → " + toType,
					},
				}, nil
			}
		}
	}

	// No matching mapping found
	return &EnrichmentResult{}, nil
}
