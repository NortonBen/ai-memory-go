package vector

// NewLMStudioEmbeddingProvider creates an embedding provider pointing to a local LM Studio instance.
// LM Studio provides an OpenAI-compatible API, so we wrap the OpenAI provider and override the endpoint.
// Default endpoint is usually http://localhost:1234/v1
func NewLMStudioEmbeddingProvider(endpoint, model string) *OpenAIEmbeddingProvider {
	if endpoint == "" {
		endpoint = "http://localhost:1234/v1"
	}
	if model == "" {
		model = "nomic-embed-text-v1.5"
	}

	// Create a standard OpenAI provider with a dummy API key
	provider := NewOpenAIEmbeddingProvider("lm-studio", model)
	
	// Override the endpoint to point to LM Studio's local server
	// Since we are in the same package, we can access the unexported endpoint field.
	provider.endpoint = endpoint + "/embeddings"
	
	return provider
}
