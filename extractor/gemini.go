// Package extractor - Google Gemini provider implementation
// Supports both LLM (Gemini Pro) and embedding (text-embedding-004) capabilities
package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// GeminiProvider implements LLMProvider for Google Gemini models
type GeminiProvider struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
	timeout  time.Duration
	config   *ProviderConfig
	mu       sync.RWMutex
}

// GeminiEmbeddingProvider implements EmbeddingProvider for Google text-embedding-004
type GeminiEmbeddingProvider struct {
	apiKey     string
	endpoint   string
	model      string
	dimensions int
	client     *http.Client
	timeout    time.Duration
	config     *EmbeddingProviderConfig
	metrics    *EmbeddingProviderMetrics
	mu         sync.RWMutex
}

// Gemini API request/response structures
type GeminiRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings    []GeminiSafetySetting   `json:"safetySettings,omitempty"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiGenerationConfig struct {
	Temperature      float64     `json:"temperature,omitempty"`
	TopP             float64     `json:"topP,omitempty"`
	TopK             int         `json:"topK,omitempty"`
	MaxOutputTokens  int         `json:"maxOutputTokens,omitempty"`
	StopSequences    []string    `json:"stopSequences,omitempty"`
	ResponseMimeType string      `json:"responseMimeType,omitempty"`
	ResponseSchema   interface{} `json:"responseSchema,omitempty"`
}

type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type GeminiResponse struct {
	Candidates    []GeminiCandidate `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type GeminiCandidate struct {
	Content struct {
		Parts []GeminiPart `json:"parts"`
		Role  string       `json:"role"`
	} `json:"content"`
	FinishReason  string        `json:"finishReason"`
	Index         int           `json:"index"`
	SafetyRatings []interface{} `json:"safetyRatings"`
}

// Gemini embedding structures
type GeminiEmbeddingRequest struct {
	Model                string                 `json:"model"`
	Content              GeminiEmbeddingContent `json:"content"`
	TaskType             string                 `json:"taskType,omitempty"`
	Title                string                 `json:"title,omitempty"`
	OutputDimensionality int                    `json:"outputDimensionality,omitempty"`
}

type GeminiEmbeddingContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiEmbeddingResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

type GeminiBatchEmbeddingRequest struct {
	Requests []GeminiEmbeddingRequest `json:"requests"`
}

type GeminiBatchEmbeddingResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

// Gemini error response
type GeminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// Gemini model constants
const (
	GeminiPro        = "gemini-pro"
	GeminiProVision  = "gemini-pro-vision"
	GeminiPro15      = "gemini-1.5-pro"
	GeminiPro15Flash = "gemini-1.5-flash"

	// Embedding models
	TextEmbedding004 = "text-embedding-004"
)

// Safety settings
const (
	HarmCategoryHarassment       = "HARM_CATEGORY_HARASSMENT"
	HarmCategoryHateSpeech       = "HARM_CATEGORY_HATE_SPEECH"
	HarmCategorySexuallyExplicit = "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	HarmCategoryDangerousContent = "HARM_CATEGORY_DANGEROUS_CONTENT"

	HarmBlockThresholdBlockNone = "BLOCK_NONE"
	HarmBlockThresholdBlockLow  = "BLOCK_LOW_AND_ABOVE"
	HarmBlockThresholdBlockMed  = "BLOCK_MEDIUM_AND_ABOVE"
	HarmBlockThresholdBlockHigh = "BLOCK_HIGH_AND_ABOVE"
)

// NewGeminiProvider creates a new Gemini LLM provider
func NewGeminiProvider(apiKey, model string) (*GeminiProvider, error) {
	if apiKey == "" {
		return nil, NewExtractorError("validation", "Gemini API key is required", 400)
	}

	if model == "" {
		model = GeminiPro15Flash // Default to Flash for cost efficiency
	}

	config := DefaultProviderConfig(ProviderGemini)
	config.APIKey = apiKey
	config.Model = model

	return &GeminiProvider{
		apiKey:   apiKey,
		endpoint: "https://generativelanguage.googleapis.com/v1beta",
		model:    model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		timeout: 120 * time.Second,
		config:  config,
	}, nil
}

// NewGeminiProviderFromConfig creates a new Gemini provider from configuration
func NewGeminiProviderFromConfig(config *ProviderConfig) (*GeminiProvider, error) {
	if config == nil {
		return nil, NewExtractorError("validation", "config is nil", 400)
	}

	return NewGeminiProvider(config.APIKey, config.Model)
}

