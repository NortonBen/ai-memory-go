package vector

import "fmt"

// defaultVectorFactory implements VectorFactory for supported store types.
type defaultVectorFactory struct{}

// NewVectorFactory returns the default VectorFactory.
func NewVectorFactory() VectorFactory {
	return &defaultVectorFactory{}
}

// CreateVectorStore creates a VectorStore based on the provided config.
func (f *defaultVectorFactory) CreateVectorStore(config *VectorConfig) (VectorStore, error) {
	if config == nil {
		return nil, fmt.Errorf("vector config is required")
	}

	switch config.Type {
	case StoreTypeQdrant:
		return NewQdrantStore(config)
	case StoreTypePgVector:
		return NewPgVectorStore(config)
	case StoreTypeInMemory:
		return NewInMemoryStore(config), nil
	case StoreTypeSQLite:
		dbPath := config.Host // reuse Host as path for SQLite
		if dbPath == "" {
			dbPath = "memory_vectors.db"
		}
		return NewSQLiteVectorStore(dbPath, config.Dimension)
	default:
		return nil, fmt.Errorf("unsupported vector store type: %s", config.Type)
	}
}

// ListSupportedTypes returns the store types this factory supports.
func (f *defaultVectorFactory) ListSupportedTypes() []VectorStoreType {
	return []VectorStoreType{
		StoreTypeQdrant,
		StoreTypePgVector,
		StoreTypeInMemory,
		StoreTypeSQLite,
	}
}
