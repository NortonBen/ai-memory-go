// Package vector provides vector storage interfaces and implementations for embeddings.
// It supports Qdrant and pgvector for storing embeddings and performing similarity search.
package vector

import (
	"context"
	"fmt"
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
	StoreTypeLanceDB  VectorStoreType = "lancedb"
	StoreTypePgVector VectorStoreType = "pgvector"
	StoreTypeChromaDB VectorStoreType = "chromadb"
	StoreTypeRedis    VectorStoreType = "redis"
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

	// Provider-specific configurations
	Qdrant   *QdrantConfig   `json:"qdrant,omitempty"`
	LanceDB  *LanceDBConfig  `json:"lancedb,omitempty"`
	PgVector *PgVectorConfig `json:"pgvector,omitempty"`
	ChromaDB *ChromaDBConfig `json:"chromadb,omitempty"`
	Redis    *RedisConfig    `json:"redis,omitempty"`
}

// OpenRouterConfig holds configuration for OpenRouter provider
type OpenRouterConfig struct {
	APIKey  string `json:"api_key"`
	Model   string `json:"model,omitempty"`
	SiteURL string `json:"site_url,omitempty"`
	AppName string `json:"app_name,omitempty"`
}

// Qdrant-specific configuration
type QdrantConfig struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	APIKey string `json:"api_key,omitempty"`

	// Connection settings
	Timeout        time.Duration `json:"timeout"`
	MaxConnections int           `json:"max_connections"`
	EnableTLS      bool          `json:"enable_tls"`
	TLSConfig      *TLSConfig    `json:"tls_config,omitempty"`

	// Qdrant-specific settings
	Prefix           string `json:"prefix,omitempty"`
	EnableGRPC       bool   `json:"enable_grpc"`
	GRPCPort         int    `json:"grpc_port,omitempty"`
	CompressionLevel string `json:"compression_level"` // none, gzip, lz4

	// Collection settings
	DefaultVectorSize int    `json:"default_vector_size"`
	DefaultDistance   string `json:"default_distance"` // cosine, euclidean, dot
	OnDiskPayload     bool   `json:"on_disk_payload"`

	// Performance settings
	MaxSegmentSize    int64 `json:"max_segment_size"`
	MemmapThreshold   int64 `json:"memmap_threshold"`
	IndexingThreshold int64 `json:"indexing_threshold"`
}

// LanceDB-specific configuration
type LanceDBConfig struct {
	URI         string `json:"uri"`          // File path or S3/GCS URI
	StorageType string `json:"storage_type"` // local, s3, gcs, azure

	// Local storage settings
	DataDir   string `json:"data_dir,omitempty"`
	TableName string `json:"table_name"`

	// Cloud storage settings (S3/GCS/Azure)
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	Bucket          string `json:"bucket,omitempty"`

	// LanceDB-specific settings
	BlockSize       int   `json:"block_size"`
	MaxRowsPerFile  int   `json:"max_rows_per_file"`
	MaxRowsPerGroup int   `json:"max_rows_per_group"`
	MaxBytesPerFile int64 `json:"max_bytes_per_file"`

	// Index settings
	IndexCacheSize int    `json:"index_cache_size"`
	MetricType     string `json:"metric_type"` // l2, cosine, dot

	// Performance settings
	EnableStatistics  bool `json:"enable_statistics"`
	EnableBloomFilter bool `json:"enable_bloom_filter"`
}

// PgVector-specific configuration (PostgreSQL with pgvector extension)
type PgVectorConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"` // disable, require, verify-ca, verify-full

	// Connection pooling
	MaxConnections    int           `json:"max_connections"`
	MinConnections    int           `json:"min_connections"`
	MaxConnLifetime   time.Duration `json:"max_conn_lifetime"`
	MaxConnIdleTime   time.Duration `json:"max_conn_idle_time"`
	HealthCheckPeriod time.Duration `json:"health_check_period"`

	// pgvector-specific settings
	VectorDimensions int                    `json:"vector_dimensions"`
	IndexType        string                 `json:"index_type"` // ivfflat, hnsw
	IndexOptions     map[string]interface{} `json:"index_options,omitempty"`

	// Table settings
	TableName      string `json:"table_name"`
	VectorColumn   string `json:"vector_column"`
	MetadataColumn string `json:"metadata_column"`

	// Performance settings
	MaintenanceWorkMem string `json:"maintenance_work_mem,omitempty"`
	MaxParallelWorkers int    `json:"max_parallel_workers,omitempty"`
	EffectiveCacheSize string `json:"effective_cache_size,omitempty"`
}

