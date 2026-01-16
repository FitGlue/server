package enricher_providers

import (
	"log"
	"sync"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	registryMu   sync.RWMutex
	registry     = make(map[string]Provider)
	typeRegistry = make(map[pb.EnricherProviderType]Provider)
)

// Register adds a provider to the registry.
// Ideally called in init() functions of providers.
func Register(p Provider) {
	registryMu.Lock()
	defer registryMu.Unlock()

	name := p.Name()
	if _, exists := registry[name]; exists {
		log.Panicf("Provider already registered for name: %s", name)
	}
	registry[name] = p

	// Register by type if it has a specific type
	t := p.ProviderType()
	if t != pb.EnricherProviderType_ENRICHER_PROVIDER_UNSPECIFIED {
		// Warn on duplicate types? Or panic?
		// Panic seems appropriate as types should be unique
		if _, exists := typeRegistry[t]; exists {
			log.Panicf("Provider already registered for type: %v", t)
		}
		typeRegistry[t] = p
	}
}

// GetAll returns all registered providers.
func GetAll() []Provider {
	registryMu.RLock()
	defer registryMu.RUnlock()

	providers := make([]Provider, 0, len(registry))
	for _, p := range registry {
		providers = append(providers, p)
	}
	return providers
}

// GetByName returns a specific provider by name.
func GetByName(name string) (Provider, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	p, ok := registry[name]
	return p, ok
}

// GetByType returns a specific provider by type.
func GetByType(t pb.EnricherProviderType) (Provider, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	p, ok := typeRegistry[t]
	return p, ok
}

// ClearRegistry removes all providers (useful for tests)
func ClearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]Provider)
	typeRegistry = make(map[pb.EnricherProviderType]Provider)
}
