package graph

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/surrealdb/surrealdb.go" // Import Driver
)

// TraverseGraph traverses the graph from a starting node up to maxDepth
func (s *SurrealDBStore) TraverseGraph(ctx context.Context, startNodeID string, maxDepth int) ([]*schema.Edge, []*schema.Node, error) {
	// SurrealQL to fetch all nodes and edges up to maxDepth
	// e.g. SELECT * FROM node:123->edge[*0..maxDepth]->node
	// Wait, surrealdb supports ->edge[*0..3]->node. But standard TraverseGraph returns all edges/nodes.
	// Since SurrealDB doesn't return full path edges easily, we just fetch nodes for simplicity,
	// or we can use two queries: one for edges, one for nodes.
	// A basic implementation:
	q := fmt.Sprintf("SELECT * FROM type::thing('node', $id)->edge[*0..%d]->node", maxDepth)
	params := map[string]interface{}{
		"id": startNodeID,
	}

	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, q, params)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to traverse graph: %w", err)
	}

	if len(*results) == 0 || (*results)[0].Error != nil {
		if len(*results) > 0 {
			return nil, nil, (*results)[0].Error
		}
		return nil, nil, nil
	}

	nodesMap := (*results)[0].Result

	nodes := make([]*schema.Node, 0)
	for _, rec := range nodesMap {
		nodes = append(nodes, mapToNode(rec))
	}

	return nil, nodes, nil // Retrieving all nested edges is complex in SurrealQL 1.0, omitting edges for now or doing multiple queries
}

// FindConnected finds nodes connected to a start node via specific edge types
func (s *SurrealDBStore) FindConnected(ctx context.Context, startNodeID string, edgeTypes []schema.EdgeType) ([]*schema.Node, error) {
	edgesFilter := ""
	if len(edgeTypes) > 0 {
		types := []string{}
		for _, et := range edgeTypes {
			types = append(types, fmt.Sprintf("'%s'", string(et)))
		}
		// In SurrealDB, we might store type as a property on the `edge` table.
		edgesFilter = fmt.Sprintf(" WHERE type IN [%s]", "foo") // Wait, go syntax issue, doing dynamically
	}

	// Dynamic approach
	q := "SELECT * FROM type::thing('node', $id)->edge"
	if edgesFilter != "" {
		// using parameters
		q += " WHERE type IN $types"
	}
	q += "->node"

	params := map[string]interface{}{
		"id": startNodeID,
	}

	if len(edgeTypes) > 0 {
		types := make([]string, len(edgeTypes))
		for i, et := range edgeTypes {
			types[i] = string(et)
		}
		params["types"] = types
	}

	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, q, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find connected nodes: %w", err)
	}

	if len(*results) == 0 || (*results)[0].Error != nil {
		if len(*results) > 0 {
			return nil, (*results)[0].Error
		}
		return nil, nil
	}

	nodesMap := (*results)[0].Result

	nodes := make([]*schema.Node, 0)
	for _, rec := range nodesMap {
		nodes = append(nodes, mapToNode(rec))
	}

	return nodes, nil
}

// FindPath finds the shortest path between two nodes
func (s *SurrealDBStore) FindPath(ctx context.Context, startNodeID, endNodeID string, maxDepth int) ([]*schema.Edge, []*schema.Node, error) {
	// Simple path finding using Traverse up to maxDepth and checking ID
	// This isn't true shortest path without a specific graph algorithm, but satisfies interface for now
	return nil, nil, fmt.Errorf("FindPath not fully implemented for SurrealDB yet")
}

// FindNodesByType retrieves nodes by their type
func (s *SurrealDBStore) FindNodesByType(ctx context.Context, nodeType schema.NodeType, limit, offset int) ([]*schema.Node, error) {
	q := "SELECT * FROM node WHERE type = $type LIMIT $limit START $offset"
	params := map[string]interface{}{
		"type":   string(nodeType),
		"limit":  limit,
		"offset": offset,
	}

	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, q, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by type: %w", err)
	}

	if len(*results) == 0 || (*results)[0].Error != nil {
		if len(*results) > 0 {
			return nil, (*results)[0].Error
		}
		return nil, nil
	}

	nodesMap := (*results)[0].Result

	nodes := make([]*schema.Node, 0)
	for _, rec := range nodesMap {
		nodes = append(nodes, mapToNode(rec))
	}

	return nodes, nil
}

// FindNodesByProperty retrieves nodes that match a specific property exactly
func (s *SurrealDBStore) FindNodesByProperty(ctx context.Context, propertyKey string, value interface{}, limit, offset int) ([]*schema.Node, error) {
	// properties.key = value
	q := fmt.Sprintf("SELECT * FROM node WHERE properties.%s = $value LIMIT $limit START $offset", propertyKey)
	params := map[string]interface{}{
		"value":  value,
		"limit":  limit,
		"offset": offset,
	}

	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, q, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by property: %w", err)
	}

	if len(*results) == 0 || (*results)[0].Error != nil {
		if len(*results) > 0 {
			return nil, (*results)[0].Error
		}
		return nil, nil
	}

	nodesMap := (*results)[0].Result

	nodes := make([]*schema.Node, 0)
	for _, rec := range nodesMap {
		nodes = append(nodes, mapToNode(rec))
	}

	return nodes, nil
}

// FindNodesByEntity specifically looks for entity properties
func (s *SurrealDBStore) FindNodesByEntity(ctx context.Context, entityName string) ([]*schema.Node, error) {
	// Assume entities are standard concepts or entities
	return s.FindNodesByProperty(ctx, "name", entityName, 10, 0)
}

// GetNodeCount returns the total number of nodes
func (s *SurrealDBStore) GetNodeCount(ctx context.Context) (int64, error) {
	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, "SELECT count() FROM node GROUP BY all", nil)
	if err != nil {
		return 0, err
	}

	if len(*results) == 0 || (*results)[0].Error != nil || len((*results)[0].Result) == 0 {
		return 0, nil
	}

	counts := (*results)[0].Result
	
	count, _ := counts[0]["count"].(float64)
	return int64(count), nil
}

// GetEdgeCount returns the total number of edges
func (s *SurrealDBStore) GetEdgeCount(ctx context.Context) (int64, error) {
	results, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, "SELECT count() FROM edge GROUP BY all", nil)
	if err != nil {
		return 0, err
	}

	if len(*results) == 0 || (*results)[0].Error != nil || len((*results)[0].Result) == 0 {
		return 0, nil
	}

	counts := (*results)[0].Result
	
	count, _ := counts[0]["count"].(float64)
	return int64(count), nil
}
