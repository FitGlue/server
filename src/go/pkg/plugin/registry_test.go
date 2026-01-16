package plugin

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestRegisterSource(t *testing.T) {
	ClearRegistry()

	manifest := &pb.PluginManifest{
		Id:          "test-source",
		Type:        pb.PluginType_PLUGIN_TYPE_SOURCE,
		Name:        "Test Source",
		Description: "A test source",
		Icon:        "🧪",
		Enabled:     true,
	}

	RegisterSource(manifest)

	got, ok := GetSourceManifest("test-source")
	if !ok {
		t.Fatal("expected source to be registered")
	}
	if got.Name != "Test Source" {
		t.Errorf("got name %q, want %q", got.Name, "Test Source")
	}
}

func TestRegisterEnricher(t *testing.T) {
	ClearRegistry()

	manifest := &pb.PluginManifest{
		Id:          "test-enricher",
		Type:        pb.PluginType_PLUGIN_TYPE_ENRICHER,
		Name:        "Test Enricher",
		Description: "A test enricher",
		Icon:        "✨",
		ConfigSchema: []*pb.ConfigFieldSchema{
			{
				Key:          "format",
				Label:        "Format",
				FieldType:    pb.ConfigFieldType_CONFIG_FIELD_TYPE_SELECT,
				Required:     true,
				DefaultValue: "compact",
				Options: []*pb.ConfigFieldOption{
					{Value: "compact", Label: "Compact"},
					{Value: "detailed", Label: "Detailed"},
				},
			},
		},
		Enabled: true,
	}

	RegisterEnricher(pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK, manifest)

	got, ok := GetEnricherManifest(pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK)
	if !ok {
		t.Fatal("expected enricher to be registered")
	}
	if got.Name != "Test Enricher" {
		t.Errorf("got name %q, want %q", got.Name, "Test Enricher")
	}
	if got.EnricherProviderType == nil || *got.EnricherProviderType != int32(pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK) {
		t.Error("expected enricher_provider_type to be set")
	}
	if len(got.ConfigSchema) != 1 {
		t.Errorf("expected 1 config field, got %d", len(got.ConfigSchema))
	}
}

func TestRegisterDestination(t *testing.T) {
	ClearRegistry()

	manifest := &pb.PluginManifest{
		Id:          "test-dest",
		Type:        pb.PluginType_PLUGIN_TYPE_DESTINATION,
		Name:        "Test Destination",
		Description: "A test destination",
		Icon:        "📤",
		Enabled:     true,
	}

	RegisterDestination(pb.Destination_DESTINATION_MOCK, manifest)

	got, ok := GetDestinationManifest(pb.Destination_DESTINATION_MOCK)
	if !ok {
		t.Fatal("expected destination to be registered")
	}
	if got.Name != "Test Destination" {
		t.Errorf("got name %q, want %q", got.Name, "Test Destination")
	}
	if got.DestinationType == nil || *got.DestinationType != int32(pb.Destination_DESTINATION_MOCK) {
		t.Error("expected destination_type to be set")
	}
}

func TestGetRegistry(t *testing.T) {
	ClearRegistry()

	RegisterSource(&pb.PluginManifest{Id: "src1", Type: pb.PluginType_PLUGIN_TYPE_SOURCE})
	RegisterSource(&pb.PluginManifest{Id: "src2", Type: pb.PluginType_PLUGIN_TYPE_SOURCE})
	RegisterEnricher(pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK, &pb.PluginManifest{Id: "enr1", Type: pb.PluginType_PLUGIN_TYPE_ENRICHER})
	RegisterDestination(pb.Destination_DESTINATION_STRAVA, &pb.PluginManifest{Id: "dest1", Type: pb.PluginType_PLUGIN_TYPE_DESTINATION})

	registry := GetRegistry()

	if len(registry.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(registry.Sources))
	}
	if len(registry.Enrichers) != 1 {
		t.Errorf("expected 1 enricher, got %d", len(registry.Enrichers))
	}
	if len(registry.Destinations) != 1 {
		t.Errorf("expected 1 destination, got %d", len(registry.Destinations))
	}
}

func TestClearRegistry(t *testing.T) {
	RegisterSource(&pb.PluginManifest{Id: "test"})
	ClearRegistry()

	registry := GetRegistry()
	if len(registry.Sources) != 0 {
		t.Error("expected registry to be cleared")
	}
}

func TestGetManifestNotFound(t *testing.T) {
	ClearRegistry()

	_, ok := GetSourceManifest("nonexistent")
	if ok {
		t.Error("expected source not to be found")
	}

	_, ok = GetEnricherManifest(pb.EnricherProviderType_ENRICHER_PROVIDER_MOCK)
	if ok {
		t.Error("expected enricher not to be found")
	}

	_, ok = GetDestinationManifest(pb.Destination_DESTINATION_STRAVA)
	if ok {
		t.Error("expected destination not to be found")
	}
}
