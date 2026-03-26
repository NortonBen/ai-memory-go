package neo4j

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/NortonBen/ai-memory-go/graph"
)

func init() {
	graph.RegisterStore(graph.StoreTypeNeo4j, func(config *graph.GraphConfig) (graph.GraphStore, error) {
		return NewNeo4jStore(config)
	})
}

// Neo4jStore implements GraphStore and TransactionalGraphStore using Neo4j
type Neo4jStore struct {
	driver neo4j.DriverWithContext
	config *graph.GraphConfig
	dbName string
}

// NewNeo4jStore creates a new Neo4j graph store instance
func NewNeo4jStore(config *graph.GraphConfig) (*Neo4jStore, error) {
	auth := neo4j.BasicAuth(config.Username, config.Password, "")
	
	// Create URI from host/port or options if present
	uri := "neo4j://localhost:7687"
	if config.Host != "" {
		port := config.Port
		if port == 0 {
			port = 7687
		}
		uri = fmt.Sprintf("neo4j://%s:%d", config.Host, port)
	}
	if optURI, ok := config.Options["uri"].(string); ok {
		uri = optURI
	}

	driver, err := neo4j.NewDriverWithContext(uri, auth, func(c *neo4j.Config) {
		c.MaxConnectionPoolSize = config.MaxConnections
		// Default to 100 max connections if not specified
		if c.MaxConnectionPoolSize == 0 {
			c.MaxConnectionPoolSize = 100
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	dbName := config.Database
	if dbName == "" {
		dbName = "neo4j"
	}

	store := &Neo4jStore{
		driver: driver,
		config: config,
		dbName: dbName,
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()
	
	if err := store.Health(ctx); err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to connect to neo4j: %w", err)
	}

	return store, nil
}

// Health checks the health of the Neo4j connection
func (s *Neo4jStore) Health(ctx context.Context) error {
	return s.driver.VerifyConnectivity(ctx)
}

// Close closes the Neo4j driver
func (s *Neo4jStore) Close() error {
	ctx := context.Background()
	return s.driver.Close(ctx)
}

// executeWrite is a helper to run write transactions
func (s *Neo4jStore) executeWrite(ctx context.Context, work neo4j.ManagedTransactionWork) (any, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.dbName,
	})
	defer session.Close(ctx)

	return session.ExecuteWrite(ctx, work)
}

// executeRead is a helper to run read transactions
func (s *Neo4jStore) executeRead(ctx context.Context, work neo4j.ManagedTransactionWork) (any, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.dbName,
		AccessMode:   neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	return session.ExecuteRead(ctx, work)
}
