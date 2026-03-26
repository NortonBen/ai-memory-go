// Package anthropic - Anthropic provider with Claude models support
package anthropic

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

// AnthropicProvider implements extractor.LLMProvider for Anthropic Claude models
type AnthropicProvider struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
	timeout  time.Duration
}

// AnthropicRequest represents a request to Anthropic API
type AnthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []AnthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   float64            `json:"temperature,omitempty"`
	TopP          float64            `json:"top_p,omitempty"`
	TopK          int                `json:"top_k,omitempty"`
	System        string             `json:"system,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream"`
}

// AnthropicMessage represents a message in the conversation
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicResponse represents a response from Anthropic API
type AnthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// AnthropicErrorResponse represents an error response from Anthropic API
type AnthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Claude model constants
const (
	Claude3Opus    = "claude-3-opus-20240229"
	Claude3Sonnet  = "claude-3-sonnet-20240229"
	Claude3Haiku   = "claude-3-haiku-20240307"
	Claude35Sonnet = "claude-3-5-sonnet-20241022"
)

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = Claude3Haiku // Default to Haiku for cost efficiency
	}

	return &AnthropicProvider{
		apiKey:   apiKey,
		endpoint: "https://api.anthropic.com/v1/messages",
		model:    model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		timeout: 120 * time.Second,
	}
}

// GenerateCompletion generates a text completion using Claude
func (ap *AnthropicProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.7,
		Stream:      false,
	}

	return ap.sendRequest(ctx, request)
}

// GenerateStructuredOutput generates structured output using JSON format
func (ap *AnthropicProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schemaStruct interface{}) (interface{}, error) {
	// Generate JSON schema from Go struct
	jsonSchema, err := schema.GenerateJSONSchema(schemaStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	schemaJSON, err := jsonSchema.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Enhance prompt with JSON schema instructions
	enhancedPrompt := fmt.Sprintf(`%s

Please respond with valid JSON matching this exact schema:
%s

Respond with ONLY the JSON object, no additional text or explanation.`, prompt, schemaJSON)

	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: enhancedPrompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.3, // Lower temperature for more consistent structured output
		System:      "You are a helpful assistant that extracts structured data. Always respond with valid JSON matching the provided schema.",
		Stream:      false,
	}

	response, err := ap.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response into the schema struct
	if err := json.Unmarshal([]byte(response), schemaStruct); err != nil {
		return nil, fmt.Errorf("failed to parse structured output: %w", err)
	}

	return schemaStruct, nil
}

// sendRequest sends a request to Anthropic API with retry logic
func (ap *AnthropicProvider) sendRequest(ctx context.Context, request AnthropicRequest) (string, error) {
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

		response, err := ap.doRequest(ctx, request)
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
func (ap *AnthropicProvider) doRequest(ctx context.Context, request AnthropicRequest) (string, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ap.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", ap.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := ap.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp AnthropicErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return "", fmt.Errorf("anthropic API error: %s - %s", errResp.Error.Type, errResp.Error.Message)
		}
		return "", fmt.Errorf("anthropic API error: %s - %s", resp.Status, string(body))
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return anthropicResp.Content[0].Text, nil
}

// GetModel returns the model name
func (ap *AnthropicProvider) GetModel() string {
	return ap.model
}

// Health checks if Anthropic API is available
func (ap *AnthropicProvider) Health(ctx context.Context) error {
	// Simple health check with a minimal request
	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		MaxTokens: 10,
		Stream:    false,
	}

	_, err := ap.doRequest(ctx, request)
	return err
}

// SetAPIKey sets the API key
func (ap *AnthropicProvider) SetAPIKey(apiKey string) {
	ap.apiKey = apiKey
}

// SetEndpoint sets the API endpoint
func (ap *AnthropicProvider) SetEndpoint(endpoint string) {
	ap.endpoint = endpoint
}

