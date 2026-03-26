// Package openai - OpenAI embedding provider implementation
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/NortonBen/ai-memory-go/vector"
)

func init() {
	vector.RegisterEmbeddingProvider(vector.EmbeddingProviderOpenAI, func(config map[string]interface{}) (vector.EmbeddingProvider, error) {
		apiKey, _ := config["api_key"].(string)
		model, _ := config["model"].(string)
		return NewOpenAIEmbeddingProvider(apiKey, model), nil
	})
}

// OpenAIEmbeddingProvider implements EmbeddingProvider for OpenAI
type OpenAIEmbeddingProvider struct {
	APIKey    string
	Endpoint  string
	Model     string
	Dimension int
	Client    *http.Client
}

// OpenAIEmbeddingRequest represents a request to OpenAI embeddings API
type OpenAIEmbeddingRequest struct {
	Input          interface{} `json:"input"` // string or []string
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     int         `json:"dimensions,omitempty"`
}

// OpenAIEmbeddingResponse represents a response from OpenAI embeddings API
type OpenAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider
func NewOpenAIEmbeddingProvider(apiKey, model string) *OpenAIEmbeddingProvider {
	if model == "" {
		model = "text-embedding-3-small"
	}

	dimension := 1536 // Default for text-embedding-3-small
	if model == "text-embedding-ada-002" {
		dimension = 1536
	} else if model == "text-embedding-3-large" {
		dimension = 3072
	}

	return &OpenAIEmbeddingProvider{
		APIKey:    apiKey,
		Endpoint:  "https://api.openai.com/v1/embeddings",
		Model:     model,
		Dimension: dimension,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GenerateEmbedding generates an embedding for a single text
func (oep *OpenAIEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	request := OpenAIEmbeddingRequest{
		Input: text,
		Model: oep.Model,
	}

	response, err := oep.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	return response.Data[0].Embedding, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (oep *OpenAIEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// OpenAI supports batch requests
	request := OpenAIEmbeddingRequest{
		Input: texts,
		Model: oep.Model,
	}

	response, err := oep.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(response.Data))
	}

	// Sort by index to ensure correct order
	embeddings := make([][]float32, len(texts))
	for _, data := range response.Data {
		if data.Index >= len(embeddings) {
			return nil, fmt.Errorf("invalid embedding index: %d", data.Index)
		}
		embeddings[data.Index] = data.Embedding
	}

	return embeddings, nil
}

// sendRequest sends a request to OpenAI API
func (oep *OpenAIEmbeddingProvider) sendRequest(ctx context.Context, request OpenAIEmbeddingRequest) (*OpenAIEmbeddingResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", oep.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+oep.APIKey)

	resp, err := oep.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error: %s - %s", resp.Status, string(body))
	}

	var openaiResp OpenAIEmbeddingResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &openaiResp, nil
}

// GetDimensions returns the embedding dimensions
func (oep *OpenAIEmbeddingProvider) GetDimensions() int {
	return oep.Dimension
}

// GetModel returns the model name
func (oep *OpenAIEmbeddingProvider) GetModel() string {
	return oep.Model
}

// Health checks if OpenAI API is available
func (oep *OpenAIEmbeddingProvider) Health(ctx context.Context) error {
	// Simple health check with a minimal request
	_, err := oep.GenerateEmbedding(ctx, "test")
	return err
}

// SetModel sets the model to use
func (oep *OpenAIEmbeddingProvider) SetModel(model string) {
	oep.Model = model
	
	// Update dimension based on model
	if model == "text-embedding-ada-002" {
		oep.Dimension = 1536
	} else if model == "text-embedding-3-small" {
		oep.Dimension = 1536
	} else if model == "text-embedding-3-large" {
		oep.Dimension = 3072
	}
}

// SetAPIKey sets the API key
func (oep *OpenAIEmbeddingProvider) SetAPIKey(apiKey string) {
	oep.APIKey = apiKey
}

// SetEndpoint sets the API endpoint (useful for proxies or custom deployments)
func (oep *OpenAIEmbeddingProvider) SetEndpoint(endpoint string) {
	oep.Endpoint = endpoint
}

// SetDimension sets custom embedding dimensions (for models that support it)
func (oep *OpenAIEmbeddingProvider) SetDimension(dimension int) {
	oep.Dimension = dimension
}
