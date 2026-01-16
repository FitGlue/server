package plugin

import (
	"sync"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	registryMu   sync.RWMutex
	sources      = make(map[string]*pb.PluginManifest)
	enrichers    = make(map[pb.EnricherProviderType]*pb.PluginManifest)
	destinations = make(map[pb.Destination]*pb.PluginManifest)
)

// RegisterSource registers a source plugin manifest
func RegisterSource(manifest *pb.PluginManifest) {
	registryMu.Lock()
	defer registryMu.Unlock()
	sources[manifest.Id] = manifest
}

// RegisterEnricher registers an enricher plugin manifest
func RegisterEnricher(providerType pb.EnricherProviderType, manifest *pb.PluginManifest) {
	registryMu.Lock()
	defer registryMu.Unlock()
	// Store the provider type in the manifest for API consumers
	pt := int32(providerType)
	manifest.EnricherProviderType = &pt
	enrichers[providerType] = manifest
}

// RegisterDestination registers a destination plugin manifest
func RegisterDestination(dest pb.Destination, manifest *pb.PluginManifest) {
	registryMu.Lock()
	defer registryMu.Unlock()
	dt := int32(dest)
	manifest.DestinationType = &dt
	destinations[dest] = manifest
}

// GetRegistry returns all registered plugins
func GetRegistry() *pb.PluginRegistryResponse {
	registryMu.RLock()
	defer registryMu.RUnlock()

	resp := &pb.PluginRegistryResponse{}
	for _, m := range sources {
		resp.Sources = append(resp.Sources, m)
	}
	for _, m := range enrichers {
		resp.Enrichers = append(resp.Enrichers, m)
	}
	for _, m := range destinations {
		resp.Destinations = append(resp.Destinations, m)
	}
	return resp
}

// GetEnricherManifest returns a specific enricher's manifest
func GetEnricherManifest(providerType pb.EnricherProviderType) (*pb.PluginManifest, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	m, ok := enrichers[providerType]
	return m, ok
}

// GetSourceManifest returns a specific source's manifest
func GetSourceManifest(id string) (*pb.PluginManifest, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	m, ok := sources[id]
	return m, ok
}

// GetDestinationManifest returns a specific destination's manifest
func GetDestinationManifest(dest pb.Destination) (*pb.PluginManifest, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	m, ok := destinations[dest]
	return m, ok
}

// ClearRegistry removes all plugins (useful for tests)
func ClearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	sources = make(map[string]*pb.PluginManifest)
	enrichers = make(map[pb.EnricherProviderType]*pb.PluginManifest)
	destinations = make(map[pb.Destination]*pb.PluginManifest)
}
