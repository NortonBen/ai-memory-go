// Package openai - OpenAI embedding provider implementation
package openai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIEmbeddingProvider implements extractor.EmbeddingProvider for OpenAI embedding models
type OpenAIEmbeddingProvider struct {
	client     *openai.Client
	model      string
	dimensions int
	config     *extractor.EmbeddingProviderConfig
	metrics    *extractor.EmbeddingProviderMetrics
	mu         sync.RWMutex
}

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider
func NewOpenAIEmbeddingProvider(apiKey, model string) (*OpenAIEmbeddingProvider, error) {
	if apiKey == "" {
		return nil, extractor.NewExtractorError("validation", "OpenAI API key is required", 400)
	}

	if model == "" {
		model = string(openai.SmallEmbedding3)
	}

	// Set default dimensions based on model
	dimensions := 1536
	if model == string(openai.SmallEmbedding3) {
		dimensions = 1536
	} else if model == string(openai.LargeEmbedding3) {
		dimensions = 3072
	} else if model == string(openai.AdaEmbeddingV2) {
		dimensions = 1536
	}

	client := openai.NewClient(apiKey)

	config := extractor.DefaultEmbeddingProviderConfig(extractor.EmbeddingProviderOpenAI)
	config.APIKey = apiKey
	config.Model = model
	config.Dimensions = dimensions

	return &OpenAIEmbeddingProvider{
		client:     client,
		model:      model,
		dimensions: dimensions,
		config:     config,
		metrics: &extractor.EmbeddingProviderMetrics{
			FirstRequest: time.Now(),
		},
	}, nil
}

// NewOpenAIEmbeddingProviderWithClient creates a new OpenAI embedding provider with a custom client
func NewOpenAIEmbeddingProviderWithClient(client *openai.Client, model string, dimensions int, config *extractor.EmbeddingProviderConfig) *OpenAIEmbeddingProvider {
	return &OpenAIEmbeddingProvider{
		client:     client,
		model:      model,
		dimensions: dimensions,
		config:     config,
		metrics: &extractor.EmbeddingProviderMetrics{
			FirstRequest: time.Now(),
		},
	}
}

// NewOpenAIEmbeddingProviderFromConfig creates a new OpenAI embedding provider from configuration
func NewOpenAIEmbeddingProviderFromConfig(config *extractor.EmbeddingProviderConfig) (*OpenAIEmbeddingProvider, error) {
	if err := extractor.ValidateEmbeddingProviderConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg := openai.DefaultConfig(config.APIKey)
	if config.Endpoint != "" {
		cfg.BaseURL = config.Endpoint
	}
	client := openai.NewClientWithConfig(cfg)

	return &OpenAIEmbeddingProvider{
		client:     client,
		model:      config.Model,
		dimensions: config.Dimensions,
		config:     config,
		metrics: &extractor.EmbeddingProviderMetrics{
			FirstRequest: time.Now(),
		},
	}, nil
}

