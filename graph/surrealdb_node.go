package graph

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/surrealdb/surrealdb.go" // Important: Need to import the official driver
)

// StoreNode implements the GraphStore interface
func (s *SurrealDBStore) StoreNode(ctx context.Context, node *schema.Node) error {
	// Table is 'node'
	recordID := fmt.Sprintf("node:%s", node.ID)

	data := map[string]interface{}{
		"id":         recordID, // SurrealDB specific ID
		"node_id":    node.ID,
		"type":       string(node.Type),
		"session_id": node.SessionID,
		"user_id":    node.UserID,
		"weight":     node.Weight,
		"properties": node.Properties,
		"created_at": node.CreatedAt,
		"updated_at": node.UpdatedAt,
	}

	// SurrealDB Create or Update
	// Using Query for MERGE-like behavior or UPSERT
	q := "UPDATE type::thing('node', $id) MERGE $data"
	
	params := map[string]interface{}{
		"id":   node.ID,
		"data": data,
	}

	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to store node %s: %w", node.ID, err)
	}

	return nil
}

// GetNode retrieves a single node by ID
func (s *SurrealDBStore) GetNode(ctx context.Context, id string) (*schema.Node, error) {
	q := "SELECT * FROM type::thing('node', $id)"
	params := map[string]interface{}{
		"id": id,
	}

	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, q, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", id, err)
	}

	if len(*results) == 0 || len((*results)[0].Result) == 0 {
		return nil, fmt.Errorf("node not found")
	}
	
	if (*results)[0].Error != nil {
		return nil, (*results)[0].Error
	}

	record := (*results)[0].Result[0]
	return mapToNode(record), nil
}

// UpdateNode updates an existing node
func (s *SurrealDBStore) UpdateNode(ctx context.Context, node *schema.Node) error {
	data := map[string]interface{}{
		"type":       string(node.Type),
		"session_id": node.SessionID,
		"user_id":    node.UserID,
		"weight":     node.Weight,
		"properties": node.Properties,
		"updated_at": node.UpdatedAt,
	}

	q := "UPDATE type::thing('node', $id) MERGE $data"
	params := map[string]interface{}{
		"id":   node.ID,
		"data": data,
	}

	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to update node %s: %w", node.ID, err)
	}

	return nil
}

// DeleteNode removes a node
func (s *SurrealDBStore) DeleteNode(ctx context.Context, id string) error {
	q := "DELETE type::thing('node', $id)"
	params := map[string]interface{}{
		"id": id,
	}
	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to delete node %s: %w", id, err)
	}
	return nil
}

// Helper to map SurrealDB record to schema.Node
func mapToNode(record map[string]interface{}) *schema.Node {
	node := &schema.Node{
		Properties: make(map[string]interface{}),
	}

	if val, ok := record["node_id"].(string); ok {
		node.ID = val
	}
	if val, ok := record["type"].(string); ok {
		node.Type = schema.NodeType(val)
	}
	if val, ok := record["session_id"].(string); ok {
		node.SessionID = val
	}
	if val, ok := record["user_id"].(string); ok {
		node.UserID = val
	}
	if val, ok := record["weight"].(float64); ok {
		node.Weight = val
	}
	
	if props, ok := record["properties"].(map[string]interface{}); ok {
		node.Properties = props
	}

	return node
}