// ChromaDB-specific configuration
type ChromaDBConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`

	// Authentication
	APIKey  string            `json:"api_key,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// Connection settings
	Timeout        time.Duration `json:"timeout"`
	MaxConnections int           `json:"max_connections"`
	EnableTLS      bool          `json:"enable_tls"`
	TLSConfig      *TLSConfig    `json:"tls_config,omitempty"`

	// ChromaDB-specific settings
	Tenant   string `json:"tenant,omitempty"`
	Database string `json:"database,omitempty"`

	// Collection settings
	DefaultCollectionName string                 `json:"default_collection_name"`
	CollectionMetadata    map[string]interface{} `json:"collection_metadata,omitempty"`

	// Embedding settings
	EmbeddingFunction string `json:"embedding_function,omitempty"`

	// Performance settings
	BatchSize         int    `json:"batch_size"`
	EnablePersistence bool   `json:"enable_persistence"`
	PersistDirectory  string `json:"persist_directory,omitempty"`
}

// Redis-specific configuration (for vector storage)
type RedisConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
	Database int    `json:"database"` // Redis database number

	// Connection pooling
	PoolSize           int           `json:"pool_size"`
	MinIdleConns       int           `json:"min_idle_conns"`
	MaxConnAge         time.Duration `json:"max_conn_age"`
	PoolTimeout        time.Duration `json:"pool_timeout"`
	IdleTimeout        time.Duration `json:"idle_timeout"`
	IdleCheckFrequency time.Duration `json:"idle_check_frequency"`

	// Redis-specific
	MaxRetries      int           `json:"max_retries"`
	MinRetryBackoff time.Duration `json:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `json:"max_retry_backoff"`
	DialTimeout     time.Duration `json:"dial_timeout"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`

	// RedisSearch/RediSearch settings for vector search
	IndexName       string `json:"index_name"`
	VectorField     string `json:"vector_field"`
	VectorAlgorithm string `json:"vector_algorithm"` // FLAT, HNSW
	VectorType      string `json:"vector_type"`      // FLOAT32, FLOAT64

	// HNSW-specific parameters
	HNSWMaxConnections int `json:"hnsw_max_connections,omitempty"`
	HNSWEfConstruction int `json:"hnsw_ef_construction,omitempty"`
	HNSWEfRuntime      int `json:"hnsw_ef_runtime,omitempty"`

	// TLS configuration
	TLSConfig *TLSConfig `json:"tls_config,omitempty"`
}

// TLS configuration for secure connections
type TLSConfig struct {
	Enabled            bool     `json:"enabled"`
	CertFile           string   `json:"cert_file,omitempty"`
	KeyFile            string   `json:"key_file,omitempty"`
	CAFile             string   `json:"ca_file,omitempty"`
	ServerName         string   `json:"server_name,omitempty"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify"`
	MinVersion         string   `json:"min_version,omitempty"` // 1.0, 1.1, 1.2, 1.3
	MaxVersion         string   `json:"max_version,omitempty"`
	CipherSuites       []string `json:"cipher_suites,omitempty"`
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

// VectorStoreFactory defines the function signature for creating a VectorStore
type VectorStoreFactory func(config *VectorConfig) (VectorStore, error)

var storeRegistry = make(map[VectorStoreType]VectorStoreFactory)

// RegisterStore registers a new vector store implementation
func RegisterStore(storeType VectorStoreType, factory VectorStoreFactory) {
	storeRegistry[storeType] = factory
}

