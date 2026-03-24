package extractor

import (
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// LMStudioEmbeddingProvider wraps OpenAIEmbeddingProvider to override the provider type
type LMStudioEmbeddingProvider struct {
	*OpenAIEmbeddingProvider
}

// GetProviderType returns the LM Studio provider type
func (p *LMStudioEmbeddingProvider) GetProviderType() EmbeddingProviderType {
	return EmbeddingProviderLMStudio
}

// NewLMStudioEmbeddingProvider creates a new embedding provider that uses LM Studio's local OpenAI-compatible API
func NewLMStudioEmbeddingProvider(config *EmbeddingProviderConfig) (*LMStudioEmbeddingProvider, error) {
	if config.Endpoint == "" {
		config.Endpoint = "http://localhost:1234/v1"
	}
	if config.Model == "" {
		config.Model = "nomic-embed-text-v1.5"
	}

	dimensions := config.Dimensions
	if dimensions == 0 {
		dimensions = 768 // Typical for many local embedding models
	}

	// Configure the official OpenAI SDK to point to the local LM Studio instance
	sdkConfig := openai.DefaultConfig("lm-studio")
	sdkConfig.BaseURL = config.Endpoint

	client := openai.NewClientWithConfig(sdkConfig)

	baseProvider := &OpenAIEmbeddingProvider{
		client:     client,
		model:      config.Model,
		dimensions: dimensions,
		config:     config,
		metrics: &EmbeddingProviderMetrics{
			FirstRequest: time.Now(),
		},
	}

	return &LMStudioEmbeddingProvider{
		OpenAIEmbeddingProvider: baseProvider,
	}, nil
}
