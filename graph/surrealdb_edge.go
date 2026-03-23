package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/surrealdb/surrealdb.go" // Important: Need to import the official unmarshal method or similar
)

// CreateRelationship implements the GraphStore interface
func (s *SurrealDBStore) CreateRelationship(ctx context.Context, edge *schema.Edge) error {
	q := "RELATE type::thing('node', $from) -> type::thing('edge', $id) -> type::thing('node', $to) SET type = $type, weight = $weight, properties = $props, created_at = $created, updated_at = $updated"
	
	params := map[string]interface{}{
		"from":    edge.From,
		"id":      edge.ID,
		"to":      edge.To,
		"type":    string(edge.Type),
		"weight":  edge.Weight,
		"props":   edge.Properties,
		"created": time.Now().Format(time.RFC3339),
		"updated": time.Now().Format(time.RFC3339),
	}

	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to create relationship %s: %w", edge.ID, err)
	}

	return nil
}

// GetRelationship retrieves a single edge by ID
func (s *SurrealDBStore) GetRelationship(ctx context.Context, id string) (*schema.Edge, error) {
	q := "SELECT * FROM type::thing('edge', $id)"
	params := map[string]interface{}{
		"id": id,
	}

	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, q, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get relationship %s: %w", id, err)
	}

	if len(*results) == 0 || len((*results)[0].Result) == 0 {
		return nil, fmt.Errorf("edge not found")
	}
	
	if (*results)[0].Error != nil {
		return nil, (*results)[0].Error
	}

	record := (*results)[0].Result[0]
	return mapToEdge(record), nil
}

// UpdateRelationship updates an existing edge
func (s *SurrealDBStore) UpdateRelationship(ctx context.Context, edge *schema.Edge) error {
	data := map[string]interface{}{
		"type":       string(edge.Type),
		"weight":     edge.Weight,
		"properties": edge.Properties,
		"updated_at": time.Now().Format(time.RFC3339),
	}

	q := "UPDATE type::thing('edge', $id) MERGE $data"
	params := map[string]interface{}{
		"id":   edge.ID,
		"data": data,
	}

	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to update relationship %s: %w", edge.ID, err)
	}

	return nil
}

// DeleteRelationship removes an edge
func (s *SurrealDBStore) DeleteRelationship(ctx context.Context, id string) error {
	q := "DELETE type::thing('edge', $id)"
	params := map[string]interface{}{
		"id": id,
	}
	_, err := surrealdb.Query[any](ctx, s.db, q, params)
	if err != nil {
		return fmt.Errorf("failed to delete relationship %s: %w", id, err)
	}
	return nil
}

// Helper to map SurrealDB record to schema.Edge
func mapToEdge(record map[string]interface{}) *schema.Edge {
	edge := &schema.Edge{
		Properties: make(map[string]interface{}),
	}

	// Extract ID (SurrealDB IDs are like 'edge:xyz', we want 'xyz')
	if val, ok := record["id"].(string); ok {
		// id usually is 'edge:123', strip 'edge:'
		edge.ID = strings.TrimPrefix(val, "edge:")
	}

	// Extract in and out (SurrealDB uses 'in' for from, 'out' for to)
	if in, ok := record["in"].(string); ok {
		edge.From = strings.TrimPrefix(in, "node:")
	}
	if out, ok := record["out"].(string); ok {
		edge.To = strings.TrimPrefix(out, "node:")
	}

	if val, ok := record["type"].(string); ok {
		edge.Type = schema.EdgeType(val)
	}
	if val, ok := record["weight"].(float64); ok {
		edge.Weight = val
	}
	
	if props, ok := record["properties"].(map[string]interface{}); ok {
		edge.Properties = props
	}

	return edge
}
