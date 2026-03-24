package schema

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNodeCreation tests basic Node creation and initialization
func TestNodeCreation(t *testing.T) {
	properties := map[string]interface{}{
		"name":        "Present Perfect",
		"description": "A grammar tense",
		"difficulty":  "intermediate",
	}

	node := NewNode(NodeTypeGrammarRule, properties)

	assert.NotEmpty(t, node.ID, "Node ID should be generated")
	assert.Equal(t, NodeTypeGrammarRule, node.Type)
	assert.Equal(t, properties, node.Properties)
	assert.False(t, node.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, node.UpdatedAt.IsZero(), "UpdatedAt should be set")
	assert.Equal(t, 1.0, node.Weight, "Default weight should be 1.0")
}

// TestNodeTypes verifies all required node types are defined
func TestNodeTypes(t *testing.T) {
	tests := []struct {
		name     string
		nodeType NodeType
		expected string
	}{
		{"Concept", NodeTypeConcept, "Concept"},
		{"Word", NodeTypeWord, "Word"},
		{"UserPreference", NodeTypeUserPreference, "UserPreference"},
		{"GrammarRule", NodeTypeGrammarRule, "GrammarRule"},
		{"Entity", NodeTypeEntity, "Entity"},
		{"Document", NodeTypeDocument, "Document"},
		{"Session", NodeTypeSession, "Session"},
		{"User", NodeTypeUser, "User"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.nodeType))
		})
	}
}

// TestNodeValidation tests Node validation logic
func TestNodeValidation(t *testing.T) {
	tests := []struct {
		name      string
		node      *Node
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid node",
			node: &Node{
				ID:         "node_123",
				Type:       NodeTypeConcept,
				Properties: map[string]interface{}{"name": "test"},
			},
			expectErr: false,
		},
		{
			name: "missing ID",
			node: &Node{
				Type:       NodeTypeConcept,
				Properties: map[string]interface{}{"name": "test"},
			},
			expectErr: true,
			errMsg:    "Node ID is required",
		},
		{
			name: "missing Type",
			node: &Node{
				ID:         "node_123",
				Properties: map[string]interface{}{"name": "test"},
			},
			expectErr: true,
			errMsg:    "Node Type is required",
		},
		{
			name: "nil properties auto-initialized",
			node: &Node{
				ID:   "node_123",
				Type: NodeTypeConcept,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tt.node.Properties, "Properties should be initialized")
			}
		})
	}
}

// TestNodeClone tests deep cloning of Node structs
func TestNodeClone(t *testing.T) {
	original := &Node{
		ID:   "node_123",
		Type: NodeTypeConcept,
		Properties: map[string]interface{}{
			"name":  "Original",
			"value": 42,
		},
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-30 * time.Minute),
		SessionID: "session_1",
		UserID:    "user_1",
		Weight:    0.8,
	}

	clone := original.Clone()

	// Verify all fields are copied
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Type, clone.Type)
	assert.Equal(t, original.CreatedAt, clone.CreatedAt)
	assert.Equal(t, original.SessionID, clone.SessionID)
	assert.Equal(t, original.UserID, clone.UserID)
	assert.Equal(t, original.Weight, clone.Weight)

	// Verify UpdatedAt is refreshed
	assert.True(t, clone.UpdatedAt.After(original.UpdatedAt))

	// Verify deep copy of properties
	assert.Equal(t, original.Properties["name"], clone.Properties["name"])
	clone.Properties["name"] = "Modified"
	assert.NotEqual(t, original.Properties["name"], clone.Properties["name"])
}

// TestNodeJSONSerialization tests JSON marshaling and unmarshaling
func TestNodeJSONSerialization(t *testing.T) {
	node := &Node{
		ID:   "node_123",
		Type: NodeTypeGrammarRule,
		Properties: map[string]interface{}{
			"name":        "Present Perfect",
			"description": "Have/Has + Past Participle",
		},
		CreatedAt: time.Now().Round(time.Second),
		UpdatedAt: time.Now().Round(time.Second),
		SessionID: "session_1",
		UserID:    "user_1",
		Weight:    0.9,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(node)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Unmarshal back
	var decoded Node
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, node.ID, decoded.ID)
	assert.Equal(t, node.Type, decoded.Type)
	assert.Equal(t, node.SessionID, decoded.SessionID)
	assert.Equal(t, node.UserID, decoded.UserID)
	assert.Equal(t, node.Weight, decoded.Weight)
	assert.Equal(t, node.Properties["name"], decoded.Properties["name"])
}

