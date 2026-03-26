// Package extractor - EmbeddingProvider interface tests
package extractor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)



// Test EmbeddingProvider interface compliance
func TestEmbeddingProviderInterface(t *testing.T) {
	provider := NewMockEmbeddingProvider("test-model", 384)
	ctx := context.Background()

	// Test basic embedding generation
	t.Run("GenerateEmbedding", func(t *testing.T) {
		text := "This is a test text"
		embedding, err := provider.GenerateEmbedding(ctx, text)

		require.NoError(t, err)
		assert.Len(t, embedding, 384)
		assert.Equal(t, float32(0), embedding[0])
	})

	// Test batch embedding generation
	t.Run("GenerateBatchEmbeddings", func(t *testing.T) {
		texts := []string{"text1", "text2", "text3"}
		embeddings, err := provider.GenerateBatchEmbeddings(ctx, texts)

		require.NoError(t, err)
		assert.Len(t, embeddings, 3)
		for _, embedding := range embeddings {
			assert.Len(t, embedding, 384)
		}
	})

	// Test model information
	t.Run("ModelInformation", func(t *testing.T) {
		assert.Equal(t, "test-model", provider.GetModel())
		assert.Equal(t, 384, provider.GetDimensions())
		assert.Equal(t, EmbeddingProviderLocal, provider.GetProviderType())
		assert.Equal(t, []string{"mock-model-v1", "mock-model-v2"}, provider.GetSupportedModels())
		assert.Equal(t, 100, provider.GetMaxBatchSize())
		assert.Equal(t, 512, provider.GetMaxTokensPerText())
	})

	// Test health check
	t.Run("HealthCheck", func(t *testing.T) {
		err := provider.Health(ctx)
		assert.NoError(t, err)
	})

	// Test usage statistics
	t.Run("UsageStatistics", func(t *testing.T) {
		usage, err := provider.GetUsage(ctx)
		require.NoError(t, err)
		assert.NotNil(t, usage)
		assert.True(t, usage.TotalRequests > 0)
	})

	// Test rate limit status
	t.Run("RateLimitStatus", func(t *testing.T) {
		rateLimit, err := provider.GetRateLimit(ctx)
		require.NoError(t, err)
		assert.NotNil(t, rateLimit)
		assert.Equal(t, 1000, rateLimit.RequestsPerMinute)
	})

	// Test configuration
	t.Run("Configuration", func(t *testing.T) {
		config := provider.GetConfiguration()
		assert.NotNil(t, config)

		newConfig := DefaultEmbeddingProviderConfig(EmbeddingProviderLocal)
		err := provider.Configure(newConfig)
		assert.NoError(t, err)
	})

	// Test capabilities
	t.Run("Capabilities", func(t *testing.T) {
		caps := provider.GetCapabilities()
		assert.NotNil(t, caps)
		assert.True(t, caps.SupportsBatching)
		assert.True(t, caps.SupportsCustomDims)
		assert.Equal(t, 100, caps.MaxBatchSize)
	})

	// Test deduplication
	t.Run("DeduplicateAndEmbed", func(t *testing.T) {
		texts := []string{"text1", "text2", "text1", "text3", "text2"}
		result, err := provider.DeduplicateAndEmbed(ctx, texts)

		require.NoError(t, err)
		assert.Len(t, result, 3) // Should have only unique texts
		assert.Contains(t, result, "text1")
		assert.Contains(t, result, "text2")
		assert.Contains(t, result, "text3")
	})

	// Test token estimation
	t.Run("TokenEstimation", func(t *testing.T) {
		text := "This is a test text with some words"
		tokenCount, err := provider.GetTokenCount(text)

		require.NoError(t, err)
		assert.Greater(t, tokenCount, 0)

		cost, err := provider.EstimateCost(tokenCount)
		require.NoError(t, err)
		assert.Greater(t, cost, 0.0)
	})
}

