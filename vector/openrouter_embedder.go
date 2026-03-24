// Package vector - OpenRouter embedding provider
// OpenRouter supports the OpenAI-compatible embeddings API so we reuse the
// OpenAI wire format and just swap the base URL + auth header.
package vector

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	openRouterEmbeddingURL = "https://openrouter.ai/api/v1/embeddings"
	// Default model: openai/text-embedding-3-small (available via OpenRouter)
	defaultOpenRouterModel = "openai/text-embedding-3-small"
)

// OpenRouterEmbeddingProvider implements EmbeddingProvider via OpenRouter.
// OpenRouter uses the same request/response format as OpenAI, so we embed
// OpenAIEmbeddingProvider and override the endpoint + headers.
type OpenRouterEmbeddingProvider struct {
	inner   *OpenAIEmbeddingProvider
	siteURL string // Optional for app-level X-Origin-Site header
	appName string // Optional for X-Title header
}

// OpenRouterConfig holds configuration for the OpenRouter provider.
type OpenRouterConfig struct {
	// APIKey is your OpenRouter API key (sk-or-...).
	APIKey string
	// Model is the model slug. Use OpenAI-style IDs prefixed with the provider,
	// e.g. "openai/text-embedding-3-small", "cohere/embed-multilingual-v3.0".
	Model string
	// Dimension is the expected embedding dimension (0 = auto-detect from model).
	Dimension int
	// SiteURL is sent in X-Origin-Site (optional but recommended).
	SiteURL string
	// AppName is sent in X-Title (optional but recommended).
	AppName string
}

// NewOpenRouterEmbeddingProvider creates an OpenRouter embedding provider.
//
// Supported embedding models (as of 2025):
//   - openai/text-embedding-3-small (1536-dim) ← default
//   - openai/text-embedding-3-large (3072-dim)
//   - openai/text-embedding-ada-002 (1536-dim)
//   - cohere/embed-multilingual-v3.0 (1024-dim)
func NewOpenRouterEmbeddingProvider(cfg OpenRouterConfig) *OpenRouterEmbeddingProvider {
	if cfg.Model == "" {
		cfg.Model = defaultOpenRouterModel
	}
	if cfg.Dimension == 0 {
		cfg.Dimension = inferDimension(cfg.Model)
	}

	inner := &OpenAIEmbeddingProvider{
		apiKey:    cfg.APIKey,
		endpoint:  openRouterEmbeddingURL,
		model:     cfg.Model,
		dimension: cfg.Dimension,
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &openRouterTransport{
				base:    http.DefaultTransport,
				apiKey:  cfg.APIKey,
				siteURL: cfg.SiteURL,
				appName: cfg.AppName,
			},
		},
	}

	return &OpenRouterEmbeddingProvider{
		inner:   inner,
		siteURL: cfg.SiteURL,
		appName: cfg.AppName,
	}
}

// GenerateEmbedding implements EmbeddingProvider.
func (p *OpenRouterEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	emb, err := p.inner.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("openrouter embed: %w", err)
	}
	return emb, nil
}

// GenerateBatchEmbeddings implements EmbeddingProvider.
func (p *OpenRouterEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	embs, err := p.inner.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("openrouter batch embed: %w", err)
	}
	return embs, nil
}

// GetDimensions implements EmbeddingProvider.
func (p *OpenRouterEmbeddingProvider) GetDimensions() int { return p.inner.GetDimensions() }

// GetModel implements EmbeddingProvider.
func (p *OpenRouterEmbeddingProvider) GetModel() string { return p.inner.GetModel() }

// Health implements EmbeddingProvider.
func (p *OpenRouterEmbeddingProvider) Health(ctx context.Context) error {
	_, err := p.GenerateEmbedding(ctx, "health check")
	return err
}

// ─── OpenRouter HTTP transport ────────────────────────────────────────────────

type openRouterTransport struct {
	base    http.RoundTripper
	apiKey  string
	siteURL string
	appName string
}

func (t *openRouterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid mutating the original
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+t.apiKey)
	r.Header.Set("Content-Type", "application/json")
	if t.siteURL != "" {
		r.Header.Set("X-Origin-Site", t.siteURL)
	}
	if t.appName != "" {
		r.Header.Set("X-Title", t.appName)
	}
	return t.base.RoundTrip(r)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func inferDimension(model string) int {
	switch model {
	case "openai/text-embedding-3-large":
		return 3072
	case "cohere/embed-multilingual-v3.0", "cohere/embed-english-v3.0":
		return 1024
	default:
		return 1536 // openai/text-embedding-3-small, ada-002
	}
}
