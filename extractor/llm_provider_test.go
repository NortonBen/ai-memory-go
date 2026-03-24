package extractor

import (
	"context"
	"testing"
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
	// Simple mock: estimate 4 characters per token
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

// Test functions
func TestLLMProviderInterface(t *testing.T) {
	provider := NewMockLLMProvider(ProviderOpenAI, "gpt-4")
	ctx := context.Background()

	// Test basic completion
	response, err := provider.GenerateCompletion(ctx, "Hello, world!")
	if err != nil {
		t.Fatalf("GenerateCompletion failed: %v", err)
	}
	if response == "" {
		t.Error("Expected non-empty response")
	}

	// Test structured output
	result, err := provider.GenerateStructuredOutput(ctx, "Extract data", map[string]interface{}{})
	if err != nil {
		t.Fatalf("GenerateStructuredOutput failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil structured result")
	}

	// Test entity extraction
	entities, err := provider.ExtractEntities(ctx, "Test text")
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}
	if len(entities) == 0 {
		t.Error("Expected at least one entity")
	}

	// Test model operations
	if provider.GetModel() != "gpt-4" {
		t.Error("Expected model to be gpt-4")
	}

	err = provider.SetModel("gpt-3.5-turbo")
	if err != nil {
		t.Fatalf("SetModel failed: %v", err)
	}
	if provider.GetModel() != "gpt-3.5-turbo" {
		t.Error("Expected model to be updated")
	}

	// Test capabilities
	caps := provider.GetCapabilities()
	if !caps.SupportsCompletion {
		t.Error("Expected provider to support completion")
	}

	// Test health check
	err = provider.Health(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// Test usage stats
	usage, err := provider.GetUsage(ctx)
	if err != nil {
		t.Fatalf("GetUsage failed: %v", err)
	}
	if usage.TotalRequests == 0 {
		t.Error("Expected non-zero total requests")
	}

	// Test rate limit
	rateLimit, err := provider.GetRateLimit(ctx)
	if err != nil {
		t.Fatalf("GetRateLimit failed: %v", err)
	}
	if rateLimit.RequestsPerMinute == 0 {
		t.Error("Expected non-zero requests per minute")
	}
}

func TestProviderConfig(t *testing.T) {
	// Test default config creation
	config := DefaultProviderConfig(ProviderOpenAI)
	if config.Type != ProviderOpenAI {
		t.Error("Expected provider type to be OpenAI")
	}
	if config.Model == "" {
		t.Error("Expected default model to be set")
	}

	// Add API key for validation
	config.APIKey = "test-api-key"

	// Test config validation
	err := ValidateProviderConfig(config)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Test invalid config
	invalidConfig := &ProviderConfig{
		Type: ProviderOpenAI,
		// Missing model and API key
	}
	err = ValidateProviderConfig(invalidConfig)
	if err == nil {
		t.Error("Expected validation to fail for invalid config")
	}
}

func TestCompletionOptions(t *testing.T) {
	options := DefaultCompletionOptions()
	if options.Temperature <= 0 {
		t.Error("Expected positive temperature")
	}
	if options.MaxTokens <= 0 {
		t.Error("Expected positive max tokens")
	}
	if options.Timeout <= 0 {
		t.Error("Expected positive timeout")
	}
}

func TestMessageCreation(t *testing.T) {
	// Test message creation
	userMsg := NewUserMessage("Hello")
	if userMsg.Role != RoleUser {
		t.Error("Expected user role")
	}
	if userMsg.Content != "Hello" {
		t.Error("Expected correct content")
	}

	systemMsg := NewSystemMessage("You are a helpful assistant")
	if systemMsg.Role != RoleSystem {
		t.Error("Expected system role")
	}

	assistantMsg := NewAssistantMessage("How can I help?")
	if assistantMsg.Role != RoleAssistant {
		t.Error("Expected assistant role")
	}
}

func TestProviderCapabilities(t *testing.T) {
	capMap := GetProviderCapabilitiesMap()

	// Test OpenAI capabilities
	openaiCaps, exists := capMap[ProviderOpenAI]
	if !exists {
		t.Error("Expected OpenAI capabilities to exist")
	}
	if !openaiCaps.SupportsCompletion {
		t.Error("Expected OpenAI to support completion")
	}
	if !openaiCaps.SupportsJSONMode {
		t.Error("Expected OpenAI to support JSON mode")
	}

	// Test Ollama capabilities
	ollamaCaps, exists := capMap[ProviderOllama]
	if !exists {
		t.Error("Expected Ollama capabilities to exist")
	}
	if ollamaCaps.SupportsUsageTracking {
		t.Error("Expected Ollama to not support usage tracking")
	}
}

func TestStreamingCallback(t *testing.T) {
	provider := NewMockLLMProvider(ProviderOpenAI, "gpt-4")
	ctx := context.Background()

	var chunks []string
	var done bool

	callback := func(chunk string, isDone bool, err error) {
		if err != nil {
			t.Fatalf("Streaming callback error: %v", err)
		}
		chunks = append(chunks, chunk)
		done = isDone
	}

	err := provider.GenerateStreamingCompletion(ctx, "Test prompt", callback)
	if err != nil {
		t.Fatalf("Streaming completion failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
	if !done {
		t.Error("Expected streaming to be marked as done")
	}
}
