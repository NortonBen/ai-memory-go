package lmstudio

import (
	embopenai "github.com/NortonBen/ai-memory-go/extractor/providers/embedding/openai"
	"github.com/NortonBen/ai-memory-go/extractor"

	openai "github.com/sashabaranov/go-openai"
)

// LMStudioEmbeddingProvider wraps embopenai.OpenAIEmbeddingProvider to override the provider type
type LMStudioEmbeddingProvider struct {
	*embopenai.OpenAIEmbeddingProvider
}

// GetProviderType returns the LM Studio provider type
func (p *LMStudioEmbeddingProvider) GetProviderType() extractor.EmbeddingProviderType {
	return extractor.EmbeddingProviderLMStudio
}

// NewLMStudioEmbeddingProvider creates a new embedding provider that uses LM Studio's local OpenAI-compatible API
func NewLMStudioEmbeddingProvider(config *extractor.EmbeddingProviderConfig) (*LMStudioEmbeddingProvider, error) {
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

	baseProvider := embopenai.NewOpenAIEmbeddingProviderWithClient(client, config.Model, dimensions, config)
	
	return &LMStudioEmbeddingProvider{
		OpenAIEmbeddingProvider: baseProvider,
	}, nil
}
