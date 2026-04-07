package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

func testNode(id, name, session string) *schema.Node {
	now := time.Now()
	return &schema.Node{
		ID:         id,
		Type:       schema.NodeTypeConcept,
		Properties: map[string]interface{}{"name": name, "category": "test"},
		Weight:     1,
		SessionID:  session,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func testEdge(id, from, to, session string) *schema.Edge {
	now := time.Now()
	return &schema.Edge{
		ID:        id,
		From:      from,
		To:        to,
		Type:      schema.EdgeTypeRelatedTo,
		Weight:    1,
		SessionID: session,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestInMemoryGraphStore_NodeAndEdgeCRUD(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	n1 := testNode("n1", "Alice", "s1")
	n2 := testNode("n2", "Bob", "s1")
	require.NoError(t, s.StoreNode(ctx, n1))
	require.NoError(t, s.StoreNode(ctx, n2))

	got, err := s.GetNode(ctx, "n1")
	require.NoError(t, err)
	require.Equal(t, "Alice", got.Properties["name"])

	n1.Properties["name"] = "Alice Updated"
	require.NoError(t, s.UpdateNode(ctx, n1))
	got, err = s.GetNode(ctx, "n1")
	require.NoError(t, err)
	require.Equal(t, "Alice Updated", got.Properties["name"])

	e1 := testEdge("e1", "n1", "n2", "s1")
	require.NoError(t, s.CreateRelationship(ctx, e1))
	gotEdge, err := s.GetRelationship(ctx, "e1")
	require.NoError(t, err)
	require.Equal(t, "n1", gotEdge.From)

	require.NoError(t, s.DeleteRelationship(ctx, "e1"))
	_, err = s.GetRelationship(ctx, "e1")
	require.Error(t, err)
}

func TestInMemoryGraphStore_TraversalPathAndQueries(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	nodes := []*schema.Node{
		testNode("a", "A", "s1"),
		testNode("b", "B", "s1"),
		testNode("c", "C", "s2"),
	}
	edges := []*schema.Edge{
		testEdge("e1", "a", "b", "s1"),
		testEdge("e2", "b", "c", "s2"),
	}
	require.NoError(t, s.StoreBatch(ctx, nodes, edges))

	traversed, err := s.TraverseGraph(ctx, "a", 2, nil)
	require.NoError(t, err)
	require.NotEmpty(t, traversed)

	connected, err := s.FindConnected(ctx, "a", []schema.EdgeType{schema.EdgeTypeRelatedTo})
	require.NoError(t, err)
	require.Len(t, connected, 1)
	require.Equal(t, "b", connected[0].ID)

	path, err := s.FindPath(ctx, "a", "c", 4)
	require.NoError(t, err)
	require.Len(t, path, 3)
}

func TestInMemoryGraphStore_DeleteBySessionCountsAndClose(t *testing.T) {
	s := NewInMemoryGraphStore()
	ctx := context.Background()

	require.NoError(t, s.StoreBatch(ctx,
		[]*schema.Node{
			testNode("n1", "N1", "keep"),
			testNode("n2", "N2", "drop"),
		},
		[]*schema.Edge{
			testEdge("e1", "n1", "n2", "drop"),
		},
	))

	require.NoError(t, s.DeleteGraphBySessionID(ctx, "drop"))

	nodes, err := s.ListNodes(ctx)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "n1", nodes[0].ID)

	edges, err := s.ListEdges(ctx)
	require.NoError(t, err)
	require.Len(t, edges, 0)

	nc, err := s.GetNodeCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, nc)

	ec, err := s.GetEdgeCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 0, ec)

	require.NoError(t, s.Close())
	nc, _ = s.GetNodeCount(ctx)
	require.EqualValues(t, 0, nc)
}
