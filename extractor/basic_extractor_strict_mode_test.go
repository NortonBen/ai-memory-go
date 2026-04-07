package extractor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
)

type strictProvider struct {
	entityRaw any
	relRaw    any
}

func (s *strictProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	return "", nil
}
func (s *strictProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error) {
	return "", nil
}
func (s *strictProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schema interface{}) (interface{}, error) {
	if strings.Contains(strings.ToLower(prompt), "relationships") {
		b, _ := jsonMarshal(s.relRaw)
		_ = jsonUnmarshal(b, schema)
		return schema, nil
	}
	b, _ := jsonMarshal(s.entityRaw)
	_ = jsonUnmarshal(b, schema)
	return schema, nil
}
func (s *strictProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schema interface{}, options *CompletionOptions) (interface{}, error) {
	return s.GenerateStructuredOutput(ctx, prompt, schema)
}
func (s *strictProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	return nil, nil
}
func (s *strictProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return nil, nil
}
func (s *strictProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	return nil, nil
}
func (s *strictProvider) GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error) {
	return "", nil
}
func (s *strictProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error {
	return nil
}
func (s *strictProvider) GetModel() string                       { return "mock" }
func (s *strictProvider) SetModel(model string) error            { return nil }
func (s *strictProvider) GetProviderType() ProviderType          { return ProviderOpenAI }
func (s *strictProvider) GetCapabilities() ProviderCapabilities  { return ProviderCapabilities{} }
func (s *strictProvider) GetTokenCount(text string) (int, error) { return 0, nil }
func (s *strictProvider) GetMaxTokens() int                      { return 0 }
func (s *strictProvider) Health(ctx context.Context) error       { return nil }
func (s *strictProvider) GetUsage(ctx context.Context) (*UsageStats, error) {
	return nil, nil
}
func (s *strictProvider) GetRateLimit(ctx context.Context) (*RateLimitStatus, error) {
	return nil, nil
}
func (s *strictProvider) Configure(config *ProviderConfig) error { return nil }
func (s *strictProvider) GetConfiguration() *ProviderConfig      { return &ProviderConfig{} }
func (s *strictProvider) Close() error                           { return nil }

func TestStrictModeRejectsUnknownNodeType(t *testing.T) {
	p := &strictProvider{
		entityRaw: map[string]any{
			"entities": []map[string]any{
				{"name": "X", "type": "UnknownType"},
			},
		},
	}
	be := NewBasicExtractor(p, &ExtractionConfig{UseJSONSchema: true, StrictMode: true})
	_, err := be.ExtractEntities(context.Background(), "X")
	if err == nil || !strings.Contains(err.Error(), "unknown node type") {
		t.Fatalf("expected strict unknown node type error, got: %v", err)
	}
}

func TestStrictModeRejectsUnknownEdgeType(t *testing.T) {
	p := &strictProvider{
		relRaw: map[string]any{
			"relationships": []map[string]any{
				{"from": "A", "to": "B", "type": "UnknownRel"},
			},
		},
	}
	be := NewBasicExtractor(p, &ExtractionConfig{UseJSONSchema: true, StrictMode: true})
	entities := []schema.Node{
		*schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "A"}),
		*schema.NewNode(schema.NodeTypeOrg, map[string]interface{}{"name": "B"}),
	}
	_, err := be.ExtractRelationships(context.Background(), "A B", entities)
	if err == nil || !strings.Contains(err.Error(), "unknown edge type") {
		t.Fatalf("expected strict unknown edge type error, got: %v", err)
	}
}

// tiny wrappers avoid importing encoding/json twice in test helper methods
func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
