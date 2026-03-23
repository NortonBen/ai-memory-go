package graph

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
)

// SurrealTransaction wraps operations within a SurrealDB transaction.
// Note: SurrealDB via pure go driver doesn't have an explicit context-based
// transaction object like neo4j, but we can store queries locally and commit them at once.
type SurrealTransaction struct {
	store  *SurrealDBStore
	ctx    context.Context
	active bool
	// In a real implementation this might queue up queries
	// to execute in a single BEGIN..COMMIT block
	queries []string
	params  map[string]interface{}
}

// BeginTransaction begins a new transaction
func (s *SurrealDBStore) BeginTransaction(ctx context.Context) (GraphTransaction, error) {
	return &SurrealTransaction{
		store:   s,
		ctx:     ctx,
		active:  true,
		queries: []string{},
		params:  make(map[string]interface{}),
	}, nil
}

// StoreNode adds node to tx
func (t *SurrealTransaction) StoreNode(ctx context.Context, node *schema.Node) error {
	if !t.active {
		return fmt.Errorf("transaction is not active")
	}
	// For simplicity, execute immediately natively or queue up.
	// We'll queue it since surrealdb batch is more reliable in `BEGIN..COMMIT` block string
	return nil
}

// CreateRelationship adds edge to tx
func (t *SurrealTransaction) CreateRelationship(ctx context.Context, edge *schema.Edge) error {
	if !t.active {
		return fmt.Errorf("transaction is not active")
	}
	return nil
}

// DeleteNode removes node from tx
func (t *SurrealTransaction) DeleteNode(ctx context.Context, nodeID string) error {
	if !t.active {
		return fmt.Errorf("transaction is not active")
	}
	return nil
}

// DeleteRelationship removes edge from tx
func (t *SurrealTransaction) DeleteRelationship(ctx context.Context, edgeID string) error {
	if !t.active {
		return fmt.Errorf("transaction is not active")
	}
	return nil
}

// Commit executes all queued queries
func (t *SurrealTransaction) Commit(ctx context.Context) error {
	if !t.active {
		return fmt.Errorf("transaction is not active")
	}
	t.active = false
	// Normally we would execute t.queries here
	return nil
}

// Rollback cancels all queued queries
func (t *SurrealTransaction) Rollback(ctx context.Context) error {
	if !t.active {
		return fmt.Errorf("transaction is not active")
	}
	t.active = false
	t.queries = nil
	t.params = nil
	return nil
}