// SetTimeout sets the request timeout
func (ap *AnthropicProvider) SetTimeout(timeout time.Duration) {
	ap.timeout = timeout
	ap.client.Timeout = timeout
}

// GetSupportedModels returns the list of supported Claude models
func (ap *AnthropicProvider) GetSupportedModels() []string {
	return []string{
		Claude3Opus,
		Claude3Sonnet,
		Claude3Haiku,
		Claude35Sonnet,
	}
}

// GetMaxTokensForModel returns the maximum output tokens for a given model
func (ap *AnthropicProvider) GetMaxTokensForModel(model string) int {
	// Claude 3 models support up to 4096 output tokens
	// Context window is 200K tokens for all Claude 3 models
	return 4096
}

// GetContextWindowForModel returns the context window size for a given model
func (ap *AnthropicProvider) GetContextWindowForModel(model string) int {
	// All Claude 3 models have 200K token context window
	return 200000
}

// GenerateCompletionWithOptions generates a text completion with custom options
func (ap *AnthropicProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *extractor.CompletionOptions) (string, error) {
	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens: 4096,
		Stream:    false,
	}

	// Apply options if provided
	if options != nil {
		if options.Temperature > 0 {
			request.Temperature = options.Temperature
		} else {
			request.Temperature = 0.7
		}
		if options.MaxTokens > 0 {
			request.MaxTokens = options.MaxTokens
		}
		if options.TopP > 0 {
			request.TopP = options.TopP
		}
		if options.TopK > 0 {
			request.TopK = options.TopK
		}
		if options.SystemPrompt != "" {
			request.System = options.SystemPrompt
		}
		if len(options.Stop) > 0 {
			request.StopSequences = options.Stop
		}
	} else {
		request.Temperature = 0.7
	}

	return ap.sendRequest(ctx, request)
}

// GenerateStructuredOutputWithOptions generates structured output with custom options
func (ap *AnthropicProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *extractor.CompletionOptions) (interface{}, error) {
	// Generate JSON schema from Go struct
	jsonSchema, err := schema.GenerateJSONSchema(schemaStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	schemaJSON, err := jsonSchema.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Enhance prompt with JSON schema instructions
	enhancedPrompt := fmt.Sprintf(`%s

Please respond with valid JSON matching this exact schema:
%s

Respond with ONLY the JSON object, no additional text or explanation.`, prompt, schemaJSON)

	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: enhancedPrompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.3, // Lower temperature for more consistent structured output
		System:      "You are a helpful assistant that extracts structured data. Always respond with valid JSON matching the provided schema.",
		Stream:      false,
	}

	// Apply options if provided
	if options != nil {
		if options.Temperature > 0 {
			request.Temperature = options.Temperature
		}
		if options.MaxTokens > 0 {
			request.MaxTokens = options.MaxTokens
		}
		if options.SystemPrompt != "" {
			request.System = options.SystemPrompt
		}
	}

	response, err := ap.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response into the schema struct
	if err := json.Unmarshal([]byte(response), schemaStruct); err != nil {
		return nil, fmt.Errorf("failed to parse structured output: %w", err)
	}

	return schemaStruct, nil
}

// ExtractEntities extracts entities from text using Claude
func (ap *AnthropicProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	prompt := fmt.Sprintf(`Extract all entities from the following text. Return a JSON array of entities with the following structure:
[
  {
    "id": "unique-id",
    "type": "entity-type",
    "properties": {
      "name": "entity-name",
      "confidence": 0.95
    }
  }
]

Text: %s

Respond with ONLY the JSON array, no additional text.`, text)

	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
		System:      "You are a helpful assistant that extracts entities from text. Always respond with valid JSON.",
		Stream:      false,
	}

	response, err := ap.sendRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to extract entities: %w", err)
	}

	// Parse the JSON response
	var nodes []schema.Node
	if err := json.Unmarshal([]byte(response), &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse entities: %w", err)
	}

	return nodes, nil
}

