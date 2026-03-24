// Package vector - Ollama native embedding provider
// Calls the local Ollama /api/embeddings endpoint (no OpenAI compatibility required).
package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultOllamaEndpoint = "http://localhost:11434/api/embeddings"

// OllamaEmbeddingProvider implements EmbeddingProvider using the local Ollama server.
// Recommended models (GGUF, run via `ollama pull <model>`):
//   - nomic-embed-text      (768-dim, fast, multilingual-capable)
//   - mxbai-embed-large     (1024-dim, high quality)
//   - all-minilm            (384-dim, very fast, small footprint)
//   - bge-large-zh          (1024-dim, best for Chinese/Vietnamese)
type OllamaEmbeddingProvider struct {
	endpoint  string
	model     string
	dimension int
	client    *http.Client
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float64 `json:"embedding"`
}

// NewOllamaEmbeddingProvider creates a new Ollama embedding provider.
// endpoint defaults to http://localhost:11434/api/embeddings.
// dimension is the expected output size (0 = use model default, 768 for nomic-embed-text).
func NewOllamaEmbeddingProvider(endpoint, model string, dimension int) *OllamaEmbeddingProvider {
	if endpoint == "" {
		endpoint = defaultOllamaEndpoint
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	if dimension == 0 {
		dimension = ollamaDimension(model)
	}
	return &OllamaEmbeddingProvider{
		endpoint:  endpoint,
		model:     model,
		dimension: dimension,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// GenerateEmbedding implements EmbeddingProvider.
func (p *OllamaEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	payload, _ := json.Marshal(ollamaEmbedRequest{Model: p.model, Prompt: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, body)
	}

	var result ollamaEmbedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ollama parse: %w", err)
	}

	f32 := make([]float32, len(result.Embedding))
	for i, v := range result.Embedding {
		f32[i] = float32(v)
	}
	return f32, nil
}

// GenerateBatchEmbeddings calls GenerateEmbedding sequentially (Ollama has no native batch API).
func (p *OllamaEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		emb, err := p.GenerateEmbedding(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("ollama batch[%d]: %w", i, err)
		}
		out[i] = emb
	}
	return out, nil
}

// GetDimensions implements EmbeddingProvider.
func (p *OllamaEmbeddingProvider) GetDimensions() int { return p.dimension }

// GetModel implements EmbeddingProvider.
func (p *OllamaEmbeddingProvider) GetModel() string { return p.model }

// Health implements EmbeddingProvider.
func (p *OllamaEmbeddingProvider) Health(ctx context.Context) error {
	_, err := p.GenerateEmbedding(ctx, "health")
	return err
}

func ollamaDimension(model string) int {
	switch model {
	case "mxbai-embed-large", "bge-large-zh", "bge-large-en":
		return 1024
	case "all-minilm", "all-minilm-l6-v2":
		return 384
	default: // nomic-embed-text and most others
		return 768
	}
}
