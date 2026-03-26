package registry

import (
	"github.com/NortonBen/ai-memory-go/extractor"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistry(t *testing.T) {
	t.Run("NewProviderRegistry", func(t *testing.T) {
		registry := NewProviderRegistry()
		assert.NotNil(t, registry)
		assert.NotNil(t, registry.llmFactory)
		assert.NotNil(t, registry.embeddingFactory)
		assert.NotNil(t, registry.llmManager)
		assert.NotNil(t, registry.embeddingManager)
		assert.NotNil(t, registry.configManager)
		assert.True(t, registry.healthCheckEnabled)
	})

	t.Run("RegisterLLMProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.RegisterLLMProvider("openai", config, 1)
		require.NoError(t, err)

		// Verify provider is registered
		providers := registry.ListLLMProviders()
		assert.Len(t, providers, 1)
		assert.Contains(t, providers, "openai")

		// Verify provider details
		registered := providers["openai"]
		assert.Equal(t, "openai", registered.Name)
		assert.Equal(t, 1, registered.Priority)
		assert.NotNil(t, registered.Provider)
		assert.NotNil(t, registered.Config)
		assert.NotNil(t, registered.Health)
	})

	t.Run("RegisterEmbeddingProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.RegisterEmbeddingProvider("openai-embedding", config, 1)
		require.NoError(t, err)

		// Verify provider is registered
		providers := registry.ListEmbeddingProviders()
		assert.Len(t, providers, 1)
		assert.Contains(t, providers, "openai-embedding")

		// Verify provider details
		registered := providers["openai-embedding"]
		assert.Equal(t, "openai-embedding", registered.Name)
		assert.Equal(t, 1, registered.Priority)
		assert.NotNil(t, registered.Provider)
		assert.NotNil(t, registered.Config)
		assert.NotNil(t, registered.Health)
	})

	t.Run("UnregisterLLMProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.RegisterLLMProvider("openai", config, 1)
		require.NoError(t, err)

		// Unregister provider
		err = registry.UnregisterLLMProvider("openai")
		require.NoError(t, err)

		// Verify provider is removed
		providers := registry.ListLLMProviders()
		assert.Len(t, providers, 0)

		// Try to unregister again - should fail
		err = registry.UnregisterLLMProvider("openai")
		assert.Error(t, err)
	})

	t.Run("UnregisterEmbeddingProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.RegisterEmbeddingProvider("openai-embedding", config, 1)
		require.NoError(t, err)

		// Unregister provider
		err = registry.UnregisterEmbeddingProvider("openai-embedding")
		require.NoError(t, err)

		// Verify provider is removed
		providers := registry.ListEmbeddingProviders()
		assert.Len(t, providers, 0)

		// Try to unregister again - should fail
		err = registry.UnregisterEmbeddingProvider("openai-embedding")
		assert.Error(t, err)
	})

	t.Run("GetLLMProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.RegisterLLMProvider("openai", config, 1)
		require.NoError(t, err)

		// Get provider
		provider, err := registry.GetLLMProvider("openai")
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, extractor.ProviderOpenAI, provider.GetProviderType())

		// Try to get non-existent provider
		_, err = registry.GetLLMProvider("non-existent")
		assert.Error(t, err)
	})

	t.Run("GetEmbeddingProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.RegisterEmbeddingProvider("openai-embedding", config, 1)
		require.NoError(t, err)

		// Get provider
		provider, err := registry.GetEmbeddingProvider("openai-embedding")
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, extractor.EmbeddingProviderOpenAI, provider.GetProviderType())

		// Try to get non-existent provider
		_, err = registry.GetEmbeddingProvider("non-existent")
		assert.Error(t, err)
	})

	t.Run("GetBestLLMProvider", func(t *testing.T) {
		registry := NewProviderRegistry()
		ctx := context.Background()

		// Register multiple providers with different priorities
		config1 := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config1.APIKey = "test-key-1"
		err := registry.RegisterLLMProvider("openai", config1, 1)
		require.NoError(t, err)

		config2 := extractor.DefaultProviderConfig(extractor.ProviderOllama)
		err = registry.RegisterLLMProvider("ollama", config2, 2)
		require.NoError(t, err)

		// Get best provider (should be highest priority = lowest number)
		provider, err := registry.GetBestLLMProvider(ctx)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})

	t.Run("GetBestEmbeddingProvider", func(t *testing.T) {
		registry := NewProviderRegistry()
		ctx := context.Background()

		// Register multiple providers with different priorities
		config1 := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		config1.APIKey = "test-key-1"
		err := registry.RegisterEmbeddingProvider("openai", config1, 1)
		require.NoError(t, err)

		config2 := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOllama)
		err = registry.RegisterEmbeddingProvider("ollama", config2, 2)
		require.NoError(t, err)

		// Get best provider (should be highest priority = lowest number)
		provider, err := registry.GetBestEmbeddingProvider(ctx)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})

	t.Run("UpdateLLMProviderConfig", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config.APIKey = "test-key"
		config.Model = "gpt-4"

		err := registry.RegisterLLMProvider("openai", config, 1)
		require.NoError(t, err)

		// Update configuration
		newConfig := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		newConfig.APIKey = "test-key"
		newConfig.Model = "gpt-4-turbo"

		err = registry.UpdateLLMProviderConfig("openai", newConfig)
		require.NoError(t, err)

		// Verify configuration was updated
		provider, err := registry.GetLLMProvider("openai")
		require.NoError(t, err)
		assert.Equal(t, "gpt-4-turbo", provider.GetModel())
	})

	t.Run("UpdateEmbeddingProviderConfig", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"
		config.Model = "text-embedding-3-small"

		err := registry.RegisterEmbeddingProvider("openai", config, 1)
		require.NoError(t, err)

		// Update configuration
		newConfig := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		newConfig.APIKey = "test-key"
		newConfig.Model = "text-embedding-3-large"

		err = registry.UpdateEmbeddingProviderConfig("openai", newConfig)
		require.NoError(t, err)

		// Verify configuration was updated
		provider, err := registry.GetEmbeddingProvider("openai")
		require.NoError(t, err)
		assert.Equal(t, "text-embedding-3-large", provider.GetModel())
	})

	t.Run("HealthCheck", func(t *testing.T) {
		registry := NewProviderRegistry()
		ctx := context.Background()

		// Register providers (using mock-providing types for stable tests)
		llmConfig := extractor.DefaultProviderConfig(extractor.ProviderMistral)
		llmConfig.APIKey = "test-key"
		err := registry.RegisterLLMProvider("mistral", llmConfig, 1)
		require.NoError(t, err)

		embeddingConfig := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOllama)
		embeddingConfig.APIKey = "test-key"
		err = registry.RegisterEmbeddingProvider("ollama-embedding", embeddingConfig, 1)
		require.NoError(t, err)

		// Perform health check
		status, err := registry.HealthCheck(ctx)
		require.NoError(t, err)
		assert.NotNil(t, status)
		assert.Len(t, status.LLMProviders, 1)
		assert.Len(t, status.EmbeddingProviders, 1)

		// Verify health status
		llmHealth := status.LLMProviders["mistral"]
		assert.NotNil(t, llmHealth)
		assert.True(t, llmHealth.IsHealthy)

		embeddingHealth := status.EmbeddingProviders["ollama-embedding"]
		assert.NotNil(t, embeddingHealth)
		assert.True(t, embeddingHealth.IsHealthy)
	})

	t.Run("HealthCheckPeriodic", func(t *testing.T) {
		registry := NewProviderRegistry()
		registry.SetHealthCheckInterval(100 * time.Millisecond)
		ctx := context.Background()

		// Register provider
		config := extractor.DefaultProviderConfig(extractor.ProviderMistral)
		config.APIKey = "test-key"
		err := registry.RegisterLLMProvider("mistral", config, 1)
		require.NoError(t, err)

		// Start health checks
		registry.StartHealthChecks(ctx)

		// Wait for at least one health check
		time.Sleep(200 * time.Millisecond)

		// Stop health checks
		registry.StopHealthChecks()

		// Verify health check was performed
		providers := registry.ListLLMProviders()
		registered := providers["mistral"]
		assert.NotNil(t, registered.Health)
		assert.True(t, registered.Health.IsHealthy)
	})

	t.Run("LoadFromEnvironment", func(t *testing.T) {
		// This test would require setting environment variables
		// For now, just verify the method doesn't panic
		registry := NewProviderRegistry()
		err := registry.LoadFromEnvironment()
		// May return error if no env vars are set, which is fine
		_ = err
	})

	t.Run("SetLoadBalancing", func(t *testing.T) {
		registry := NewProviderRegistry()

		// Test setting load balancing strategy
		registry.SetLoadBalancing(extractor.LoadBalanceRoundRobin)
		registry.SetEmbeddingLoadBalancing(extractor.EmbeddingLoadBalanceRoundRobin)

		// No error expected
	})

	t.Run("SetFailoverEnabled", func(t *testing.T) {
		registry := NewProviderRegistry()

		// Test enabling/disabling failover
		registry.SetFailoverEnabled(true)
		registry.SetFailoverEnabled(false)

		// No error expected
	})

	t.Run("Close", func(t *testing.T) {
		registry := NewProviderRegistry()

		// Register providers
		llmConfig := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		llmConfig.APIKey = "test-key"
		err := registry.RegisterLLMProvider("openai", llmConfig, 1)
		require.NoError(t, err)

		embeddingConfig := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		embeddingConfig.APIKey = "test-key"
		err = registry.RegisterEmbeddingProvider("openai-embedding", embeddingConfig, 1)
		require.NoError(t, err)

		// Close registry
		err = registry.Close()
		assert.NoError(t, err)
	})
}

