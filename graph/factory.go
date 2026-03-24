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
	switch config.Type {
	case StoreTypeNeo4j:
		return NewNeo4jStore(config)
	case StoreTypeSQLite:
		dbPath := config.Database
		if dbPath == "" {
			dbPath = "memory_graph.db"
		}
		return NewSQLiteGraphStore(dbPath)
	case StoreTypeInMemory:
		return NewInMemoryGraphStore(), nil
	default:
		return nil, fmt.Errorf("unsupported graph store type: %s", config.Type)
	}
}

// ListSupportedTypes returns all supported graph store types.
func (f *defaultGraphFactory) ListSupportedTypes() []GraphStoreType {
	return []GraphStoreType{
		StoreTypeNeo4j,
		StoreTypeSQLite,
		StoreTypeInMemory,
	}
}
