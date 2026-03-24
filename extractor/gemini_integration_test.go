package extractor

import (
	"context"
	"testing"
)

// TestGeminiProviderFactoryIntegration tests the complete Gemini provider integration
func TestGeminiProviderFactoryIntegration(t *testing.T) {
	t.Run("LLMProviderFactory", func(t *testing.T) {
		factory := NewProviderFactory()

		// Test that Gemini is in supported providers
		supportedProviders := factory.ListSupportedProviders()
		found := false
		for _, provider := range supportedProviders {
			if provider == ProviderGemini {
				found = true
				break
			}
		}
		if !found {
			t.Error("Gemini provider not found in supported providers list")
		}

		// Test getting Gemini capabilities
		caps, err := factory.GetProviderCapabilities(ProviderGemini)
		if err != nil {
			t.Errorf("Failed to get Gemini capabilities: %v", err)
		}
		if caps == nil {
			t.Error("Gemini capabilities are nil")
		}

		// Test creating Gemini provider with defaults
		provider, err := factory.CreateProviderWithDefaults(ProviderGemini, "test-api-key", "gemini-1.5-flash")
		if err != nil {
			t.Errorf("Failed to create Gemini provider with defaults: %v", err)
		}
		if provider == nil {
			t.Error("Created Gemini provider is nil")
		}

		// Test provider properties
		if provider.GetProviderType() != ProviderGemini {
			t.Errorf("Provider type = %v, want %v", provider.GetProviderType(), ProviderGemini)
		}
		if provider.GetModel() != "gemini-1.5-flash" {
			t.Errorf("Provider model = %v, want %v", provider.GetModel(), "gemini-1.5-flash")
		}

		// Test provider capabilities
		providerCaps := provider.GetCapabilities()
		if !providerCaps.SupportsCompletion {
			t.Error("Expected Gemini to support completion")
		}
		if !providerCaps.SupportsJSONSchema {
			t.Error("Expected Gemini to support JSON schema")
		}
		if providerCaps.MaxContextLength != 1000000 {
			t.Errorf("Expected max context length 1000000, got %d", providerCaps.MaxContextLength)
		}

		// Test configuration
		config := provider.GetConfiguration()
		if config.Type != ProviderGemini {
			t.Errorf("Config type = %v, want %v", config.Type, ProviderGemini)
		}
	})

	t.Run("EmbeddingProviderFactory", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		// Test that Gemini is in supported providers
		supportedProviders := factory.ListSupportedProviders()
		found := false
		for _, provider := range supportedProviders {
			if provider == EmbeddingProviderGemini {
				found = true
				break
			}
		}
		if !found {
			t.Error("Gemini embedding provider not found in supported providers list")
		}

		// Test getting Gemini embedding capabilities
		caps, err := factory.GetProviderCapabilities(EmbeddingProviderGemini)
		if err != nil {
			t.Errorf("Failed to get Gemini embedding capabilities: %v", err)
		}
		if caps == nil {
			t.Error("Gemini embedding capabilities are nil")
		}

		// Test creating Gemini embedding provider with defaults
		provider, err := factory.CreateProviderWithDefaults(EmbeddingProviderGemini, "test-api-key", "text-embedding-004")
		if err != nil {
			t.Errorf("Failed to create Gemini embedding provider with defaults: %v", err)
		}
		if provider == nil {
			t.Error("Created Gemini embedding provider is nil")
		}

		// Test provider properties
		if provider.GetProviderType() != EmbeddingProviderGemini {
			t.Errorf("Provider type = %v, want %v", provider.GetProviderType(), EmbeddingProviderGemini)
		}
		if provider.GetModel() != "text-embedding-004" {
			t.Errorf("Provider model = %v, want %v", provider.GetModel(), "text-embedding-004")
		}
		if provider.GetDimensions() != 768 {
			t.Errorf("Provider dimensions = %d, want %d", provider.GetDimensions(), 768)
		}

		// Test provider capabilities
		providerCaps := provider.GetCapabilities()
		if !providerCaps.SupportsBatching {
			t.Error("Expected Gemini embedding to support batching")
		}
		if !providerCaps.SupportsCustomDims {
			t.Error("Expected Gemini embedding to support custom dimensions")
		}
		if providerCaps.MaxTokensPerText != 2048 {
			t.Errorf("Expected max tokens per text 2048, got %d", providerCaps.MaxTokensPerText)
		}

		// Test configuration
		config := provider.GetConfiguration()
		if config.Type != EmbeddingProviderGemini {
			t.Errorf("Config type = %v, want %v", config.Type, EmbeddingProviderGemini)
		}
		if config.Dimensions != 768 {
			t.Errorf("Config dimensions = %d, want %d", config.Dimensions, 768)
		}
	})
}

