package registry

import (
	"context"
	"testing"
	"time"

	ext "github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/require"
)

func TestConfiguredMockLLMProvider_AllMethods(t *testing.T) {
	cfg := ext.DefaultProviderConfig(ext.ProviderMistral)
	cfg.APIKey = "k"
	cfg.Model = "mistral-small"
	p := NewConfiguredMockLLMProvider(ext.ProviderMistral, cfg).(*ConfiguredMockLLMProvider)
	ctx := context.Background()

	s, err := p.GenerateCompletion(ctx, "hello")
	require.NoError(t, err)
	require.NotEmpty(t, s)

	s, err = p.GenerateCompletionWithOptions(ctx, "hello", &ext.CompletionOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, s)

	out, err := p.GenerateStructuredOutput(ctx, "x", map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, out)
	out, err = p.GenerateStructuredOutputWithOptions(ctx, "x", map[string]any{}, &ext.CompletionOptions{})
	require.NoError(t, err)
	require.NotNil(t, out)

	ents, err := p.ExtractEntities(ctx, "x")
	require.NoError(t, err)
	require.NotEmpty(t, ents)
	rels, err := p.ExtractRelationships(ctx, "x", ents)
	require.NoError(t, err)
	require.NotEmpty(t, rels)

	out, err = p.ExtractWithCustomSchema(ctx, "x", map[string]interface{}{"type": "object"})
	require.NoError(t, err)
	require.NotNil(t, out)

	msg := []ext.Message{{Role: ext.RoleUser, Content: "hi"}}
	s, err = p.GenerateWithContext(ctx, msg, nil)
	require.NoError(t, err)
	require.NotEmpty(t, s)

	streamCalls := 0
	err = p.GenerateStreamingCompletion(ctx, "hello", func(chunk string, done bool, e error) {
		streamCalls++
	})
	require.NoError(t, err)
	require.Greater(t, streamCalls, 1)

	require.Equal(t, "mistral-small", p.GetModel())
	require.NoError(t, p.SetModel("mistral-medium"))
	require.Equal(t, "mistral-medium", p.GetModel())
	require.Equal(t, ext.ProviderMistral, p.GetProviderType())
	require.Greater(t, p.GetMaxTokens(), 0)
	tok, err := p.GetTokenCount("12345678")
	require.NoError(t, err)
	require.Equal(t, 2, tok)

	caps := p.GetCapabilities()
	require.True(t, caps.SupportsCompletion)

	require.NoError(t, p.Health(ctx))
	p.isHealthy = false
	require.Error(t, p.Health(ctx))
	p.isHealthy = true

	usage, err := p.GetUsage(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, usage.TotalRequests, int64(1))
	rl, err := p.GetRateLimit(ctx)
	require.NoError(t, err)
	require.NotZero(t, rl.RequestsPerMinute)

	bad := &ext.ProviderConfig{Type: ext.ProviderOpenAI}
	require.Error(t, p.Configure(bad))
	require.NoError(t, p.Configure(cfg))
	gotCfg := p.GetConfiguration()
	require.Equal(t, cfg.Model, gotCfg.Model)
	require.NoError(t, p.Close())
}

func TestConfiguredMockEmbeddingProvider_AllMethods(t *testing.T) {
	cfg := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderLocal)
	cfg.Dimensions = 16
	cfg.Model = "local-e"
	p := NewConfiguredMockEmbeddingProvider(cfg).(*ConfiguredMockEmbeddingProvider)
	ctx := context.Background()

	v, err := p.GenerateEmbedding(ctx, "hello")
	require.NoError(t, err)
	require.Len(t, v, 16)

	b, err := p.GenerateBatchEmbeddings(ctx, []string{"a", "b"})
	require.NoError(t, err)
	require.Len(t, b, 2)

	_, err = p.GenerateEmbeddingWithOptions(ctx, "x", &ext.EmbeddingOptions{})
	require.NoError(t, err)
	_, err = p.GenerateBatchEmbeddingsWithOptions(ctx, []string{"x"}, &ext.EmbeddingOptions{})
	require.NoError(t, err)

	require.Equal(t, 16, p.GetDimensions())
	require.Equal(t, "local-e", p.GetModel())
	require.NoError(t, p.SetModel("local-e-2"))
	require.Equal(t, "local-e-2", p.GetModel())
	require.Equal(t, cfg.Type, p.GetProviderType())
	require.Greater(t, p.GetMaxBatchSize(), 0)
	require.Greater(t, p.GetMaxTokensPerText(), 0)

	_, err = p.GenerateEmbeddingCached(ctx, "x", time.Second)
	require.NoError(t, err)
	_, err = p.GenerateBatchEmbeddingsCached(ctx, []string{"x"}, time.Second)
	require.NoError(t, err)

	m, err := p.DeduplicateAndEmbed(ctx, []string{"a", "a", "b"})
	require.NoError(t, err)
	require.Len(t, m, 2)

	tok, err := p.GetTokenCount("12345678")
	require.NoError(t, err)
	require.Equal(t, 2, tok)
	cost, err := p.EstimateCost(100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, cost, 0.0)

	require.NoError(t, p.Health(ctx))
	p.isHealthy = false
	require.Error(t, p.Health(ctx))
	p.isHealthy = true

	usage, err := p.GetUsage(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, usage.TotalRequests, int64(1))
	rl, err := p.GetRateLimit(ctx)
	require.NoError(t, err)
	require.NotZero(t, rl.RequestsPerMinute)

	bad := &ext.EmbeddingProviderConfig{Type: ext.EmbeddingProviderOpenAI, Model: "m"}
	require.Error(t, p.Configure(bad))
	require.NoError(t, p.Configure(cfg))
	got := p.GetConfiguration()
	require.Equal(t, cfg.Model, got.Model)
	require.NoError(t, p.ValidateConfiguration(cfg))
	require.NoError(t, p.Close())

	require.False(t, p.SupportsStreaming())
	require.Error(t, p.GenerateStreamingEmbedding(ctx, "x", func([]float32, bool, error) {}))
	require.NotNil(t, p.GetCapabilities())

	require.Error(t, p.SetCustomDimensions(0))
	require.NoError(t, p.SetCustomDimensions(8))
	require.True(t, p.SupportsCustomDimensions())
}

func TestFactory_CreateAdditionalMockBackedProviders(t *testing.T) {
	lf := NewProviderFactory()
	for _, pt := range []ext.ProviderType{
		ext.ProviderBedrock, ext.ProviderAzure, ext.ProviderCohere, ext.ProviderHuggingFace, ext.ProviderLocal,
	} {
		cfg := ext.DefaultProviderConfig(pt)
		cfg.Model = "x"
		cfg.APIKey = "k"
		p, err := lf.CreateProvider(cfg)
		require.NoError(t, err)
		require.Equal(t, pt, p.GetProviderType())
	}

	ef := NewEmbeddingProviderFactory()
	for _, et := range []ext.EmbeddingProviderType{
		ext.EmbeddingProviderLocal, ext.EmbeddingProviderSentenceTransform, ext.EmbeddingProviderHuggingFace,
		ext.EmbeddingProviderCohere, ext.EmbeddingProviderAzure, ext.EmbeddingProviderBedrock, ext.EmbeddingProviderVertex,
	} {
		cfg := ext.DefaultEmbeddingProviderConfig(et)
		cfg.Model = "x"
		cfg.Dimensions = 8
		cfg.APIKey = "k"
		p, err := ef.CreateProvider(cfg)
		require.NoError(t, err)
		require.Equal(t, et, p.GetProviderType())
	}
}