// GenerateCompletion generates a text completion using Gemini
func (gp *GeminiProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 4096,
		},
		SafetySettings: getDefaultSafetySettings(),
	}

	return gp.sendRequest(ctx, request)
}

// GenerateCompletionWithOptions generates a text completion with custom options
func (gp *GeminiProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error) {
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 4096,
		},
		SafetySettings: getDefaultSafetySettings(),
	}

	// Apply options if provided
	if options != nil {
		if options.Temperature > 0 {
			request.GenerationConfig.Temperature = options.Temperature
		}
		if options.MaxTokens > 0 {
			request.GenerationConfig.MaxOutputTokens = options.MaxTokens
		}
		if options.TopP > 0 {
			request.GenerationConfig.TopP = options.TopP
		}
		if options.TopK > 0 {
			request.GenerationConfig.TopK = options.TopK
		}
		if len(options.Stop) > 0 {
			request.GenerationConfig.StopSequences = options.Stop
		}
		if options.SystemPrompt != "" {
			request.SystemInstruction = &GeminiContent{
				Parts: []GeminiPart{
					{Text: options.SystemPrompt},
				},
			}
		}
	}

	return gp.sendRequest(ctx, request)
}

// GenerateStructuredOutput generates structured output using JSON schema mode
func (gp *GeminiProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schemaStruct interface{}) (interface{}, error) {
	// Generate JSON schema from Go struct
	jsonSchema, err := schema.GenerateJSONSchema(schemaStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	schemaJSON, err := jsonSchema.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Parse schema for Gemini format
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Gemini rejects "$schema" key
	delete(schemaMap, "$schema")

	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:      0.3,
			MaxOutputTokens:  4096,
			ResponseMimeType: "application/json",
			ResponseSchema:   schemaMap,
		},
		SafetySettings: getDefaultSafetySettings(),
	}

	response, err := gp.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response into the schema struct
	if err := json.Unmarshal([]byte(response), schemaStruct); err != nil {
		return nil, fmt.Errorf("failed to parse structured output: %w", err)
	}

	return schemaStruct, nil
}

// GenerateStructuredOutputWithOptions generates structured output with custom options
func (gp *GeminiProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *CompletionOptions) (interface{}, error) {
	// Generate JSON schema from Go struct
	jsonSchema, err := schema.GenerateJSONSchema(schemaStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	schemaJSON, err := jsonSchema.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	// Parse schema for Gemini format
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Gemini rejects "$schema" key
	delete(schemaMap, "$schema")

	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:      0.3,
			MaxOutputTokens:  4096,
			ResponseMimeType: "application/json",
			ResponseSchema:   schemaMap,
		},
		SafetySettings: getDefaultSafetySettings(),
	}

	// Apply options if provided
	if options != nil {
		if options.Temperature > 0 {
			request.GenerationConfig.Temperature = options.Temperature
		}
		if options.MaxTokens > 0 {
			request.GenerationConfig.MaxOutputTokens = options.MaxTokens
		}
		if options.SystemPrompt != "" {
			request.SystemInstruction = &GeminiContent{
				Parts: []GeminiPart{
					{Text: options.SystemPrompt},
				},
			}
		}
	}

	response, err := gp.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response into the schema struct
	if err := json.Unmarshal([]byte(response), schemaStruct); err != nil {
		return nil, fmt.Errorf("failed to parse structured output: %w", err)
	}

	return schemaStruct, nil
}

