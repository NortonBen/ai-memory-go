// Package deepseek - DeepSeek provider with JSON Schema Mode
package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/extractor/utils"
	"github.com/NortonBen/ai-memory-go/schema"
)

// DeepSeekProvider implements extractor.LLMProvider for DeepSeek with JSON Schema Mode
type DeepSeekProvider struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
	timeout  time.Duration
}

// DeepSeekRequest represents a request to DeepSeek API
type DeepSeekRequest struct {
	Model          string            `json:"model"`
	Messages       []DeepSeekMessage `json:"messages"`
	Temperature    float64           `json:"temperature,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	ResponseFormat *ResponseFormat   `json:"response_format,omitempty"`
	Stream         bool              `json:"stream"`
}

// DeepSeekMessage represents a message in the conversation
type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat specifies JSON schema for structured output
type ResponseFormat struct {
	Type       string                 `json:"type"`
	JSONSchema map[string]interface{} `json:"json_schema,omitempty"`
}

// DeepSeekResponse represents a response from DeepSeek API
type DeepSeekResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewDeepSeekProvider creates a new DeepSeek provider
func NewDeepSeekProvider(apiKey, model string) *DeepSeekProvider {
	if model == "" {
		model = "deepseek-chat"
	}

	return &DeepSeekProvider{
		apiKey:   apiKey,
		endpoint: "https://api.deepseek.com/v1/chat/completions",
		model:    model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		timeout: 120 * time.Second,
	}
}

// GenerateCompletion generates a text completion using DeepSeek
func (dp *DeepSeekProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	request := DeepSeekRequest{
		Model: dp.model,
		Messages: []DeepSeekMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
		MaxTokens:   2000,
		Stream:      false,
	}

	return dp.sendRequest(ctx, request)
}

// GenerateStructuredOutput generates structured output using JSON Schema Mode
func (dp *DeepSeekProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schemaStruct interface{}) (interface{}, error) {
	// Generate JSON schema from Go struct
	jsonSchema, err := schema.GenerateJSONSchema(schemaStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	// Convert schema to map
	schemaMap := make(map[string]interface{})
	schemaJSON, err := jsonSchema.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	request := DeepSeekRequest{
		Model: dp.model,
		Messages: []DeepSeekMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant that extracts structured data. Always respond with valid JSON matching the provided schema.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.3, // Lower temperature for more consistent structured output
		MaxTokens:   4000,
		ResponseFormat: &ResponseFormat{
			Type:       "json_object",
			JSONSchema: schemaMap,
		},
		Stream: false,
	}

	response, err := dp.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response into the schema struct
	if err := json.Unmarshal([]byte(response), schemaStruct); err != nil {
		return nil, extractor.NewExtractorError("parse", fmt.Sprintf("failed to parse structured output: %v", err), 500)
	}

	return schemaStruct, nil
}

// sendRequest sends a request to DeepSeek API with retry logic
func (dp *DeepSeekProvider) sendRequest(ctx context.Context, request DeepSeekRequest) (string, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		response, err := dp.doRequest(ctx, request)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if error is retryable
		if !utils.IsRetryableError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// doRequest performs the actual HTTP request
func (dp *DeepSeekProvider) doRequest(ctx context.Context, request DeepSeekRequest) (string, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dp.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+dp.apiKey)

	resp, err := dp.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepseek API error: %s - %s", resp.Status, string(body))
	}

	var deepseekResp DeepSeekResponse
	if err := json.Unmarshal(body, &deepseekResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(deepseekResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return deepseekResp.Choices[0].Message.Content, nil
}

// GetModel returns the model name
func (dp *DeepSeekProvider) GetModel() string {
	return dp.model
}

// Health checks if DeepSeek API is available
func (dp *DeepSeekProvider) Health(ctx context.Context) error {
	// Simple health check with a minimal request
	request := DeepSeekRequest{
		Model: dp.model,
		Messages: []DeepSeekMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		MaxTokens: 10,
		Stream:    false,
	}

	_, err := dp.doRequest(ctx, request)
	return err
}

// SetModel sets the model to use
func (dp *DeepSeekProvider) SetModel(model string) error {
	dp.model = model
	return nil
}

// SetAPIKey sets the API key
func (dp *DeepSeekProvider) SetAPIKey(apiKey string) {
	dp.apiKey = apiKey
}

// SetEndpoint sets the API endpoint
func (dp *DeepSeekProvider) SetEndpoint(endpoint string) {
	dp.endpoint = endpoint
}

// SetTimeout sets the request timeout
func (dp *DeepSeekProvider) SetTimeout(timeout time.Duration) {
	dp.timeout = timeout
	dp.client.Timeout = timeout
}