// ExtractRelationships detects relationships between entities in text
func (ap *AnthropicProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	// Build entity list for context
	entityList := make([]string, len(entities))
	for i, entity := range entities {
		if name, ok := entity.Properties["name"].(string); ok {
			entityList[i] = fmt.Sprintf("%s (%s)", name, entity.Type)
		} else {
			entityList[i] = fmt.Sprintf("%s (%s)", entity.ID, entity.Type)
		}
	}

	prompt := fmt.Sprintf(`Given the following entities and text, extract relationships between them. Return a JSON array of relationships with the following structure:
[
  {
    "id": "unique-id",
    "from": "source-entity-id",
    "to": "target-entity-id",
    "type": "RELATIONSHIP_TYPE",
    "properties": {
      "confidence": 0.9
    }
  }
]

Entities: %s

Text: %s

Respond with ONLY the JSON array, no additional text.`, entityList, text)

	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
		System:      "You are a helpful assistant that extracts relationships from text. Always respond with valid JSON.",
		Stream:      false,
	}

	response, err := ap.sendRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to extract relationships: %w", err)
	}

	// Parse the JSON response
	var edges []schema.Edge
	if err := json.Unmarshal([]byte(response), &edges); err != nil {
		return nil, fmt.Errorf("failed to parse relationships: %w", err)
	}

	return edges, nil
}

// ExtractWithCustomSchema extracts data using a custom JSON schema
func (ap *AnthropicProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	schemaJSON, err := json.Marshal(jsonSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON schema: %w", err)
	}

	prompt := fmt.Sprintf(`Extract data from the following text according to this JSON schema:
%s

Text: %s

Respond with ONLY the JSON object matching the schema, no additional text.`, string(schemaJSON), text)

	request := AnthropicRequest{
		Model: ap.model,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
		System:      "You are a helpful assistant that extracts structured data. Always respond with valid JSON matching the provided schema.",
		Stream:      false,
	}

	response, err := ap.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response
	var result interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse custom schema output: %w", err)
	}

	return result, nil
}