// TestNodeToDataPoint tests conversion from Node to DataPoint
func TestNodeToDataPoint(t *testing.T) {
	node := &Node{
		ID:   "node_123",
		Type: NodeTypeGrammarRule,
		Properties: map[string]interface{}{
			"name":        "Present Perfect",
			"description": "Have/Has + Past Participle",
			"difficulty":  "intermediate",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		SessionID: "session_1",
		UserID:    "user_1",
		Weight:    0.9,
	}

	dataPoint := node.ToDataPoint()

	assert.Equal(t, node.ID, dataPoint.ID)
	assert.Equal(t, "Present Perfect", dataPoint.Content)
	assert.Equal(t, "node", dataPoint.ContentType)
	assert.Equal(t, node.SessionID, dataPoint.SessionID)
	assert.Equal(t, node.UserID, dataPoint.UserID)
	assert.Equal(t, StatusCompleted, dataPoint.ProcessingStatus)

	// Verify metadata includes node properties
	assert.Equal(t, string(NodeTypeGrammarRule), dataPoint.Metadata["node_type"])
	assert.Equal(t, 0.9, dataPoint.Metadata["node_weight"])
	assert.Equal(t, "Have/Has + Past Participle", dataPoint.Metadata["description"])
}

// TestNodeWithDifferentTypes tests Node creation with all supported types
func TestNodeWithDifferentTypes(t *testing.T) {
	types := []NodeType{
		NodeTypeConcept,
		NodeTypeWord,
		NodeTypeUserPreference,
		NodeTypeGrammarRule,
		NodeTypeEntity,
		NodeTypeDocument,
		NodeTypeSession,
		NodeTypeUser,
	}

	for _, nodeType := range types {
		t.Run(string(nodeType), func(t *testing.T) {
			properties := map[string]interface{}{
				"name": "Test " + string(nodeType),
			}
			node := NewNode(nodeType, properties)

			assert.NotEmpty(t, node.ID)
			assert.Equal(t, nodeType, node.Type)
			assert.Equal(t, properties, node.Properties)
			require.NoError(t, node.Validate())
		})
	}
}

// TestNodeMetadataFields tests optional metadata fields
func TestNodeMetadataFields(t *testing.T) {
	node := NewNode(NodeTypeConcept, map[string]interface{}{
		"name": "Test Concept",
	})

	// Set metadata fields
	node.SessionID = "session_123"
	node.UserID = "user_456"
	node.Weight = 0.75

	assert.Equal(t, "session_123", node.SessionID)
	assert.Equal(t, "user_456", node.UserID)
	assert.Equal(t, 0.75, node.Weight)

	// Verify JSON serialization includes metadata
	jsonData, err := json.Marshal(node)
	require.NoError(t, err)

	var decoded Node
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, node.SessionID, decoded.SessionID)
	assert.Equal(t, node.UserID, decoded.UserID)
	assert.Equal(t, node.Weight, decoded.Weight)
}

// TestNodePropertiesIsolation tests that properties are properly isolated between nodes
func TestNodePropertiesIsolation(t *testing.T) {
	props1 := map[string]interface{}{
		"name": "Node 1",
	}
	node1 := NewNode(NodeTypeConcept, props1)

	props2 := map[string]interface{}{
		"name": "Node 2",
	}
	node2 := NewNode(NodeTypeConcept, props2)

	// Modify node1 properties
	node1.Properties["name"] = "Modified Node 1"
	node1.Properties["extra"] = "value"

	// Verify node2 is unaffected
	assert.Equal(t, "Node 2", node2.Properties["name"])
	assert.Nil(t, node2.Properties["extra"])
}

// TestNodeIDGeneration tests that Node IDs are unique
func TestNodeIDGeneration(t *testing.T) {
	node1 := NewNode(NodeTypeConcept, map[string]interface{}{})
	node2 := NewNode(NodeTypeConcept, map[string]interface{}{})

	assert.NotEmpty(t, node1.ID)
	assert.NotEmpty(t, node2.ID)
	assert.NotEqual(t, node1.ID, node2.ID, "Node IDs should be unique")
}

// TestNodeTimestamps tests that timestamps are properly set
func TestNodeTimestamps(t *testing.T) {
	before := time.Now()
	node := NewNode(NodeTypeConcept, map[string]interface{}{})
	after := time.Now()

	assert.False(t, node.CreatedAt.IsZero())
	assert.False(t, node.UpdatedAt.IsZero())
	assert.True(t, node.CreatedAt.After(before) || node.CreatedAt.Equal(before))
	assert.True(t, node.CreatedAt.Before(after) || node.CreatedAt.Equal(after))
	assert.True(t, node.UpdatedAt.After(before) || node.UpdatedAt.Equal(before))
	assert.True(t, node.UpdatedAt.Before(after) || node.UpdatedAt.Equal(after))
}

