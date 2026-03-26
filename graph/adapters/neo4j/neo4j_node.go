package neo4j

import (
	"context"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// StoreNode stores or updates a node in the graph
func (s *Neo4jStore) StoreNode(ctx context.Context, node *schema.Node) error {
	if err := node.Validate(); err != nil {
		return err
	}

	query := fmt.Sprintf(`
		MERGE (n:Node {id: $id})
		SET n:%s,
			n += $props,
			n.type = $type,
			n.created_at = $created_at,
			n.updated_at = $updated_at,
			n.session_id = $session_id,
			n.user_id = $user_id,
			n.weight = $weight
	`, node.Type)

	params := map[string]interface{}{
		"id":         node.ID,
		"type":       string(node.Type),
		"props":      node.Properties,
		"created_at": node.CreatedAt.Format(time.RFC3339),
		"updated_at": node.UpdatedAt.Format(time.RFC3339),
		"session_id": node.SessionID,
		"user_id":    node.UserID,
		"weight":     node.Weight,
	}

	_, err := s.executeWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, params)
	})

	return err
}

// GetNode retrieves a node by its ID
func (s *Neo4jStore) GetNode(ctx context.Context, nodeID string) (*schema.Node, error) {
	query := `
		MATCH (n:Node {id: $id})
		RETURN n
	`

	res, err := s.executeRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"id": nodeID})
		if err != nil {
			return nil, err
		}

		if !result.Next(ctx) {
			return nil, fmt.Errorf("node not found: %s", nodeID)
		}

		record := result.Record()
		nodeVal, _ := record.Get("n")
		neoNode := nodeVal.(neo4j.Node)

		return mapNeo4jNodeToSchema(neoNode), nil
	})

	if err != nil {
		return nil, err
	}
	return res.(*schema.Node), nil
}

// UpdateNode updates an existing node
func (s *Neo4jStore) UpdateNode(ctx context.Context, node *schema.Node) error {
	node.UpdatedAt = time.Now()
	// In Neo4j MERGE/SET will update if it exists or create if not.
	// But according to interface semantic UpdateNode might just reuse StoreNode
	return s.StoreNode(ctx, node)
}

// DeleteNode removes a node and its related edges
func (s *Neo4jStore) DeleteNode(ctx context.Context, nodeID string) error {
	query := `
		MATCH (n:Node {id: $id})
		DETACH DELETE n
	`

	_, err := s.executeWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, map[string]interface{}{"id": nodeID})
	})

	return err
}

// mapNeo4jNodeToSchema converts a neo4j.Node to a schema.Node
func mapNeo4jNodeToSchema(neoNode neo4j.Node) *schema.Node {
	props := neoNode.Props
	
	node := &schema.Node{
		ID:         getStringProp(props, "id"),
		Type:       schema.NodeType(getStringProp(props, "type")),
		SessionID:  getStringProp(props, "session_id"),
		UserID:     getStringProp(props, "user_id"),
		Weight:     getFloatProp(props, "weight"),
		Properties: make(map[string]interface{}),
	}
	
	if createdAt, err := time.Parse(time.RFC3339, getStringProp(props, "created_at")); err == nil {
		node.CreatedAt = createdAt
	}
	if updatedAt, err := time.Parse(time.RFC3339, getStringProp(props, "updated_at")); err == nil {
		node.UpdatedAt = updatedAt
	}

	// Extract extra properties
	reservedKeys := map[string]bool{
		"id": true, "type": true, "created_at": true, "updated_at": true,
		"session_id": true, "user_id": true, "weight": true,
	}
	
	for k, v := range props {
		if !reservedKeys[k] {
			node.Properties[k] = v
		}
	}

	return node
}

func getStringProp(props map[string]any, key string) string {
	if val, ok := props[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}

func getFloatProp(props map[string]any, key string) float64 {
	if val, ok := props[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 1.0 // default weight
}
