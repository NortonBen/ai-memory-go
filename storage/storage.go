// Package storage provides storage layer abstractions and implementations.
// It defines unified interfaces for relational databases (PostgreSQL, SQLite)
// and coordinates between graph, vector, and relational storage backends.
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/vector"
)

// Storage defines the unified storage interface combining all storage types
type Storage interface {
	// DataPoint operations
	StoreDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error
	GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error)
	UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error
	DeleteDataPoint(ctx context.Context, id string) error
	QueryDataPoints(ctx context.Context, query *DataPointQuery) ([]*schema.DataPoint, error)

	// Session operations
	StoreSession(ctx context.Context, session *schema.MemorySession) error
	GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error)
	UpdateSession(ctx context.Context, session *schema.MemorySession) error
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error)

	// Batch operations
	StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error
	DeleteBatch(ctx context.Context, ids []string) error

	// Health and lifecycle
	Health(ctx context.Context) error
	Close() error
}

// RelationalStore defines the interface for relational database operations
type RelationalStore interface {
	// DataPoint operations
	StoreDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error
	GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error)
	UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error
	DeleteDataPoint(ctx context.Context, id string) error

	// Query operations
	QueryDataPoints(ctx context.Context, query *DataPointQuery) ([]*schema.DataPoint, error)
	SearchDataPoints(ctx context.Context, searchQuery string, filters map[string]interface{}) ([]*schema.DataPoint, error)

	// Session operations
	StoreSession(ctx context.Context, session *schema.MemorySession) error
	GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error)
	UpdateSession(ctx context.Context, session *schema.MemorySession) error
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error)

	// Batch operations
	StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error
	DeleteBatch(ctx context.Context, ids []string) error

	// Analytics
	GetDataPointCount(ctx context.Context) (int64, error)
	GetSessionCount(ctx context.Context) (int64, error)

	// Health and lifecycle
	Health(ctx context.Context) error
	Close() error
}

// StorageType defines supported storage backend types
type StorageType string

const (
	StorageTypePostgreSQL StorageType = "postgresql"
	StorageTypeSQLite     StorageType = "sqlite"
	StorageTypeInMemory   StorageType = "inmemory"
)

// StorageConfig holds configuration for storage backends
type StorageConfig struct {
	// Relational store config
	Relational *RelationalConfig `json:"relational"`

	// Graph store config
	Graph *GraphConfig `json:"graph"`

	// Vector store config
	Vector *VectorConfig `json:"vector"`

	// Global settings
	EnableTransactions bool          `json:"enable_transactions"`
	ConnectionTimeout  time.Duration `json:"connection_timeout"`
	QueryTimeout       time.Duration `json:"query_timeout"`
	MaxRetries         int           `json:"max_retries"`

	// Environment and deployment settings
	Environment string            `json:"environment"` // development, staging, production
	ConfigFile  string            `json:"config_file,omitempty"`
	Profiles    map[string]string `json:"profiles,omitempty"` // Named configuration profiles
}

// GraphConfig holds configuration for graph stores
type GraphConfig struct {
	Type     GraphStoreType         `json:"type"`
	Host     string                 `json:"host,omitempty"`
	Port     int                    `json:"port,omitempty"`
	Database string                 `json:"database,omitempty"`
	Username string                 `json:"username,omitempty"`
	Password string                 `json:"password,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`

	// Connection pooling
	MaxConnections int           `json:"max_connections"`
	ConnTimeout    time.Duration `json:"conn_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout"`

	// Performance settings
	BatchSize      int  `json:"batch_size"`
	EnableIndexing bool `json:"enable_indexing"`
	EnableCaching  bool `json:"enable_caching"`

	// Provider-specific configurations
	Neo4j     *Neo4jConfig     `json:"neo4j,omitempty"`
	SurrealDB *SurrealDBConfig `json:"surrealdb,omitempty"`
	Kuzu      *KuzuConfig      `json:"kuzu,omitempty"`
	FalkorDB  *FalkorDBConfig  `json:"falkordb,omitempty"`
}

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