// ============================================================================
// Edge Tests
// ============================================================================

// TestEdgeCreation tests basic Edge creation and initialization
func TestEdgeCreation(t *testing.T) {
	edge := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, 0.8)

	assert.NotEmpty(t, edge.ID, "Edge ID should be generated")
	assert.Equal(t, "node_1", edge.From)
	assert.Equal(t, "node_2", edge.To)
	assert.Equal(t, EdgeTypeRelatedTo, edge.Type)
	assert.Equal(t, 0.8, edge.Weight)
	assert.NotNil(t, edge.Properties, "Properties should be initialized")
	assert.False(t, edge.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, edge.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

// TestEdgeTypes verifies all required edge types are defined
func TestEdgeTypes(t *testing.T) {
	tests := []struct {
		name     string
		edgeType EdgeType
		expected string
	}{
		{"RelatedTo", EdgeTypeRelatedTo, "RELATED_TO"},
		{"FailedAt", EdgeTypeFailedAt, "FAILED_AT"},
		{"Synonym", EdgeTypeSynonym, "SYNONYM"},
		{"StrugglesWIth", EdgeTypeStrugglesWIth, "STRUGGLES_WITH"},
		{"Contains", EdgeTypeContains, "CONTAINS"},
		{"ReferencedBy", EdgeTypeReferencedBy, "REFERENCED_BY"},
		{"SimilarTo", EdgeTypeSimilarTo, "SIMILAR_TO"},
		{"PartOf", EdgeTypePartOf, "PART_OF"},
		{"CreatedBy", EdgeTypeCreatedBy, "CREATED_BY"},
		{"UsedIn", EdgeTypeUsedIn, "USED_IN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.edgeType))
		})
	}
}

// TestEdgeValidation tests Edge validation logic
func TestEdgeValidation(t *testing.T) {
	tests := []struct {
		name      string
		edge      *Edge
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid edge",
			edge: &Edge{
				ID:         "edge_123",
				From:       "node_1",
				To:         "node_2",
				Type:       EdgeTypeRelatedTo,
				Weight:     0.5,
				Properties: map[string]interface{}{},
			},
			expectErr: false,
		},
		{
			name: "missing ID",
			edge: &Edge{
				From:   "node_1",
				To:     "node_2",
				Type:   EdgeTypeRelatedTo,
				Weight: 0.5,
			},
			expectErr: true,
			errMsg:    "Edge ID is required",
		},
		{
			name: "missing From",
			edge: &Edge{
				ID:     "edge_123",
				To:     "node_2",
				Type:   EdgeTypeRelatedTo,
				Weight: 0.5,
			},
			expectErr: true,
			errMsg:    "Edge From node is required",
		},
		{
			name: "missing To",
			edge: &Edge{
				ID:     "edge_123",
				From:   "node_1",
				Type:   EdgeTypeRelatedTo,
				Weight: 0.5,
			},
			expectErr: true,
			errMsg:    "Edge To node is required",
		},
		{
			name: "missing Type",
			edge: &Edge{
				ID:     "edge_123",
				From:   "node_1",
				To:     "node_2",
				Weight: 0.5,
			},
			expectErr: true,
			errMsg:    "Edge Type is required",
		},
		{
			name: "self-referencing edge",
			edge: &Edge{
				ID:     "edge_123",
				From:   "node_1",
				To:     "node_1",
				Type:   EdgeTypeRelatedTo,
				Weight: 0.5,
			},
			expectErr: true,
			errMsg:    "Edge cannot connect a node to itself",
		},
		{
			name: "nil properties auto-initialized",
			edge: &Edge{
				ID:     "edge_123",
				From:   "node_1",
				To:     "node_2",
				Type:   EdgeTypeRelatedTo,
				Weight: 0.5,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.edge.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tt.edge.Properties, "Properties should be initialized")
			}
		})
	}
}