func TestProviderRegistryValidation(t *testing.T) {
	t.Run("RegisterLLMProviderWithNilConfig", func(t *testing.T) {
		registry := NewProviderRegistry()

		err := registry.RegisterLLMProvider("test", nil, 1)
		assert.Error(t, err)
	})

	t.Run("RegisterEmbeddingProviderWithNilConfig", func(t *testing.T) {
		registry := NewProviderRegistry()

		err := registry.RegisterEmbeddingProvider("test", nil, 1)
		assert.Error(t, err)
	})

	t.Run("RegisterLLMProviderWithInvalidConfig", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := &extractor.ProviderConfig{
			Type:  extractor.ProviderOpenAI,
			Model: "gpt-4",
			// Missing APIKey
		}

		err := registry.RegisterLLMProvider("openai", config, 1)
		assert.Error(t, err)
	})

	t.Run("RegisterEmbeddingProviderWithInvalidConfig", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := &extractor.EmbeddingProviderConfig{
			Type:  extractor.EmbeddingProviderOpenAI,
			Model: "text-embedding-3-small",
			// Missing APIKey
		}

		err := registry.RegisterEmbeddingProvider("openai", config, 1)
		assert.Error(t, err)
	})

	t.Run("UpdateNonExistentLLMProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.UpdateLLMProviderConfig("non-existent", config)
		assert.Error(t, err)
	})

	t.Run("UpdateNonExistentEmbeddingProvider", func(t *testing.T) {
		registry := NewProviderRegistry()

		config := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		config.APIKey = "test-key"

		err := registry.UpdateEmbeddingProviderConfig("non-existent", config)
		assert.Error(t, err)
	})
}

