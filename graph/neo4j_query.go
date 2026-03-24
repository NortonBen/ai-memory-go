package graph

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// TraverseGraph performs a breadth-first search from a start node up to a maximum depth
func (s *Neo4jStore) TraverseGraph(ctx context.Context, startNodeID string, depth int, filters map[string]interface{}) ([]*schema.Node, error) {
	// We use a variable-length path query `[*1..depth]`
	query := fmt.Sprintf(`
		MATCH (start:Node {id: $start_id})-[*1..%d]-(n:Node)
		RETURN DISTINCT n
	`, depth)

	// Since we might need filters, this is a basic implementation.
	// Advanced filtering would dynamically build the WHERE clause based on `filters`.
	
	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"start_id": startNodeID})
		if err != nil {
			return nil, err
		}

		var nodes []*schema.Node
		for result.Next(ctx) {
			record := result.Record()
			nVal, _ := record.Get("n")
			neoNode := nVal.(neo4j.Node)
			nodes = append(nodes, mapNeo4jNodeToSchema(neoNode))
		}

		return nodes, result.Err()
	})

	if err != nil {
		return nil, err
	}
	return res.([]*schema.Node), nil
}

// FindConnected finds nodes connected to a given node by specific edge types
func (s *Neo4jStore) FindConnected(ctx context.Context, nodeID string, edgeTypes []schema.EdgeType) ([]*schema.Node, error) {
	edgeTypesStr := ""
	for i, et := range edgeTypes {
		if i > 0 {
			edgeTypesStr += "|"
		}
		edgeTypesStr += string(et)
	}

	var relPattern string
	if edgeTypesStr != "" {
		relPattern = fmt.Sprintf("[:%s]", edgeTypesStr)
	} else {
		relPattern = "[]" // any relationship
	}

	query := fmt.Sprintf(`
		MATCH (start:Node {id: $id})-%s-(n:Node)
		RETURN DISTINCT n
	`, relPattern)

	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"id": nodeID})
		if err != nil {
			return nil, err
		}

		var nodes []*schema.Node
		for result.Next(ctx) {
			record := result.Record()
			nVal, _ := record.Get("n")
			nodes = append(nodes, mapNeo4jNodeToSchema(nVal.(neo4j.Node)))
		}

		return nodes, result.Err()
	})

	if err != nil {
		return nil, err
	}
	return res.([]*schema.Node), nil
}

// FindPath finds the shortest path between two nodes up to maxDepth
func (s *Neo4jStore) FindPath(ctx context.Context, fromNodeID, toNodeID string, maxDepth int) ([]*schema.Node, error) {
	// Using Neo4j shortestPath function
	query := fmt.Sprintf(`
		MATCH p=shortestPath((start:Node {id: $from_id})-[*1..%d]-(end:Node {id: $to_id}))
		RETURN nodes(p) AS path_nodes
	`, maxDepth)

	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{
			"from_id": fromNodeID,
			"to_id":   toNodeID,
		})
		if err != nil {
			return nil, err
		}

		if !result.Next(ctx) {
			return []*schema.Node{}, nil
		}

		record := result.Record()
		pathNodesVal, _ := record.Get("path_nodes")
		neoNodes := pathNodesVal.([]any)
		
		var nodes []*schema.Node
		for _, rawNode := range neoNodes {
			neoNode := rawNode.(neo4j.Node)
			nodes = append(nodes, mapNeo4jNodeToSchema(neoNode))
		}

		return nodes, nil
	})

	if err != nil {
		return nil, err
	}
	return res.([]*schema.Node), nil
}

// FindNodesByType finds all nodes of a specific type
func (s *Neo4jStore) FindNodesByType(ctx context.Context, nodeType schema.NodeType) ([]*schema.Node, error) {
	query := fmt.Sprintf(`
		MATCH (n:%s)
		RETURN n
	`, nodeType)

	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var nodes []*schema.Node
		for result.Next(ctx) {
			record := result.Record()
			nVal, _ := record.Get("n")
			nodes = append(nodes, mapNeo4jNodeToSchema(nVal.(neo4j.Node)))
		}

		return nodes, result.Err()
	})

	if err != nil {
		return nil, err
	}
	return res.([]*schema.Node), nil
}

// FindNodesByProperty finds nodes matching a specific property value
func (s *Neo4jStore) FindNodesByProperty(ctx context.Context, property string, value interface{}) ([]*schema.Node, error) {
	query := fmt.Sprintf(`
		MATCH (n:Node)
		WHERE n.%s = $value
		RETURN n
	`, property)

	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"value": value})
		if err != nil {
			return nil, err
		}

		var nodes []*schema.Node
		for result.Next(ctx) {
			record := result.Record()
			nVal, _ := record.Get("n")
			nodes = append(nodes, mapNeo4jNodeToSchema(nVal.(neo4j.Node)))
		}

		return nodes, result.Err()
	})

	if err != nil {
		return nil, err
	}
	return res.([]*schema.Node), nil
}

// FindNodesByEntity finds nodes based on entity name and type (mostly searching by name property)
func (s *Neo4jStore) FindNodesByEntity(ctx context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error) {
	query := fmt.Sprintf(`
		MATCH (n:%s)
		WHERE n.name = $name OR n.id = $name
		RETURN n
	`, entityType)

	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"name": entityName})
		if err != nil {
			return nil, err
		}

		var nodes []*schema.Node
		for result.Next(ctx) {
			record := result.Record()
			nVal, _ := record.Get("n")
			nodes = append(nodes, mapNeo4jNodeToSchema(nVal.(neo4j.Node)))
		}

		return nodes, result.Err()
	})

	if err != nil {
		return nil, err
	}
	return res.([]*schema.Node), nil
}
