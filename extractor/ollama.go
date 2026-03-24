// Package extractor - Ollama provider implementation
package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaProvider implements LLMProvider for Ollama local models
type OllamaProvider struct {
	endpoint string
	model    string
	client   *http.Client
	timeout  time.Duration
}

// OllamaRequest represents a request to Ollama API
type OllamaRequest struct {
	Model  string                 `json:"model"`
	Prompt string                 `json:"prompt"`
	Stream bool                   `json:"stream"`
	Format string                 `json:"format,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaResponse represents a response from Ollama API
type OllamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(endpoint, model string) *OllamaProvider {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "llama2"
	}

	return &OllamaProvider{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		timeout: 120 * time.Second,
	}
}

// GenerateCompletion generates a text completion using Ollama
func (op *OllamaProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	request := OllamaRequest{
		Model:  op.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", op.endpoint+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := op.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama API error: %s - %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return ollamaResp.Response, nil
}

// GenerateStructuredOutput generates structured output using JSON format
func (op *OllamaProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schemaStruct interface{}) (interface{}, error) {
	// Add JSON format instruction to prompt
	enhancedPrompt := prompt + "\n\nRespond with valid JSON only. Do not include any explanatory text."

	request := OllamaRequest{
		Model:  op.model,
		Prompt: enhancedPrompt,
		Stream: false,
		Format: "json",
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", op.endpoint+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := op.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s - %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Parse the JSON response into the schema struct
	if err := json.Unmarshal([]byte(ollamaResp.Response), schemaStruct); err != nil {
		return nil, fmt.Errorf("failed to parse structured output: %w", err)
	}

	return schemaStruct, nil
}

// GetModel returns the model name
func (op *OllamaProvider) GetModel() string {
	return op.model
}

// Health checks if Ollama is available
func (op *OllamaProvider) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", op.endpoint+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := op.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama is not available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check failed: %s", resp.Status)
	}

	return nil
}

// SetModel sets the model to use
func (op *OllamaProvider) SetModel(model string) error {
	op.model = model
	return nil
}

// SetEndpoint sets the Ollama endpoint
func (op *OllamaProvider) SetEndpoint(endpoint string) {
	op.endpoint = endpoint
}

// SetTimeout sets the request timeout
func (op *OllamaProvider) SetTimeout(timeout time.Duration) {
	op.timeout = timeout
	op.client.Timeout = timeout
}

// ListModels lists available models from Ollama
func (op *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", op.endpoint+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := op.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list models: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	models := make([]string, len(result.Models))
	for i, model := range result.Models {
		models[i] = model.Name
	}

	return models, nil
}
