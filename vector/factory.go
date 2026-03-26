package vector



// defaultVectorFactory implements VectorFactory for supported store types.
type defaultVectorFactory struct{}

// NewVectorFactory returns the default VectorFactory.
func NewVectorFactory() VectorFactory {
	return &defaultVectorFactory{}
}

// CreateVectorStore creates a VectorStore based on the provided config.
func (f *defaultVectorFactory) CreateVectorStore(config *VectorConfig) (VectorStore, error) {
	return NewVectorStore(config)
}

// ListSupportedTypes returns the store types this factory supports.
func (f *defaultVectorFactory) ListSupportedTypes() []VectorStoreType {
	// Return a static list of "standard" types or dynamic from registry
	// For backward compatibility, we return what was there before
	return []VectorStoreType{
		StoreTypeQdrant,
		StoreTypePgVector,
		StoreTypeInMemory,
		StoreTypeSQLite,
		StoreTypeRedis,
	}
}