// RelationalConfig holds configuration for relational databases
type RelationalConfig struct {
	Type     StorageType            `json:"type"`
	Host     string                 `json:"host,omitempty"`
	Port     int                    `json:"port,omitempty"`
	Database string                 `json:"database"`
	Username string                 `json:"username,omitempty"`
	Password string                 `json:"password,omitempty"`
	SSLMode  string                 `json:"ssl_mode,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`

	// Connection pooling
	MaxConnections int           `json:"max_connections"`
	MinConnections int           `json:"min_connections"`
	ConnTimeout    time.Duration `json:"conn_timeout"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	MaxLifetime    time.Duration `json:"max_lifetime"`

	// Performance settings
	BatchSize         int  `json:"batch_size"`
	EnableFullText    bool `json:"enable_full_text"`
	EnableJSONIndexes bool `json:"enable_json_indexes"`
}

// DefaultStorageConfig returns sensible defaults
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		Relational: &RelationalConfig{
			Type:              StorageTypeSQLite,
			Database:          "memory.db",
			MaxConnections:    10,
			MinConnections:    1,
			ConnTimeout:       30 * time.Second,
			IdleTimeout:       5 * time.Minute,
			MaxLifetime:       1 * time.Hour,
			BatchSize:         100,
			EnableFullText:    true,
			EnableJSONIndexes: true,
		},
		Graph:              DefaultGraphConfig(),
		Vector:             DefaultVectorConfig(),
		EnableTransactions: true,
		ConnectionTimeout:  30 * time.Second,
		QueryTimeout:       30 * time.Second,
		MaxRetries:         3,
		Environment:        "development",
		Profiles:           make(map[string]string),
	}
}

// DataPointQuery defines query parameters for DataPoint searches
type DataPointQuery struct {
	// Filtering
	SessionID   string                 `json:"session_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	ContentType string                 `json:"content_type,omitempty"`
	Filters     map[string]interface{} `json:"filters,omitempty"`

	// Text search
	SearchText string `json:"search_text,omitempty"`
	SearchMode string `json:"search_mode,omitempty"` // fulltext, fuzzy, exact

	// Time range
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	UpdatedAfter  *time.Time `json:"updated_after,omitempty"`
	UpdatedBefore *time.Time `json:"updated_before,omitempty"`

	// Pagination and sorting
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"` // asc, desc

	// Include options
	IncludeEmbeddings    bool `json:"include_embeddings"`
	IncludeRelationships bool `json:"include_relationships"`
}

// DefaultGraphConfig returns sensible defaults for graph storage
func DefaultGraphConfig() *GraphConfig {
	return &GraphConfig{
		Type:           GraphStoreTypeInMemory,
		MaxConnections: 10,
		ConnTimeout:    30 * time.Second,
		IdleTimeout:    5 * time.Minute,
		BatchSize:      100,
		EnableIndexing: true,
		EnableCaching:  true,
	}
}

