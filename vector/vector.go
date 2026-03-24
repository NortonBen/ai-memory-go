// Package vector provides vector storage interfaces and implementations for embeddings.
// It supports Qdrant and pgvector for storing embeddings and performing similarity search.
package vector

import (
	"context"
	"time"
)

// VectorStore defines the interface for vector database operations
type VectorStore interface {
	// Embedding operations
	StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error
	GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error)
	UpdateEmbedding(ctx context.Context, id string, embedding []float32) error
	DeleteEmbedding(ctx context.Context, id string) error

	// Similarity search operations
	SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*SimilarityResult, error)
	SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*SimilarityResult, error)

	// Batch operations
	StoreBatchEmbeddings(ctx context.Context, embeddings []*EmbeddingData) error
	DeleteBatchEmbeddings(ctx context.Context, ids []string) error

	// Collection/Index management
	CreateCollection(ctx context.Context, name string, dimension int, config *CollectionConfig) error
	DeleteCollection(ctx context.Context, name string) error
	ListCollections(ctx context.Context) ([]string, error)

	// Analytics and metrics
	GetCollectionInfo(ctx context.Context, name string) (*CollectionInfo, error)
	GetEmbeddingCount(ctx context.Context) (int64, error)

	// Health and lifecycle
	Health(ctx context.Context) error
	Close() error
}

// VectorStoreType defines supported vector database types
type VectorStoreType string

const (
	StoreTypeQdrant   VectorStoreType = "qdrant"
	StoreTypePgVector VectorStoreType = "pgvector"
	StoreTypeInMemory VectorStoreType = "inmemory"
	StoreTypeSQLite   VectorStoreType = "sqlite"
)

// VectorConfig holds configuration for vector stores
type VectorConfig struct {
	Type       VectorStoreType        `json:"type"`
	Host       string                 `json:"host,omitempty"`
	Port       int                    `json:"port,omitempty"`
	Database   string                 `json:"database,omitempty"`
	Collection string                 `json:"collection,omitempty"`
	Username   string                 `json:"username,omitempty"`
	Password   string                 `json:"password,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`

	// Vector-specific settings
	Dimension      int    `json:"dimension"`
	DistanceMetric string `json:"distance_metric"` // cosine, euclidean, dot_product
	IndexType      string `json:"index_type"`      // hnsw, ivf, flat

	// Connection pooling
	MaxConnections int           `json:"max_connections"`
	ConnTimeout    time.Duration `json:"conn_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout"`

	// Performance settings
	BatchSize     int  `json:"batch_size"`
	EnableCaching bool `json:"enable_caching"`
}

// DefaultVectorConfig returns sensible defaults
func DefaultVectorConfig() *VectorConfig {
	return &VectorConfig{
		Type:           StoreTypeInMemory,
		Dimension:      768, // Common embedding dimension
		DistanceMetric: "cosine",
		IndexType:      "hnsw",
		MaxConnections: 10,
		ConnTimeout:    30 * time.Second,
		IdleTimeout:    5 * time.Minute,
		BatchSize:      100,
		EnableCaching:  true,
	}
}

// EmbeddingData represents an embedding with metadata
type EmbeddingData struct {
	ID        string                 `json:"id"`
	Embedding []float32              `json:"embedding"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// SimilarityResult represents a similarity search result
type SimilarityResult struct {
	ID        string                 `json:"id"`
	Score     float64                `json:"score"`
	Embedding []float32              `json:"embedding,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Distance  float64                `json:"distance,omitempty"`
}

// CollectionConfig configures vector collection creation
type CollectionConfig struct {
	Dimension      int                    `json:"dimension"`
	DistanceMetric string                 `json:"distance_metric"`
	IndexType      string                 `json:"index_type"`
	IndexParams    map[string]interface{} `json:"index_params,omitempty"`
	Replicas       int                    `json:"replicas,omitempty"`
	ShardCount     int                    `json:"shard_count,omitempty"`
}

// CollectionInfo provides information about a vector collection
type CollectionInfo struct {
	Name           string                 `json:"name"`
	Dimension      int                    `json:"dimension"`
	DistanceMetric string                 `json:"distance_metric"`
	IndexType      string                 `json:"index_type"`
	VectorCount    int64                  `json:"vector_count"`
	IndexedCount   int64                  `json:"indexed_count"`
	Status         string                 `json:"status"`
	Config         map[string]interface{} `json:"config,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// SearchOptions configures similarity search behavior
type SearchOptions struct {
	Limit           int                    `json:"limit"`
	Threshold       float64                `json:"threshold"`
	Filters         map[string]interface{} `json:"filters,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata"`
	IncludeVectors  bool                   `json:"include_vectors"`
	Offset          int                    `json:"offset,omitempty"`
}

// DefaultSearchOptions returns sensible defaults
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		Limit:           10,
		Threshold:       0.0,
		IncludeMetadata: true,
		IncludeVectors:  false,
		Offset:          0,
	}
}

// VectorMetrics provides analytics about vector storage
type VectorMetrics struct {
	TotalVectors     int64                      `json:"total_vectors"`
	Collections      map[string]*CollectionInfo `json:"collections"`
	StorageSize      int64                      `json:"storage_size_bytes"`
	IndexSize        int64                      `json:"index_size_bytes"`
	AverageQueryTime time.Duration              `json:"average_query_time"`
	LastUpdated      time.Time                  `json:"last_updated"`
}

// VectorFactory creates vector store instances
type VectorFactory interface {
	CreateVectorStore(config *VectorConfig) (VectorStore, error)
	ListSupportedTypes() []VectorStoreType
}

// EmbeddingProvider defines the interface for generating embeddings
type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	GetDimensions() int
	GetModel() string
	Health(ctx context.Context) error
}

// EmbeddingCache defines caching interface for embeddings
type EmbeddingCache interface {
	Get(ctx context.Context, key string) ([]float32, bool)
	Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// VectorIndex defines indexing strategies for vector search
type VectorIndex struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // hnsw, ivf, flat, pq
	Dimension  int                    `json:"dimension"`
	Metric     string                 `json:"metric"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Status     string                 `json:"status"`
	CreatedAt  time.Time              `json:"created_at"`
}

// VectorQuery represents a complex vector query
type VectorQuery struct {
	Vector    []float32              `json:"vector"`
	Filters   map[string]interface{} `json:"filters,omitempty"`
	Limit     int                    `json:"limit"`
	Threshold float64                `json:"threshold"`
	Offset    int                    `json:"offset,omitempty"`
	Include   []string               `json:"include,omitempty"` // metadata, vectors, distances
}

// QueryResult represents the result of a vector query
type QueryResult struct {
	Results   []*SimilarityResult    `json:"results"`
	Total     int64                  `json:"total"`
	QueryTime time.Duration          `json:"query_time"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
