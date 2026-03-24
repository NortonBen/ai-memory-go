package graph

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempGraphStore(t *testing.T) *SQLiteGraphStore {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "graph-*.db")
	require.NoError(t, err)
	f.Close()
	s, err := NewSQLiteGraphStore(f.Name())
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteGraphStore_NodeCRUD(t *testing.T) {
	s := tempGraphStore(t)
	ctx := context.Background()

	node := &schema.Node{
		ID:         "n1",
		Type:       schema.NodeTypeConcept,
		Properties: map[string]interface{}{"name": "Alice", "age": 30},
		Weight:     1.0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	require.NoError(t, s.StoreNode(ctx, node))

	got, err := s.GetNode(ctx, "n1")
	require.NoError(t, err)
	assert.Equal(t, "n1", got.ID)
	assert.Equal(t, "Alice", got.Properties["name"])

	node.Properties["name"] = "Alice Updated"
	require.NoError(t, s.UpdateNode(ctx, node))
	got, _ = s.GetNode(ctx, "n1")
	assert.Equal(t, "Alice Updated", got.Properties["name"])

	require.NoError(t, s.DeleteNode(ctx, "n1"))
	_, err = s.GetNode(ctx, "n1")
	assert.Error(t, err)
}

func TestSQLiteGraphStore_EdgeAndTraversal(t *testing.T) {
	s := tempGraphStore(t)
	ctx := context.Background()

	now := time.Now()
	nodes := []*schema.Node{
		{ID: "a", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "A"}, Weight: 1.0, CreatedAt: now, UpdatedAt: now},
		{ID: "b", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "B"}, Weight: 1.0, CreatedAt: now, UpdatedAt: now},
		{ID: "c", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "C"}, Weight: 1.0, CreatedAt: now, UpdatedAt: now},
	}
	edges := []*schema.Edge{
		{ID: "e1", From: "a", To: "b", Type: schema.EdgeTypeRelatedTo, Weight: 1, Properties: map[string]interface{}{}, CreatedAt: now, UpdatedAt: now},
		{ID: "e2", From: "b", To: "c", Type: schema.EdgeTypeRelatedTo, Weight: 1, Properties: map[string]interface{}{}, CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, s.StoreBatch(ctx, nodes, edges))

	// 2-hop traversal from "a" → should reach b and c
	result, err := s.TraverseGraph(ctx, "a", 2, nil)
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, n := range result {
		ids[n.ID] = true
	}
	assert.True(t, ids["b"])
	assert.True(t, ids["c"])

	// 1-hop connected
	connected, err := s.FindConnected(ctx, "a", []schema.EdgeType{schema.EdgeTypeRelatedTo})
	require.NoError(t, err)
	assert.Len(t, connected, 1)
	assert.Equal(t, "b", connected[0].ID)

	// Counts
	nc, _ := s.GetNodeCount(ctx)
	ec, _ := s.GetEdgeCount(ctx)
	assert.Equal(t, int64(3), nc)
	assert.Equal(t, int64(2), ec)
}

func TestSQLiteGraphStore_FindByType(t *testing.T) {
	s := tempGraphStore(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, s.StoreNode(ctx, &schema.Node{
		ID: "w1", Type: schema.NodeTypeWord, Properties: map[string]interface{}{"name": "run"}, Weight: 1.0, CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, s.StoreNode(ctx, &schema.Node{
		ID: "c1", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "speed"}, Weight: 1.0, CreatedAt: now, UpdatedAt: now,
	}))

	words, err := s.FindNodesByType(ctx, schema.NodeTypeWord)
	require.NoError(t, err)
	assert.Len(t, words, 1)
	assert.Equal(t, "w1", words[0].ID)
}