// ExtractEntities extracts entities from text using Gemini
func (gp *GeminiProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
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

	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:      0.3,
			MaxOutputTokens:  4096,
			ResponseMimeType: "application/json",
		},
		SafetySettings: getDefaultSafetySettings(),
		SystemInstruction: &GeminiContent{
			Parts: []GeminiPart{
				{Text: "You are a helpful assistant that extracts entities from text. Always respond with valid JSON."},
			},
		},
	}

	response, err := gp.sendRequest(ctx, request)
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
func (gp *GeminiProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
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

Respond with ONLY the JSON array, no additional text.`, strings.Join(entityList, ", "), text)

	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:      0.3,
			MaxOutputTokens:  4096,
			ResponseMimeType: "application/json",
		},
		SafetySettings: getDefaultSafetySettings(),
		SystemInstruction: &GeminiContent{
			Parts: []GeminiPart{
				{Text: "You are a helpful assistant that extracts relationships from text. Always respond with valid JSON."},
			},
		},
	}

	response, err := gp.sendRequest(ctx, request)
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
func (gp *GeminiProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	// Gemini rejects "$schema" key
	cleanSchema := make(map[string]interface{})
	for k, v := range jsonSchema {
		if k != "$schema" {
			cleanSchema[k] = v
		}
	}

	schemaJSON, err := json.Marshal(cleanSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON schema: %w", err)
	}

	prompt := fmt.Sprintf(`Extract data from the following text according to this JSON schema:
%s

Text: %s

Respond with ONLY the JSON object matching the schema, no additional text.`, string(schemaJSON), text)

	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:      0.3,
			MaxOutputTokens:  4096,
			ResponseMimeType: "application/json",
			ResponseSchema:   cleanSchema,
		},
		SafetySettings: getDefaultSafetySettings(),
		SystemInstruction: &GeminiContent{
			Parts: []GeminiPart{
				{Text: "You are a helpful assistant that extracts structured data. Always respond with valid JSON matching the provided schema."},
			},
		},
	}

	response, err := gp.sendRequest(ctx, request)
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
func (gp *GeminiProvider) GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error) {
	// Convert messages to Gemini format
	contents := make([]GeminiContent, 0, len(messages))
	var systemInstruction *GeminiContent

	for _, msg := range messages {
		if msg.Role == RoleSystem {
			// Gemini uses a separate system instruction field
			systemInstruction = &GeminiContent{
				Parts: []GeminiPart{
					{Text: msg.Content},
				},
			}
		} else {
			role := string(msg.Role)
			if role == "assistant" {
				role = "model" // Gemini uses "model" instead of "assistant"
			}
			contents = append(contents, GeminiContent{
				Role: role,
				Parts: []GeminiPart{
					{Text: msg.Content},
				},
			})
		}
	}

	request := GeminiRequest{
		Contents: contents,
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 4096,
		},
		SafetySettings: getDefaultSafetySettings(),
	}

	if systemInstruction != nil {
		request.SystemInstruction = systemInstruction
	}

	// Apply options if provided
	if options != nil {
		if options.Temperature > 0 {
			request.GenerationConfig.Temperature = options.Temperature
		}
		if options.MaxTokens > 0 {
			request.GenerationConfig.MaxOutputTokens = options.MaxTokens
		}
		if options.TopP > 0 {
			request.GenerationConfig.TopP = options.TopP
		}
		if options.TopK > 0 {
			request.GenerationConfig.TopK = options.TopK
		}
		if len(options.Stop) > 0 {
			request.GenerationConfig.StopSequences = options.Stop
		}
		if options.SystemPrompt != "" && systemInstruction == nil {
			request.SystemInstruction = &GeminiContent{
				Parts: []GeminiPart{
					{Text: options.SystemPrompt},
				},
			}
		}
	}

	return gp.sendRequest(ctx, request)
}

// GenerateStreamingCompletion generates streaming text completion
func (gp *GeminiProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error {
	// Gemini supports streaming, but for simplicity we'll implement non-streaming first
	// TODO: Implement actual streaming support using Server-Sent Events
	result, err := gp.GenerateCompletion(ctx, prompt)
	if err != nil {
		callback("", false, err)
		return err
	}

	// Simulate streaming by calling callback with full result
	callback(result, true, nil)
	return nil
}

// sendRequest sends a request to Gemini API with retry logic
func (gp *GeminiProvider) sendRequest(ctx context.Context, request GeminiRequest) (string, error) {
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

		response, err := gp.doRequest(ctx, request)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// doRequest performs the actual HTTP request
func (gp *GeminiProvider) doRequest(ctx context.Context, request GeminiRequest) (string, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", gp.endpoint, gp.model, gp.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := gp.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp GeminiErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return "", fmt.Errorf("Gemini API error: %s - %s", errResp.Error.Status, errResp.Error.Message)
		}
		return "", fmt.Errorf("Gemini API error: %s - %s", resp.Status, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content parts in response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// LLMProvider interface implementation methods

// GetModel returns the model name
func (gp *GeminiProvider) GetModel() string {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
	return gp.model
}

// SetModel sets the model to use
func (gp *GeminiProvider) SetModel(model string) error {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	gp.model = model
	gp.config.Model = model
	return nil
}

// GetProviderType returns the provider type
func (gp *GeminiProvider) GetProviderType() ProviderType {
	return ProviderGemini
}

// GetCapabilities returns the capabilities supported by this provider
func (gp *GeminiProvider) GetCapabilities() ProviderCapabilities {
	caps := GetProviderCapabilitiesMap()[ProviderGemini]
	if caps == nil {
		return ProviderCapabilities{
			SupportsCompletion:     true,
			SupportsChat:           true,
			SupportsStreaming:      true,
			SupportsJSONMode:       true,
			SupportsJSONSchema:     true,
			SupportsSystemPrompts:  true,
			SupportsConversation:   true,
			MaxContextLength:       1000000, // Gemini 1.5 Pro has 1M token context
			SupportsImageInput:     true,
			SupportsCodeGeneration: true,
			SupportsRetries:        true,
			SupportsRateLimiting:   true,
			SupportsUsageTracking:  true,
			AvailableModels:        gp.GetSupportedModels(),
			DefaultModel:           GeminiPro15Flash,
		}
	}
	return *caps
}

// GetTokenCount estimates token count for text
func (gp *GeminiProvider) GetTokenCount(text string) (int, error) {
	// Simple estimation: ~4 characters per token
	// Gemini uses a similar tokenization to other models
	return len(text) / 4, nil
}

// GetMaxTokens returns the maximum token limit for this provider/model
func (gp *GeminiProvider) GetMaxTokens() int {
	return gp.GetMaxTokensForModel(gp.model)
}

// Health checks if Gemini API is available
func (gp *GeminiProvider) Health(ctx context.Context) error {
	// Simple health check with a minimal request
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: "Hello"},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			MaxOutputTokens: 10,
		},
		SafetySettings: getDefaultSafetySettings(),
	}

	_, err := gp.doRequest(ctx, request)
	return err
}

// GetUsage returns usage statistics (if available)
func (gp *GeminiProvider) GetUsage(ctx context.Context) (*UsageStats, error) {
	// Gemini doesn't provide a usage API endpoint
	// Return empty stats
	return &UsageStats{
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
func (gp *GeminiProvider) GetRateLimit(ctx context.Context) (*RateLimitStatus, error) {
	// Gemini doesn't expose rate limit info in responses
	// Return default values based on tier
	return &RateLimitStatus{
		RequestsPerMinute: 60, // Free tier limit
		TokensPerMinute:   32000,
		RequestsRemaining: 60,
		TokensRemaining:   32000,
		ResetTime:         time.Now().Add(1 * time.Minute),
		IsLimited:         false,
	}, nil
}

// Configure updates provider configuration
func (gp *GeminiProvider) Configure(config *ProviderConfig) error {
	if config == nil {
		return NewExtractorError("validation", "provider config is nil", 400)
	}

	if err := ValidateProviderConfig(config); err != nil {
		return err
	}

	gp.mu.Lock()
	defer gp.mu.Unlock()

	// Update configuration
	if config.APIKey != "" {
		gp.apiKey = config.APIKey
	}
	if config.Model != "" {
		gp.model = config.Model
	}
	if config.Endpoint != "" {
		gp.endpoint = config.Endpoint
	}
	if config.Timeout > 0 {
		gp.timeout = config.Timeout
		gp.client.Timeout = config.Timeout
	}

	gp.config = config
	return nil
}

// GetConfiguration returns current provider configuration
func (gp *GeminiProvider) GetConfiguration() *ProviderConfig {
	gp.mu.RLock()
	defer gp.mu.RUnlock()

	// Return a copy to prevent modification
	configCopy := *gp.config
	return &configCopy
}

// Close closes the provider and cleans up resources
func (gp *GeminiProvider) Close() error {
	// Close HTTP client connections
	gp.client.CloseIdleConnections()
	return nil
}

// Helper methods

// GetSupportedModels returns the list of supported Gemini models
func (gp *GeminiProvider) GetSupportedModels() []string {
	return []string{
		GeminiPro,
		GeminiProVision,
		GeminiPro15,
		GeminiPro15Flash,
	}
}

// GetMaxTokensForModel returns the maximum output tokens for a given model
func (gp *GeminiProvider) GetMaxTokensForModel(model string) int {
	// Gemini models support up to 8192 output tokens
	return 8192
}

// GetContextWindowForModel returns the context window size for a given model
func (gp *GeminiProvider) GetContextWindowForModel(model string) int {
	switch model {
	case GeminiPro15, GeminiPro15Flash:
		return 1000000 // 1M tokens for Gemini 1.5
	case GeminiPro, GeminiProVision:
		return 32768 // 32K tokens for Gemini Pro
	default:
		return 32768
	}
}

// getDefaultSafetySettings returns default safety settings for Gemini
func getDefaultSafetySettings() []GeminiSafetySetting {
	return []GeminiSafetySetting{
		{
			Category:  HarmCategoryHarassment,
			Threshold: HarmBlockThresholdBlockMed,
		},
		{
			Category:  HarmCategoryHateSpeech,
			Threshold: HarmBlockThresholdBlockMed,
		},
		{
			Category:  HarmCategorySexuallyExplicit,
			Threshold: HarmBlockThresholdBlockMed,
		},
		{
			Category:  HarmCategoryDangerousContent,
			Threshold: HarmBlockThresholdBlockMed,
		},
	}
}

// Helper function to check if error is retryable - using existing implementation from deepseek.go

// ============================================================================
// Gemini Embedding Provider Implementation
// ============================================================================

// NewGeminiEmbeddingProvider creates a new Gemini embedding provider
func NewGeminiEmbeddingProvider(apiKey, model string) (*GeminiEmbeddingProvider, error) {
	if apiKey == "" {
		return nil, NewExtractorError("validation", "Gemini API key is required", 400)
	}

	if model == "" {
		model = TextEmbedding004
	}

	// text-embedding-004 has 768 dimensions
	dimensions := 768

	config := DefaultEmbeddingProviderConfig(EmbeddingProviderGemini)
	config.APIKey = apiKey
	config.Model = model
	config.Dimensions = dimensions

	return &GeminiEmbeddingProvider{
		apiKey:     apiKey,
		endpoint:   "https://generativelanguage.googleapis.com/v1beta",
		model:      model,
		dimensions: dimensions,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		timeout: 60 * time.Second,
		config:  config,
		metrics: &EmbeddingProviderMetrics{
			FirstRequest: time.Now(),
		},
	}, nil
}

// NewGeminiEmbeddingProviderFromConfig creates a new Gemini embedding provider from configuration
func NewGeminiEmbeddingProviderFromConfig(config *EmbeddingProviderConfig) (*GeminiEmbeddingProvider, error) {
	if err := ValidateEmbeddingProviderConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return NewGeminiEmbeddingProvider(config.APIKey, config.Model)
}

// GenerateEmbedding generates an embedding for a single text
func (gep *GeminiEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	gep.mu.Lock()
	gep.metrics.TotalRequests++
	gep.mu.Unlock()

	start := time.Now()

	request := GeminiEmbeddingRequest{
		Model: fmt.Sprintf("models/%s", gep.model),
		Content: GeminiEmbeddingContent{
			Parts: []GeminiPart{
				{Text: text},
			},
		},
		TaskType: "RETRIEVAL_DOCUMENT",
	}

	// Set output dimensionality if supported
	if gep.dimensions > 0 {
		request.OutputDimensionality = gep.dimensions
	}

	embedding, err := gep.doEmbeddingRequest(ctx, request)
	if err != nil {
		gep.mu.Lock()
		gep.metrics.FailedRequests++
		gep.mu.Unlock()
		return nil, err
	}

	latency := time.Since(start)
	gep.mu.Lock()
	gep.metrics.SuccessfulRequests++
	gep.metrics.TotalTextsProcessed++
	gep.metrics.TotalEmbeddings++
	gep.metrics.AverageLatency = (gep.metrics.AverageLatency + latency) / 2
	gep.mu.Unlock()

	return embedding, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (gep *GeminiEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	gep.mu.Lock()
	gep.metrics.TotalRequests++
	gep.metrics.TotalBatchRequests++
	gep.mu.Unlock()

	start := time.Now()

	// Gemini embedding API supports batch requests
	requests := make([]GeminiEmbeddingRequest, len(texts))
	for i, text := range texts {
		requests[i] = GeminiEmbeddingRequest{
			Model: fmt.Sprintf("models/%s", gep.model),
			Content: GeminiEmbeddingContent{
				Parts: []GeminiPart{
					{Text: text},
				},
			},
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// Set output dimensionality if supported
		if gep.dimensions > 0 && gep.dimensions != 768 {
			requests[i].OutputDimensionality = gep.dimensions
		}
	}

	batchRequest := GeminiBatchEmbeddingRequest{
		Requests: requests,
	}

	embeddings, err := gep.doBatchEmbeddingRequest(ctx, batchRequest)
	if err != nil {
		gep.mu.Lock()
		gep.metrics.FailedRequests++
		gep.mu.Unlock()
		return nil, err
	}

	latency := time.Since(start)
	gep.mu.Lock()
	gep.metrics.SuccessfulRequests++
	gep.metrics.TotalTextsProcessed += int64(len(texts))
	gep.metrics.TotalEmbeddings += int64(len(texts))
	gep.metrics.AverageLatency = (gep.metrics.AverageLatency + latency) / 2
	gep.mu.Unlock()

	return embeddings, nil
}

// GenerateEmbeddingWithOptions generates an embedding with custom options
func (gep *GeminiEmbeddingProvider) GenerateEmbeddingWithOptions(ctx context.Context, text string, options *EmbeddingOptions) ([]float32, error) {
	// For Gemini, options mainly affect dimensions and task type
	if options != nil && options.Dimensions > 0 {
		// Temporarily set dimensions
		originalDims := gep.dimensions
		gep.mu.Lock()
		gep.dimensions = options.Dimensions
		gep.mu.Unlock()

		embedding, err := gep.GenerateEmbedding(ctx, text)

		// Restore original dimensions
		gep.mu.Lock()
		gep.dimensions = originalDims
		gep.mu.Unlock()

		return embedding, err
	}

	return gep.GenerateEmbedding(ctx, text)
}

// GenerateBatchEmbeddingsWithOptions generates batch embeddings with custom options
func (gep *GeminiEmbeddingProvider) GenerateBatchEmbeddingsWithOptions(ctx context.Context, texts []string, options *EmbeddingOptions) ([][]float32, error) {
	// For Gemini, options mainly affect dimensions and task type
	if options != nil && options.Dimensions > 0 {
		// Temporarily set dimensions
		originalDims := gep.dimensions
		gep.mu.Lock()
		gep.dimensions = options.Dimensions
		gep.mu.Unlock()

		embeddings, err := gep.GenerateBatchEmbeddings(ctx, texts)

		// Restore original dimensions
		gep.mu.Lock()
		gep.dimensions = originalDims
		gep.mu.Unlock()

		return embeddings, err
	}

	return gep.GenerateBatchEmbeddings(ctx, texts)
}

// doEmbeddingRequest performs a single embedding request
func (gep *GeminiEmbeddingProvider) doEmbeddingRequest(ctx context.Context, request GeminiEmbeddingRequest) ([]float32, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", gep.endpoint, gep.model, gep.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := gep.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp GeminiErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("Gemini embedding API error: %s - %s", errResp.Error.Status, errResp.Error.Message)
		}
		return nil, fmt.Errorf("Gemini embedding API error: %s - %s", resp.Status, string(body))
	}

	var geminiResp GeminiEmbeddingResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return geminiResp.Embedding.Values, nil
}

// doBatchEmbeddingRequest performs a batch embedding request
func (gep *GeminiEmbeddingProvider) doBatchEmbeddingRequest(ctx context.Context, batchRequest GeminiBatchEmbeddingRequest) ([][]float32, error) {
	// For now, process batch requests sequentially
	// TODO: Implement actual batch API when available
	embeddings := make([][]float32, len(batchRequest.Requests))
	for i, req := range batchRequest.Requests {
		embedding, err := gep.doEmbeddingRequest(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("batch request failed at index %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// EmbeddingProvider interface implementation methods

// GetDimensions returns the embedding dimensions
func (gep *GeminiEmbeddingProvider) GetDimensions() int {
	gep.mu.RLock()
	defer gep.mu.RUnlock()
	return gep.dimensions
}

// GetModel returns the current model name
func (gep *GeminiEmbeddingProvider) GetModel() string {
	gep.mu.RLock()
	defer gep.mu.RUnlock()
	return gep.model
}

// SetModel sets the model to use
func (gep *GeminiEmbeddingProvider) SetModel(model string) error {
	gep.mu.Lock()
	defer gep.mu.Unlock()

	// Update dimensions based on model
	switch model {
	case TextEmbedding004:
		gep.dimensions = 768
	default:
		return NewExtractorError("validation", fmt.Sprintf("unsupported model: %s", model), 400)
	}

	gep.model = model
	gep.config.Model = model
	gep.config.Dimensions = gep.dimensions
	return nil
}

// GetProviderType returns the provider type
func (gep *GeminiEmbeddingProvider) GetProviderType() EmbeddingProviderType {
	return EmbeddingProviderGemini
}

// GetSupportedModels returns the list of supported models
func (gep *GeminiEmbeddingProvider) GetSupportedModels() []string {
	return []string{
		TextEmbedding004,
	}
}

// GetMaxBatchSize returns the maximum batch size
func (gep *GeminiEmbeddingProvider) GetMaxBatchSize() int {
	return 100 // Conservative batch size for Gemini
}

// GetMaxTokensPerText returns the maximum tokens per text
func (gep *GeminiEmbeddingProvider) GetMaxTokensPerText() int {
	return 2048 // Conservative token limit
}

// GenerateEmbeddingCached generates an embedding with caching support
func (gep *GeminiEmbeddingProvider) GenerateEmbeddingCached(ctx context.Context, text string, ttl time.Duration) ([]float32, error) {
	// TODO: Implement caching layer
	// For now, just call the regular method
	return gep.GenerateEmbedding(ctx, text)
}

// GenerateBatchEmbeddingsCached generates batch embeddings with caching
func (gep *GeminiEmbeddingProvider) GenerateBatchEmbeddingsCached(ctx context.Context, texts []string, ttl time.Duration) ([][]float32, error) {
	// TODO: Implement caching layer
	// For now, just call the regular method
	return gep.GenerateBatchEmbeddings(ctx, texts)
}

// DeduplicateAndEmbed removes duplicate texts and generates embeddings efficiently
func (gep *GeminiEmbeddingProvider) DeduplicateAndEmbed(ctx context.Context, texts []string) (map[string][]float32, error) {
	// Remove duplicates
	uniqueTexts := make([]string, 0)
	seen := make(map[string]bool)
	textToIndex := make(map[string]int)

	for _, text := range texts {
		if !seen[text] {
			textToIndex[text] = len(uniqueTexts)
			uniqueTexts = append(uniqueTexts, text)
			seen[text] = true
		}
	}

	// Generate embeddings for unique texts
	embeddings, err := gep.GenerateBatchEmbeddings(ctx, uniqueTexts)
	if err != nil {
		return nil, err
	}

	// Create result map
	result := make(map[string][]float32)
	for text, index := range textToIndex {
		result[text] = embeddings[index]
	}

	return result, nil
}

// EstimateTokenCount estimates token count for text
func (gep *GeminiEmbeddingProvider) EstimateTokenCount(text string) (int, error) {
	// Simple estimation: ~4 characters per token
	return len(text) / 4, nil
}

// EstimateCost estimates the cost for embedding generation
func (gep *GeminiEmbeddingProvider) EstimateCost(tokenCount int) (float64, error) {
	// Gemini embedding pricing (as of 2024)
	// text-embedding-004 is free up to certain limits
	costPerToken := 0.0 // Free tier
	return float64(tokenCount) * costPerToken, nil
}

// Health checks if Gemini embedding API is available
func (gep *GeminiEmbeddingProvider) Health(ctx context.Context) error {
	// Simple health check: generate a test embedding
	_, err := gep.GenerateEmbedding(ctx, "health check")
	if err != nil {
		return fmt.Errorf("Gemini embedding health check failed: %w", err)
	}
	return nil
}

// GetUsage returns usage statistics
func (gep *GeminiEmbeddingProvider) GetUsage(ctx context.Context) (*EmbeddingUsageStats, error) {
	gep.mu.RLock()
	defer gep.mu.RUnlock()

	totalRequests := gep.metrics.TotalRequests
	successRate := float64(0)
	if totalRequests > 0 {
		successRate = float64(gep.metrics.SuccessfulRequests) / float64(totalRequests)
	}

	batchEfficiency := float64(0)
	if gep.metrics.TotalRequests > 0 {
		batchEfficiency = float64(gep.metrics.TotalBatchRequests) / float64(gep.metrics.TotalRequests)
	}

	return &EmbeddingUsageStats{
		TotalRequests:       gep.metrics.TotalRequests,
		SuccessfulRequests:  gep.metrics.SuccessfulRequests,
		FailedRequests:      gep.metrics.FailedRequests,
		TotalTextsProcessed: gep.metrics.TotalTextsProcessed,
		TotalTokensUsed:     gep.metrics.TotalTokensUsed,
		TotalEmbeddings:     gep.metrics.TotalEmbeddings,
		AverageLatency:      gep.metrics.AverageLatency,
		TotalBatchRequests:  gep.metrics.TotalBatchRequests,
		BatchEfficiency:     batchEfficiency,
		CacheHitRate:        0, // TODO: Implement caching
		CacheMissRate:       1 - successRate,
		PeriodStart:         gep.metrics.FirstRequest,
		PeriodEnd:           time.Now(),
	}, nil
}

// GetRateLimit returns current rate limit status
func (gep *GeminiEmbeddingProvider) GetRateLimit(ctx context.Context) (*EmbeddingRateLimitStatus, error) {
	// Gemini doesn't provide real-time rate limit info via API
	// Return estimated values based on free tier
	return &EmbeddingRateLimitStatus{
		RequestsPerMinute: 1500,
		TokensPerMinute:   1000000,
		RequestsRemaining: 1400,
		TokensRemaining:   990000,
		ResetTime:         time.Now().Add(1 * time.Minute),
	}, nil
}

// Configure updates provider configuration
func (gep *GeminiEmbeddingProvider) Configure(config *EmbeddingProviderConfig) error {
	if err := ValidateEmbeddingProviderConfig(config); err != nil {
		return err
	}

	gep.mu.Lock()
	defer gep.mu.Unlock()

	// Update configuration
	if config.APIKey != "" {
		gep.apiKey = config.APIKey
	}

	// Update model if changed
	if config.Model != "" {
		gep.model = config.Model
	}

	// Update dimensions if changed
	if config.Dimensions > 0 {
		gep.dimensions = config.Dimensions
	}

	gep.config = config
	return nil
}

// GetConfiguration returns current provider configuration
func (gep *GeminiEmbeddingProvider) GetConfiguration() *EmbeddingProviderConfig {
	gep.mu.RLock()
	defer gep.mu.RUnlock()

	// Return a copy to prevent modification
	configCopy := *gep.config
	return &configCopy
}

// ValidateConfiguration validates the provider configuration
func (gep *GeminiEmbeddingProvider) ValidateConfiguration(config *EmbeddingProviderConfig) error {
	return ValidateEmbeddingProviderConfig(config)
}

// Close closes the provider and cleans up resources
func (gep *GeminiEmbeddingProvider) Close() error {
	// Close HTTP client connections
	gep.client.CloseIdleConnections()
	return nil
}

// SupportsStreaming checks if provider supports streaming embeddings
func (gep *GeminiEmbeddingProvider) SupportsStreaming() bool {
	return false
}

// GenerateStreamingEmbedding generates embedding with streaming callback
func (gep *GeminiEmbeddingProvider) GenerateStreamingEmbedding(ctx context.Context, text string, callback EmbeddingStreamCallback) error {
	return NewExtractorError("unsupported", "Gemini embedding provider does not support streaming", 501)
}

// SupportsCustomDimensions checks if provider supports custom dimensions
func (gep *GeminiEmbeddingProvider) SupportsCustomDimensions() bool {
	// text-embedding-004 supports custom dimensions
	return gep.model == TextEmbedding004
}

// SetCustomDimensions sets custom embedding dimensions
func (gep *GeminiEmbeddingProvider) SetCustomDimensions(dimensions int) error {
	if !gep.SupportsCustomDimensions() {
		return NewExtractorError("unsupported", fmt.Sprintf("model %s does not support custom dimensions", gep.model), 400)
	}

	// Validate dimensions range for text-embedding-004
	if dimensions < 1 || dimensions > 768 {
		return NewExtractorError("validation", "dimensions must be between 1 and 768", 400)
	}

	gep.mu.Lock()
	defer gep.mu.Unlock()

	gep.dimensions = dimensions
	gep.config.Dimensions = dimensions
	return nil
}

// GetCapabilities returns the capabilities supported by this provider
func (gep *GeminiEmbeddingProvider) GetCapabilities() *EmbeddingProviderCapabilities {
	return &EmbeddingProviderCapabilities{
		SupportsBatching:      true,
		SupportsStreaming:     false,
		SupportsCustomDims:    gep.SupportsCustomDimensions(),
		SupportsNormalization: true,
		MaxTokensPerText:      2048,
		MaxBatchSize:          100,
		SupportedModels: []string{
			TextEmbedding004,
		},
		DefaultModel:          TextEmbedding004,
		SupportedDimensions:   []int{768},
		DefaultDimension:      768,
		MinDimension:          1,
		MaxDimension:          768,
		SupportsRateLimiting:  true,
		SupportsUsageTracking: true,
		SupportsCaching:       true,
		SupportsDeduplication: true,
		SupportsMultiModal:    false,
		SupportedInputTypes:   []string{"text"},
		SupportsFineTuning:    false,
		SupportsCustomModels:  false,
		CostPerToken:          0.0, // Free tier
		RateLimitRPM:          1500,
		RateLimitTPM:          1000000,
	}
}
