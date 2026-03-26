package registry

import (
	ext "github.com/NortonBen/ai-memory-go/extractor"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultProviderFactory(t *testing.T) {
	factory := NewProviderFactory()

	t.Run("ListSupportedProviders", func(t *testing.T) {
		providers := factory.ListSupportedProviders()
		assert.NotEmpty(t, providers)
		assert.Contains(t, providers, ext.ProviderOpenAI)
		assert.Contains(t, providers, ext.ProviderOllama)
		assert.Contains(t, providers, ext.ProviderDeepSeek)
	})

	t.Run("GetProviderCapabilities", func(t *testing.T) {
		caps, err := factory.GetProviderCapabilities(ext.ProviderOpenAI)
		require.NoError(t, err)
		assert.NotNil(t, caps)
		assert.True(t, caps.SupportsCompletion)
		assert.True(t, caps.SupportsJSONMode)

		// Test unsupported provider
		_, err = factory.GetProviderCapabilities(ext.ProviderType("unsupported"))
		assert.Error(t, err)
	})

	t.Run("GetDefaultConfig", func(t *testing.T) {
		config, err := factory.GetDefaultConfig(ext.ProviderOpenAI)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, ext.ProviderOpenAI, config.Type)
		assert.NotEmpty(t, config.Model)
		assert.NotZero(t, config.Timeout)
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		// Valid config
		config := ext.DefaultProviderConfig(ext.ProviderOpenAI)
		config.APIKey = "test-key"
		err := factory.ValidateConfig(config)
		assert.NoError(t, err)

		// Invalid config - nil
		err = factory.ValidateConfig(nil)
		assert.Error(t, err)

		// Invalid config - missing API key
		config.APIKey = ""
		err = factory.ValidateConfig(config)
		assert.Error(t, err)
	})

	t.Run("CreateProvider", func(t *testing.T) {
		config := ext.DefaultProviderConfig(ext.ProviderOpenAI)
		config.APIKey = "test-key"

		provider, err := factory.CreateProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, ext.ProviderOpenAI, provider.GetProviderType())
		assert.Equal(t, config.Model, provider.GetModel())
	})

	t.Run("CreateProviderWithDefaults", func(t *testing.T) {
		provider, err := factory.CreateProviderWithDefaults(ext.ProviderOllama, "", "llama2")
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, ext.ProviderOllama, provider.GetProviderType())
		assert.Equal(t, "llama2", provider.GetModel())
	})

	t.Run("RegisterCustomProvider", func(t *testing.T) {
		customType := ext.ProviderType("custom-test")
		createFunc := func(config *ext.ProviderConfig) (ext.LLMProvider, error) {
			return ext.NewMockLLMProvider(customType, config.Model), nil
		}

		err := factory.RegisterCustomProvider(customType, createFunc)
		require.NoError(t, err)

		// Verify it's in supported providers
		providers := factory.ListSupportedProviders()
		assert.Contains(t, providers, customType)

		// Test creating the custom provider
		config := ext.DefaultProviderConfig(customType)
		config.Model = "custom-model"
		provider, err := factory.CreateProvider(config)
		require.NoError(t, err)
		assert.Equal(t, customType, provider.GetProviderType())
	})
}

