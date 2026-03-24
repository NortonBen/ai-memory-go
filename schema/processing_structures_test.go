package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// AnchorNode Tests
// ============================================================================

// TestAnchorNodeCreation tests basic AnchorNode creation
func TestAnchorNodeCreation(t *testing.T) {
	node := NewNode(NodeTypeConcept, map[string]interface{}{
		"name": "Present Perfect",
	})

	anchorNode := &AnchorNode{
		Node:   node,
		Score:  0.85,
		Source: "vector",
		Rank:   1,
	}

	assert.NotNil(t, anchorNode)
	assert.Equal(t, node, anchorNode.Node)
	assert.Equal(t, 0.85, anchorNode.Score)
	assert.Equal(t, "vector", anchorNode.Source)
	assert.Equal(t, 1, anchorNode.Rank)
}

// TestAnchorNodeSources tests different source types
func TestAnchorNodeSources(t *testing.T) {
	node := NewNode(NodeTypeConcept, map[string]interface{}{})

	sources := []string{"vector", "entity", "keyword"}

	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			anchorNode := &AnchorNode{
				Node:   node,
				Score:  0.75,
				Source: source,
				Rank:   1,
			}

			assert.Equal(t, source, anchorNode.Source)
		})
	}
}

// TestAnchorNodeScoreRange tests various score values
func TestAnchorNodeScoreRange(t *testing.T) {
	node := NewNode(NodeTypeConcept, map[string]interface{}{})

	tests := []struct {
		name  string
		score float64
	}{
		{"zero score", 0.0},
		{"low score", 0.25},
		{"medium score", 0.5},
		{"high score", 0.85},
		{"max score", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchorNode := &AnchorNode{
				Node:   node,
				Score:  tt.score,
				Source: "vector",
				Rank:   1,
			}

			assert.Equal(t, tt.score, anchorNode.Score)
		})
	}
}

// TestAnchorNodeRanking tests rank ordering
func TestAnchorNodeRanking(t *testing.T) {
	nodes := make([]*AnchorNode, 5)

	for i := 0; i < 5; i++ {
		node := NewNode(NodeTypeConcept, map[string]interface{}{
			"name": "Concept " + string(rune(i)),
		})

		nodes[i] = &AnchorNode{
			Node:   node,
			Score:  float64(5-i) * 0.2, // Descending scores
			Source: "vector",
			Rank:   i + 1,
		}
	}

	// Verify ranks are sequential
	for i, anchorNode := range nodes {
		assert.Equal(t, i+1, anchorNode.Rank)
	}
}

// TestAnchorNodeWithNilNode tests handling of nil node
func TestAnchorNodeWithNilNode(t *testing.T) {
	anchorNode := &AnchorNode{
		Node:   nil,
		Score:  0.5,
		Source: "vector",
		Rank:   1,
	}

	assert.Nil(t, anchorNode.Node)
	assert.Equal(t, 0.5, anchorNode.Score)
}

// ============================================================================
// EnrichedNode Tests
// ============================================================================

// TestEnrichedNodeCreation tests basic EnrichedNode creation
func TestEnrichedNodeCreation(t *testing.T) {
	core := NewNode(NodeTypeConcept, map[string]interface{}{
		"name": "Core Concept",
	})

	directNeighbor1 := NewNode(NodeTypeWord, map[string]interface{}{
		"name": "Related Word 1",
	})
	directNeighbor2 := NewNode(NodeTypeWord, map[string]interface{}{
		"name": "Related Word 2",
	})

	indirectNeighbor := NewNode(NodeTypeConcept, map[string]interface{}{
		"name": "Indirect Concept",
	})

	enrichedNode := &EnrichedNode{
		Core:              core,
		DirectNeighbors:   []*Node{directNeighbor1, directNeighbor2},
		IndirectNeighbors: []*Node{indirectNeighbor},
		RelevanceScore:    0.9,
		ContextSummary:    "Core concept with related words",
		TraversalDepth:    2,
	}

	assert.NotNil(t, enrichedNode)
	assert.Equal(t, core, enrichedNode.Core)
	assert.Equal(t, 2, len(enrichedNode.DirectNeighbors))
	assert.Equal(t, 1, len(enrichedNode.IndirectNeighbors))
	assert.Equal(t, 0.9, enrichedNode.RelevanceScore)
	assert.Equal(t, "Core concept with related words", enrichedNode.ContextSummary)
	assert.Equal(t, 2, enrichedNode.TraversalDepth)
}

