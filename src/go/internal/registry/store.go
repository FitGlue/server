// nolint:proto-json
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	_ "embed"

	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	"google.golang.org/protobuf/encoding/protojson"
)

//go:embed data/registry.json
var registryJSON []byte

type Store interface {
	ListPlugins(ctx context.Context, category string) ([]*pbplugin.PluginManifest, error)
	GetPlugin(ctx context.Context, id string) (*pbplugin.PluginManifest, error)
	ListCategories(ctx context.Context) ([]string, error)
	ListSources(ctx context.Context) ([]*pbplugin.PluginManifest, error)
	ListDestinations(ctx context.Context) ([]*pbplugin.PluginManifest, error)
	ListIntegrations(ctx context.Context) ([]*pbplugin.IntegrationManifest, error)
	GetFullRegistry(ctx context.Context) (*pbplugin.PluginRegistryResponse, error)
	GetPluginIcon(ctx context.Context, id string) ([]byte, string, error)
}

type staticStore struct {
	data *pbplugin.PluginRegistryResponse
}

func NewStaticStore() (Store, error) {
	// We need to unmarshal from the embedded JSON via a map first to strip unsupported fields
	// but protojson with DiscardUnknown handles this directly.
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	var resp pbplugin.PluginRegistryResponse
	if err := unmarshaler.Unmarshal(registryJSON, &resp); err != nil {
		// Fallback to manual map unmarshal to debug
		var raw map[string]interface{}
		if err2 := json.Unmarshal(registryJSON, &raw); err2 != nil {
			return nil, fmt.Errorf("invalid json: %w", err2)
		}
		// Convert back to JSON and try again
		cleanJSON, _ := json.Marshal(raw)
		if err := unmarshaler.Unmarshal(cleanJSON, &resp); err != nil {
			return nil, fmt.Errorf("protojson unmarshal failed: %w", err)
		}
	}

	return &staticStore{data: &resp}, nil
}

func (s *staticStore) ListPlugins(ctx context.Context, category string) ([]*pbplugin.PluginManifest, error) {
	var results []*pbplugin.PluginManifest

	// Combine all plugins
	all := make([]*pbplugin.PluginManifest, 0)
	all = append(all, s.data.Sources...)
	all = append(all, s.data.Enrichers...)
	all = append(all, s.data.Destinations...)

	for _, p := range all {
		if category == "" || strings.EqualFold(p.GetCategory(), category) {
			results = append(results, p)
		}
	}
	// Sort order handled by TS already, but here we just return the filtered list
	return results, nil
}

func (s *staticStore) GetPlugin(ctx context.Context, id string) (*pbplugin.PluginManifest, error) {
	all := make([]*pbplugin.PluginManifest, 0)
	all = append(all, s.data.Sources...)
	all = append(all, s.data.Enrichers...)
	all = append(all, s.data.Destinations...)

	for _, p := range all {
		if p.Id == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("plugin not found: %s", id)
}

func (s *staticStore) ListCategories(ctx context.Context) ([]string, error) {
	categories := make(map[string]bool)
	all := make([]*pbplugin.PluginManifest, 0)
	all = append(all, s.data.Sources...)
	all = append(all, s.data.Enrichers...)
	all = append(all, s.data.Destinations...)

	for _, p := range all {
		if p.GetCategory() != "" {
			categories[p.GetCategory()] = true
		}
	}

	var results []string
	for c := range categories {
		results = append(results, c)
	}
	return results, nil
}

func (s *staticStore) ListSources(ctx context.Context) ([]*pbplugin.PluginManifest, error) {
	return s.data.Sources, nil
}

func (s *staticStore) ListDestinations(ctx context.Context) ([]*pbplugin.PluginManifest, error) {
	return s.data.Destinations, nil
}

func (s *staticStore) ListIntegrations(ctx context.Context) ([]*pbplugin.IntegrationManifest, error) {
	return s.data.Integrations, nil
}

func (s *staticStore) GetFullRegistry(ctx context.Context) (*pbplugin.PluginRegistryResponse, error) {
	return s.data, nil
}

func (s *staticStore) GetPluginIcon(ctx context.Context, id string) ([]byte, string, error) {
	// Not implemented by default in static store without disk access
	// Could be enhanced to read from local file system
	return nil, "", fmt.Errorf("GetPluginIcon not implemented in static store")
}
