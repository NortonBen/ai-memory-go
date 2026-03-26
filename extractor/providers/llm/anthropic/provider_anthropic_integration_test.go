// Package anthropic - Anthropic provider integration tests
package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)



// TestAnthropicProviderEntityExtraction tests entity extraction with Anthropic
func TestAnthropicProviderEntityExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := AnthropicResponse{
			ID:   "msg_entity",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{
					Type: "text",
					Text: `[
						{
							"id": "entity-1",
							"type": "City",
							"properties": {
								"name": "Paris",
								"confidence": 0.95
							}
						},
						{
							"id": "entity-2",
							"type": "Country",
							"properties": {
								"name": "France",
								"confidence": 0.98
							}
						}
					]`,
				},
			},
			Model:      Claude3Haiku,
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  50,
				OutputTokens: 30,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	ctx := context.Background()
	entities, err := provider.ExtractEntities(ctx, "Paris is the capital of France")

	require.NoError(t, err)
	require.Len(t, entities, 2)

	assert.Equal(t, "entity-1", entities[0].ID)
	assert.Equal(t, schema.NodeType("City"), entities[0].Type)
	assert.Equal(t, "Paris", entities[0].Properties["name"])

	assert.Equal(t, "entity-2", entities[1].ID)
	assert.Equal(t, schema.NodeType("Country"), entities[1].Type)
	assert.Equal(t, "France", entities[1].Properties["name"])
}

// TestAnthropicProviderRelationshipExtraction tests relationship extraction
func TestAnthropicProviderRelationshipExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := AnthropicResponse{
			ID:   "msg_rel",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{
					Type: "text",
					Text: `[
						{
							"id": "rel-1",
							"from": "entity-1",
							"to": "entity-2",
							"type": "CAPITAL_OF",
							"properties": {
								"confidence": 0.99
							}
						}
					]`,
				},
			},
			Model:      Claude3Haiku,
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  60,
				OutputTokens: 25,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	entities := []schema.Node{
		{
			ID:   "entity-1",
			Type: "City",
			Properties: map[string]interface{}{
				"name": "Paris",
			},
		},
		{
			ID:   "entity-2",
			Type: "Country",
			Properties: map[string]interface{}{
				"name": "France",
			},
		},
	}

	ctx := context.Background()
	relationships, err := provider.ExtractRelationships(ctx, "Paris is the capital of France", entities)

	require.NoError(t, err)
	require.Len(t, relationships, 1)

	assert.Equal(t, "rel-1", relationships[0].ID)
	assert.Equal(t, "entity-1", relationships[0].From)
	assert.Equal(t, "entity-2", relationships[0].To)
	assert.Equal(t, schema.EdgeType("CAPITAL_OF"), relationships[0].Type)
}

// TestAnthropicProviderCompletionWithOptions tests completion with custom options
func TestAnthropicProviderCompletionWithOptions(t *testing.T) {
	var capturedRequest AnthropicRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)

		w.WriteHeader(http.StatusOK)
		response := AnthropicResponse{
			ID:   "msg_options",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Test response with options"},
			},
			Model:      Claude3Haiku,
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	options := &extractor.CompletionOptions{
		Temperature:  0.5,
		MaxTokens:    2000,
		TopP:         0.9,
		TopK:         40,
		SystemPrompt: "You are a helpful assistant",
		Stop:         []string{"END"},
	}

	ctx := context.Background()
	result, err := provider.GenerateCompletionWithOptions(ctx, "Test prompt", options)

	require.NoError(t, err)
	assert.Equal(t, "Test response with options", result)

	// Verify options were applied
	assert.Equal(t, 0.5, capturedRequest.Temperature)
	assert.Equal(t, 2000, capturedRequest.MaxTokens)
	assert.Equal(t, 0.9, capturedRequest.TopP)
	assert.Equal(t, 40, capturedRequest.TopK)
	assert.Equal(t, "You are a helpful assistant", capturedRequest.System)
	assert.Equal(t, []string{"END"}, capturedRequest.StopSequences)
}