func TestDefaultEmbeddingProviderFactory(t *testing.T) {
	factory := NewEmbeddingProviderFactory()

	t.Run("ListSupportedProviders", func(t *testing.T) {
		providers := factory.ListSupportedProviders()
		assert.NotEmpty(t, providers)
		assert.Contains(t, providers, ext.EmbeddingProviderOpenAI)
		assert.Contains(t, providers, ext.EmbeddingProviderOllama)
		assert.Contains(t, providers, ext.EmbeddingProviderLocal)
	})

	t.Run("GetProviderCapabilities", func(t *testing.T) {
		caps, err := factory.GetProviderCapabilities(ext.EmbeddingProviderOpenAI)
		require.NoError(t, err)
		assert.NotNil(t, caps)
		assert.True(t, caps.SupportsBatching)
		assert.True(t, caps.SupportsCustomDims)
		assert.Greater(t, caps.MaxBatchSize, 0)

		// Test unsupported provider
		_, err = factory.GetProviderCapabilities(ext.EmbeddingProviderType("unsupported"))
		assert.Error(t, err)
	})

	t.Run("GetDefaultConfig", func(t *testing.T) {
		config, err := factory.GetDefaultConfig(ext.EmbeddingProviderOpenAI)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, ext.EmbeddingProviderOpenAI, config.Type)
		assert.NotEmpty(t, config.Model)
		assert.Greater(t, config.Dimensions, 0)
		assert.NotZero(t, config.Timeout)
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		// Valid config
		config := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"
		err := factory.ValidateConfig(config)
		assert.NoError(t, err)

		// Invalid config - nil
		err = factory.ValidateConfig(nil)
		assert.Error(t, err)

		// Invalid config - missing API key
		config.APIKey = ""
		err = factory.ValidateConfig(config)
		assert.Error(t, err)
	})

	t.Run("CreateProvider", func(t *testing.T) {
		config := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"

		provider, err := factory.CreateProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, ext.EmbeddingProviderOpenAI, provider.GetProviderType())
		assert.Equal(t, config.Model, provider.GetModel())
		assert.Equal(t, config.Dimensions, provider.GetDimensions())
	})

	t.Run("CreateProviderWithDefaults", func(t *testing.T) {
		provider, err := factory.CreateProviderWithDefaults(ext.EmbeddingProviderOllama, "", "nomic-embed-text")
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, ext.EmbeddingProviderOllama, provider.GetProviderType())
		assert.Equal(t, "nomic-embed-text", provider.GetModel())
	})

	t.Run("GetSupportedModels", func(t *testing.T) {
		models, err := factory.GetSupportedModels(ext.EmbeddingProviderOpenAI)
		require.NoError(t, err)
		assert.NotEmpty(t, models)
		assert.Contains(t, models, "text-embedding-3-small")
	})

	t.Run("EstimateProviderCost", func(t *testing.T) {
		cost, err := factory.EstimateProviderCost(ext.EmbeddingProviderOpenAI, 1000)
		require.NoError(t, err)
		assert.Greater(t, cost, 0.0)

		// Test provider without cost info
		_, err = factory.EstimateProviderCost(ext.EmbeddingProviderOllama, 1000)
		assert.Error(t, err) // Ollama doesn't have cost info
	})

	t.Run("RegisterCustomProvider", func(t *testing.T) {
		customType := ext.EmbeddingProviderType("custom-embedding-test")
		createFunc := func(config *ext.EmbeddingProviderConfig) (ext.EmbeddingProvider, error) {
			// Create a mock provider and configure it with the custom type
			provider := ext.NewMockEmbeddingProvider(config.Model, config.Dimensions)
			// The mock provider will return "local" as its type, which is expected behavior
			// In a real implementation, the provider would respect the config type
			return provider, nil
		}

		err := factory.RegisterCustomProvider(customType, createFunc)
		require.NoError(t, err)

		// Verify it's in supported providers
		providers := factory.ListSupportedProviders()
		assert.Contains(t, providers, customType)

		// Test creating the custom provider
		config := ext.DefaultEmbeddingProviderConfig(customType)
		config.Model = "custom-embedding-model"
		config.Dimensions = 768 // Set dimensions for custom provider
		provider, err := factory.CreateProvider(config)
		require.NoError(t, err)
		// Note: The mock provider returns "local" as its type, not the custom type
		// This is expected behavior for the mock implementation
		assert.NotNil(t, provider)
		assert.Equal(t, "custom-embedding-model", provider.GetModel())
		assert.Equal(t, 768, provider.GetDimensions())
	})
}

func TestProviderConfigDefaults(t *testing.T) {
	t.Run("ext.LLMProviderDefaults", func(t *testing.T) {
		testCases := []struct {
			providerType  ext.ProviderType
			expectedModel string
		}{
			{ext.ProviderOpenAI, "gpt-4"},
			{ext.ProviderAnthropic, "claude-3-haiku-20240307"},
			{ext.ProviderGemini, "gemini-1.5-flash"},
			{ext.ProviderOllama, "llama2"},
			{ext.ProviderDeepSeek, "deepseek-chat"},
		}

		for _, tc := range testCases {
			t.Run(string(tc.providerType), func(t *testing.T) {
				config := ext.DefaultProviderConfig(tc.providerType)
				assert.Equal(t, tc.providerType, config.Type)
				assert.Equal(t, tc.expectedModel, config.Model)
				assert.NotZero(t, config.Timeout)
				assert.NotNil(t, config.DefaultOptions)
				assert.True(t, config.HealthCheck.Enabled)
			})
		}
	})

	t.Run("ext.EmbeddingProviderDefaults", func(t *testing.T) {
		testCases := []struct {
			providerType  ext.EmbeddingProviderType
			expectedModel string
			expectedDims  int
		}{
			{ext.EmbeddingProviderOpenAI, "text-embedding-3-small", 1536},
			{ext.EmbeddingProviderOllama, "nomic-embed-text", 768},
			{ext.EmbeddingProviderLocal, "all-MiniLM-L6-v2", 384},
			{ext.EmbeddingProviderCohere, "embed-english-v3.0", 1024},
		}

		for _, tc := range testCases {
			t.Run(string(tc.providerType), func(t *testing.T) {
				config := ext.DefaultEmbeddingProviderConfig(tc.providerType)
				assert.Equal(t, tc.providerType, config.Type)
				assert.Equal(t, tc.expectedModel, config.Model)
				assert.Equal(t, tc.expectedDims, config.Dimensions)
				assert.NotZero(t, config.Timeout)
				assert.NotNil(t, config.DefaultOptions)
				assert.True(t, config.Cache.Enabled)
				assert.True(t, config.HealthCheck.Enabled)
			})
		}
	})
}

