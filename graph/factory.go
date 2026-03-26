package graph

import "fmt"

// defaultGraphFactory implements GraphFactory.
type defaultGraphFactory struct{}

// NewGraphFactory returns the default GraphFactory.
func NewGraphFactory() GraphFactory {
	return &defaultGraphFactory{}
}

// CreateGraphStore creates a GraphStore based on the config type.
func (f *defaultGraphFactory) CreateGraphStore(config *GraphConfig) (GraphStore, error) {
	if config == nil {
		return nil, fmt.Errorf("graph config is required")
	}
	return NewStore(config)
}

// ListSupportedTypes returns all supported graph store types.
func (f *defaultGraphFactory) ListSupportedTypes() []GraphStoreType {
	return GetRegisteredStores()
}