// DefaultVectorConfig returns sensible defaults for vector storage
func DefaultVectorConfig() *VectorConfig {
	return &VectorConfig{
		Type:           VectorStoreTypeInMemory,
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

// DefaultDataPointQuery returns sensible defaults
func DefaultDataPointQuery() *DataPointQuery {
	return &DataPointQuery{
		Limit:                100,
		Offset:               0,
		SortBy:               "created_at",
		SortOrder:            "desc",
		SearchMode:           "fulltext",
		IncludeEmbeddings:    false,
		IncludeRelationships: true,
	}
}

// HybridStorage combines graph, vector, and relational storage
type HybridStorage struct {
	relational RelationalStore
	graph      graph.GraphStore
	vector     vector.VectorStore
	config     *StorageConfig
}

// StorageMetrics provides analytics about storage usage
type StorageMetrics struct {
	DataPointCount int64                 `json:"datapoint_count"`
	SessionCount   int64                 `json:"session_count"`
	StorageSize    int64                 `json:"storage_size_bytes"`
	GraphMetrics   *graph.GraphMetrics   `json:"graph_metrics,omitempty"`
	VectorMetrics  *vector.VectorMetrics `json:"vector_metrics,omitempty"`
	LastUpdated    time.Time             `json:"last_updated"`
}

// StorageFactory creates storage instances
type StorageFactory interface {
	CreateStorage(config *StorageConfig) (Storage, error)
	CreateRelationalStore(config *RelationalConfig) (RelationalStore, error)
	ListSupportedTypes() []StorageType
}

// Transaction defines transaction operations across storage backends
type Transaction interface {
	// DataPoint operations
	StoreDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error
	UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error
	DeleteDataPoint(ctx context.Context, id string) error

	// Session operations
	StoreSession(ctx context.Context, session *schema.MemorySession) error
	UpdateSession(ctx context.Context, session *schema.MemorySession) error

	// Transaction control
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// TransactionalStorage extends Storage with transaction support
type TransactionalStorage interface {
	Storage
	BeginTransaction(ctx context.Context) (Transaction, error)
}

// StorageIndex defines indexing strategies for performance
type StorageIndex struct {
	Name      string                 `json:"name"`
	Table     string                 `json:"table"`
	Columns   []string               `json:"columns"`
	IndexType string                 `json:"index_type"` // btree, hash, gin, gist
	Unique    bool                   `json:"unique"`
	Partial   string                 `json:"partial,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// MigrationScript defines database schema migrations
type MigrationScript struct {
	Version     int       `json:"version"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UpSQL       string    `json:"up_sql"`
	DownSQL     string    `json:"down_sql"`
	CreatedAt   time.Time `json:"created_at"`
}

// Migrator handles database schema migrations
type Migrator interface {
	GetCurrentVersion(ctx context.Context) (int, error)
	MigrateUp(ctx context.Context, targetVersion int) error
	MigrateDown(ctx context.Context, targetVersion int) error
	ListMigrations() ([]*MigrationScript, error)
}

// Provider-specific configuration structs

// GraphStoreType defines supported graph database types
type GraphStoreType string

const (
	GraphStoreTypeNeo4j     GraphStoreType = "neo4j"
	GraphStoreTypeSurrealDB GraphStoreType = "surrealdb"
	GraphStoreTypeKuzu      GraphStoreType = "kuzu"
	GraphStoreTypeInMemory  GraphStoreType = "inmemory"
	GraphStoreTypeFalkorDB  GraphStoreType = "falkordb"
)

// VectorStoreType defines supported vector database types
type VectorStoreType string

const (
	VectorStoreTypeQdrant   VectorStoreType = "qdrant"
	VectorStoreTypeLanceDB  VectorStoreType = "lancedb"
	VectorStoreTypePgVector VectorStoreType = "pgvector"
	VectorStoreTypeChromaDB VectorStoreType = "chromadb"
	VectorStoreTypeRedis    VectorStoreType = "redis"
	VectorStoreTypeInMemory VectorStoreType = "inmemory"
)

// Neo4j-specific configuration
type Neo4jConfig struct {
	URI                   string        `json:"uri"`
	Realm                 string        `json:"realm,omitempty"`
	UserAgent             string        `json:"user_agent,omitempty"`
	MaxConnectionLife     time.Duration `json:"max_connection_life"`
	MaxConnectionPool     int           `json:"max_connection_pool"`
	ConnectionTimeout     time.Duration `json:"connection_timeout"`
	SocketTimeout         time.Duration `json:"socket_timeout"`
	EncryptionLevel       string        `json:"encryption_level"` // none, required, strict
	TrustStrategy         string        `json:"trust_strategy"`   // trust_all_certificates, trust_system_ca_signed_certificates
	ServerAddressResolver string        `json:"server_address_resolver,omitempty"`

	// Neo4j-specific features
	EnableBookmarks  bool   `json:"enable_bookmarks"`
	EnableRouting    bool   `json:"enable_routing"`
	EnableMetrics    bool   `json:"enable_metrics"`
	DefaultDatabase  string `json:"default_database,omitempty"`
	ImpersonatedUser string `json:"impersonated_user,omitempty"`
}

// SurrealDB-specific configuration
type SurrealDBConfig struct {
	Endpoint  string `json:"endpoint"`
	Namespace string `json:"namespace"`
	Database  string `json:"database"`
	Scope     string `json:"scope,omitempty"`

	// Authentication
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`

	// Connection settings
	Timeout        time.Duration `json:"timeout"`
	MaxConnections int           `json:"max_connections"`
	EnableTLS      bool          `json:"enable_tls"`
	TLSConfig      *TLSConfig    `json:"tls_config,omitempty"`

	// SurrealDB-specific features
	EnableLivequeries  bool `json:"enable_livequeries"`
	EnableTransactions bool `json:"enable_transactions"`
	StrictMode         bool `json:"strict_mode"`
}

// Kuzu-specific configuration
type KuzuConfig struct {
	DatabasePath      string `json:"database_path"`
	BufferPoolSize    int64  `json:"buffer_pool_size"` // in bytes
	MaxNumThreads     int    `json:"max_num_threads"`
	EnableCompression bool   `json:"enable_compression"`

	// Kuzu-specific settings
	CheckpointWaitTimeout time.Duration `json:"checkpoint_wait_timeout"`
	EnableCheckpoint      bool          `json:"enable_checkpoint"`
	LogLevel              string        `json:"log_level"` // debug, info, warn, error

	// Performance tuning
	HashJoinSizeRatio float64 `json:"hash_join_size_ratio"`
	EnableSemiMask    bool    `json:"enable_semi_mask"`
	EnableZoneMap     bool    `json:"enable_zone_map"`
	EnableProgressBar bool    `json:"enable_progress_bar"`
}

// FalkorDB-specific configuration (Redis-based graph database)
type FalkorDBConfig struct {
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

	// Redis/FalkorDB-specific
	MaxRetries      int           `json:"max_retries"`
	MinRetryBackoff time.Duration `json:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `json:"max_retry_backoff"`
	DialTimeout     time.Duration `json:"dial_timeout"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`

	// TLS configuration
	TLSConfig *TLSConfig `json:"tls_config,omitempty"`
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

// StorageConfigManager manages storage configurations with support for multiple sources
type StorageConfigManager struct {
	config          *StorageConfig
	configFile      string
	environmentVars map[string]string
	profiles        map[string]*StorageConfig
	currentProfile  string
	autoReload      bool
	lastModified    time.Time
}

// NewStorageConfigManager creates a new storage configuration manager
func NewStorageConfigManager() *StorageConfigManager {
	return &StorageConfigManager{
		config:          DefaultStorageConfig(),
		environmentVars: make(map[string]string),
		profiles:        make(map[string]*StorageConfig),
		currentProfile:  "default",
	}
}

// LoadFromFile loads storage configuration from a JSON or YAML file
func (scm *StorageConfigManager) LoadFromFile(filename string) error {
	// Implementation would parse JSON/YAML file and populate config
	// This is a placeholder for the actual implementation
	scm.configFile = filename
	return nil
}

// LoadFromEnvironment loads storage configuration from environment variables
func (scm *StorageConfigManager) LoadFromEnvironment() error {
	// Implementation would read environment variables and populate config
	// This is a placeholder for the actual implementation
	return nil
}

// GetConfig returns the current storage configuration
func (scm *StorageConfigManager) GetConfig() *StorageConfig {
	return scm.config
}

// SetProfile switches to a named configuration profile
func (scm *StorageConfigManager) SetProfile(profileName string) error {
	if profile, exists := scm.profiles[profileName]; exists {
		scm.config = profile
		scm.currentProfile = profileName
		return nil
	}
	return fmt.Errorf("profile not found: %s", profileName)
}

// AddProfile adds a new configuration profile
func (scm *StorageConfigManager) AddProfile(name string, config *StorageConfig) {
	scm.profiles[name] = config
}

// ValidateConfig validates the storage configuration
func (scm *StorageConfigManager) ValidateConfig() error {
	// Implementation would validate all configuration settings
	// This is a placeholder for the actual implementation
	return nil
}

// GetEnvironmentVariableNames returns the list of supported environment variables
func GetEnvironmentVariableNames() []string {
	return []string{
		// Global storage settings
		"AI_MEMORY_STORAGE_ENVIRONMENT",
		"AI_MEMORY_STORAGE_ENABLE_TRANSACTIONS",
		"AI_MEMORY_STORAGE_CONNECTION_TIMEOUT",
		"AI_MEMORY_STORAGE_QUERY_TIMEOUT",
		"AI_MEMORY_STORAGE_MAX_RETRIES",

		// Relational database settings
		"AI_MEMORY_RELATIONAL_TYPE",
		"AI_MEMORY_RELATIONAL_HOST",
		"AI_MEMORY_RELATIONAL_PORT",
		"AI_MEMORY_RELATIONAL_DATABASE",
		"AI_MEMORY_RELATIONAL_USERNAME",
		"AI_MEMORY_RELATIONAL_PASSWORD",
		"AI_MEMORY_RELATIONAL_SSL_MODE",
		"AI_MEMORY_RELATIONAL_MAX_CONNECTIONS",

		// Graph database settings
		"AI_MEMORY_GRAPH_TYPE",
		"AI_MEMORY_GRAPH_HOST",
		"AI_MEMORY_GRAPH_PORT",
		"AI_MEMORY_GRAPH_DATABASE",
		"AI_MEMORY_GRAPH_USERNAME",
		"AI_MEMORY_GRAPH_PASSWORD",

		// Neo4j-specific
		"AI_MEMORY_NEO4J_URI",
		"AI_MEMORY_NEO4J_REALM",
		"AI_MEMORY_NEO4J_ENCRYPTION_LEVEL",
		"AI_MEMORY_NEO4J_TRUST_STRATEGY",

		// SurrealDB-specific
		"AI_MEMORY_SURREALDB_ENDPOINT",
		"AI_MEMORY_SURREALDB_NAMESPACE",
		"AI_MEMORY_SURREALDB_SCOPE",
		"AI_MEMORY_SURREALDB_TOKEN",

		// Kuzu-specific
		"AI_MEMORY_KUZU_DATABASE_PATH",
		"AI_MEMORY_KUZU_BUFFER_POOL_SIZE",
		"AI_MEMORY_KUZU_MAX_NUM_THREADS",

		// Vector database settings
		"AI_MEMORY_VECTOR_TYPE",
		"AI_MEMORY_VECTOR_HOST",
		"AI_MEMORY_VECTOR_PORT",
		"AI_MEMORY_VECTOR_DATABASE",
		"AI_MEMORY_VECTOR_COLLECTION",
		"AI_MEMORY_VECTOR_DIMENSION",
		"AI_MEMORY_VECTOR_DISTANCE_METRIC",
		"AI_MEMORY_VECTOR_INDEX_TYPE",

		// Qdrant-specific
		"AI_MEMORY_QDRANT_API_KEY",
		"AI_MEMORY_QDRANT_ENABLE_GRPC",
		"AI_MEMORY_QDRANT_GRPC_PORT",

		// LanceDB-specific
		"AI_MEMORY_LANCEDB_URI",
		"AI_MEMORY_LANCEDB_STORAGE_TYPE",
		"AI_MEMORY_LANCEDB_DATA_DIR",
		"AI_MEMORY_LANCEDB_TABLE_NAME",

		// pgvector-specific
		"AI_MEMORY_PGVECTOR_VECTOR_DIMENSIONS",
		"AI_MEMORY_PGVECTOR_INDEX_TYPE",
		"AI_MEMORY_PGVECTOR_TABLE_NAME",

		// ChromaDB-specific
		"AI_MEMORY_CHROMADB_API_KEY",
		"AI_MEMORY_CHROMADB_TENANT",
		"AI_MEMORY_CHROMADB_DEFAULT_COLLECTION",

		// Redis-specific
		"AI_MEMORY_REDIS_INDEX_NAME",
		"AI_MEMORY_REDIS_VECTOR_FIELD",
		"AI_MEMORY_REDIS_VECTOR_ALGORITHM",
	}
}
