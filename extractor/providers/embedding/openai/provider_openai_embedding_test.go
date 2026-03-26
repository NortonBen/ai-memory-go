// Package openai - OpenAI embedding provider tests
package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	openai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIEmbeddingProvider(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		model   string
		wantErr bool
	}{
		{
			name:    "valid provider with default model",
			apiKey:  "test-api-key",
			model:   "",
			wantErr: false,
		},
		{
			name:    "valid provider with custom model",
			apiKey:  "test-api-key",
			model:   string(string(openai.SmallEmbedding3)),
			wantErr: false,
		},
		{
			name:    "missing API key",
			apiKey:  "",
			model:   string(string(openai.SmallEmbedding3)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIEmbeddingProvider(tt.apiKey, tt.model)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestOpenAIEmbeddingProvider_GetDimensions(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		{
			name:     "text-embedding-3-small",
			model:    string(string(openai.SmallEmbedding3)),
			expected: 1536,
		},
		{
			name:     "text-embedding-3-large",
			model:    string(string(openai.LargeEmbedding3)),
			expected: 3072,
		},
		{
			name:     "text-embedding-ada-002",
			model:    string(string(openai.AdaEmbeddingV2)),
			expected: 1536,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIEmbeddingProvider("test-api-key", tt.model)
			require.NoError(t, err)

			dims := provider.GetDimensions()
			assert.Equal(t, tt.expected, dims)
		})
	}
}

func TestOpenAIEmbeddingProvider_GetModel(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(string(openai.SmallEmbedding3)))
	require.NoError(t, err)

	assert.Equal(t, string(string(openai.SmallEmbedding3)), provider.GetModel())
}

func TestOpenAIEmbeddingProvider_SetModel(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(string(openai.SmallEmbedding3)))
	require.NoError(t, err)

	err = provider.SetModel(string(string(openai.LargeEmbedding3)))
	assert.NoError(t, err)
	assert.Equal(t, string(string(openai.LargeEmbedding3)), provider.GetModel())
	assert.Equal(t, 3072, provider.GetDimensions())
}

func TestOpenAIEmbeddingProvider_SetModel_Invalid(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(string(openai.SmallEmbedding3)))
	require.NoError(t, err)

	err = provider.SetModel("invalid-model")
	assert.Error(t, err)
}

func TestOpenAIEmbeddingProvider_GetProviderType(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(string(openai.SmallEmbedding3)))
	require.NoError(t, err)

	assert.Equal(t, extractor.EmbeddingProviderOpenAI, provider.GetProviderType())
}

func TestOpenAIEmbeddingProvider_GetSupportedModels(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	models := provider.GetSupportedModels()
	assert.NotEmpty(t, models)
	assert.Contains(t, models, string(openai.SmallEmbedding3))
	assert.Contains(t, models, string(openai.LargeEmbedding3))
	assert.Contains(t, models, string(openai.AdaEmbeddingV2))
}

func TestOpenAIEmbeddingProvider_GetMaxBatchSize(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	batchSize := provider.GetMaxBatchSize()
	assert.Equal(t, 2048, batchSize)
}

func TestOpenAIEmbeddingProvider_GetMaxTokensPerText(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	maxTokens := provider.GetMaxTokensPerText()
	assert.Equal(t, 8192, maxTokens)
}

func TestOpenAIEmbeddingProvider_GetTokenCount(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	text := "This is a test text for token counting"
	count, err := provider.GetTokenCount(text)
	assert.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestOpenAIEmbeddingProvider_EstimateCost(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	cost, err := provider.EstimateCost(1000)
	assert.NoError(t, err)
	assert.Greater(t, cost, 0.0)
}

func TestOpenAIEmbeddingProvider_SupportsStreaming(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	assert.False(t, provider.SupportsStreaming())
}

func TestOpenAIEmbeddingProvider_SupportsCustomDimensions(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "text-embedding-3-small supports custom dims",
			model:    string(openai.SmallEmbedding3),
			expected: true,
		},
		{
			name:     "text-embedding-3-large supports custom dims",
			model:    string(openai.LargeEmbedding3),
			expected: true,
		},
		{
			name:     "text-embedding-ada-002 does not support custom dims",
			model:    string(openai.AdaEmbeddingV2),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIEmbeddingProvider("test-api-key", tt.model)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, provider.SupportsCustomDimensions())
		})
	}
}

func TestOpenAIEmbeddingProvider_SetCustomDimensions(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	err = provider.SetCustomDimensions(512)
	assert.NoError(t, err)
	assert.Equal(t, 512, provider.GetDimensions())
}