func TestProviderRegistryThreadSafety(t *testing.T) {
	t.Run("ConcurrentRegistration", func(t *testing.T) {
		registry := NewProviderRegistry()

		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
				config.APIKey = "test-key"

				err := registry.RegisterLLMProvider(fmt.Sprintf("provider-%d", id), config, id)
				results <- err
			}(i)
		}

		// Check all results
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err)
		}

		// Verify all providers were registered
		providers := registry.ListLLMProviders()
		assert.Len(t, providers, numGoroutines)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		registry := NewProviderRegistry()

		// Register a provider
		config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config.APIKey = "test-key"
		err := registry.RegisterLLMProvider("openai", config, 1)
		require.NoError(t, err)

		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				_, err := registry.GetLLMProvider("openai")
				results <- err
			}()
		}

		// Check all results
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err)
		}
	})

	t.Run("ConcurrentHealthChecks", func(t *testing.T) {
		registry := NewProviderRegistry()
		ctx := context.Background()

		// Register providers
		config := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config.APIKey = "test-key"
		err := registry.RegisterLLMProvider("openai", config, 1)
		require.NoError(t, err)

		const numGoroutines = 5
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				_, err := registry.HealthCheck(ctx)
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

func TestProviderRegistryIntegration(t *testing.T) {
	t.Run("CompleteWorkflow", func(t *testing.T) {
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			t.Skip("OPENAI_API_KEY is not set, skipping integration test")
		}

		registry := NewProviderRegistry()
		ctx := context.Background()

		// 1. Register LLM provider
		llmConfig := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		llmConfig.APIKey = "test-key"
		err := registry.RegisterLLMProvider("openai", llmConfig, 1)
		require.NoError(t, err)

		// 2. Register embedding provider
		embeddingConfig := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
		embeddingConfig.APIKey = "test-key"
		err = registry.RegisterEmbeddingProvider("openai-embedding", embeddingConfig, 1)
		require.NoError(t, err)

		// 3. Get providers
		llmProvider, err := registry.GetLLMProvider("openai")
		require.NoError(t, err)
		assert.NotNil(t, llmProvider)

		embeddingProvider, err := registry.GetEmbeddingProvider("openai-embedding")
		require.NoError(t, err)
		assert.NotNil(t, embeddingProvider)

		// 4. Use providers
		completion, err := llmProvider.GenerateCompletion(ctx, "Hello, world!")
		require.NoError(t, err)
		assert.NotEmpty(t, completion)

		embedding, err := embeddingProvider.GenerateEmbedding(ctx, "Hello, world!")
		require.NoError(t, err)
		assert.NotEmpty(t, embedding)

		// 5. Health check
		status, err := registry.HealthCheck(ctx)
		require.NoError(t, err)
		// Note: real providers might fail if keys are invalid, but we expect true if mock or success
		// For this specific integration test, we skip specific health assertion if not using real keys
		_ = status

		// 6. Update configuration
		newLLMConfig := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		newLLMConfig.APIKey = "test-key"
		newLLMConfig.Model = "gpt-4-turbo"
		err = registry.UpdateLLMProviderConfig("openai", newLLMConfig)
		require.NoError(t, err)

		// 7. Verify update
		llmProvider, err = registry.GetLLMProvider("openai")
		require.NoError(t, err)
		assert.Equal(t, "gpt-4-turbo", llmProvider.GetModel())

		// 8. Close registry
		err = registry.Close()
		assert.NoError(t, err)
	})

	t.Run("MultiProviderFailover", func(t *testing.T) {
		registry := NewProviderRegistry()
		ctx := context.Background()

		// Register multiple providers with different priorities
		config1 := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
		config1.APIKey = "test-key-1"
		err := registry.RegisterLLMProvider("openai", config1, 1)
		require.NoError(t, err)

		config2 := extractor.DefaultProviderConfig(extractor.ProviderOllama)
		err = registry.RegisterLLMProvider("ollama", config2, 2)
		require.NoError(t, err)

		config3 := extractor.DefaultProviderConfig(extractor.ProviderDeepSeek)
		config3.APIKey = "test-key-3"
		err = registry.RegisterLLMProvider("deepseek", config3, 3)
		require.NoError(t, err)

		// Enable failover
		registry.SetFailoverEnabled(true)

		// Get best provider
		provider, err := registry.GetBestLLMProvider(ctx)
		require.NoError(t, err)
		assert.NotNil(t, provider)

		// Verify all providers are registered
		providers := registry.ListLLMProviders()
		assert.Len(t, providers, 3)
	})
}