// TestEdgeClone tests deep cloning of Edge structs
func TestEdgeClone(t *testing.T) {
	original := &Edge{
		ID:   "edge_123",
		From: "node_1",
		To:   "node_2",
		Type: EdgeTypeRelatedTo,
		Properties: map[string]interface{}{
			"context":  "grammar learning",
			"strength": 0.9,
		},
		Weight:    0.8,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-30 * time.Minute),
		SessionID: "session_1",
		UserID:    "user_1",
	}

	clone := original.Clone()

	// Verify all fields are copied
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.From, clone.From)
	assert.Equal(t, original.To, clone.To)
	assert.Equal(t, original.Type, clone.Type)
	assert.Equal(t, original.Weight, clone.Weight)
	assert.Equal(t, original.CreatedAt, clone.CreatedAt)
	assert.Equal(t, original.SessionID, clone.SessionID)
	assert.Equal(t, original.UserID, clone.UserID)

	// Verify UpdatedAt is refreshed
	assert.True(t, clone.UpdatedAt.After(original.UpdatedAt))

	// Verify deep copy of properties
	assert.Equal(t, original.Properties["context"], clone.Properties["context"])
	clone.Properties["context"] = "Modified"
	assert.NotEqual(t, original.Properties["context"], clone.Properties["context"])
}

// TestEdgeJSONSerialization tests JSON marshaling and unmarshaling
func TestEdgeJSONSerialization(t *testing.T) {
	edge := &Edge{
		ID:   "edge_123",
		From: "node_1",
		To:   "node_2",
		Type: EdgeTypeStrugglesWIth,
		Properties: map[string]interface{}{
			"context":    "Present Perfect usage",
			"difficulty": "high",
		},
		Weight:    0.75,
		CreatedAt: time.Now().Round(time.Second),
		UpdatedAt: time.Now().Round(time.Second),
		SessionID: "session_1",
		UserID:    "user_1",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(edge)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Unmarshal back
	var decoded Edge
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, edge.ID, decoded.ID)
	assert.Equal(t, edge.From, decoded.From)
	assert.Equal(t, edge.To, decoded.To)
	assert.Equal(t, edge.Type, decoded.Type)
	assert.Equal(t, edge.Weight, decoded.Weight)
	assert.Equal(t, edge.SessionID, decoded.SessionID)
	assert.Equal(t, edge.UserID, decoded.UserID)
	assert.Equal(t, edge.Properties["context"], decoded.Properties["context"])
}

// TestEdgeToRelationship tests conversion from Edge to Relationship
func TestEdgeToRelationship(t *testing.T) {
	edge := &Edge{
		ID:   "edge_123",
		From: "node_1",
		To:   "node_2",
		Type: EdgeTypeRelatedTo,
		Properties: map[string]interface{}{
			"context": "grammar learning",
			"score":   0.95,
		},
		Weight: 0.8,
	}

	relationship := edge.ToRelationship()

	assert.Equal(t, edge.Type, relationship.Type)
	assert.Equal(t, edge.To, relationship.Target)
	assert.Equal(t, edge.Weight, relationship.Weight)
	assert.Equal(t, edge.Properties, relationship.Metadata)
}

// TestEdgeWithDifferentTypes tests Edge creation with all supported types
func TestEdgeWithDifferentTypes(t *testing.T) {
	types := []EdgeType{
		EdgeTypeRelatedTo,
		EdgeTypeFailedAt,
		EdgeTypeSynonym,
		EdgeTypeStrugglesWIth,
		EdgeTypeContains,
		EdgeTypeReferencedBy,
		EdgeTypeSimilarTo,
		EdgeTypePartOf,
		EdgeTypeCreatedBy,
		EdgeTypeUsedIn,
	}

	for _, edgeType := range types {
		t.Run(string(edgeType), func(t *testing.T) {
			edge := NewEdge("node_1", "node_2", edgeType, 0.5)

			assert.NotEmpty(t, edge.ID)
			assert.Equal(t, "node_1", edge.From)
			assert.Equal(t, "node_2", edge.To)
			assert.Equal(t, edgeType, edge.Type)
			assert.Equal(t, 0.5, edge.Weight)
			require.NoError(t, edge.Validate())
		})
	}
}

// TestEdgeMetadataFields tests optional metadata fields
func TestEdgeMetadataFields(t *testing.T) {
	edge := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, 0.7)

	// Set metadata fields
	edge.SessionID = "session_123"
	edge.UserID = "user_456"
	edge.Properties["context"] = "learning session"

	assert.Equal(t, "session_123", edge.SessionID)
	assert.Equal(t, "user_456", edge.UserID)
	assert.Equal(t, "learning session", edge.Properties["context"])

	// Verify JSON serialization includes metadata
	jsonData, err := json.Marshal(edge)
	require.NoError(t, err)

	var decoded Edge
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, edge.SessionID, decoded.SessionID)
	assert.Equal(t, edge.UserID, decoded.UserID)
	assert.Equal(t, edge.Properties["context"], decoded.Properties["context"])
}