func TestOpenAIEmbeddingProvider_SetCustomDimensions_Invalid(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		dimensions int
		wantErr    bool
	}{
		{
			name:       "valid dimensions for small model",
			model:      string(openai.SmallEmbedding3),
			dimensions: 512,
			wantErr:    false,
		},
		{
			name:       "dimensions too large for small model",
			model:      string(openai.SmallEmbedding3),
			dimensions: 2000,
			wantErr:    true,
		},
		{
			name:       "dimensions too small",
			model:      string(openai.SmallEmbedding3),
			dimensions: 0,
			wantErr:    true,
		},
		{
			name:       "model does not support custom dimensions",
			model:      string(openai.AdaEmbeddingV2),
			dimensions: 512,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIEmbeddingProvider("test-api-key", tt.model)
			require.NoError(t, err)

			err = provider.SetCustomDimensions(tt.dimensions)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOpenAIEmbeddingProvider_GetCapabilities(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	caps := provider.GetCapabilities()

	assert.True(t, caps.SupportsBatching)
	assert.False(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsCustomDims)
	assert.True(t, caps.SupportsNormalization)
	assert.Greater(t, caps.MaxTokensPerText, 0)
	assert.Greater(t, caps.MaxBatchSize, 0)
	assert.NotEmpty(t, caps.SupportedModels)
	assert.NotEmpty(t, caps.SupportedDimensions)
}

func TestOpenAIEmbeddingProvider_Configure(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	newConfig := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
	newConfig.APIKey = "new-api-key"
	newConfig.Model = string(openai.LargeEmbedding3)
	newConfig.Dimensions = 3072

	err = provider.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, string(openai.LargeEmbedding3), provider.GetModel())
	assert.Equal(t, 3072, provider.GetDimensions())
}

func TestOpenAIEmbeddingProvider_GetConfiguration(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	config := provider.GetConfiguration()
	assert.NotNil(t, config)
	assert.Equal(t, extractor.EmbeddingProviderOpenAI, config.Type)
	assert.Equal(t, string(openai.SmallEmbedding3), config.Model)
}

func TestOpenAIEmbeddingProvider_ValidateConfiguration(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	validConfig := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
	validConfig.APIKey = "test-key"
	err = provider.ValidateConfiguration(validConfig)
	assert.NoError(t, err)

	invalidConfig := &extractor.EmbeddingProviderConfig{}
	err = provider.ValidateConfiguration(invalidConfig)
	assert.Error(t, err)
}

func TestOpenAIEmbeddingProvider_Close(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
}

func TestOpenAIEmbeddingProvider_GetUsage(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	ctx := context.Background()
	usage, err := provider.GetUsage(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, usage)
	assert.GreaterOrEqual(t, usage.TotalRequests, int64(0))
}

func TestOpenAIEmbeddingProvider_GetRateLimit(t *testing.T) {
	provider, err := NewOpenAIEmbeddingProvider("test-api-key", string(openai.SmallEmbedding3))
	require.NoError(t, err)

	ctx := context.Background()
	rateLimit, err := provider.GetRateLimit(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, rateLimit)
	assert.Greater(t, rateLimit.RequestsPerMinute, 0)
}

// Integration tests (require valid API key)

func TestOpenAIEmbeddingProvider_Integration_GenerateEmbedding(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIEmbeddingProvider(apiKey, string(openai.SmallEmbedding3))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text := "This is a test text for embedding generation"
	embedding, err := provider.GenerateEmbedding(ctx, text)
	assert.NoError(t, err)
	assert.NotEmpty(t, embedding)
	assert.Equal(t, 1536, len(embedding))
	t.Logf("Generated embedding with %d dimensions", len(embedding))
}

func TestOpenAIEmbeddingProvider_Integration_GenerateBatchEmbeddings(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIEmbeddingProvider(apiKey, string(openai.SmallEmbedding3))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	texts := []string{
		"First test text",
		"Second test text",
		"Third test text",
	}

	embeddings, err := provider.GenerateBatchEmbeddings(ctx, texts)
	assert.NoError(t, err)
	assert.Len(t, embeddings, 3)
	for i, embedding := range embeddings {
		assert.Equal(t, 1536, len(embedding))
		t.Logf("Embedding %d has %d dimensions", i+1, len(embedding))
	}
}

func TestOpenAIEmbeddingProvider_Integration_DeduplicateAndEmbed(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIEmbeddingProvider(apiKey, string(openai.SmallEmbedding3))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	texts := []string{
		"Unique text 1",
		"Unique text 2",
		"Unique text 1", // Duplicate
		"Unique text 3",
	}

	result, err := provider.DeduplicateAndEmbed(ctx, texts)
	assert.NoError(t, err)
	assert.Len(t, result, 3) // Only 3 unique texts
	t.Logf("Generated embeddings for %d unique texts", len(result))
}

func TestOpenAIEmbeddingProvider_Integration_CustomDimensions(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIEmbeddingProvider(apiKey, string(openai.SmallEmbedding3))
	require.NoError(t, err)

	// Set custom dimensions
	err = provider.SetCustomDimensions(512)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text := "Test text with custom dimensions"
	embedding, err := provider.GenerateEmbedding(ctx, text)
	assert.NoError(t, err)
	assert.Equal(t, 512, len(embedding))
	t.Logf("Generated embedding with custom %d dimensions", len(embedding))
}

func TestOpenAIEmbeddingProvider_Integration_Health(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIEmbeddingProvider(apiKey, string(openai.SmallEmbedding3))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = provider.Health(ctx)
	assert.NoError(t, err)
}

func BenchmarkOpenAIEmbeddingProvider_GenerateEmbedding(b *testing.B) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		b.Skip("OPENAI_API_KEY not set, skipping benchmark")
	}

	provider, err := NewOpenAIEmbeddingProvider(apiKey, string(openai.SmallEmbedding3))
	require.NoError(b, err)

	ctx := context.Background()
	text := "Benchmark test text"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.GenerateEmbedding(ctx, text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOpenAIEmbeddingProvider_GenerateBatchEmbeddings(b *testing.B) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		b.Skip("OPENAI_API_KEY not set, skipping benchmark")
	}

	provider, err := NewOpenAIEmbeddingProvider(apiKey, string(openai.SmallEmbedding3))
	require.NoError(b, err)

	ctx := context.Background()
	texts := []string{
		"Benchmark text 1",
		"Benchmark text 2",
		"Benchmark text 3",
		"Benchmark text 4",
		"Benchmark text 5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.GenerateBatchEmbeddings(ctx, texts)
		if err != nil {
			b.Fatal(err)
		}
	}
}