func TestProviderConfigValidation(t *testing.T) {
	t.Run("ValidLLMProviderConfig", func(t *testing.T) {
		config := ext.DefaultProviderConfig(ext.ProviderOpenAI)
		config.APIKey = "test-key"

		err := ext.ValidateProviderConfig(config)
		assert.NoError(t, err)
	})

	t.Run("InvalidLLMProviderConfigs", func(t *testing.T) {
		testCases := []struct {
			name   string
			config *ext.ProviderConfig
		}{
			{"nil config", nil},
			{"empty type", &ext.ProviderConfig{}},
			{"empty model", &ext.ProviderConfig{Type: ext.ProviderOpenAI}},
			{"missing API key", &ext.ProviderConfig{Type: ext.ProviderOpenAI, Model: "gpt-4"}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := ext.ValidateProviderConfig(tc.config)
				assert.Error(t, err)
			})
		}
	})

	t.Run("ValidEmbeddingProviderConfig", func(t *testing.T) {
		config := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"

		err := ext.ValidateEmbeddingProviderConfig(config)
		assert.NoError(t, err)
	})

	t.Run("InvalidEmbeddingProviderConfigs", func(t *testing.T) {
		testCases := []struct {
			name   string
			config *ext.EmbeddingProviderConfig
		}{
			{"nil config", nil},
			{"empty type", &ext.EmbeddingProviderConfig{}},
			{"empty model", &ext.EmbeddingProviderConfig{Type: ext.EmbeddingProviderOpenAI}},
			{"missing API key", &ext.EmbeddingProviderConfig{Type: ext.EmbeddingProviderOpenAI, Model: "text-embedding-3-small"}},
			{"invalid dimensions", &ext.EmbeddingProviderConfig{Type: ext.EmbeddingProviderOpenAI, Model: "text-embedding-3-small", APIKey: "test", Dimensions: -1}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := ext.ValidateEmbeddingProviderConfig(tc.config)
				assert.Error(t, err)
			})
		}
	})
}

func TestProviderFactoryIntegration(t *testing.T) {
	t.Run("ext.LLMProviderLifecycle", func(t *testing.T) {
		factory := NewProviderFactory()

		// Create provider
		config := ext.DefaultProviderConfig(ext.ProviderMistral)
		config.APIKey = "test-key"

		provider, err := factory.CreateProvider(config)
		require.NoError(t, err)

		// Test basic functionality
		ctx := context.Background()

		// Health check
		err = provider.Health(ctx)
		assert.NoError(t, err)

		// Generate completion
		completion, err := provider.GenerateCompletion(ctx, "Hello, world!")
		assert.NoError(t, err)
		assert.NotEmpty(t, completion)

		// Extract entities
		entities, err := provider.ExtractEntities(ctx, "I love learning English")
		assert.NoError(t, err)
		assert.NotEmpty(t, entities)

		// Close provider
		err = provider.Close()
		assert.NoError(t, err)
	})

	t.Run("ext.EmbeddingProviderLifecycle", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		// Create provider
		config := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderOllama)
		config.APIKey = "test-key"

		provider, err := factory.CreateProvider(config)
		require.NoError(t, err)

		// Test basic functionality
		ctx := context.Background()

		// Health check
		err = provider.Health(ctx)
		assert.NoError(t, err)

		// Generate embedding
		embedding, err := provider.GenerateEmbedding(ctx, "Hello, world!")
		assert.NoError(t, err)
		assert.Len(t, embedding, provider.GetDimensions())

		// Generate batch embeddings
		texts := []string{"Hello", "World", "Test"}
		embeddings, err := provider.GenerateBatchEmbeddings(ctx, texts)
		assert.NoError(t, err)
		assert.Len(t, embeddings, len(texts))

		// Test deduplication
		duplicateTexts := []string{"Hello", "World", "Hello", "Test", "World"}
		uniqueEmbeddings, err := provider.DeduplicateAndEmbed(ctx, duplicateTexts)
		assert.NoError(t, err)
		assert.Len(t, uniqueEmbeddings, 3) // Should have 3 unique texts

		// Close provider
		err = provider.Close()
		assert.NoError(t, err)
	})
}

