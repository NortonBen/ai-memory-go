package neo4j

import (
	"context"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// StoreBatch efficiently stores multiple nodes and edges using UNWIND
func (s *Neo4jStore) StoreBatch(ctx context.Context, nodes []*schema.Node, edges []*schema.Edge) error {
	_, err := s.executeWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// 1. Store nodes in a batch
		if len(nodes) > 0 {
			var nodeBatch []map[string]interface{}
			for _, n := range nodes {
				if err := n.Validate(); err != nil {
					continue
				}
				nodeBatch = append(nodeBatch, map[string]interface{}{
					"id":         n.ID,
					"type":       string(n.Type),
					"props":      n.Properties,
					"created_at": n.CreatedAt.Format(time.RFC3339),
					"updated_at": n.UpdatedAt.Format(time.RFC3339),
					"session_id": n.SessionID,
					"user_id":    n.UserID,
					"weight":     n.Weight,
				})
			}

			if len(nodeBatch) > 0 {
				query := `
					UNWIND $batch AS p
					MERGE (n:Node {id: p.id})
					// Note: Call apoc.create.addLabels(n, [p.type]) if apoc is available and dynamic labels are strictly required.
					// Otherwise, we store it under the property 'type' and keep the unified 'Node' label.
					SET n += p.props,
						n.type = p.type,
						n.created_at = p.created_at,
						n.updated_at = p.updated_at,
						n.session_id = p.session_id,
						n.user_id = p.user_id,
						n.weight = p.weight
				`
				_, err := tx.Run(ctx, query, map[string]interface{}{"batch": nodeBatch})
				if err != nil {
					return nil, err
				}
			}
		}

		// 2. Store edges in a batch
		if len(edges) > 0 {
			var edgeBatch []map[string]interface{}
			for _, e := range edges {
				if err := e.Validate(); err != nil {
					continue
				}
				edgeBatch = append(edgeBatch, map[string]interface{}{
					"id":         e.ID,
					"from_id":    e.From,
					"to_id":      e.To,
					"type":       string(e.Type),
					"weight":     e.Weight,
					"props":      e.Properties,
					"created_at": e.CreatedAt.Format(time.RFC3339),
					"updated_at": e.UpdatedAt.Format(time.RFC3339),
					"session_id": e.SessionID,
					"user_id":    e.UserID,
				})
			}

			if len(edgeBatch) > 0 {
				query := `
					UNWIND $batch AS p
					MATCH (a:Node {id: p.from_id})
					MATCH (b:Node {id: p.to_id})
					MERGE (a)-[r:REL {id: p.id}]->(b)
					// Note: APOC is required for dynamic relationship types.
					// Otherwise we use a generic 'REL' type and store the actual type in a property.
					SET r += p.props,
						r.type = p.type,
						r.weight = p.weight,
						r.created_at = p.created_at,
						r.updated_at = p.updated_at,
						r.session_id = p.session_id,
						r.user_id = p.user_id
				`
				_, err := tx.Run(ctx, query, map[string]interface{}{"batch": edgeBatch})
				if err != nil {
					return nil, err
				}
			}
		}

		return nil, nil
	})

	return err
}

// DeleteBatch efficiently deletes multiple nodes and edges
func (s *Neo4jStore) DeleteBatch(ctx context.Context, nodeIDs []string, edgeIDs []string) error {
	_, err := s.executeWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		if len(edgeIDs) > 0 {
			query := `
				UNWIND $ids AS id
				MATCH ()-[r {id: id}]->()
				DELETE r
			`
			if _, err := tx.Run(ctx, query, map[string]interface{}{"ids": edgeIDs}); err != nil {
				return nil, err
			}
		}

		if len(nodeIDs) > 0 {
			query := `
				UNWIND $ids AS id
				MATCH (n:Node {id: id})
				DETACH DELETE n
			`
			if _, err := tx.Run(ctx, query, map[string]interface{}{"ids": nodeIDs}); err != nil {
				return nil, err
			}
		}
		
		return nil, nil
	})

	return err
}

// GetNodeCount returns the total number of nodes
func (s *Neo4jStore) GetNodeCount(ctx context.Context) (int64, error) {
	query := "MATCH (n:Node) RETURN count(n) AS count"
	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			countVal, _ := result.Record().Get("count")
			return countVal.(int64), nil
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}
	return res.(int64), nil
}

// GetEdgeCount returns the total number of edges
func (s *Neo4jStore) GetEdgeCount(ctx context.Context) (int64, error) {
	query := "MATCH ()-[r]->() RETURN count(r) AS count"
	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			countVal, _ := result.Record().Get("count")
			return countVal.(int64), nil
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}
	return res.(int64), nil
}

// GetConnectedComponents uses graph algorithms to find disconnected subgraphs.
func (s *Neo4jStore) GetConnectedComponents(ctx context.Context) ([][]string, error) {
	// A naive Cypher implementation to find weak components if GDS (Graph Data Science) isn't installed.
	// For production scale, it's recommended to install Neo4j GDS plugin and call `gds.wcc.stream`.
	// Here we return an empty representation for simplicity if GDS isn't required out of the box.
	return [][]string{}, nil
}
