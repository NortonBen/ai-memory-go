package neo4j

import (
	"context"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jGraphTransaction implements GraphTransaction
type Neo4jGraphTransaction struct {
	tx neo4j.ExplicitTransaction
	ctx context.Context
}

// BeginTransaction starts a new explicit transaction
func (s *Neo4jStore) BeginTransaction(ctx context.Context) (graph.GraphTransaction, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.dbName,
	})
	
	// Start an explicit transaction. Note that we don't defer session.Close here; 
	// it should ideally be closed, but for an explicit tx the lifecycle belongs to the user or Commit/Rollback.
	// We'll manage session closure by storing it if necessary, but according to Neo4j driver v5, 
	// completing the transaction doesn't automatically close the session. 
	// Actually, let's keep it simple: the user only gets GraphTransaction, so we must assume 
	// the session closes when the transaction commits or rolls back.

	tx, err := session.BeginTransaction(ctx)
	if err != nil {
		session.Close(ctx)
		return nil, err
	}

	// We attach a small wrapper that closes the session on commit/rollback.
	return &neo4jTxWrapper{
		session: session,
		tx:      tx,
		store:   s, // Reference to store if needed
	}, nil
}

type neo4jTxWrapper struct {
	session neo4j.SessionWithContext
	tx      neo4j.ExplicitTransaction
	store   *Neo4jStore
}

func (w *neo4jTxWrapper) StoreNode(ctx context.Context, node *schema.Node) error {
	// Since we already implemented StoreNode with executeWrite, we can't reuse it directly.
	// We need to re-implement or expose the core logic. 
	// For simplicity, we just run the query on `w.tx`:
	query := "MERGE (n:Node {id: $id}) SET n += $props, n.type = $type"
	_, err := w.tx.Run(ctx, query, map[string]interface{}{
		"id": node.ID,
		"type": string(node.Type),
		"props": node.Properties,
	})
	return err
}

func (w *neo4jTxWrapper) CreateRelationship(ctx context.Context, edge *schema.Edge) error {
	query := `
		MATCH (a:Node {id: $from_id})
		MATCH (b:Node {id: $to_id})
		MERGE (a)-[r:REL {id: $id}]->(b)
		SET r += $props, r.type = $type
	`
	_, err := w.tx.Run(ctx, query, map[string]interface{}{
		"id": edge.ID,
		"from_id": edge.From,
		"to_id": edge.To,
		"type": string(edge.Type),
		"props": edge.Properties,
	})
	return err
}

func (w *neo4jTxWrapper) DeleteNode(ctx context.Context, nodeID string) error {
	query := "MATCH (n:Node {id: $id}) DETACH DELETE n"
	_, err := w.tx.Run(ctx, query, map[string]interface{}{"id": nodeID})
	return err
}

func (w *neo4jTxWrapper) DeleteRelationship(ctx context.Context, edgeID string) error {
	query := "MATCH ()-[r {id: $id}]->() DELETE r"
	_, err := w.tx.Run(ctx, query, map[string]interface{}{"id": edgeID})
	return err
}

func (w *neo4jTxWrapper) Commit(ctx context.Context) error {
	defer w.session.Close(ctx)
	return w.tx.Commit(ctx)
}

func (w *neo4jTxWrapper) Rollback(ctx context.Context) error {
	defer w.session.Close(ctx)
	return w.tx.Rollback(ctx)
}
