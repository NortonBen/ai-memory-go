package extractor

import (
	"context"
	"testing"
)


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