// TestEdgePropertiesIsolation tests that properties are properly isolated between edges
func TestEdgePropertiesIsolation(t *testing.T) {
	edge1 := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, 0.5)
	edge1.Properties["context"] = "Edge 1"

	edge2 := NewEdge("node_3", "node_4", EdgeTypeSynonym, 0.9)
	edge2.Properties["context"] = "Edge 2"

	// Modify edge1 properties
	edge1.Properties["context"] = "Modified Edge 1"
	edge1.Properties["extra"] = "value"

	// Verify edge2 is unaffected
	assert.Equal(t, "Edge 2", edge2.Properties["context"])
	assert.Nil(t, edge2.Properties["extra"])
}

// TestEdgeIDGeneration tests that Edge IDs are unique
func TestEdgeIDGeneration(t *testing.T) {
	edge1 := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, 0.5)
	edge2 := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, 0.5)

	assert.NotEmpty(t, edge1.ID)
	assert.NotEmpty(t, edge2.ID)
	assert.NotEqual(t, edge1.ID, edge2.ID, "Edge IDs should be unique")
}

// TestEdgeTimestamps tests that timestamps are properly set
func TestEdgeTimestamps(t *testing.T) {
	before := time.Now()
	edge := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, 0.5)
	after := time.Now()

	assert.False(t, edge.CreatedAt.IsZero())
	assert.False(t, edge.UpdatedAt.IsZero())
	assert.True(t, edge.CreatedAt.After(before) || edge.CreatedAt.Equal(before))
	assert.True(t, edge.CreatedAt.Before(after) || edge.CreatedAt.Equal(after))
	assert.True(t, edge.UpdatedAt.After(before) || edge.UpdatedAt.Equal(before))
	assert.True(t, edge.UpdatedAt.Before(after) || edge.UpdatedAt.Equal(after))
}

// TestEdgeWeightRange tests various weight values
func TestEdgeWeightRange(t *testing.T) {
	tests := []struct {
		name   string
		weight float64
	}{
		{"zero weight", 0.0},
		{"low weight", 0.1},
		{"medium weight", 0.5},
		{"high weight", 0.9},
		{"max weight", 1.0},
		{"negative weight", -0.5},
		{"over max weight", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, tt.weight)
			assert.Equal(t, tt.weight, edge.Weight)
			require.NoError(t, edge.Validate(), "Edge should be valid regardless of weight value")
		})
	}
}

// TestEdgeWithEmptyProperties tests Edge with no properties
func TestEdgeWithEmptyProperties(t *testing.T) {
	edge := NewEdge("node_1", "node_2", EdgeTypeRelatedTo, 0.5)

	assert.NotNil(t, edge.Properties)
	assert.Empty(t, edge.Properties)

	// Add a property
	edge.Properties["test"] = "value"
	assert.Equal(t, "value", edge.Properties["test"])
}

// TestEdgeDirectionality tests that edges maintain directionality
func TestEdgeDirectionality(t *testing.T) {
	edge1 := NewEdge("node_A", "node_B", EdgeTypeRelatedTo, 0.5)
	edge2 := NewEdge("node_B", "node_A", EdgeTypeRelatedTo, 0.5)

	// Edges should be different even with same nodes and type
	assert.NotEqual(t, edge1.ID, edge2.ID)
	assert.Equal(t, edge1.From, edge2.To)
	assert.Equal(t, edge1.To, edge2.From)
}

// TestEdgeRequiredTypes tests the four required edge types from task 2.1.2
func TestEdgeRequiredTypes(t *testing.T) {
	requiredTypes := []struct {
		edgeType EdgeType
		expected string
	}{
		{EdgeTypeRelatedTo, "RELATED_TO"},
		{EdgeTypeFailedAt, "FAILED_AT"},
		{EdgeTypeSynonym, "SYNONYM"},
		{EdgeTypeStrugglesWIth, "STRUGGLES_WITH"},
	}

	for _, tt := range requiredTypes {
		t.Run(string(tt.edgeType), func(t *testing.T) {
			// Verify constant value
			assert.Equal(t, tt.expected, string(tt.edgeType))

			// Verify edge can be created with this type
			edge := NewEdge("node_1", "node_2", tt.edgeType, 0.5)
			assert.Equal(t, tt.edgeType, edge.Type)
			require.NoError(t, edge.Validate())
		})
	}
}
