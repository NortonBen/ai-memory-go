// Package graph provides graph storage interfaces and implementations for the knowledge graph.
// It supports Neo4j, SQLite (with recursive CTE traversal), and in-memory implementations.
package graph

import (
	"context"
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
	StoreTypeNeo4j    GraphStoreType = "neo4j"
	StoreTypeSQLite   GraphStoreType = "sqlite"
	StoreTypeInMemory GraphStoreType = "inmemory"
)

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
