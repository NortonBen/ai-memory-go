package extractor

import (
	"context"
	"time"
)

// MockEmbeddingProvider implements EmbeddingProvider for testing
type MockEmbeddingProvider struct {
	model      string
	dimensions int
	embeddings map[string][]float32
	healthy    bool
	usage      *EmbeddingUsageStats
	rateLimit  *EmbeddingRateLimitStatus
	config     *EmbeddingProviderConfig
}

// NewMockEmbeddingProvider creates a new mock embedding provider
func NewMockEmbeddingProvider(model string, dimensions int) *MockEmbeddingProvider {
	return &MockEmbeddingProvider{
		model:      model,
		dimensions: dimensions,
		embeddings: make(map[string][]float32),
		healthy:    true,
		usage: &EmbeddingUsageStats{
			TotalRequests:      0,
			SuccessfulRequests: 0,
			FailedRequests:     0,
			PeriodStart:        time.Now(),
			PeriodEnd:          time.Now(),
		},
		rateLimit: &EmbeddingRateLimitStatus{
			RequestsPerMinute: 1000,
			TokensPerMinute:   100000,
			RequestsRemaining: 1000,
			TokensRemaining:   100000,
			IsLimited:         false,
		},
		config: DefaultEmbeddingProviderConfig(EmbeddingProviderLocal),
	}
}

// Core embedding generation methods
func (m *MockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.usage.TotalRequests++
	m.usage.SuccessfulRequests++
	m.usage.TotalTextsProcessed++

	embedding := make([]float32, m.dimensions)
	for i := range embedding {
		embedding[i] = float32(i) / float32(m.dimensions)
	}

	m.embeddings[text] = embedding
	return embedding, nil
}

func (m *MockEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := m.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (m *MockEmbeddingProvider) GenerateEmbeddingWithOptions(ctx context.Context, text string, options *EmbeddingOptions) ([]float32, error) {
	return m.GenerateEmbedding(ctx, text)
}

func (m *MockEmbeddingProvider) GenerateBatchEmbeddingsWithOptions(ctx context.Context, texts []string, options *EmbeddingOptions) ([][]float32, error) {
	return m.GenerateBatchEmbeddings(ctx, texts)
}

func (m *MockEmbeddingProvider) GetDimensions() int {
	return m.dimensions
}

func (m *MockEmbeddingProvider) GetModel() string {
	return m.model
}

func (m *MockEmbeddingProvider) SetModel(model string) error {
	m.model = model
	return nil
}

func (m *MockEmbeddingProvider) GetProviderType() EmbeddingProviderType {
	return m.config.Type
}

func (m *MockEmbeddingProvider) GetCapabilities() *EmbeddingProviderCapabilities {
	return &EmbeddingProviderCapabilities{
		SupportsBatching:      true,
		SupportsStreaming:     false,
		SupportsCustomDims:    true,
		SupportsNormalization: true,
		MaxTokensPerText:      512,
		MaxBatchSize:          100,
		SupportedModels:       []string{"mock-model-v1", "mock-model-v2"},
		DefaultModel:          "mock-model-v1",
		SupportedDimensions:   []int{128, 256, 384, 512, 768, 1024, 1536},
		DefaultDimension:      384,
	}
}

func (m *MockEmbeddingProvider) GetSupportedModels() []string {
	return []string{"mock-model-v1", "mock-model-v2"}
}

func (m *MockEmbeddingProvider) GetMaxBatchSize() int {
	return 100
}

func (m *MockEmbeddingProvider) GetMaxTokensPerText() int {
	return 512
}

func (m *MockEmbeddingProvider) GetTokenCount(text string) (int, error) {
	return len(text) / 4, nil
}

func (m *MockEmbeddingProvider) GetMaxTokens() int {
	return 8192
}

func (m *MockEmbeddingProvider) Health(ctx context.Context) error {
	if !m.healthy {
		return NewExtractorError("health_check_failed", "mock provider is unhealthy", 503)
	}
	return nil
}

func (m *MockEmbeddingProvider) GetUsage(ctx context.Context) (*EmbeddingUsageStats, error) {
	return m.usage, nil
}

func (m *MockEmbeddingProvider) GetRateLimit(ctx context.Context) (*EmbeddingRateLimitStatus, error) {
	return m.rateLimit, nil
}

func (m *MockEmbeddingProvider) Configure(config *EmbeddingProviderConfig) error {
	m.config = config
	return nil
}

func (m *MockEmbeddingProvider) GetConfiguration() *EmbeddingProviderConfig {
	return m.config
}

func (m *MockEmbeddingProvider) Close() error {
	return nil
}

// Extra methods for testing convenience
func (m *MockEmbeddingProvider) GenerateEmbeddingCached(ctx context.Context, text string, ttl time.Duration) ([]float32, error) {
	return m.GenerateEmbedding(ctx, text)
}

func (m *MockEmbeddingProvider) GenerateBatchEmbeddingsCached(ctx context.Context, texts []string, ttl time.Duration) ([][]float32, error) {
	return m.GenerateBatchEmbeddings(ctx, texts)
}

func (m *MockEmbeddingProvider) DeduplicateAndEmbed(ctx context.Context, texts []string) (map[string][]float32, error) {
	result := make(map[string][]float32)
	for _, text := range texts {
		if _, ok := result[text]; !ok {
			emb, _ := m.GenerateEmbedding(ctx, text)
			result[text] = emb
		}
	}
	return result, nil
}

func (m *MockEmbeddingProvider) EstimateCost(tokenCount int) (float64, error) {
	return float64(tokenCount) * 0.0001, nil
}

func (m *MockEmbeddingProvider) ValidateConfiguration(config *EmbeddingProviderConfig) error {
	return ValidateEmbeddingProviderConfig(config)
}

func (m *MockEmbeddingProvider) SupportsStreaming() bool {
	return false
}

func (m *MockEmbeddingProvider) GenerateStreamingEmbedding(ctx context.Context, text string, callback EmbeddingStreamCallback) error {
	emb, err := m.GenerateEmbedding(ctx, text)
	callback(emb, true, err)
	return err
}

func (m *MockEmbeddingProvider) SupportsCustomDimensions() bool {
	return true
}

func (m *MockEmbeddingProvider) SetCustomDimensions(dimensions int) error {
	m.dimensions = dimensions
	return nil
}