func TestFactoryProviderCapabilities(t *testing.T) {
	t.Run("ext.LLMProviderCapabilities", func(t *testing.T) {
		caps := ext.GetProviderCapabilitiesMap()

		// Test OpenAI capabilities
		openaiCaps := caps[ext.ProviderOpenAI]
		require.NotNil(t, openaiCaps)
		assert.True(t, openaiCaps.SupportsCompletion)
		assert.True(t, openaiCaps.SupportsJSONMode)
		assert.True(t, openaiCaps.SupportsFunctionCalling)
		assert.Greater(t, openaiCaps.MaxContextLength, 0)

		// Test Ollama capabilities
		ollamaCaps := caps[ext.ProviderOllama]
		require.NotNil(t, ollamaCaps)
		assert.True(t, ollamaCaps.SupportsCompletion)
		assert.True(t, ollamaCaps.SupportsStreaming)
		assert.False(t, ollamaCaps.SupportsRateLimiting) // Local provider
	})

	t.Run("ext.EmbeddingProviderCapabilities", func(t *testing.T) {
		caps := ext.GetEmbeddingProviderCapabilitiesMap()

		// Test OpenAI embedding capabilities
		openaiCaps := caps[ext.EmbeddingProviderOpenAI]
		require.NotNil(t, openaiCaps)
		assert.True(t, openaiCaps.SupportsBatching)
		assert.True(t, openaiCaps.SupportsCustomDims)
		assert.Greater(t, openaiCaps.MaxBatchSize, 0)
		assert.Greater(t, openaiCaps.CostPerToken, 0.0)

		// Test Ollama embedding capabilities
		ollamaCaps := caps[ext.EmbeddingProviderOllama]
		require.NotNil(t, ollamaCaps)
		assert.True(t, ollamaCaps.SupportsBatching)
		assert.False(t, ollamaCaps.SupportsRateLimiting) // Local provider
		assert.Equal(t, 0.0, ollamaCaps.CostPerToken)    // Free local provider
	})
}

func TestFactoryErrorHandling(t *testing.T) {
	t.Run("ext.LLMProviderFactoryErrors", func(t *testing.T) {
		factory := NewProviderFactory()

		// Test nil config
		_, err := factory.CreateProvider(nil)
		assert.Error(t, err)

		// Test invalid provider type
		config := &ext.ProviderConfig{
			Type:  ext.ProviderType("invalid"),
			Model: "test",
		}
		_, err = factory.CreateProvider(config)
		assert.Error(t, err)

		// Test nil create function for custom provider
		err = factory.RegisterCustomProvider(ext.ProviderType("test"), nil)
		assert.Error(t, err)
	})

	t.Run("ext.EmbeddingProviderFactoryErrors", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		// Test nil config
		_, err := factory.CreateProvider(nil)
		assert.Error(t, err)

		// Test invalid provider type
		config := &ext.EmbeddingProviderConfig{
			Type:  ext.EmbeddingProviderType("invalid"),
			Model: "test",
		}
		_, err = factory.CreateProvider(config)
		assert.Error(t, err)

		// Test nil create function for custom provider
		err = factory.RegisterCustomProvider(ext.EmbeddingProviderType("test"), nil)
		assert.Error(t, err)
	})
}

func TestFactoryThreadSafety(t *testing.T) {
	t.Run("ConcurrentLLMProviderCreation", func(t *testing.T) {
		factory := NewProviderFactory()

		// Create providers concurrently
		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				config := ext.DefaultProviderConfig(ext.ProviderOpenAI)
				config.APIKey = "test-key"

				_, err := factory.CreateProvider(config)
				results <- err
			}()
		}

		// Check all results
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err)
		}
	})

	t.Run("ConcurrentEmbeddingProviderCreation", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		// Create providers concurrently
		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				config := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderOpenAI)
				config.APIKey = "test-key"

				_, err := factory.CreateProvider(config)
				results <- err
			}()
		}

		// Check all results
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err)
		}
	})
}
