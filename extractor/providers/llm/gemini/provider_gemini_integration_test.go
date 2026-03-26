package gemini

import (
	"github.com/NortonBen/ai-memory-go/extractor"
	"context"
	"testing"
)

// TestGeminiProviderFactoryIntegration tests the complete Gemini provider integration
func TestGeminiProviderFactoryIntegration(t *testing.T) {
	// Factory tests are covered in registry package to avoid circular dependencies
}

// TestGeminiProviderMethods tests core provider methods without API calls
func TestGeminiProviderMethods(t *testing.T) {
	t.Run("extractor.LLMProvider", func(t *testing.T) {
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
		tokenCount, err := provider.GetTokenCount("This is a test message")
		if err != nil {
			t.Errorf("GetTokenCount failed: %v", err)
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
	t.Run("extractor.LLMProvider", func(t *testing.T) {
		provider, err := NewGeminiProvider("test-api-key", "gemini-1.5-flash")
		if err != nil {
			t.Fatalf("Failed to create Gemini provider: %v", err)
		}

		// Test initial configuration
		config := provider.GetConfiguration()
		if config.Type != extractor.ProviderGemini {
			t.Errorf("Expected type %v, got %v", extractor.ProviderGemini, config.Type)
		}
		if config.Model != "gemini-1.5-flash" {
			t.Errorf("Expected model gemini-1.5-flash, got %s", config.Model)
		}

		// Test configuration update
		newConfig := &extractor.ProviderConfig{
			Type:   extractor.ProviderGemini,
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
		if config.Type != extractor.EmbeddingProviderGemini {
			t.Errorf("Expected type %v, got %v", extractor.EmbeddingProviderGemini, config.Type)
		}
		if config.Model != "text-embedding-004" {
			t.Errorf("Expected model text-embedding-004, got %s", config.Model)
		}
		if config.Dimensions != 768 {
			t.Errorf("Expected dimensions 768, got %d", config.Dimensions)
		}

		// Test configuration update
		newConfig := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderGemini)
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
	t.Run("extractor.LLMProvider", func(t *testing.T) {
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

	t.Run("extractor.LLMProvider", func(t *testing.T) {
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
	t.Run("extractor.LLMProvider", func(t *testing.T) {
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
