package extractor

import (
	"context"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

type stubGraphExtractor struct {
	nodes []schema.Node
	edges []schema.Edge
}

func (s *stubGraphExtractor) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	return s.nodes, nil
}
func (s *stubGraphExtractor) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return s.edges, nil
}
func (s *stubGraphExtractor) ExtractBridgingRelationship(ctx context.Context, question string, answer string) (*schema.Edge, error) {
	return nil, nil
}
func (s *stubGraphExtractor) ExtractRequestIntent(ctx context.Context, text string) (*schema.RequestIntent, error) {
	return nil, nil
}
func (s *stubGraphExtractor) CompareEntities(ctx context.Context, existing schema.Node, newEntity schema.Node) (*schema.ConsistencyResult, error) {
	return &schema.ConsistencyResult{Action: schema.ResolutionKeepSeparate}, nil
}
func (s *stubGraphExtractor) ExtractWithSchema(ctx context.Context, text string, schemaStruct interface{}) (interface{}, error) {
	return nil, nil
}
func (s *stubGraphExtractor) AnalyzeQuery(ctx context.Context, text string) (*schema.ThinkQueryAnalysis, error) {
	return &schema.ThinkQueryAnalysis{}, nil
}
func (s *stubGraphExtractor) SetProvider(provider LLMProvider) {}
func (s *stubGraphExtractor) GetProvider() LLMProvider         { return nil }

func TestNullExtractor_AllMethods(t *testing.T) {
	n := NewNullExtractor()
	require.NotNil(t, n)
	ctx := context.Background()

	nodes, err := n.ExtractEntities(ctx, "x")
	require.NoError(t, err)
	require.Nil(t, nodes)

	edges, err := n.ExtractRelationships(ctx, "x", nil)
	require.NoError(t, err)
	require.Nil(t, edges)

	edge, err := n.ExtractBridgingRelationship(ctx, "q", "a")
	require.NoError(t, err)
	require.Nil(t, edge)

	intent, err := n.ExtractRequestIntent(ctx, "x")
	require.NoError(t, err)
	require.Nil(t, intent)

	cmp, err := n.CompareEntities(ctx, schema.Node{}, schema.Node{})
	require.NoError(t, err)
	require.Equal(t, schema.ResolutionKeepSeparate, cmp.Action)

	out, err := n.ExtractWithSchema(ctx, "x", struct{}{})
	require.NoError(t, err)
	require.Nil(t, out)

	analysis, err := n.AnalyzeQuery(ctx, "x")
	require.NoError(t, err)
	require.NotNil(t, analysis)

	n.SetProvider(nil)
	require.Nil(t, n.GetProvider())
}

func TestHybridGraphExtractor_DelegatesGraphAndLLMParts(t *testing.T) {
	graphExt := &stubGraphExtractor{
		nodes: []schema.Node{*schema.NewNode(schema.NodeTypeConcept, map[string]interface{}{"name": "A"})},
		edges: []schema.Edge{*schema.NewEdge("a", "b", schema.EdgeTypeRelatedTo, 1)},
	}
	llm := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "m"), nil)
	h := NewHybridGraphExtractor(graphExt, llm)
	ctx := context.Background()

	nodes, err := h.ExtractEntities(ctx, "hello")
	require.NoError(t, err)
	require.Len(t, nodes, 1)

	edges, err := h.ExtractRelationships(ctx, "hello", nodes)
	require.NoError(t, err)
	require.Len(t, edges, 1)

	bridge, err := h.ExtractBridgingRelationship(ctx, "q", "a")
	require.Error(t, err)
	require.Nil(t, bridge)

	intent, err := h.ExtractRequestIntent(ctx, "store this")
	require.NoError(t, err)
	require.NotNil(t, intent)

	cons, err := h.CompareEntities(ctx, schema.Node{}, schema.Node{})
	require.NoError(t, err)
	require.NotNil(t, cons)

	_, err = h.ExtractWithSchema(ctx, "x", struct {
		Name string `json:"name"`
	}{})
	require.NoError(t, err)

	analysis, err := h.AnalyzeQuery(ctx, "who is ben")
	require.NoError(t, err)
	require.NotNil(t, analysis)

	h.SetProvider(nil)
	require.Nil(t, h.GetProvider())
}

