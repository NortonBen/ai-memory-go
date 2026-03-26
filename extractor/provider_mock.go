package extractor

import (
	"context"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// MockLLMProvider implements LLMProvider for testing
type MockLLMProvider struct {
	model        string
	providerType ProviderType
	capabilities *ProviderCapabilities
	config       *ProviderConfig
}

// NewMockLLMProvider creates a new mock provider
func NewMockLLMProvider(providerType ProviderType, model string) *MockLLMProvider {
	return &MockLLMProvider{
		model:        model,
		providerType: providerType,
		capabilities: &ProviderCapabilities{
			SupportsCompletion:    true,
			SupportsChat:          true,
			SupportsJSONMode:      true,
			SupportsSystemPrompts: true,
			MaxContextLength:      4096,
			DefaultModel:          model,
		},
		config: DefaultProviderConfig(providerType),
	}
}

// Implement LLMProvider interface methods
func (m *MockLLMProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	return "Mock completion response", nil
}

func (m *MockLLMProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error) {
	return "Mock completion with options", nil
}

func (m *MockLLMProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schema interface{}) (interface{}, error) {
	return map[string]interface{}{"result": "mock structured output"}, nil
}

func (m *MockLLMProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schema interface{}, options *CompletionOptions) (interface{}, error) {
	return map[string]interface{}{"result": "mock structured output with options"}, nil
}

func (m *MockLLMProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	node := schema.NewNode(schema.NodeTypeConcept, map[string]interface{}{
		"name": "mock entity",
		"text": text,
	})
	return []schema.Node{*node}, nil
}

func (m *MockLLMProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	if len(entities) >= 2 {
		edge := schema.NewEdge(entities[0].ID, entities[1].ID, schema.EdgeTypeRelatedTo, 1.0)
		return []schema.Edge{*edge}, nil
	}
	return []schema.Edge{}, nil
}

func (m *MockLLMProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"custom": "mock result"}, nil
}

func (m *MockLLMProvider) GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error) {
	return "Mock context response", nil
}

func (m *MockLLMProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error {
	callback("Mock ", false, nil)
	callback("streaming ", false, nil)
	callback("response", true, nil)
	return nil
}

func (m *MockLLMProvider) GetModel() string {
	return m.model
}

func (m *MockLLMProvider) SetModel(model string) error {
	m.model = model
	return nil
}

func (m *MockLLMProvider) GetProviderType() ProviderType {
	return m.providerType
}

func (m *MockLLMProvider) GetCapabilities() ProviderCapabilities {
	return *m.capabilities
}

func (m *MockLLMProvider) GetTokenCount(text string) (int, error) {
	return len(text) / 4, nil
}

func (m *MockLLMProvider) GetMaxTokens() int {
	return m.capabilities.MaxContextLength
}

func (m *MockLLMProvider) Health(ctx context.Context) error {
	return nil
}

func (m *MockLLMProvider) GetUsage(ctx context.Context) (*UsageStats, error) {
	return &UsageStats{
		TotalTokensUsed:      1000,
		PromptTokensUsed:     600,
		CompletionTokensUsed: 400,
		TotalRequests:        10,
		SuccessfulRequests:   9,
		FailedRequests:       1,
		AverageLatency:       100 * time.Millisecond,
		PeriodStart:          time.Now().Add(-1 * time.Hour),
		PeriodEnd:            time.Now(),
	}, nil
}

func (m *MockLLMProvider) GetRateLimit(ctx context.Context) (*RateLimitStatus, error) {
	return &RateLimitStatus{
		RequestsPerMinute: 60,
		TokensPerMinute:   100000,
		RequestsUsed:      10,
		TokensUsed:        5000,
		RequestsRemaining: 50,
		TokensRemaining:   95000,
		ResetTime:         time.Now().Add(30 * time.Second),
		IsLimited:         false,
	}, nil
}

func (m *MockLLMProvider) Configure(config *ProviderConfig) error {
	m.config = config
	return nil
}

func (m *MockLLMProvider) GetConfiguration() *ProviderConfig {
	return m.config
}

func (m *MockLLMProvider) Close() error {
	return nil
}
