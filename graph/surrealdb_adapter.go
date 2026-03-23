package graph

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
)

// SurrealDBStore implements the GraphStore interface for SurrealDB
type SurrealDBStore struct {
	db     *surrealdb.DB
	config *GraphConfig
	ns     string
	dbName string
}

// NewSurrealDBStore creates a new SurrealDB graph store instance
func NewSurrealDBStore(config *GraphConfig) (*SurrealDBStore, error) {
	// 1. Create URI
	uri := "ws://localhost:8000/rpc"
	if config.Host != "" {
		port := config.Port
		if port == 0 {
			port = 8000
		}
		
		scheme := "ws"
		if optScheme, ok := config.Options["scheme"].(string); ok && optScheme == "http" {
			scheme = "http"
		}
		
		uri = fmt.Sprintf("%s://%s:%d/rpc", scheme, config.Host, port)
	}
	if optURI, ok := config.Options["uri"].(string); ok {
		uri = optURI
	}

	// 2. Connect to DB
	db, err := surrealdb.New(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to surrealdb: %w", err)
	}

	// 3. Authenticate
	username := config.Username
	password := config.Password
	if username == "" {
		username = "root"
	}
	if password == "" {
		password = "root"
	}

	authData := map[string]interface{}{
		"user": username,
		"pass": password,
	}

	if _, err = db.SignIn(context.Background(), authData); err != nil {
		db.Close(context.Background())
		return nil, fmt.Errorf("failed to sign in to surrealdb: %w", err)
	}

	// 4. Use namespace and database
	ns := "memory"
	dbName := config.Database
	if optNS, ok := config.Options["namespace"].(string); ok {
		ns = optNS
	}
	if dbName == "" {
		dbName = "memory_graph"
	}

	if err = db.Use(context.Background(), ns, dbName); err != nil {
		db.Close(context.Background())
		return nil, fmt.Errorf("failed to use namespace/database: %w", err)
	}

	return &SurrealDBStore{
		db:     db,
		config: config,
		ns:     ns,
		dbName: dbName,
	}, nil
}

// Health checks if the connection to the database is alive
func (s *SurrealDBStore) Health(ctx context.Context) error {
	// A simple ping query
	_, err := surrealdb.Query[any](ctx, s.db, "RETURN time::now()", nil)
	return err
}

// Close gracefully shuts down the database connection
func (s *SurrealDBStore) Close() error {
	if s.db != nil {
		s.db.Close(context.Background())
	}
	return nil
}
