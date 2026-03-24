// Package extractor - Anthropic provider tests
package extractor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnthropicProvider(t *testing.T) {
	tests := []struct {
		name          string
		apiKey        string
		model         string
		expectedModel string
	}{
		{
			name:          "with_custom_model",
			apiKey:        "test-api-key",
			model:         Claude3Opus,
			expectedModel: Claude3Opus,
		},
		{
			name:          "with_default_model",
			apiKey:        "test-api-key",
			model:         "",
			expectedModel: Claude3Haiku,
		},
		{
			name:          "with_sonnet_model",
			apiKey:        "test-api-key",
			model:         Claude3Sonnet,
			expectedModel: Claude3Sonnet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewAnthropicProvider(tt.apiKey, tt.model)
			assert.NotNil(t, provider)
			assert.Equal(t, tt.expectedModel, provider.GetModel())
			assert.Equal(t, tt.apiKey, provider.apiKey)
			assert.Equal(t, "https://api.anthropic.com/v1/messages", provider.endpoint)
		})
	}
}

func TestAnthropicProvider_GenerateCompletion(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		mockResponse   AnthropicResponse
		mockStatusCode int
		expectError    bool
	}{
		{
			name:   "successful_completion",
			prompt: "What is the capital of France?",
			mockResponse: AnthropicResponse{
				ID:   "msg_123",
				Type: "message",
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{Type: "text", Text: "The capital of France is Paris."},
				},
				Model:      Claude3Haiku,
				StopReason: "end_turn",
				Usage: struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				}{
					InputTokens:  10,
					OutputTokens: 8,
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:   "empty_content",
			prompt: "Test prompt",
			mockResponse: AnthropicResponse{
				ID:   "msg_124",
				Type: "message",
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{},
				Model: Claude3Haiku,
			},
			mockStatusCode: http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.NotEmpty(t, r.Header.Get("x-api-key"))
				assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

				w.WriteHeader(tt.mockStatusCode)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
			provider.SetEndpoint(server.URL)

			ctx := context.Background()
			result, err := provider.GenerateCompletion(ctx, tt.prompt)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResponse.Content[0].Text, result)
			}
		})
	}
}

func TestAnthropicProvider_GenerateStructuredOutput(t *testing.T) {
	type TestEntity struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	tests := []struct {
		name           string
		prompt         string
		schema         interface{}
		mockResponse   string
		mockStatusCode int
		expectError    bool
	}{
		{
			name:   "successful_structured_output",
			prompt: "Extract entity from: Paris is the capital of France",
			schema: &TestEntity{},
			mockResponse: `{
				"id": "msg_123",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "{\"name\": \"Paris\", \"type\": \"City\"}"}],
				"model": "claude-3-haiku-20240307",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 20, "output_tokens": 10}
			}`,
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:   "invalid_json_response",
			prompt: "Extract entity",
			schema: &TestEntity{},
			mockResponse: `{
				"id": "msg_124",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "This is not valid JSON"}],
				"model": "claude-3-haiku-20240307",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 10, "output_tokens": 5}
			}`,
			mockStatusCode: http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
			provider.SetEndpoint(server.URL)

			ctx := context.Background()
			result, err := provider.GenerateStructuredOutput(ctx, tt.prompt, tt.schema)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				entity, ok := result.(*TestEntity)
				assert.True(t, ok)
				assert.Equal(t, "Paris", entity.Name)
				assert.Equal(t, "City", entity.Type)
			}
		})
	}
}

func TestAnthropicProvider_Health(t *testing.T) {
	tests := []struct {
		name           string
		mockStatusCode int
		mockResponse   string
		expectError    bool
	}{
		{
			name:           "healthy",
			mockStatusCode: http.StatusOK,
			mockResponse: `{
				"id": "msg_health",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "Hello!"}],
				"model": "claude-3-haiku-20240307",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 5, "output_tokens": 2}
			}`,
			expectError: false,
		},
		{
			name:           "unhealthy",
			mockStatusCode: http.StatusServiceUnavailable,
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "overloaded_error",
					"message": "Service temporarily unavailable"
				}
			}`,
			expectError: true,
		},
		{
			name:           "authentication_error",
			mockStatusCode: http.StatusUnauthorized,
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "authentication_error",
					"message": "Invalid API key"
				}
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
			provider.SetEndpoint(server.URL)

			ctx := context.Background()
			err := provider.Health(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAnthropicProvider_RetryLogic(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			// Fail first 2 attempts with retryable error
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{
				"type": "error",
				"error": {
					"type": "rate_limit_error",
					"message": "Rate limit exceeded"
				}
			}`))
			return
		}
		// Succeed on 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "msg_retry",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Success after retries"}],
			"model": "claude-3-haiku-20240307",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 5, "output_tokens": 3}
		}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	ctx := context.Background()
	result, err := provider.GenerateCompletion(ctx, "Test prompt")

	assert.NoError(t, err)
	assert.Equal(t, "Success after retries", result)
	assert.Equal(t, 3, attemptCount, "Should have retried 3 times")
}

func TestAnthropicProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "msg_slow",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Slow response"}],
			"model": "claude-3-haiku-20240307",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 5, "output_tokens": 2}
		}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := provider.GenerateCompletion(ctx, "Test prompt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestAnthropicProvider_SettersAndGetters(t *testing.T) {
	provider := NewAnthropicProvider("initial-key", Claude3Haiku)

	// Test SetModel
	provider.SetModel(Claude3Opus)
	assert.Equal(t, Claude3Opus, provider.GetModel())

	// Test SetAPIKey
	provider.SetAPIKey("new-api-key")
	assert.Equal(t, "new-api-key", provider.apiKey)

	// Test SetEndpoint
	provider.SetEndpoint("https://custom-endpoint.com")
	assert.Equal(t, "https://custom-endpoint.com", provider.endpoint)

	// Test SetTimeout
	provider.SetTimeout(60 * time.Second)
	assert.Equal(t, 60*time.Second, provider.timeout)
	assert.Equal(t, 60*time.Second, provider.client.Timeout)
}

func TestAnthropicProvider_GetSupportedModels(t *testing.T) {
	provider := NewAnthropicProvider("test-key", Claude3Haiku)
	models := provider.GetSupportedModels()

	assert.Len(t, models, 4)
	assert.Contains(t, models, Claude3Opus)
	assert.Contains(t, models, Claude3Sonnet)
	assert.Contains(t, models, Claude3Haiku)
	assert.Contains(t, models, Claude35Sonnet)
}

func TestAnthropicProvider_GetMaxTokensForModel(t *testing.T) {
	provider := NewAnthropicProvider("test-key", Claude3Haiku)

	tests := []struct {
		model          string
		expectedTokens int
	}{
		{Claude3Opus, 4096},
		{Claude3Sonnet, 4096},
		{Claude3Haiku, 4096},
		{Claude35Sonnet, 4096},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			maxTokens := provider.GetMaxTokensForModel(tt.model)
			assert.Equal(t, tt.expectedTokens, maxTokens)
		})
	}
}

func TestAnthropicProvider_GetContextWindowForModel(t *testing.T) {
	provider := NewAnthropicProvider("test-key", Claude3Haiku)

	tests := []struct {
		model          string
		expectedWindow int
	}{
		{Claude3Opus, 200000},
		{Claude3Sonnet, 200000},
		{Claude3Haiku, 200000},
		{Claude35Sonnet, 200000},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			contextWindow := provider.GetContextWindowForModel(tt.model)
			assert.Equal(t, tt.expectedWindow, contextWindow)
		})
	}
}

func TestAnthropicProvider_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		mockStatusCode int
		mockResponse   string
		expectedError  string
	}{
		{
			name:           "rate_limit_error",
			mockStatusCode: http.StatusTooManyRequests,
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "rate_limit_error",
					"message": "Rate limit exceeded"
				}
			}`,
			expectedError: "rate_limit_error",
		},
		{
			name:           "invalid_request_error",
			mockStatusCode: http.StatusBadRequest,
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "invalid_request_error",
					"message": "Invalid request parameters"
				}
			}`,
			expectedError: "invalid_request_error",
		},
		{
			name:           "authentication_error",
			mockStatusCode: http.StatusUnauthorized,
			mockResponse: `{
				"type": "error",
				"error": {
					"type": "authentication_error",
					"message": "Invalid API key"
				}
			}`,
			expectedError: "authentication_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
			provider.SetEndpoint(server.URL)

			ctx := context.Background()
			_, err := provider.GenerateCompletion(ctx, "Test prompt")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestAnthropicProvider_RequestFormat(t *testing.T) {
	var capturedRequest AnthropicRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request
		json.NewDecoder(r.Body).Decode(&capturedRequest)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Test response"}],
			"model": "claude-3-haiku-20240307",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 5, "output_tokens": 2}
		}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	ctx := context.Background()
	_, err := provider.GenerateCompletion(ctx, "Test prompt")
	require.NoError(t, err)

	// Verify request format
	assert.Equal(t, Claude3Haiku, capturedRequest.Model)
	assert.Len(t, capturedRequest.Messages, 1)
	assert.Equal(t, "user", capturedRequest.Messages[0].Role)
	assert.Equal(t, "Test prompt", capturedRequest.Messages[0].Content)
	assert.Equal(t, 4096, capturedRequest.MaxTokens)
	assert.Equal(t, 0.7, capturedRequest.Temperature)
	assert.False(t, capturedRequest.Stream)
}

func TestAnthropicProvider_StructuredOutputPromptEnhancement(t *testing.T) {
	type TestSchema struct {
		Field1 string `json:"field1"`
		Field2 int    `json:"field2"`
	}

	var capturedRequest AnthropicRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "msg_test",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "{\"field1\": \"value1\", \"field2\": 42}"}],
			"model": "claude-3-haiku-20240307",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 20, "output_tokens": 10}
		}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	ctx := context.Background()
	schema := &TestSchema{}
	_, err := provider.GenerateStructuredOutput(ctx, "Extract data", schema)
	require.NoError(t, err)

	// Verify prompt enhancement
	assert.Contains(t, capturedRequest.Messages[0].Content, "Extract data")
	assert.Contains(t, capturedRequest.Messages[0].Content, "JSON")
	assert.Contains(t, capturedRequest.Messages[0].Content, "schema")
	assert.Equal(t, "You are a helpful assistant that extracts structured data. Always respond with valid JSON matching the provided schema.", capturedRequest.System)
	assert.Equal(t, 0.3, capturedRequest.Temperature)
}