// TestAnthropicProviderConversationContext tests multi-turn conversation
func TestAnthropicProviderConversationContext(t *testing.T) {
	var capturedRequest AnthropicRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)

		w.WriteHeader(http.StatusOK)
		response := AnthropicResponse{
			ID:   "msg_conv",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "I understand the context"},
			},
			Model:      Claude3Haiku,
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  30,
				OutputTokens: 5,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	messages := []extractor.Message{
		{Role: extractor.RoleSystem, Content: "You are a helpful assistant"},
		{Role: extractor.RoleUser, Content: "Hello"},
		{Role: extractor.RoleAssistant, Content: "Hi there!"},
		{Role: extractor.RoleUser, Content: "Can you help me?"},
	}

	ctx := context.Background()
	result, err := provider.GenerateWithContext(ctx, messages, nil)

	require.NoError(t, err)
	assert.Equal(t, "I understand the context", result)

	// Verify system message was extracted
	assert.Equal(t, "You are a helpful assistant", capturedRequest.System)

	// Verify conversation messages (excluding system)
	require.Len(t, capturedRequest.Messages, 3)
	assert.Equal(t, "user", capturedRequest.Messages[0].Role)
	assert.Equal(t, "Hello", capturedRequest.Messages[0].Content)
	assert.Equal(t, "assistant", capturedRequest.Messages[1].Role)
	assert.Equal(t, "Hi there!", capturedRequest.Messages[1].Content)
	assert.Equal(t, "user", capturedRequest.Messages[2].Role)
	assert.Equal(t, "Can you help me?", capturedRequest.Messages[2].Content)
}

// TestAnthropicProviderCustomSchema tests extraction with custom JSON schema
func TestAnthropicProviderCustomSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := AnthropicResponse{
			ID:   "msg_custom",
			Type: "message",
			Role: "assistant",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{
					Type: "text",
					Text: `{
						"person": {
							"name": "John Doe",
							"age": 30,
							"city": "New York"
						}
					}`,
				},
			},
			Model:      Claude3Haiku,
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  40,
				OutputTokens: 20,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)
	provider.SetEndpoint(server.URL)

	customSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"person": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "integer"},
					"city": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := provider.ExtractWithCustomSchema(ctx, "John Doe is 30 years old and lives in New York", customSchema)

	require.NoError(t, err)
	require.NotNil(t, result)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)

	person, ok := resultMap["person"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "John Doe", person["name"])
	assert.Equal(t, float64(30), person["age"])
	assert.Equal(t, "New York", person["city"])
}

// TestAnthropicProviderAllModels tests all supported Claude models
func TestAnthropicProviderAllModels(t *testing.T) {
	models := []string{
		Claude3Opus,
		Claude3Sonnet,
		Claude3Haiku,
		Claude35Sonnet,
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req AnthropicRequest
				json.NewDecoder(r.Body).Decode(&req)

				// Verify correct model is being used
				assert.Equal(t, model, req.Model)

				w.WriteHeader(http.StatusOK)
				response := AnthropicResponse{
					ID:   "msg_model_test",
					Type: "message",
					Role: "assistant",
					Content: []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					}{
						{Type: "text", Text: "Response from " + model},
					},
					Model:      model,
					StopReason: "end_turn",
					Usage: struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					}{
						InputTokens:  5,
						OutputTokens: 5,
					},
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			provider := NewAnthropicProvider("test-api-key", model)
			provider.SetEndpoint(server.URL)

			ctx := context.Background()
			result, err := provider.GenerateCompletion(ctx, "Test")

			require.NoError(t, err)
			assert.Contains(t, result, model)
		})
	}
}

// TestAnthropicProviderConfigure tests provider configuration updates
func TestAnthropicProviderConfigure(t *testing.T) {
	provider := NewAnthropicProvider("initial-key", Claude3Haiku)

	newConfig := &extractor.ProviderConfig{
		Type:     extractor.ProviderAnthropic,
		Model:    Claude3Opus,
		APIKey:   "new-api-key",
		Endpoint: "https://new-endpoint.com",
		Timeout:  45 * time.Second,
	}

	err := provider.Configure(newConfig)
	require.NoError(t, err)

	// Verify configuration was updated
	assert.Equal(t, Claude3Opus, provider.GetModel())
	assert.Equal(t, "new-api-key", provider.apiKey)
	assert.Equal(t, "https://new-endpoint.com", provider.endpoint)
	assert.Equal(t, 45*time.Second, provider.timeout)

	// Verify GetConfiguration returns updated config
	config := provider.GetConfiguration()
	assert.Equal(t, extractor.ProviderAnthropic, config.Type)
	assert.Equal(t, Claude3Opus, config.Model)
	assert.Equal(t, "new-api-key", config.APIKey)
	assert.Equal(t, "https://new-endpoint.com", config.Endpoint)
	assert.Equal(t, 45*time.Second, config.Timeout)
}

// TestAnthropicProviderClose tests resource cleanup
func TestAnthropicProviderClose(t *testing.T) {
	provider := NewAnthropicProvider("test-api-key", Claude3Haiku)

	err := provider.Close()
	assert.NoError(t, err)

	// Verify HTTP client connections are closed
	// (This is implicit in the Close() implementation)
}