// TestEnrichedNodeWithNoNeighbors tests EnrichedNode with empty neighbors
func TestEnrichedNodeWithNoNeighbors(t *testing.T) {
	core := NewNode(NodeTypeConcept, map[string]interface{}{
		"name": "Isolated Concept",
	})

	enrichedNode := &EnrichedNode{
		Core:              core,
		DirectNeighbors:   []*Node{},
		IndirectNeighbors: []*Node{},
		RelevanceScore:    0.5,
		TraversalDepth:    0,
	}

	assert.NotNil(t, enrichedNode)
	assert.Equal(t, core, enrichedNode.Core)
	assert.Empty(t, enrichedNode.DirectNeighbors)
	assert.Empty(t, enrichedNode.IndirectNeighbors)
	assert.Equal(t, 0, enrichedNode.TraversalDepth)
}

// TestEnrichedNodeTraversalDepth tests different traversal depths
func TestEnrichedNodeTraversalDepth(t *testing.T) {
	core := NewNode(NodeTypeConcept, map[string]interface{}{})

	tests := []struct {
		name  string
		depth int
	}{
		{"no traversal", 0},
		{"1-hop", 1},
		{"2-hop", 2},
		{"3-hop", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enrichedNode := &EnrichedNode{
				Core:              core,
				DirectNeighbors:   []*Node{},
				IndirectNeighbors: []*Node{},
				RelevanceScore:    0.8,
				TraversalDepth:    tt.depth,
			}

			assert.Equal(t, tt.depth, enrichedNode.TraversalDepth)
		})
	}
}

// TestEnrichedNodeNeighborCounts tests various neighbor configurations
func TestEnrichedNodeNeighborCounts(t *testing.T) {
	core := NewNode(NodeTypeConcept, map[string]interface{}{})

	tests := []struct {
		name          string
		directCount   int
		indirectCount int
	}{
		{"no neighbors", 0, 0},
		{"only direct", 3, 0},
		{"only indirect", 0, 5},
		{"both types", 2, 3},
		{"many neighbors", 10, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directNeighbors := make([]*Node, tt.directCount)
			for i := 0; i < tt.directCount; i++ {
				directNeighbors[i] = NewNode(NodeTypeWord, map[string]interface{}{})
			}

			indirectNeighbors := make([]*Node, tt.indirectCount)
			for i := 0; i < tt.indirectCount; i++ {
				indirectNeighbors[i] = NewNode(NodeTypeConcept, map[string]interface{}{})
			}

			enrichedNode := &EnrichedNode{
				Core:              core,
				DirectNeighbors:   directNeighbors,
				IndirectNeighbors: indirectNeighbors,
				RelevanceScore:    0.7,
				TraversalDepth:    2,
			}

			assert.Equal(t, tt.directCount, len(enrichedNode.DirectNeighbors))
			assert.Equal(t, tt.indirectCount, len(enrichedNode.IndirectNeighbors))
		})
	}
}

// TestEnrichedNodeContextSummary tests context summary field
func TestEnrichedNodeContextSummary(t *testing.T) {
	core := NewNode(NodeTypeConcept, map[string]interface{}{
		"name": "Grammar Rule",
	})

	summaries := []string{
		"",
		"Simple summary",
		"Detailed summary with multiple sentences. This concept is related to grammar learning.",
	}

	for _, summary := range summaries {
		t.Run("summary_length_"+string(rune(len(summary))), func(t *testing.T) {
			enrichedNode := &EnrichedNode{
				Core:              core,
				DirectNeighbors:   []*Node{},
				IndirectNeighbors: []*Node{},
				RelevanceScore:    0.8,
				ContextSummary:    summary,
				TraversalDepth:    1,
			}

			assert.Equal(t, summary, enrichedNode.ContextSummary)
		})
	}
}

// TestEnrichedNodeRelevanceScore tests relevance scoring
func TestEnrichedNodeRelevanceScore(t *testing.T) {
	core := NewNode(NodeTypeConcept, map[string]interface{}{})

	tests := []struct {
		name  string
		score float64
	}{
		{"very low", 0.1},
		{"low", 0.3},
		{"medium", 0.5},
		{"high", 0.8},
		{"very high", 0.95},
		{"perfect", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enrichedNode := &EnrichedNode{
				Core:              core,
				DirectNeighbors:   []*Node{},
				IndirectNeighbors: []*Node{},
				RelevanceScore:    tt.score,
				TraversalDepth:    1,
			}

			assert.Equal(t, tt.score, enrichedNode.RelevanceScore)
		})
	}
}

// TestEnrichedNodeWithNilCore tests handling of nil core
func TestEnrichedNodeWithNilCore(t *testing.T) {
	enrichedNode := &EnrichedNode{
		Core:              nil,
		DirectNeighbors:   []*Node{},
		IndirectNeighbors: []*Node{},
		RelevanceScore:    0.5,
		TraversalDepth:    0,
	}

	assert.Nil(t, enrichedNode.Core)
}
