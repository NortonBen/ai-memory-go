// Package graph provides graph storage interfaces and implementations for the knowledge graph.
// It supports Neo4j, SQLite (with recursive CTE traversal), and in-memory implementations.
package graph

import (
	"context"
	"fmt"
	"sync"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// GraphStore defines the interface for graph database operations
type GraphStore interface {
	// Node operations
	StoreNode(ctx context.Context, node *schema.Node) error
	GetNode(ctx context.Context, nodeID string) (*schema.Node, error)
	UpdateNode(ctx context.Context, node *schema.Node) error
	DeleteNode(ctx context.Context, nodeID string) error

	// Edge operations
	CreateRelationship(ctx context.Context, edge *schema.Edge) error
	GetRelationship(ctx context.Context, edgeID string) (*schema.Edge, error)
	UpdateRelationship(ctx context.Context, edge *schema.Edge) error
	DeleteRelationship(ctx context.Context, edgeID string) error

	// Graph traversal operations
	TraverseGraph(ctx context.Context, startNodeID string, depth int, filters map[string]interface{}) ([]*schema.Node, error)
	FindConnected(ctx context.Context, nodeID string, edgeTypes []schema.EdgeType) ([]*schema.Node, error)
	FindPath(ctx context.Context, fromNodeID, toNodeID string, maxDepth int) ([]*schema.Node, error)

	// Query operations
	FindNodesByType(ctx context.Context, nodeType schema.NodeType) ([]*schema.Node, error)
	FindNodesByProperty(ctx context.Context, property string, value interface{}) ([]*schema.Node, error)
	FindNodesByEntity(ctx context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error)

	// Batch operations
	StoreBatch(ctx context.Context, nodes []*schema.Node, edges []*schema.Edge) error
	DeleteBatch(ctx context.Context, nodeIDs []string, edgeIDs []string) error

	// DeleteGraphBySessionID removes nodes and edges tagged with this session_id.
	// sessionID "" matches NULL/empty session (global / unscoped graph rows).
	DeleteGraphBySessionID(ctx context.Context, sessionID string) error

	// Analytics and metrics
	GetNodeCount(ctx context.Context) (int64, error)
	GetEdgeCount(ctx context.Context) (int64, error)
	GetConnectedComponents(ctx context.Context) ([][]string, error)

	// Health and lifecycle
	Health(ctx context.Context) error
	Close() error
}

// GraphStoreType defines supported graph database types
type GraphStoreType string

const (
	StoreTypeNeo4j     GraphStoreType = "neo4j"
	StoreTypeSurrealDB GraphStoreType = "surrealdb"
	StoreTypeKuzu      GraphStoreType = "kuzu"
	StoreTypeInMemory  GraphStoreType = "inmemory"
	StoreTypeFalkorDB  GraphStoreType = "falkordb"
	StoreTypeRedis     GraphStoreType = "redis"
	StoreTypeSQLite    GraphStoreType = "sqlite"
)

// StoreFactory is a function that creates a GraphStore from a config
type StoreFactory func(config *GraphConfig) (GraphStore, error)

var (
	storeRegistry = make(map[GraphStoreType]StoreFactory)
	registryMu    sync.RWMutex
)

// RegisterStore registers a graph store factory
func RegisterStore(storeType GraphStoreType, factory StoreFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	storeRegistry[storeType] = factory
}

// GetRegisteredStores returns all registered store types
func GetRegisteredStores() []GraphStoreType {
	registryMu.RLock()
	defer registryMu.RUnlock()
	types := make([]GraphStoreType, 0, len(storeRegistry))
	for t := range storeRegistry {
		types = append(types, t)
	}
	return types
}

// NewStore creates a graph store from a config using the registry
func NewStore(config *GraphConfig) (GraphStore, error) {
	registryMu.RLock()
	factory, exists := storeRegistry[config.Type]
	registryMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported graph store type: %s", config.Type)
	}
	return factory(config)
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

// DefaultGraphConfig returns sensible defaults
func DefaultGraphConfig() *GraphConfig {
	return &GraphConfig{
		Type:           StoreTypeInMemory,
		MaxConnections: 10,
		ConnTimeout:    30 * time.Second,
		IdleTimeout:    5 * time.Minute,
		BatchSize:      100,
		EnableIndexing: true,
		EnableCaching:  true,
	}
}

// TraversalOptions configures graph traversal behavior
type TraversalOptions struct {
	MaxDepth      int                    `json:"max_depth"`
	EdgeTypes     []schema.EdgeType      `json:"edge_types,omitempty"`
	NodeTypes     []schema.NodeType      `json:"node_types,omitempty"`
	Filters       map[string]interface{} `json:"filters,omitempty"`
	IncludeEdges  bool                   `json:"include_edges"`
	MaxResults    int                    `json:"max_results"`
	SortBy        string                 `json:"sort_by,omitempty"`
	SortDirection string                 `json:"sort_direction,omitempty"`
}

// DefaultTraversalOptions returns sensible defaults
func DefaultTraversalOptions() *TraversalOptions {
	return &TraversalOptions{
		MaxDepth:      2,
		IncludeEdges:  true,
		MaxResults:    100,
		SortDirection: "desc",
	}
}

// GraphMetrics provides analytics about the graph
type GraphMetrics struct {
	NodeCount           int64                     `json:"node_count"`
	EdgeCount           int64                     `json:"edge_count"`
	NodesByType         map[schema.NodeType]int64 `json:"nodes_by_type"`
	EdgesByType         map[schema.EdgeType]int64 `json:"edges_by_type"`
	ConnectedComponents int                       `json:"connected_components"`
	AverageConnectivity float64                   `json:"average_connectivity"`
	LastUpdated         time.Time                 `json:"last_updated"`
}

// GraphFactory creates graph store instances
type GraphFactory interface {
	CreateGraphStore(config *GraphConfig) (GraphStore, error)
	ListSupportedTypes() []GraphStoreType
}

// GraphQuery represents a complex graph query
type GraphQuery struct {
	StartNodes   []string               `json:"start_nodes"`
	Pattern      string                 `json:"pattern,omitempty"`
	Filters      map[string]interface{} `json:"filters,omitempty"`
	Limit        int                    `json:"limit"`
	Offset       int                    `json:"offset"`
	ReturnFields []string               `json:"return_fields,omitempty"`
}

// QueryResult represents the result of a graph query
type QueryResult struct {
	Nodes    []*schema.Node         `json:"nodes"`
	Edges    []*schema.Edge         `json:"edges"`
	Paths    []GraphPath            `json:"paths,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// GraphPath represents a path through the graph
type GraphPath struct {
	Nodes  []string `json:"nodes"`
	Edges  []string `json:"edges"`
	Length int      `json:"length"`
	Weight float64  `json:"weight"`
}

// GraphIndex defines indexing strategies for performance
type GraphIndex struct {
	Name       string                 `json:"name"`
	NodeType   schema.NodeType        `json:"node_type,omitempty"`
	EdgeType   schema.EdgeType        `json:"edge_type,omitempty"`
	Properties []string               `json:"properties"`
	IndexType  string                 `json:"index_type"` // btree, hash, fulltext
	Options    map[string]interface{} `json:"options,omitempty"`
}

// GraphTransaction defines transaction operations
type GraphTransaction interface {
	StoreNode(ctx context.Context, node *schema.Node) error
	CreateRelationship(ctx context.Context, edge *schema.Edge) error
	DeleteNode(ctx context.Context, nodeID string) error
	DeleteRelationship(ctx context.Context, edgeID string) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// TransactionalGraphStore extends GraphStore with transaction support
type TransactionalGraphStore interface {
	GraphStore
	BeginTransaction(ctx context.Context) (GraphTransaction, error)
}

// NewSQLiteGraphStore creates a new SQLite-based graph store (legacy facade)
func NewSQLiteGraphStore(dbPath string) (GraphStore, error) {
	return NewStore(&GraphConfig{
		Type:     StoreTypeSQLite,
		Database: dbPath,
	})
}

// NewInMemoryGraphStore creates a new in-memory graph store (legacy facade)
func NewInMemoryGraphStore() GraphStore {
	store, _ := NewStore(&GraphConfig{
		Type: StoreTypeInMemory,
	})
	return store
}

// NewRedisGraphStore creates a new Redis-based graph store (legacy facade)
func NewRedisGraphStore(endpoint, password string) (GraphStore, error) {
	// Handle endpoint as host:port or just host
	host := endpoint
	port := 6379
	if parts := strings.Split(endpoint, ":"); len(parts) == 2 {
		host = parts[0]
		fmt.Sscanf(parts[1], "%d", &port)
	}

	return NewStore(&GraphConfig{
		Type:     StoreTypeRedis,
		Host:     host,
		Port:     port,
		Password: password,
	})
}

// NewNeo4jStore creates a new Neo4j-based graph store (legacy facade)
func NewNeo4jStore(uri, user, password string) (GraphStore, error) {
	return NewStore(&GraphConfig{
		Type: StoreTypeNeo4j,
		Host: uri,
		Neo4j: &Neo4jConfig{
			URI: uri,
		},
		Username: user,
		Password: password,
	})
}
