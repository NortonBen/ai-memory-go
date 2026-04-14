package extractor

import (
	"context"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

func TestMockLLMProvider_AllMethods(t *testing.T) {
	p := NewMockLLMProvider(ProviderOpenAI, "gpt-x")
	ctx := context.Background()

	s, err := p.GenerateCompletion(ctx, "hi")
	require.NoError(t, err)
	require.NotEmpty(t, s)
	s, err = p.GenerateCompletionWithOptions(ctx, "hi", &CompletionOptions{MaxTokens: 10})
	require.NoError(t, err)
	require.NotEmpty(t, s)

	out, err := p.GenerateStructuredOutput(ctx, "x", map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, out)
	out, err = p.GenerateStructuredOutputWithOptions(ctx, "x", map[string]interface{}{}, &CompletionOptions{})
	require.NoError(t, err)
	require.NotNil(t, out)

	ents, err := p.ExtractEntities(ctx, "text")
	require.NoError(t, err)
	require.Len(t, ents, 1)

	rels, err := p.ExtractRelationships(ctx, "text", []schema.Node{ents[0]})
	require.NoError(t, err)
	require.Len(t, rels, 0)
	rels, err = p.ExtractRelationships(ctx, "text", []schema.Node{ents[0], ents[0]})
	require.NoError(t, err)
	require.Len(t, rels, 1)

	out, err = p.ExtractWithCustomSchema(ctx, "x", map[string]interface{}{"type": "object"})
	require.NoError(t, err)
	require.NotNil(t, out)

	s, err = p.GenerateWithContext(ctx, []Message{{Role: RoleUser, Content: "hello"}}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, s)

	streamCount := 0
	err = p.GenerateStreamingCompletion(ctx, "hello", func(chunk string, done bool, err error) {
		streamCount++
	})
	require.NoError(t, err)
	require.Equal(t, 3, streamCount)

	require.Equal(t, "gpt-x", p.GetModel())
	require.NoError(t, p.SetModel("gpt-y"))
	require.Equal(t, "gpt-y", p.GetModel())
	require.Equal(t, ProviderOpenAI, p.GetProviderType())
	require.True(t, p.GetCapabilities().SupportsCompletion)
	tok, err := p.GetTokenCount("12345678")
	require.NoError(t, err)
	require.Equal(t, 2, tok)
	require.Equal(t, 4096, p.GetMaxTokens())
	require.NoError(t, p.Health(ctx))

	usage, err := p.GetUsage(ctx)
	require.NoError(t, err)
	require.Greater(t, usage.TotalRequests, int64(0))
	rl, err := p.GetRateLimit(ctx)
	require.NoError(t, err)
	require.NotZero(t, rl.RequestsPerMinute)

	cfg := DefaultProviderConfig(ProviderOpenAI)
	require.NoError(t, p.Configure(cfg))
	require.NotNil(t, p.GetConfiguration())
	require.NoError(t, p.Close())
}

func TestMockEmbeddingProvider_AllMethods(t *testing.T) {
	p := NewMockEmbeddingProvider("m", 8)
	ctx := context.Background()

	v, err := p.GenerateEmbedding(ctx, "hello")
	require.NoError(t, err)
	require.Len(t, v, 8)

	b, err := p.GenerateBatchEmbeddings(ctx, []string{"a", "b"})
	require.NoError(t, err)
	require.Len(t, b, 2)
	_, err = p.GenerateEmbeddingWithOptions(ctx, "x", &EmbeddingOptions{})
	require.NoError(t, err)
	_, err = p.GenerateBatchEmbeddingsWithOptions(ctx, []string{"x"}, &EmbeddingOptions{})
	require.NoError(t, err)

	require.Equal(t, 8, p.GetDimensions())
	require.Equal(t, "m", p.GetModel())
	require.NoError(t, p.SetModel("m2"))
	require.Equal(t, "m2", p.GetModel())

	require.Equal(t, EmbeddingProviderLocal, p.GetProviderType())
	require.NotNil(t, p.GetCapabilities())
	require.NotEmpty(t, p.GetSupportedModels())
	require.Equal(t, 100, p.GetMaxBatchSize())
	require.Equal(t, 512, p.GetMaxTokensPerText())
	tok, err := p.GetTokenCount("12345678")
	require.NoError(t, err)
	require.Equal(t, 2, tok)
	require.Equal(t, 8192, p.GetMaxTokens())

	require.NoError(t, p.Health(ctx))
	p.healthy = false
	require.Error(t, p.Health(ctx))
	p.healthy = true

	usage, err := p.GetUsage(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, usage.TotalRequests, int64(1))
	rl, err := p.GetRateLimit(ctx)
	require.NoError(t, err)
	require.NotZero(t, rl.RequestsPerMinute)

	cfg := DefaultEmbeddingProviderConfig(EmbeddingProviderLocal)
	cfg.Dimensions = 16
	require.NoError(t, p.Configure(cfg))
	require.Equal(t, 16, p.GetConfiguration().Dimensions)
	require.NoError(t, p.Close())

	_, err = p.GenerateEmbeddingCached(ctx, "x", time.Second)
	require.NoError(t, err)
	_, err = p.GenerateBatchEmbeddingsCached(ctx, []string{"x", "y"}, time.Second)
	require.NoError(t, err)
	dedup, err := p.DeduplicateAndEmbed(ctx, []string{"a", "a", "b"})
	require.NoError(t, err)
	require.Len(t, dedup, 2)
	cost, err := p.EstimateCost(100)
	require.NoError(t, err)
	require.Greater(t, cost, 0.0)
	require.NoError(t, p.ValidateConfiguration(cfg))
	require.False(t, p.SupportsStreaming())
	require.True(t, p.SupportsCustomDimensions())
	require.NoError(t, p.SetCustomDimensions(4))
	require.Equal(t, 4, p.GetDimensions())

	streamCalled := false
	err = p.GenerateStreamingEmbedding(ctx, "x", func(embedding []float32, done bool, err error) {
		streamCalled = true
		require.True(t, done)
		require.NoError(t, err)
		require.Len(t, embedding, p.GetDimensions())
	})
	require.NoError(t, err)
	require.True(t, streamCalled)
}

