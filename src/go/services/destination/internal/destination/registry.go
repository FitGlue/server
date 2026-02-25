package destination

import (
	"sync"

	"github.com/fitglue/server/src/go/pkg/destination"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
)

// Registry maps destination Enums to Destination interface implementations
type Registry struct {
	mu        sync.RWMutex
	uploaders map[pbplugin.DestinationType]destination.Destination
}

// NewRegistry initializes an empty Registry
func NewRegistry() *Registry {
	return &Registry{
		uploaders: make(map[pbplugin.DestinationType]destination.Destination),
	}
}

// Register adds an uploader to the registry
func (r *Registry) Register(destType pbplugin.DestinationType, uploader destination.Destination) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uploaders[destType] = uploader
}

// Get finds the appropriate uploader for an activity destination
func (r *Registry) Get(destType pbplugin.DestinationType) (destination.Destination, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	uploader, ok := r.uploaders[destType]
	return uploader, ok
}