// NewVectorStore creates a new vector store using the registered factory
func NewVectorStore(config *VectorConfig) (VectorStore, error) {
	if config == nil {
		return nil, fmt.Errorf("vector config is required")
	}

	factory, ok := storeRegistry[config.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported vector store type: %s", config.Type)
	}

	return factory(config)
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

// EmbeddingProviderType defines supported embedding provider types
type EmbeddingProviderType string

const (
	EmbeddingProviderOpenAI     EmbeddingProviderType = "openai"
	EmbeddingProviderOllama     EmbeddingProviderType = "ollama"
	EmbeddingProviderOpenRouter EmbeddingProviderType = "openrouter"
	EmbeddingProviderLMStudio   EmbeddingProviderType = "lmstudio"
)

// EmbeddingProviderFactory defines the function signature for creating an EmbeddingProvider
type EmbeddingProviderFactory func(config map[string]interface{}) (EmbeddingProvider, error)

var embedderRegistry = make(map[EmbeddingProviderType]EmbeddingProviderFactory)

// RegisterEmbeddingProvider registers a new embedding provider implementation
func RegisterEmbeddingProvider(providerType EmbeddingProviderType, factory EmbeddingProviderFactory) {
	embedderRegistry[providerType] = factory
}

// NewEmbeddingProvider creates a new embedding provider using the registered factory
func NewEmbeddingProvider(providerType EmbeddingProviderType, config map[string]interface{}) (EmbeddingProvider, error) {
	factory, ok := embedderRegistry[providerType]
	if !ok {
		return nil, fmt.Errorf("unsupported embedding provider type: %s", providerType)
	}

	return factory(config)
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

// NewSQLiteVectorStore creates a new SQLite-based vector store (legacy facade)
func NewSQLiteVectorStore(dbPath string, dimension int) (VectorStore, error) {
	return NewVectorStore(&VectorConfig{
		Type:      StoreTypeSQLite,
		Database:  dbPath,
		Dimension: dimension,
	})
}

// NewInMemoryStore creates a new in-memory vector store (legacy facade)
func NewInMemoryStore(dimension interface{}) VectorStore {
	dim := 768
	if d, ok := dimension.(int); ok {
		dim = d
	}
	store, _ := NewVectorStore(&VectorConfig{
		Type:      StoreTypeInMemory,
		Dimension: dim,
	})
	return store
}

// NewRedisVectorStore creates a new Redis-based vector store (legacy facade)
func NewRedisVectorStore(endpoint, password string, dimension int) (VectorStore, error) {
	return NewVectorStore(&VectorConfig{
		Type:      StoreTypeRedis,
		Host:      endpoint,
		Password:  password,
		Dimension: dimension,
	})
}

// NewPgVectorStore creates a new pgvector-based vector store (legacy facade)
func NewPgVectorStore(config *VectorConfig) (VectorStore, error) {
	return NewVectorStore(config)
}

// NewQdrantStore creates a new Qdrant-based vector store (legacy facade)
func NewQdrantStore(config *VectorConfig) (VectorStore, error) {
	return NewVectorStore(config)
}

// NewLMStudioEmbeddingProvider creates an LMStudio embedding provider (legacy facade)
func NewLMStudioEmbeddingProvider(endpoint, model string) EmbeddingProvider {
	provider, _ := NewEmbeddingProvider(EmbeddingProviderLMStudio, map[string]interface{}{
		"endpoint": endpoint,
		"model":    model,
	})
	return provider
}

// NewOpenAIEmbeddingProvider creates an OpenAI embedding provider (legacy facade)
func NewOpenAIEmbeddingProvider(apiKey, model string) EmbeddingProvider {
	provider, _ := NewEmbeddingProvider(EmbeddingProviderOpenAI, map[string]interface{}{
		"api_key": apiKey,
		"model":   model,
	})
	return provider
}

// NewOllamaEmbeddingProvider creates an Ollama embedding provider (legacy facade)
func NewOllamaEmbeddingProvider(endpoint, model string, dimensions int) EmbeddingProvider {
	provider, _ := NewEmbeddingProvider(EmbeddingProviderOllama, map[string]interface{}{
		"endpoint":   endpoint,
		"model":      model,
		"dimensions": dimensions,
	})
	return provider
}

// NewOpenRouterEmbeddingProvider creates an OpenRouter embedding provider (legacy facade)
func NewOpenRouterEmbeddingProvider(config OpenRouterConfig) EmbeddingProvider {
	provider, _ := NewEmbeddingProvider(EmbeddingProviderOpenRouter, map[string]interface{}{
		"api_key":  config.APIKey,
		"model":    config.Model,
		"site_url": config.SiteURL,
		"app_name": config.AppName,
	})
	return provider
}