// TestGeminiProviderMethods tests core provider methods without API calls
func TestGeminiProviderMethods(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		provider, err := NewGeminiProvider("test-api-key", "gemini-1.5-flash")
		if err != nil {
			t.Fatalf("Failed to create Gemini provider: %v", err)
		}

		// Test token counting
		tokenCount, err := provider.GetTokenCount("This is a test message")
		if err != nil {
			t.Errorf("GetTokenCount failed: %v", err)
		}
		if tokenCount <= 0 {
			t.Errorf("Expected positive token count, got %d", tokenCount)
		}

		// Test max tokens
		maxTokens := provider.GetMaxTokens()
		if maxTokens != 8192 {
			t.Errorf("Expected max tokens 8192, got %d", maxTokens)
		}

		// Test model setting
		err = provider.SetModel("gemini-pro")
		if err != nil {
			t.Errorf("SetModel failed: %v", err)
		}
		if provider.GetModel() != "gemini-pro" {
			t.Errorf("Model not set correctly, got %s", provider.GetModel())
		}

		// Test supported models
		models := provider.GetSupportedModels()
		expectedModels := []string{"gemini-pro", "gemini-pro-vision", "gemini-1.5-pro", "gemini-1.5-flash"}
		if len(models) != len(expectedModels) {
			t.Errorf("Expected %d models, got %d", len(expectedModels), len(models))
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		provider, err := NewGeminiEmbeddingProvider("test-api-key", "text-embedding-004")
		if err != nil {
			t.Fatalf("Failed to create Gemini embedding provider: %v", err)
		}

		// Test token estimation
		tokenCount, err := provider.EstimateTokenCount("This is a test message")
		if err != nil {
			t.Errorf("EstimateTokenCount failed: %v", err)
		}
		if tokenCount <= 0 {
			t.Errorf("Expected positive token count, got %d", tokenCount)
		}

		// Test cost estimation
		cost, err := provider.EstimateCost(1000)
		if err != nil {
			t.Errorf("EstimateCost failed: %v", err)
		}
		if cost != 0.0 {
			t.Errorf("Expected zero cost for Gemini embeddings, got %f", cost)
		}

		// Test custom dimensions
		if !provider.SupportsCustomDimensions() {
			t.Error("Expected Gemini to support custom dimensions")
		}

		err = provider.SetCustomDimensions(512)
		if err != nil {
			t.Errorf("SetCustomDimensions failed: %v", err)
		}
		if provider.GetDimensions() != 512 {
			t.Errorf("Dimensions not set correctly, got %d", provider.GetDimensions())
		}

		// Test max batch size
		maxBatch := provider.GetMaxBatchSize()
		if maxBatch != 100 {
			t.Errorf("Expected max batch size 100, got %d", maxBatch)
		}

		// Test max tokens per text
		maxTokens := provider.GetMaxTokensPerText()
		if maxTokens != 2048 {
			t.Errorf("Expected max tokens per text 2048, got %d", maxTokens)
		}
	})
}

// TestGeminiProviderConfiguration tests configuration management
func TestGeminiProviderConfiguration(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		provider, err := NewGeminiProvider("test-api-key", "gemini-1.5-flash")
		if err != nil {
			t.Fatalf("Failed to create Gemini provider: %v", err)
		}

		// Test initial configuration
		config := provider.GetConfiguration()
		if config.Type != ProviderGemini {
			t.Errorf("Expected type %v, got %v", ProviderGemini, config.Type)
		}
		if config.Model != "gemini-1.5-flash" {
			t.Errorf("Expected model gemini-1.5-flash, got %s", config.Model)
		}

		// Test configuration update
		newConfig := &ProviderConfig{
			Type:   ProviderGemini,
			APIKey: "new-api-key",
			Model:  "gemini-pro",
		}
		err = provider.Configure(newConfig)
		if err != nil {
			t.Errorf("Configure failed: %v", err)
		}

		// Verify configuration was updated
		updatedConfig := provider.GetConfiguration()
		if updatedConfig.Model != "gemini-pro" {
			t.Errorf("Expected model gemini-pro after update, got %s", updatedConfig.Model)
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		provider, err := NewGeminiEmbeddingProvider("test-api-key", "text-embedding-004")
		if err != nil {
			t.Fatalf("Failed to create Gemini embedding provider: %v", err)
		}

		// Test initial configuration
		config := provider.GetConfiguration()
		if config.Type != EmbeddingProviderGemini {
			t.Errorf("Expected type %v, got %v", EmbeddingProviderGemini, config.Type)
		}
		if config.Model != "text-embedding-004" {
			t.Errorf("Expected model text-embedding-004, got %s", config.Model)
		}
		if config.Dimensions != 768 {
			t.Errorf("Expected dimensions 768, got %d", config.Dimensions)
		}

		// Test configuration update
		newConfig := DefaultEmbeddingProviderConfig(EmbeddingProviderGemini)
		newConfig.APIKey = "new-api-key"
		newConfig.Dimensions = 512

		err = provider.Configure(newConfig)
		if err != nil {
			t.Errorf("Configure failed: %v", err)
		}

		// Verify configuration was updated
		if provider.GetDimensions() != 512 {
			t.Errorf("Expected dimensions 512 after update, got %d", provider.GetDimensions())
		}
	})
}

