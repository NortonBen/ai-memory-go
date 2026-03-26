package gemini

import (
	"github.com/NortonBen/ai-memory-go/extractor"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeminiProviderDemo demonstrates the complete Gemini provider functionality
func TestGeminiProviderDemo(t *testing.T) {
	t.Run("extractor.LLMProvider_Demo", func(t *testing.T) {
		// Create Gemini LLM provider
		provider, err := NewGeminiProvider("test-api-key", "gemini-1.5-flash")
		require.NoError(t, err)
		require.NotNil(t, provider)

		// Test basic properties
		assert.Equal(t, extractor.ProviderGemini, provider.GetProviderType())
		assert.Equal(t, "gemini-1.5-flash", provider.GetModel())
		assert.Equal(t, 8192, provider.GetMaxTokens())

		// Test capabilities
		caps := provider.GetCapabilities()
		assert.True(t, caps.SupportsCompletion)
		assert.True(t, caps.SupportsChat)
		assert.True(t, caps.SupportsJSONMode)
		assert.True(t, caps.SupportsJSONSchema)
		assert.True(t, caps.SupportsSystemPrompts)
		assert.Equal(t, 1000000, caps.MaxContextLength)

		// Test supported models
		models := provider.GetSupportedModels()
		assert.Contains(t, models, "gemini-1.5-flash")
		assert.Contains(t, models, "gemini-1.5-pro")
		assert.Contains(t, models, "gemini-pro")

		// Test model switching
		err = provider.SetModel("gemini-pro")
		assert.NoError(t, err)
		assert.Equal(t, "gemini-pro", provider.GetModel())

		// Test configuration
		config := provider.GetConfiguration()
		assert.Equal(t, extractor.ProviderGemini, config.Type)
		assert.Equal(t, "test-api-key", config.APIKey)
		assert.Equal(t, "gemini-pro", config.Model)

		// Test token counting
		tokenCount, err := provider.GetTokenCount("Hello, world!")
		assert.NoError(t, err)
		assert.Greater(t, tokenCount, 0)

		// Test health check (will fail without valid API key, but should not panic)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = provider.Health(ctx)
		// We expect this to fail with invalid API key, but it should be a proper error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key not valid")

		// Test cleanup
		err = provider.Close()
		assert.NoError(t, err)
	})

	t.Run("EmbeddingProvider_Demo", func(t *testing.T) {
		// Create Gemini embedding provider
		provider, err := NewGeminiEmbeddingProvider("test-api-key", "text-embedding-004")
		require.NoError(t, err)
		require.NotNil(t, provider)

		// Test basic properties
		assert.Equal(t, extractor.EmbeddingProviderGemini, provider.GetProviderType())
		assert.Equal(t, "text-embedding-004", provider.GetModel())
		assert.Equal(t, 768, provider.GetDimensions())

		// Test capabilities
		caps := provider.GetCapabilities()
		assert.True(t, caps.SupportsBatching)
		assert.False(t, caps.SupportsStreaming)
		assert.True(t, caps.SupportsCustomDims)
		assert.Equal(t, 100, caps.MaxBatchSize)
		assert.Equal(t, 2048, caps.MaxTokensPerText)

		// Test supported models
		models := provider.GetSupportedModels()
		assert.Contains(t, models, "text-embedding-004")

		// Test custom dimensions
		assert.True(t, provider.SupportsCustomDimensions())
		err = provider.SetCustomDimensions(512)
		assert.NoError(t, err)
		assert.Equal(t, 512, provider.GetDimensions())

		// Test configuration
		config := provider.GetConfiguration()
		assert.Equal(t, extractor.EmbeddingProviderGemini, config.Type)
		assert.Equal(t, "test-api-key", config.APIKey)
		assert.Equal(t, "text-embedding-004", config.Model)
		assert.Equal(t, 512, config.Dimensions)

		// Test token estimation
		tokenCount, err := provider.GetTokenCount("Hello, world!")
		assert.NoError(t, err)
		assert.Greater(t, tokenCount, 0)

		// Test cost estimation
		cost, err := provider.EstimateCost(100)
		assert.NoError(t, err)
		assert.Equal(t, 0.0, cost) // Free tier

		// Test health check (will fail without valid API key, but should not panic)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = provider.Health(ctx)
		// We expect this to fail with invalid API key, but it should be a proper error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key not valid")

		// Test cleanup
		err = provider.Close()
		assert.NoError(t, err)
	})


}
