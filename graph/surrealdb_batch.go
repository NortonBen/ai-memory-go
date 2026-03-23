package graph

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/surrealdb/surrealdb.go"
)

// StoreBatch implements the GraphStore interface
func (s *SurrealDBStore) StoreBatch(ctx context.Context, nodes []*schema.Node, edges []*schema.Edge) error {
	// Let's use string concatenation or a transaction block with parameters
	// Although it's safer to use transactions provided by driver or run multiple statements in single block.

	// Format:
	// BEGIN TRANSACTION;
	// UPDATE node:$1 MERGE $data1;
	// COMMIT TRANSACTION;

	if len(nodes) == 0 && len(edges) == 0 {
		return nil
	}

	q := "BEGIN TRANSACTION;\n"
	params := make(map[string]interface{})

	for i, n := range nodes {
		idKey := fmt.Sprintf("id_n%d", i)
		dataKey := fmt.Sprintf("data_n%d", i)
		
		q += fmt.Sprintf("UPDATE type::thing('node', $%s) MERGE $%s;\n", idKey, dataKey)
		
		recordID := fmt.Sprintf("node:%s", n.ID)
		params[idKey] = n.ID
		params[dataKey] = map[string]interface{}{
			"id":         recordID,
			"node_id":    n.ID,
			"type":       string(n.Type),
			"session_id": n.SessionID,
			"user_id":    n.UserID,
			"weight":     n.Weight,
			"properties": n.Properties,
			"created_at": n.CreatedAt,
			"updated_at": n.UpdatedAt,
		}
	}

	for i, e := range edges {
		idKey := fmt.Sprintf("id_e%d", i)
		fromKey := fmt.Sprintf("from_e%d", i)
		toKey := fmt.Sprintf("to_e%d", i)
		typeKey := fmt.Sprintf("type_e%d", i)
		weightKey := fmt.Sprintf("weight_e%d", i)
		propsKey := fmt.Sprintf("props_e%d", i)
		
		q += fmt.Sprintf("RELATE type::thing('node', $%s) -> type::thing('edge', $%s) -> type::thing('node', $%s) SET type=$%s, weight=$%s, properties=$%s;\n",
			fromKey, idKey, toKey, typeKey, weightKey, propsKey)
			
		params[idKey] = e.ID
		params[fromKey] = e.From
		params[toKey] = e.To
		params[typeKey] = string(e.Type)
		params[weightKey] = e.Weight
		params[propsKey] = e.Properties
	}
	
	q += "COMMIT TRANSACTION;"

	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to store batch: %w", err)
	}

	return nil
}

// DeleteBatch efficiently removes multiple nodes and edges
func (s *SurrealDBStore) DeleteBatch(ctx context.Context, nodeIDs []string, edgeIDs []string) error {
	if len(nodeIDs) == 0 && len(edgeIDs) == 0 {
		return nil
	}

	q := "BEGIN TRANSACTION;\n"
	params := make(map[string]interface{})

	for i, id := range nodeIDs {
		paramKey := fmt.Sprintf("n%d", i)
		q += fmt.Sprintf("DELETE type::thing('node', $%s);\n", paramKey)
		params[paramKey] = id
	}

	for i, id := range edgeIDs {
		paramKey := fmt.Sprintf("e%d", i)
		q += fmt.Sprintf("DELETE type::thing('edge', $%s);\n", paramKey)
		params[paramKey] = id
	}

	q += "COMMIT TRANSACTION;"

	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to delete batch: %w", err)
	}

	return nil
}