// Test EmbeddingProviderConfig validation
func TestEmbeddingProviderConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := DefaultEmbeddingProviderConfig(EmbeddingProviderOpenAI)
		config.APIKey = "test-key"

		err := ValidateEmbeddingProviderConfig(config)
		assert.NoError(t, err)
	})

	t.Run("NilConfig", func(t *testing.T) {
		err := ValidateEmbeddingProviderConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config is nil")
	})

	t.Run("MissingType", func(t *testing.T) {
		config := &EmbeddingProviderConfig{
			Model: "test-model",
		}

		err := ValidateEmbeddingProviderConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})

	t.Run("MissingModel", func(t *testing.T) {
		config := &EmbeddingProviderConfig{
			Type: EmbeddingProviderOpenAI,
		}

		err := ValidateEmbeddingProviderConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model is required")
	})

	t.Run("MissingAPIKey", func(t *testing.T) {
		config := &EmbeddingProviderConfig{
			Type:  EmbeddingProviderOpenAI,
			Model: "text-embedding-3-small",
		}

		err := ValidateEmbeddingProviderConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires API key")
	})
}

// Test default configurations
func TestDefaultEmbeddingProviderConfigs(t *testing.T) {
	providers := []EmbeddingProviderType{
		EmbeddingProviderOpenAI,
		EmbeddingProviderOllama,
		EmbeddingProviderLocal,
		EmbeddingProviderCohere,
	}

	for _, providerType := range providers {
		t.Run(string(providerType), func(t *testing.T) {
			config := DefaultEmbeddingProviderConfig(providerType)

			assert.Equal(t, providerType, config.Type)
			assert.NotEmpty(t, config.Model)
			assert.Greater(t, config.Dimensions, 0)
			assert.Greater(t, config.MaxBatchSize, 0)
			assert.NotNil(t, config.DefaultOptions)
			assert.True(t, config.Features.EnableCaching)
		})
	}
}

// Test embedding provider capabilities
func TestEmbeddingProviderCapabilities(t *testing.T) {
	capabilitiesMap := GetEmbeddingProviderCapabilitiesMap()

	assert.NotEmpty(t, capabilitiesMap)

	for providerType, capabilities := range capabilitiesMap {
		t.Run(string(providerType), func(t *testing.T) {
			assert.NotNil(t, capabilities)
			assert.Greater(t, capabilities.MaxTokensPerText, 0)
			assert.Greater(t, capabilities.MaxBatchSize, 0)
			assert.NotEmpty(t, capabilities.SupportedModels)
			assert.NotEmpty(t, capabilities.DefaultModel)
			assert.Greater(t, capabilities.DefaultDimension, 0)
			assert.NotEmpty(t, capabilities.SupportedDimensions)
		})
	}
}

// Test embedding options
func TestEmbeddingOptions(t *testing.T) {
	t.Run("DefaultOptions", func(t *testing.T) {
		options := DefaultEmbeddingOptions()

		assert.NotNil(t, options)
		assert.True(t, options.Normalize)
		assert.True(t, options.Truncate)
		assert.Equal(t, 100, options.BatchSize)
		assert.Equal(t, 60*time.Second, options.Timeout)
		assert.Equal(t, 3, options.MaxRetries)
		assert.True(t, options.EnableCaching)
		assert.Equal(t, 24*time.Hour, options.CacheTTL)
		assert.NotNil(t, options.CustomOptions)
	})
}

// Test retry configuration
func TestEmbeddingRetryConfig(t *testing.T) {
	t.Run("DefaultRetryConfig", func(t *testing.T) {
		config := DefaultEmbeddingRetryConfig()

		assert.NotNil(t, config)
		assert.Equal(t, 3, config.MaxAttempts)
		assert.Equal(t, 1*time.Second, config.InitialDelay)
		assert.Equal(t, 30*time.Second, config.MaxDelay)
		assert.Equal(t, 2.0, config.BackoffMultiplier)
		assert.True(t, config.Jitter)
		assert.NotEmpty(t, config.RetryableErrors)
		assert.Contains(t, config.RetryableErrors, "rate_limit")
		assert.Contains(t, config.RetryableErrors, "timeout")
	})
}
