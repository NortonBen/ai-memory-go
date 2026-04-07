package graph

import (
	"context"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

type testStore struct{}

func (s *testStore) StoreNode(ctx context.Context, node *schema.Node) error { return nil }
func (s *testStore) GetNode(ctx context.Context, nodeID string) (*schema.Node, error) {
	return &schema.Node{ID: nodeID, Type: schema.NodeTypeConcept, Properties: map[string]interface{}{}}, nil
}
func (s *testStore) UpdateNode(ctx context.Context, node *schema.Node) error { return nil }
func (s *testStore) DeleteNode(ctx context.Context, nodeID string) error     { return nil }
func (s *testStore) CreateRelationship(ctx context.Context, edge *schema.Edge) error {
	return nil
}
func (s *testStore) GetRelationship(ctx context.Context, edgeID string) (*schema.Edge, error) {
	return &schema.Edge{ID: edgeID, From: "a", To: "b", Type: schema.EdgeTypeRelatedTo}, nil
}
func (s *testStore) UpdateRelationship(ctx context.Context, edge *schema.Edge) error { return nil }
func (s *testStore) DeleteRelationship(ctx context.Context, edgeID string) error      { return nil }
func (s *testStore) TraverseGraph(ctx context.Context, startNodeID string, depth int, filters map[string]interface{}) ([]*schema.Node, error) {
	return []*schema.Node{}, nil
}
func (s *testStore) FindConnected(ctx context.Context, nodeID string, edgeTypes []schema.EdgeType) ([]*schema.Node, error) {
	return []*schema.Node{}, nil
}
func (s *testStore) FindPath(ctx context.Context, fromNodeID, toNodeID string, maxDepth int) ([]*schema.Node, error) {
	return []*schema.Node{}, nil
}
func (s *testStore) FindNodesByType(ctx context.Context, nodeType schema.NodeType) ([]*schema.Node, error) {
	return []*schema.Node{}, nil
}
func (s *testStore) FindNodesByProperty(ctx context.Context, property string, value interface{}) ([]*schema.Node, error) {
	return []*schema.Node{}, nil
}
func (s *testStore) FindNodesByEntity(ctx context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error) {
	return []*schema.Node{}, nil
}
func (s *testStore) StoreBatch(ctx context.Context, nodes []*schema.Node, edges []*schema.Edge) error {
	return nil
}
func (s *testStore) DeleteBatch(ctx context.Context, nodeIDs []string, edgeIDs []string) error { return nil }
func (s *testStore) DeleteGraphBySessionID(ctx context.Context, sessionID string) error         { return nil }
func (s *testStore) GetNodeCount(ctx context.Context) (int64, error)                            { return 0, nil }
func (s *testStore) GetEdgeCount(ctx context.Context) (int64, error)                            { return 0, nil }
func (s *testStore) GetConnectedComponents(ctx context.Context) ([][]string, error)             { return [][]string{}, nil }
func (s *testStore) Health(ctx context.Context) error                                            { return nil }
func (s *testStore) Close() error                                                                { return nil }

func TestDefaultConfigs(t *testing.T) {
	cfg := DefaultGraphConfig()
	require.NotNil(t, cfg)
	require.Equal(t, StoreTypeInMemory, cfg.Type)
	require.True(t, cfg.EnableIndexing)

	tr := DefaultTraversalOptions()
	require.NotNil(t, tr)
	require.Equal(t, 2, tr.MaxDepth)
	require.True(t, tr.IncludeEdges)
}

func TestFactory_CreateGraphStore_NilConfig(t *testing.T) {
	f := NewGraphFactory()
	_, err := f.CreateGraphStore(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "graph config is required")
}

func TestRegisterStoreAndNewStore(t *testing.T) {
	customType := GraphStoreType("unit_test_store")
	RegisterStore(customType, func(config *GraphConfig) (GraphStore, error) {
		return &testStore{}, nil
	})

	store, err := NewStore(&GraphConfig{Type: customType})
	require.NoError(t, err)
	require.NotNil(t, store)

	supported := GetRegisteredStores()
	require.Contains(t, supported, customType)
}

func TestNewStore_UnsupportedType(t *testing.T) {
	_, err := NewStore(&GraphConfig{Type: GraphStoreType("missing_store")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported graph store type")
}