// TestGeminiProviderErrorHandling tests error handling scenarios
func TestGeminiProviderErrorHandling(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		// Test creation with empty API key
		_, err := NewGeminiProvider("", "gemini-1.5-flash")
		if err == nil {
			t.Error("Expected error for empty API key, got nil")
		}

		// Test configuration with nil config
		provider, err := NewGeminiProvider("test-api-key", "gemini-1.5-flash")
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		err = provider.Configure(nil)
		if err == nil {
			t.Error("Expected error for nil config, got nil")
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		// Test creation with empty API key
		_, err := NewGeminiEmbeddingProvider("", "text-embedding-004")
		if err == nil {
			t.Error("Expected error for empty API key, got nil")
		}

		// Test unsupported model
		provider, err := NewGeminiEmbeddingProvider("test-api-key", "text-embedding-004")
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		err = provider.SetModel("unsupported-model")
		if err == nil {
			t.Error("Expected error for unsupported model, got nil")
		}

		// Test invalid dimensions
		err = provider.SetCustomDimensions(0)
		if err == nil {
			t.Error("Expected error for zero dimensions, got nil")
		}

		err = provider.SetCustomDimensions(1000)
		if err == nil {
			t.Error("Expected error for dimensions > 768, got nil")
		}
	})
}

// TestGeminiProviderUsageAndRateLimit tests usage and rate limit methods
func TestGeminiProviderUsageAndRateLimit(t *testing.T) {
	ctx := context.Background()

	t.Run("LLMProvider", func(t *testing.T) {
		provider, err := NewGeminiProvider("test-api-key", "gemini-1.5-flash")
		if err != nil {
			t.Fatalf("Failed to create Gemini provider: %v", err)
		}

		// Test usage stats
		usage, err := provider.GetUsage(ctx)
		if err != nil {
			t.Errorf("GetUsage failed: %v", err)
		}
		if usage == nil {
			t.Error("Usage stats are nil")
		}

		// Test rate limit
		rateLimit, err := provider.GetRateLimit(ctx)
		if err != nil {
			t.Errorf("GetRateLimit failed: %v", err)
		}
		if rateLimit == nil {
			t.Error("Rate limit is nil")
		}
		if rateLimit.RequestsPerMinute <= 0 {
			t.Errorf("Expected positive requests per minute, got %d", rateLimit.RequestsPerMinute)
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		provider, err := NewGeminiEmbeddingProvider("test-api-key", "text-embedding-004")
		if err != nil {
			t.Fatalf("Failed to create Gemini embedding provider: %v", err)
		}

		// Test usage stats
		usage, err := provider.GetUsage(ctx)
		if err != nil {
			t.Errorf("GetUsage failed: %v", err)
		}
		if usage == nil {
			t.Error("Usage stats are nil")
		}

		// Test rate limit
		rateLimit, err := provider.GetRateLimit(ctx)
		if err != nil {
			t.Errorf("GetRateLimit failed: %v", err)
		}
		if rateLimit == nil {
			t.Error("Rate limit is nil")
		}
		if rateLimit.RequestsPerMinute <= 0 {
			t.Errorf("Expected positive requests per minute, got %d", rateLimit.RequestsPerMinute)
		}
	})
}

// TestGeminiProviderLifecycle tests provider lifecycle methods
func TestGeminiProviderLifecycle(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		provider, err := NewGeminiProvider("test-api-key", "gemini-1.5-flash")
		if err != nil {
			t.Fatalf("Failed to create Gemini provider: %v", err)
		}

		// Test close
		err = provider.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		provider, err := NewGeminiEmbeddingProvider("test-api-key", "text-embedding-004")
		if err != nil {
			t.Fatalf("Failed to create Gemini embedding provider: %v", err)
		}

		// Test streaming support
		if provider.SupportsStreaming() {
			t.Error("Expected Gemini embedding provider to not support streaming")
		}

		// Test streaming method (should return error)
		ctx := context.Background()
		err = provider.GenerateStreamingEmbedding(ctx, "test", nil)
		if err == nil {
			t.Error("Expected error for streaming embedding, got nil")
		}

		// Test close
		err = provider.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}