// GenerateWithContext generates completion with conversation context
func (ap *AnthropicProvider) GenerateWithContext(ctx context.Context, messages []extractor.Message, options *extractor.CompletionOptions) (string, error) {
	// Convert messages to Anthropic format
	anthropicMessages := make([]AnthropicMessage, 0, len(messages))
	var systemPrompt string

	for _, msg := range messages {
		if msg.Role == extractor.RoleSystem {
			// Anthropic uses a separate system field
			systemPrompt = msg.Content
		} else {
			anthropicMessages = append(anthropicMessages, AnthropicMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	request := AnthropicRequest{
		Model:       ap.model,
		Messages:    anthropicMessages,
		MaxTokens:   4096,
		Temperature: 0.7,
		Stream:      false,
	}

	if systemPrompt != "" {
		request.System = systemPrompt
	}

	// Apply options if provided
	if options != nil {
		if options.Temperature > 0 {
			request.Temperature = options.Temperature
		}
		if options.MaxTokens > 0 {
			request.MaxTokens = options.MaxTokens
		}
		if options.TopP > 0 {
			request.TopP = options.TopP
		}
		if options.TopK > 0 {
			request.TopK = options.TopK
		}
		if options.SystemPrompt != "" {
			request.System = options.SystemPrompt
		}
		if len(options.Stop) > 0 {
			request.StopSequences = options.Stop
		}
	}

	return ap.sendRequest(ctx, request)
}

// GenerateStreamingCompletion generates streaming text completion
func (ap *AnthropicProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback extractor.StreamCallback) error {
	// Anthropic supports streaming, but for simplicity we'll implement non-streaming first
	// TODO: Implement actual streaming support
	result, err := ap.GenerateCompletion(ctx, prompt)
	if err != nil {
		callback("", false, err)
		return err
	}

	// Simulate streaming by calling callback with full result
	callback(result, true, nil)
	return nil
}

// SetModel sets the model to use (implements extractor.LLMProvider interface)
func (ap *AnthropicProvider) SetModel(model string) error {
	ap.model = model
	return nil
}

// GetProviderType returns the provider type
func (ap *AnthropicProvider) GetProviderType() extractor.ProviderType {
	return extractor.ProviderAnthropic
}

// GetCapabilities returns the capabilities supported by this provider
func (ap *AnthropicProvider) GetCapabilities() extractor.ProviderCapabilities {
	caps := extractor.GetProviderCapabilitiesMap()[extractor.ProviderAnthropic]
	if caps == nil {
		return extractor.ProviderCapabilities{
			SupportsCompletion:     true,
			SupportsChat:           true,
			SupportsStreaming:      true,
			SupportsSystemPrompts:  true,
			SupportsConversation:   true,
			MaxContextLength:       200000,
			SupportsImageInput:     true,
			SupportsCodeGeneration: true,
			SupportsRetries:        true,
			SupportsRateLimiting:   true,
			SupportsUsageTracking:  true,
			AvailableModels:        ap.GetSupportedModels(),
			DefaultModel:           Claude3Haiku,
		}
	}
	return *caps
}

// GetTokenCount estimates token count for text
func (ap *AnthropicProvider) GetTokenCount(text string) (int, error) {
	// Simple estimation: ~4 characters per token
	// Claude uses a similar tokenization to GPT models
	return len(text) / 4, nil
}

// GetMaxTokens returns the maximum token limit for this provider/model
func (ap *AnthropicProvider) GetMaxTokens() int {
	return ap.GetMaxTokensForModel(ap.model)
}

// GetUsage returns usage statistics (if available)
func (ap *AnthropicProvider) GetUsage(ctx context.Context) (*extractor.UsageStats, error) {
	// Anthropic doesn't provide a usage API endpoint
	// Return empty stats
	return &extractor.UsageStats{
		TotalTokensUsed:      0,
		PromptTokensUsed:     0,
		CompletionTokensUsed: 0,
		TotalRequests:        0,
		SuccessfulRequests:   0,
		FailedRequests:       0,
		PeriodStart:          time.Now(),
		PeriodEnd:            time.Now(),
	}, nil
}

// GetRateLimit returns current rate limit status (if available)
func (ap *AnthropicProvider) GetRateLimit(ctx context.Context) (*extractor.RateLimitStatus, error) {
	// Anthropic doesn't expose rate limit info in responses
	// Return default values
	return &extractor.RateLimitStatus{
		RequestsPerMinute: 50, // Anthropic's typical rate limit
		TokensPerMinute:   100000,
		RequestsRemaining: 50,
		TokensRemaining:   100000,
		ResetTime:         time.Now().Add(1 * time.Minute),
		IsLimited:         false,
	}, nil
}

// Configure updates provider configuration
func (ap *AnthropicProvider) Configure(config *extractor.ProviderConfig) error {
	if config == nil {
		return extractor.NewExtractorError("validation", "provider config is nil", 400)
	}

	if err := extractor.ValidateProviderConfig(config); err != nil {
		return err
	}

	// Update configuration
	if config.APIKey != "" {
		ap.apiKey = config.APIKey
	}
	if config.Model != "" {
		ap.model = config.Model
	}
	if config.Endpoint != "" {
		ap.endpoint = config.Endpoint
	}
	if config.Timeout > 0 {
		ap.timeout = config.Timeout
		ap.client.Timeout = config.Timeout
	}

	return nil
}

// GetConfiguration returns current provider configuration
func (ap *AnthropicProvider) GetConfiguration() *extractor.ProviderConfig {
	return &extractor.ProviderConfig{
		Type:     extractor.ProviderAnthropic,
		Model:    ap.model,
		APIKey:   ap.apiKey,
		Endpoint: ap.endpoint,
		Timeout:  ap.timeout,
	}
}

// Close closes the provider and cleans up resources
func (ap *AnthropicProvider) Close() error {
	// Close HTTP client connections
	ap.client.CloseIdleConnections()
	return nil
}
