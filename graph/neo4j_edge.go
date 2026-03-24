package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// CreateRelationship adds a directed edge between two nodes
func (s *Neo4jStore) CreateRelationship(ctx context.Context, edge *schema.Edge) error {
	if err := edge.Validate(); err != nil {
		return err
	}

	query := fmt.Sprintf(`
		MATCH (a:Node {id: $from_id})
		MATCH (b:Node {id: $to_id})
		MERGE (a)-[r:%s {id: $id}]->(b)
		SET r += $props,
			r.type = $type,
			r.weight = $weight,
			r.created_at = $created_at,
			r.updated_at = $updated_at,
			r.session_id = $session_id,
			r.user_id = $user_id
	`, edge.Type)

	params := map[string]interface{}{
		"id":         edge.ID,
		"from_id":    edge.From,
		"to_id":      edge.To,
		"type":       string(edge.Type),
		"weight":     edge.Weight,
		"props":      edge.Properties,
		"created_at": edge.CreatedAt.Format(time.RFC3339),
		"updated_at": edge.UpdatedAt.Format(time.RFC3339),
		"session_id": edge.SessionID,
		"user_id":    edge.UserID,
	}

	_, err := s.executeWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		
		summary, err := res.Consume(ctx)
		if err != nil {
			return nil, err
		}
		
		if summary.Counters().RelationshipsCreated() == 0 && summary.Counters().PropertiesSet() == 0 {
			// Either nodes didn't exist or relationship already existed without prop changes
			return nil, fmt.Errorf("failed to create or update relationship (possibly nodes not found)")
		}
		return nil, nil
	})

	return err
}

// GetRelationship retrieves an edge by its ID
func (s *Neo4jStore) GetRelationship(ctx context.Context, edgeID string) (*schema.Edge, error) {
	query := `
		MATCH (a)-[r {id: $id}]->(b)
		RETURN r, a.id AS from_id, b.id AS to_id
	`

	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"id": edgeID})
		if err != nil {
			return nil, err
		}

		if !result.Next(ctx) {
			return nil, fmt.Errorf("edge not found: %s", edgeID)
		}

		record := result.Record()
		rVal, _ := record.Get("r")
		neoRel := rVal.(neo4j.Relationship)
		
		fromID, _ := record.Get("from_id")
		toID, _ := record.Get("to_id")

		return mapNeo4jEdgeToSchema(neoRel, fromID.(string), toID.(string)), nil
	})

	if err != nil {
		return nil, err
	}
	return res.(*schema.Edge), nil
}

// UpdateRelationship modifies an existing edge
func (s *Neo4jStore) UpdateRelationship(ctx context.Context, edge *schema.Edge) error {
	edge.UpdatedAt = time.Now()
	// Since MERGE is used, CreateRelationship essentially does an upsert based on node IDs and Edge Type.
	// But it uses edge ID as uniqueness property, so it will update properties.
	return s.CreateRelationship(ctx, edge)
}

// DeleteRelationship removes an edge from the graph
func (s *Neo4jStore) DeleteRelationship(ctx context.Context, edgeID string) error {
	query := `
		MATCH ()-[r {id: $id}]->()
		DELETE r
	`

	_, err := s.executeWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, map[string]interface{}{"id": edgeID})
	})

	return err
}

// mapNeo4jEdgeToSchema converts a neo4j.Relationship to a schema.Edge
func mapNeo4jEdgeToSchema(neoRel neo4j.Relationship, fromID, toID string) *schema.Edge {
	props := neoRel.Props
	
	edge := &schema.Edge{
		ID:         getStringProp(props, "id"),
		From:       fromID,
		To:         toID,
		Type:       schema.EdgeType(neoRel.Type), // The relationship type string
		Weight:     getFloatProp(props, "weight"),
		SessionID:  getStringProp(props, "session_id"),
		UserID:     getStringProp(props, "user_id"),
		Properties: make(map[string]interface{}),
	}
	
	if createdAt, err := time.Parse(time.RFC3339, getStringProp(props, "created_at")); err == nil {
		edge.CreatedAt = createdAt
	}
	if updatedAt, err := time.Parse(time.RFC3339, getStringProp(props, "updated_at")); err == nil {
		edge.UpdatedAt = updatedAt
	}

	reservedKeys := map[string]bool{
		"id": true, "type": true, "weight": true, "created_at": true, "updated_at": true,
		"session_id": true, "user_id": true,
	}
	
	for k, v := range props {
		if !reservedKeys[k] {
			edge.Properties[k] = v
		}
	}

	return edge
}