// GenerateEmbedding generates an embedding for a single text
func (oep *OpenAIEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	oep.mu.Lock()
	oep.metrics.TotalRequests++
	oep.mu.Unlock()

	start := time.Now()

	req := openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(oep.model),
	}

	// Set dimensions for models that support it
	if oep.model == string(openai.SmallEmbedding3) || oep.model == string(openai.LargeEmbedding3) {
		req.Dimensions = oep.dimensions
	}

	resp, err := oep.client.CreateEmbeddings(ctx, req)
	if err != nil {
		oep.mu.Lock()
		oep.metrics.FailedRequests++
		oep.mu.Unlock()
		return nil, fmt.Errorf("OpenAI embedding API error: %w", err)
	}

	latency := time.Since(start)
	oep.mu.Lock()
	oep.metrics.SuccessfulRequests++
	oep.metrics.TotalTextsProcessed++
	oep.metrics.TotalTokensUsed += int64(resp.Usage.TotalTokens)
	oep.metrics.AverageLatency = (oep.metrics.AverageLatency + latency) / 2
	oep.mu.Unlock()

	if len(resp.Data) == 0 {
		return nil, extractor.NewExtractorError("no_response", "OpenAI returned no embeddings", 500)
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (oep *OpenAIEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	oep.mu.Lock()
	oep.metrics.TotalRequests++
	oep.metrics.TotalBatchRequests++
	oep.mu.Unlock()

	start := time.Now()

	// OpenAI supports batch embedding requests
	req := openai.EmbeddingRequest{
		Input: texts,
		Model: openai.EmbeddingModel(oep.model),
	}

	// Set dimensions for models that support it
	if oep.model == string(openai.SmallEmbedding3) || oep.model == string(openai.LargeEmbedding3) {
		req.Dimensions = oep.dimensions
	}

	resp, err := oep.client.CreateEmbeddings(ctx, req)
	if err != nil {
		oep.mu.Lock()
		oep.metrics.FailedRequests++
		oep.mu.Unlock()
		return nil, fmt.Errorf("OpenAI batch embedding API error: %w", err)
	}

	latency := time.Since(start)
	oep.mu.Lock()
	oep.metrics.SuccessfulRequests++
	oep.metrics.TotalTextsProcessed += int64(len(texts))
	oep.metrics.TotalTokensUsed += int64(resp.Usage.TotalTokens)
	oep.metrics.AverageLatency = (oep.metrics.AverageLatency + latency) / 2
	oep.mu.Unlock()

	if len(resp.Data) != len(texts) {
		return nil, extractor.NewExtractorError("invalid_response", fmt.Sprintf("expected %d embeddings, got %d", len(texts), len(resp.Data)), 500)
	}

	// Convert embeddings to [][]float32
	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// GenerateEmbeddingWithOptions generates an embedding with custom options
func (oep *OpenAIEmbeddingProvider) GenerateEmbeddingWithOptions(ctx context.Context, text string, options *extractor.EmbeddingOptions) ([]float32, error) {
	// For OpenAI, options mainly affect dimensions
	if options != nil && options.Dimensions > 0 {
		// Temporarily set dimensions
		originalDims := oep.dimensions
		oep.mu.Lock()
		oep.dimensions = options.Dimensions
		oep.mu.Unlock()

		embedding, err := oep.GenerateEmbedding(ctx, text)

		// Restore original dimensions
		oep.mu.Lock()
		oep.dimensions = originalDims
		oep.mu.Unlock()

		return embedding, err
	}

	return oep.GenerateEmbedding(ctx, text)
}

// GenerateBatchEmbeddingsWithOptions generates batch embeddings with custom options
func (oep *OpenAIEmbeddingProvider) GenerateBatchEmbeddingsWithOptions(ctx context.Context, texts []string, options *extractor.EmbeddingOptions) ([][]float32, error) {
	// For OpenAI, options mainly affect dimensions
	if options != nil && options.Dimensions > 0 {
		// Temporarily set dimensions
		originalDims := oep.dimensions
		oep.mu.Lock()
		oep.dimensions = options.Dimensions
		oep.mu.Unlock()

		embeddings, err := oep.GenerateBatchEmbeddings(ctx, texts)

		// Restore original dimensions
		oep.mu.Lock()
		oep.dimensions = originalDims
		oep.mu.Unlock()

		return embeddings, err
	}

	return oep.GenerateBatchEmbeddings(ctx, texts)
}

// GetDimensions returns the embedding dimensions
func (oep *OpenAIEmbeddingProvider) GetDimensions() int {
	oep.mu.RLock()
	defer oep.mu.RUnlock()
	return oep.dimensions
}

// GetModel returns the current model name
func (oep *OpenAIEmbeddingProvider) GetModel() string {
	oep.mu.RLock()
	defer oep.mu.RUnlock()
	return oep.model
}

// SetModel sets the model to use
func (oep *OpenAIEmbeddingProvider) SetModel(model string) error {
	oep.mu.Lock()
	defer oep.mu.Unlock()

	// Update dimensions based on model
	switch model {
	case string(openai.SmallEmbedding3):
		oep.dimensions = 1536
	case string(openai.LargeEmbedding3):
		oep.dimensions = 3072
	case string(openai.AdaEmbeddingV2):
		oep.dimensions = 1536
	default:
		return extractor.NewExtractorError("validation", fmt.Sprintf("unsupported model: %s", model), 400)
	}

	oep.model = model
	oep.config.Model = model
	oep.config.Dimensions = oep.dimensions
	return nil
}

// GetProviderType returns the provider type
func (oep *OpenAIEmbeddingProvider) GetProviderType() extractor.EmbeddingProviderType {
	return extractor.EmbeddingProviderOpenAI
}

// GetSupportedModels returns the list of supported models
func (oep *OpenAIEmbeddingProvider) GetSupportedModels() []string {
	return []string{
		string(openai.SmallEmbedding3),
		string(openai.LargeEmbedding3),
		string(openai.AdaEmbeddingV2),
	}
}

// GetMaxBatchSize returns the maximum batch size
func (oep *OpenAIEmbeddingProvider) GetMaxBatchSize() int {
	return 2048
}

// GetMaxTokensPerText returns the maximum tokens per text
func (oep *OpenAIEmbeddingProvider) GetMaxTokensPerText() int {
	return 8192
}

// GenerateEmbeddingCached generates an embedding with caching support
func (oep *OpenAIEmbeddingProvider) GenerateEmbeddingCached(ctx context.Context, text string, ttl time.Duration) ([]float32, error) {
	// TODO: Implement caching layer
	// For now, just call the regular method
	return oep.GenerateEmbedding(ctx, text)
}

// GenerateBatchEmbeddingsCached generates batch embeddings with caching
func (oep *OpenAIEmbeddingProvider) GenerateBatchEmbeddingsCached(ctx context.Context, texts []string, ttl time.Duration) ([][]float32, error) {
	// TODO: Implement caching layer
	// For now, just call the regular method
	return oep.GenerateBatchEmbeddings(ctx, texts)
}

// DeduplicateAndEmbed removes duplicate texts and generates embeddings efficiently
func (oep *OpenAIEmbeddingProvider) DeduplicateAndEmbed(ctx context.Context, texts []string) (map[string][]float32, error) {
	// Remove duplicates
	uniqueTexts := make([]string, 0)
	seen := make(map[string]bool)
	textToIndex := make(map[string]int)

	for _, text := range texts {
		if !seen[text] {
			textToIndex[text] = len(uniqueTexts)
			uniqueTexts = append(uniqueTexts, text)
			seen[text] = true
		}
	}

	// Generate embeddings for unique texts
	embeddings, err := oep.GenerateBatchEmbeddings(ctx, uniqueTexts)
	if err != nil {
		return nil, err
	}

	// Create result map
	result := make(map[string][]float32)
	for text, index := range textToIndex {
		result[text] = embeddings[index]
	}

	return result, nil
}

// GetTokenCount estimates token count for text
func (oep *OpenAIEmbeddingProvider) GetTokenCount(text string) (int, error) {
	// Simple estimation: ~4 characters per token
	// For production, use tiktoken library
	return len(text) / 4, nil
}

// EstimateCost estimates the cost for embedding generation
func (oep *OpenAIEmbeddingProvider) EstimateCost(tokenCount int) (float64, error) {
	// OpenAI pricing (as of 2024)
	var costPerToken float64
	switch oep.model {
	case string(openai.SmallEmbedding3):
		costPerToken = 0.00002 / 1000 // $0.00002 per 1K tokens
	case string(openai.LargeEmbedding3):
		costPerToken = 0.00013 / 1000 // $0.00013 per 1K tokens
	case string(openai.AdaEmbeddingV2):
		costPerToken = 0.0001 / 1000 // $0.0001 per 1K tokens
	default:
		costPerToken = 0.0001 / 1000
	}

	return float64(tokenCount) * costPerToken, nil
}

// Health checks if OpenAI embedding API is available
func (oep *OpenAIEmbeddingProvider) Health(ctx context.Context) error {
	// Simple health check: generate a test embedding
	_, err := oep.GenerateEmbedding(ctx, "health check")
	if err != nil {
		return fmt.Errorf("OpenAI embedding health check failed: %w", err)
	}
	return nil
}

// GetUsage returns usage statistics
func (oep *OpenAIEmbeddingProvider) GetUsage(ctx context.Context) (*extractor.EmbeddingUsageStats, error) {
	oep.mu.RLock()
	defer oep.mu.RUnlock()

	totalRequests := oep.metrics.TotalRequests
	successRate := float64(0)
	if totalRequests > 0 {
		successRate = float64(oep.metrics.SuccessfulRequests) / float64(totalRequests)
	}

	batchEfficiency := float64(0)
	if oep.metrics.TotalRequests > 0 {
		batchEfficiency = float64(oep.metrics.TotalBatchRequests) / float64(oep.metrics.TotalRequests)
	}

	return &extractor.EmbeddingUsageStats{
		TotalRequests:       oep.metrics.TotalRequests,
		SuccessfulRequests:  oep.metrics.SuccessfulRequests,
		FailedRequests:      oep.metrics.FailedRequests,
		TotalTextsProcessed: oep.metrics.TotalTextsProcessed,
		TotalTokensUsed:     oep.metrics.TotalTokensUsed,
		TotalEmbeddings:     oep.metrics.TotalTextsProcessed,
		AverageLatency:      oep.metrics.AverageLatency,
		TotalBatchRequests:  oep.metrics.TotalBatchRequests,
		BatchEfficiency:     batchEfficiency,
		CacheHitRate:        0, // TODO: Implement caching
		CacheMissRate:       1 - successRate,
		PeriodStart:         oep.metrics.FirstRequest,
		PeriodEnd:           time.Now(),
	}, nil
}

// GetRateLimit returns current rate limit status
func (oep *OpenAIEmbeddingProvider) GetRateLimit(ctx context.Context) (*extractor.EmbeddingRateLimitStatus, error) {
	// OpenAI doesn't provide real-time rate limit info via API
	// Return estimated values based on tier
	return &extractor.EmbeddingRateLimitStatus{
		RequestsPerMinute: 3000,
		TokensPerMinute:   1000000,
		RequestsRemaining: 2900,
		TokensRemaining:   990000,
		ResetTime:         time.Now().Add(1 * time.Minute),
	}, nil
}

// Configure updates provider configuration
func (oep *OpenAIEmbeddingProvider) Configure(config *extractor.EmbeddingProviderConfig) error {
	if err := extractor.ValidateEmbeddingProviderConfig(config); err != nil {
		return err
	}

	oep.mu.Lock()
	defer oep.mu.Unlock()

	// Update client if API key changed
	if config.APIKey != "" && config.APIKey != oep.config.APIKey {
		oep.client = openai.NewClient(config.APIKey)
	}

	// Update model if changed
	if config.Model != "" {
		oep.model = config.Model
	}

	// Update dimensions if changed
	if config.Dimensions > 0 {
		oep.dimensions = config.Dimensions
	}

	oep.config = config
	return nil
}

// GetConfiguration returns current provider configuration
func (oep *OpenAIEmbeddingProvider) GetConfiguration() *extractor.EmbeddingProviderConfig {
	oep.mu.RLock()
	defer oep.mu.RUnlock()

	// Return a copy to prevent modification
	configCopy := *oep.config
	return &configCopy
}

// ValidateConfiguration validates the provider configuration
func (oep *OpenAIEmbeddingProvider) ValidateConfiguration(config *extractor.EmbeddingProviderConfig) error {
	return extractor.ValidateEmbeddingProviderConfig(config)
}

// Close closes the provider and cleans up resources
func (oep *OpenAIEmbeddingProvider) Close() error {
	// OpenAI client doesn't require explicit cleanup
	return nil
}

// SupportsStreaming checks if provider supports streaming embeddings
func (oep *OpenAIEmbeddingProvider) SupportsStreaming() bool {
	return false
}

// GenerateStreamingEmbedding generates embedding with streaming callback
func (oep *OpenAIEmbeddingProvider) GenerateStreamingEmbedding(ctx context.Context, text string, callback extractor.EmbeddingStreamCallback) error {
	return extractor.NewExtractorError("unsupported", "OpenAI embedding provider does not support streaming", 501)
}

// SupportsCustomDimensions checks if provider supports custom dimensions
func (oep *OpenAIEmbeddingProvider) SupportsCustomDimensions() bool {
	// text-embedding-3-small and text-embedding-3-large support custom dimensions
	return oep.model == string(openai.SmallEmbedding3) || oep.model == string(openai.LargeEmbedding3)
}

// SetCustomDimensions sets custom embedding dimensions
func (oep *OpenAIEmbeddingProvider) SetCustomDimensions(dimensions int) error {
	if !oep.SupportsCustomDimensions() {
		return extractor.NewExtractorError("unsupported", fmt.Sprintf("model %s does not support custom dimensions", oep.model), 400)
	}

	// Validate dimensions range
	maxDims := 3072
	if oep.model == string(openai.SmallEmbedding3) {
		maxDims = 1536
	}

	if dimensions < 1 || dimensions > maxDims {
		return extractor.NewExtractorError("validation", fmt.Sprintf("dimensions must be between 1 and %d", maxDims), 400)
	}

	oep.mu.Lock()
	defer oep.mu.Unlock()

	oep.dimensions = dimensions
	oep.config.Dimensions = dimensions
	return nil
}

// GetCapabilities returns the capabilities supported by this provider
func (oep *OpenAIEmbeddingProvider) GetCapabilities() *extractor.EmbeddingProviderCapabilities {
	return &extractor.EmbeddingProviderCapabilities{
		SupportsBatching:      true,
		SupportsStreaming:     false,
		SupportsCustomDims:    oep.SupportsCustomDimensions(),
		SupportsNormalization: true,
		MaxTokensPerText:      8192,
		MaxBatchSize:          2048,
		SupportedModels: []string{
			string(openai.SmallEmbedding3),
			string(openai.LargeEmbedding3),
			string(openai.AdaEmbeddingV2),
		},
		DefaultModel:          string(openai.SmallEmbedding3),
		SupportedDimensions:   []int{512, 1024, 1536, 3072},
		DefaultDimension:      1536,
		MinDimension:          1,
		MaxDimension:          3072,
		SupportsRateLimiting:  true,
		SupportsUsageTracking: true,
		SupportsCaching:       true,
		SupportsDeduplication: true,
		SupportsMultiModal:    false,
		SupportedInputTypes:   []string{"text"},
		SupportsFineTuning:    false,
		SupportsCustomModels:  false,
		CostPerToken:          0.00002 / 1000,
		RateLimitRPM:          3000,
		RateLimitTPM:          1000000,
	}
}
