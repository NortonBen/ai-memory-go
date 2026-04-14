package extractor

import (
	"context"
	"errors"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

// overrideCompletion embeds MockLLMProvider and overrides GenerateCompletion for text-path tests.
type overrideCompletion struct {
	*MockLLMProvider
	completion    string
	completionErr error
}

func (o *overrideCompletion) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	if o.completionErr != nil {
		return "", o.completionErr
	}
	return o.completion, nil
}

func newTextPathExtractor(completion string, completionErr error, cfg *ExtractionConfig) *BasicExtractor {
	m := NewMockLLMProvider(ProviderOpenAI, "mock")
	p := &overrideCompletion{MockLLMProvider: m, completion: completion, completionErr: completionErr}
	return NewBasicExtractor(p, cfg)
}

func TestExtractEntities_TextPath_Success(t *testing.T) {
	be := newTextPathExtractor("Alice (person)\n\nBob (organization)\n", nil, &ExtractionConfig{UseJSONSchema: false})
	nodes, err := be.ExtractEntities(context.Background(), "some text")
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, schema.NodeTypePerson, nodes[0].Type)
	require.Equal(t, "Alice", nodes[0].Properties["name"])
	require.Equal(t, schema.NodeTypeOrg, nodes[1].Type)
}

func TestExtractEntities_TextPath_CompletionError(t *testing.T) {
	be := newTextPathExtractor("", errors.New("completion failed"), &ExtractionConfig{UseJSONSchema: false})
	_, err := be.ExtractEntities(context.Background(), "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get completion")
}

func TestExtractEntities_TextPath_StrictUnknownType(t *testing.T) {
	be := newTextPathExtractor("X (TotallyUnknownCategory)\n", nil, &ExtractionConfig{UseJSONSchema: false, StrictMode: true})
	_, err := be.ExtractEntities(context.Background(), "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown node type")
}

func TestExtractRelationships_TextPath_Success(t *testing.T) {
	be := newTextPathExtractor("Alice -> Bob (RELATED_TO)\n", nil, &ExtractionConfig{UseJSONSchema: false})
	a := schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "Alice"})
	b := schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "Bob"})
	edges, err := be.ExtractRelationships(context.Background(), "text", []schema.Node{*a, *b})
	require.NoError(t, err)
	require.Len(t, edges, 1)
	require.Equal(t, schema.EdgeTypeRelatedTo, edges[0].Type)
	require.Equal(t, a.ID, edges[0].From)
	require.Equal(t, b.ID, edges[0].To)
}

func TestExtractRelationships_TextPath_CompletionError(t *testing.T) {
	be := newTextPathExtractor("", errors.New("rel completion failed"), &ExtractionConfig{UseJSONSchema: false})
	a := schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A"})
	_, err := be.ExtractRelationships(context.Background(), "t", []schema.Node{*a})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get completion")
}

func TestExtractRelationships_TextPath_StrictUnknownEdge(t *testing.T) {
	be := newTextPathExtractor("Alice -> Bob (UNKNOWN_EDGE_XYZ)\n", nil, &ExtractionConfig{UseJSONSchema: false, StrictMode: true})
	a := schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "Alice"})
	b := schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "Bob"})
	_, err := be.ExtractRelationships(context.Background(), "t", []schema.Node{*a, *b})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown edge type")
}

func TestCompareEntities_TypeMismatch(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "m"), nil)
	existing := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A"})
	newEnt := *schema.NewNode(schema.NodeTypeOrg, map[string]interface{}{"name": "A"})
	res, err := be.CompareEntities(context.Background(), existing, newEnt)
	require.NoError(t, err)
	require.Equal(t, schema.ResolutionKeepSeparate, res.Action)
}

func TestCompareEntities_NameMismatch(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "m"), nil)
	existing := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "Alice"})
	newEnt := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "Bob"})
	res, err := be.CompareEntities(context.Background(), existing, newEnt)
	require.NoError(t, err)
	require.Equal(t, schema.ResolutionKeepSeparate, res.Action)
}

func TestCompareEntities_Contradict(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "m"), nil)
	existing := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A", "role": "dev"})
	newEnt := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A", "role": "mgr"})
	res, err := be.CompareEntities(context.Background(), existing, newEnt)
	require.NoError(t, err)
	require.Equal(t, schema.ResolutionContradict, res.Action)
}

func TestCompareEntities_Update(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "m"), nil)
	existing := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A"})
	newEnt := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A", "extra": "v"})
	res, err := be.CompareEntities(context.Background(), existing, newEnt)
	require.NoError(t, err)
	require.Equal(t, schema.ResolutionUpdate, res.Action)
	require.NotNil(t, res.MergedData)
	require.Equal(t, "v", res.MergedData["extra"])
}

func TestCompareEntities_Ignore(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "m"), nil)
	n := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A", "k": "v"})
	res, err := be.CompareEntities(context.Background(), n, n)
	require.NoError(t, err)
	require.Equal(t, schema.ResolutionIgnore, res.Action)
}

func TestCompareEntities_NameFromTitle(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "m"), nil)
	existing := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"title": "Same"})
	newEnt := *schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"title": "Same", "note": "n"})
	res, err := be.CompareEntities(context.Background(), existing, newEnt)
	require.NoError(t, err)
	require.Equal(t, schema.ResolutionUpdate, res.Action)
}

func TestExtractName(t *testing.T) {
	require.Equal(t, "", extractName(nil))
	require.Equal(t, "n", extractName(map[string]interface{}{"name": "n"}))
	require.Equal(t, "t", extractName(map[string]interface{}{"title": "t"}))
	require.Equal(t, "id1", extractName(map[string]interface{}{"id": "id1"}))
	require.Equal(t, "first", extractName(map[string]interface{}{"name": []interface{}{"first", "second"}}))
}
