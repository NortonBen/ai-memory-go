package lmstudio

import (
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/NortonBen/ai-memory-go/vector/embedders/openai"
)

func init() {
	vector.RegisterEmbeddingProvider(vector.EmbeddingProviderLMStudio, func(config map[string]interface{}) (vector.EmbeddingProvider, error) {
		endpoint, _ := config["endpoint"].(string)
		model, _ := config["model"].(string)
		return NewLMStudioEmbeddingProvider(endpoint, model), nil
	})
}

// NewLMStudioEmbeddingProvider creates an embedding provider pointing to a local LM Studio instance.
// LM Studio provides an OpenAI-compatible API, so we wrap the OpenAI provider and override the endpoint.
// Default endpoint is usually http://localhost:1234/v1
func NewLMStudioEmbeddingProvider(endpoint, model string) *openai.OpenAIEmbeddingProvider {
	if endpoint == "" {
		endpoint = "http://localhost:1234/v1"
	}
	if model == "" {
		model = "nomic-embed-text-v1.5"
	}

	// Create a standard OpenAI provider with a dummy API key
	provider := openai.NewOpenAIEmbeddingProvider("lm-studio", model)
	
	// Override the endpoint to point to LM Studio's local server
	provider.Endpoint = endpoint + "/embeddings"
	
	return provider
}
