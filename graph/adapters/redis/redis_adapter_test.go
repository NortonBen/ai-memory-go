package redis

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

func TestRedisGraphStore_NoOpMethods(t *testing.T) {
	s := &RedisGraphStore{}

	err := s.DeleteGraphBySessionID(context.Background(), "s1")
	require.NoError(t, err)

	cc, err := s.GetConnectedComponents(context.Background())
	require.NoError(t, err)
	require.Len(t, cc, 0)

	nodes, err := s.FindNodesByProperty(context.Background(), "k", "v")
	require.NoError(t, err)
	require.Len(t, nodes, 0)
}

func TestRedisGraphStore_CRUD_Traversal_AndCounts(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	t.Setenv("REDIS_DB_GRAPH", "2")
	cfg := &graph.GraphConfig{Host: mr.Host(), Port: mr.Server().Addr().Port}
	s, err := NewRedisGraphStore(cfg)
	require.NoError(t, err)
	defer s.Close()
	ctx := context.Background()

	n1 := schema.NewNode(schema.NodeTypeConcept, map[string]interface{}{"name": "A"})
	n1.ID = "n1"
	n2 := schema.NewNode(schema.NodeTypeConcept, map[string]interface{}{"name": "B"})
	n2.ID = "n2"
	require.NoError(t, s.StoreNode(ctx, n1))
	require.NoError(t, s.StoreNode(ctx, n2))

	got, err := s.GetNode(ctx, "n1")
	require.NoError(t, err)
	require.Equal(t, "n1", got.ID)
	require.NoError(t, s.UpdateNode(ctx, n1))

	e1 := schema.NewEdge("n1", "n2", schema.EdgeTypeRelatedTo, 1)
	e1.ID = "e1"
	require.NoError(t, s.CreateRelationship(ctx, e1))

	ge, err := s.GetRelationship(ctx, "e1")
	require.NoError(t, err)
	require.Equal(t, "e1", ge.ID)
	require.NoError(t, s.UpdateRelationship(ctx, ge))

	nodes, err := s.TraverseGraph(ctx, "n1", 2, nil)
	require.NoError(t, err)
	require.NotEmpty(t, nodes)

	connected, err := s.FindConnected(ctx, "n1", []schema.EdgeType{schema.EdgeTypeRelatedTo})
	require.NoError(t, err)
	require.NotEmpty(t, connected)

	path, err := s.FindPath(ctx, "n1", "n2", 3)
	require.NoError(t, err)
	require.NotEmpty(t, path)

	_, err = s.FindPath(ctx, "n1", "missing", 1)
	require.Error(t, err)

	byType, err := s.FindNodesByType(ctx, schema.NodeTypeConcept)
	require.NoError(t, err)
	require.Len(t, byType, 2)

	byEntity, err := s.FindNodesByEntity(ctx, "A", schema.NodeTypeConcept)
	require.NoError(t, err)
	require.Len(t, byEntity, 1)

	require.NoError(t, s.StoreBatch(ctx, []*schema.Node{n1}, []*schema.Edge{e1}))
	require.NoError(t, s.DeleteBatch(ctx, []string{"n2"}, []string{"e1"}))
	require.NoError(t, s.DeleteBatch(ctx, nil, nil))

	nodeCount, err := s.GetNodeCount(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, nodeCount, int64(1))
	edgeCount, err := s.GetEdgeCount(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, edgeCount, int64(0))
	require.NoError(t, s.Health(ctx))

	require.NoError(t, s.DeleteNode(ctx, "n1"))
	_, err = s.GetNode(ctx, "n1")
	require.Error(t, err)
}

func TestRedisGraphStore_DeleteRelationship_NotFoundAndEnvFallback(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	_ = os.Setenv("REDIS_DB_GRAPH", "not-int")
	defer os.Unsetenv("REDIS_DB_GRAPH")

	cfg := &graph.GraphConfig{Host: mr.Host(), Port: mr.Server().Addr().Port}
	s, err := NewRedisGraphStore(cfg)
	require.NoError(t, err)
	defer s.Close()

	err = s.DeleteRelationship(context.Background(), "missing")
	require.Error(t, err)
}

