package extractor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultProviderManager(t *testing.T) {
	manager := NewProviderManager()

	t.Run("AddProvider", func(t *testing.T) {
		provider := NewMockLLMProvider(ProviderOpenAI, "gpt-4")

		err := manager.AddProvider("openai", provider, 1)
		assert.NoError(t, err)

		// Test adding nil provider
		err = manager.AddProvider("nil", nil, 1)
		assert.Error(t, err)
	})

	t.Run("GetProvider", func(t *testing.T) {
		provider := NewMockLLMProvider(ProviderOllama, "llama2")

		err := manager.AddProvider("ollama", provider, 2)
		require.NoError(t, err)

		retrieved, err := manager.GetProvider("ollama")
		assert.NoError(t, err)
		assert.Equal(t, provider, retrieved)

		// Test non-existent provider
		_, err = manager.GetProvider("nonexistent")
		assert.Error(t, err)
	})

	t.Run("ListProviders", func(t *testing.T) {
		provider1 := NewMockLLMProvider(ProviderOpenAI, "gpt-4")
		provider2 := NewMockLLMProvider(ProviderOllama, "llama2")

		manager.AddProvider("openai", provider1, 1)
		manager.AddProvider("ollama", provider2, 2)

		providers := manager.ListProviders()
		assert.Len(t, providers, 2)
		assert.Contains(t, providers, "openai")
		assert.Contains(t, providers, "ollama")
	})

	t.Run("GetBestProvider", func(t *testing.T) {
		// Clear any existing providers
		manager = NewProviderManager()

		provider1 := NewMockLLMProvider(ProviderOpenAI, "gpt-4")
		provider2 := NewMockLLMProvider(ProviderOllama, "llama2")

		manager.AddProvider("openai", provider1, 1) // Higher priority (lower number)
		manager.AddProvider("ollama", provider2, 2) // Lower priority

		ctx := context.Background()
		best, err := manager.GetBestProvider(ctx)
		assert.NoError(t, err)
		assert.Equal(t, provider1, best) // Should select higher priority provider

		// Test with no providers
		emptyManager := NewProviderManager()
		_, err = emptyManager.GetBestProvider(ctx)
		assert.Error(t, err)
	})

	t.Run("HealthCheck", func(t *testing.T) {
		provider1 := NewMockLLMProvider(ProviderOpenAI, "gpt-4")
		provider2 := NewMockLLMProvider(ProviderOllama, "llama2")

		manager.AddProvider("openai", provider1, 1)
		manager.AddProvider("ollama", provider2, 2)

		ctx := context.Background()
		results := manager.HealthCheck(ctx)

		// Both providers should be healthy (mock providers return no error)
		assert.Len(t, results, 0) // No errors means all healthy
	})
}

func TestDefaultEmbeddingProviderManager(t *testing.T) {
	manager := NewEmbeddingProviderManager()

	t.Run("AddProvider", func(t *testing.T) {
		provider := NewMockEmbeddingProvider("text-embedding-3-small", 1536)

		err := manager.AddProvider("openai", provider, 1)
		assert.NoError(t, err)

		// Test adding nil provider
		err = manager.AddProvider("nil", nil, 1)
		assert.Error(t, err)
	})

	t.Run("GetProvider", func(t *testing.T) {
		provider := NewMockEmbeddingProvider("nomic-embed-text", 768)

		err := manager.AddProvider("ollama", provider, 2)
		require.NoError(t, err)

		retrieved, err := manager.GetProvider("ollama")
		assert.NoError(t, err)
		assert.Equal(t, provider, retrieved)

		// Test non-existent provider
		_, err = manager.GetProvider("nonexistent")
		assert.Error(t, err)
	})

	t.Run("GenerateEmbeddingWithFailover", func(t *testing.T) {
		manager = NewEmbeddingProviderManager()

		provider1 := NewMockEmbeddingProvider("text-embedding-3-small", 1536)
		provider2 := NewMockEmbeddingProvider("nomic-embed-text", 768)

		manager.AddProvider("openai", provider1, 1)
		manager.AddProvider("ollama", provider2, 2)

		ctx := context.Background()
		embedding, err := manager.GenerateEmbeddingWithFailover(ctx, "test text")
		assert.NoError(t, err)
		assert.Len(t, embedding, 1536) // Should use OpenAI provider (higher priority)
	})

	t.Run("GenerateBatchEmbeddingsWithFailover", func(t *testing.T) {
		manager = NewEmbeddingProviderManager()

		provider := NewMockEmbeddingProvider("text-embedding-3-small", 1536)
		manager.AddProvider("openai", provider, 1)

		ctx := context.Background()
		texts := []string{"text1", "text2", "text3"}
		embeddings, err := manager.GenerateBatchEmbeddingsWithFailover(ctx, texts)
		assert.NoError(t, err)
		assert.Len(t, embeddings, 3)
		for _, embedding := range embeddings {
			assert.Len(t, embedding, 1536)
		}
	})
}
