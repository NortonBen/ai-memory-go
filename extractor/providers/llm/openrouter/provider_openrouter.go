package openrouter

import (
	"github.com/NortonBen/ai-memory-go/extractor"
	"fmt"
	"net/http"

	openai "github.com/sashabaranov/go-openai"
	llmopenai "github.com/NortonBen/ai-memory-go/extractor/providers/llm/openai"
)

// OpenRouterProvider wraps OpenAIProvider to override the provider type
// and inject customized headers for OpenRouter
type OpenRouterProvider struct {
	*llmopenai.OpenAIProvider
}

// GetProviderType returns the OpenRouter provider type
func (p *OpenRouterProvider) GetProviderType() extractor.ProviderType {
	return extractor.ProviderOpenRouter
}

// NewOpenRouterProvider creates a new provider that uses OpenRouter's OpenAI-compatible API
func NewOpenRouterProvider(apiKey, model, siteURL, appName string) (*OpenRouterProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for OpenRouter")
	}
	if model == "" {
		model = "google/gemini-2.5-flash"
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	config.HTTPClient = &http.Client{
		Transport: &openRouterTransport{
			base:    http.DefaultTransport,
			siteURL: siteURL,
			appName: appName,
		},
	}

	client := openai.NewClientWithConfig(config)

	configObj := &extractor.ProviderConfig{
		Type:     extractor.ProviderOpenRouter,
		Model:    model,
		APIKey:   apiKey,
		Endpoint: "https://openrouter.ai/api/v1",
	}

	baseProvider := llmopenai.NewOpenAIProviderWithClient(client, model, configObj)

	return &OpenRouterProvider{
		OpenAIProvider: baseProvider,
	}, nil
}

type openRouterTransport struct {
	base    http.RoundTripper
	siteURL string
	appName string
}

func (t *openRouterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	if t.siteURL != "" {
		r.Header.Set("X-Origin-Site", t.siteURL)
	}
	if t.appName != "" {
		r.Header.Set("X-Title", t.appName)
	}
	return t.base.RoundTrip(r)
}
