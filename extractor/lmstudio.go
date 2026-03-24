package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// LMStudioProvider wraps OpenAIProvider to override the provider type
type LMStudioProvider struct {
	*OpenAIProvider
}

// GetProviderType returns the LM Studio provider type
func (p *LMStudioProvider) GetProviderType() ProviderType {
	return ProviderLMStudio
}

// NewLMStudioProvider creates a new provider that uses LM Studio's local OpenAI-compatible API
// Default endpoint is usually http://localhost:1234/v1
func NewLMStudioProvider(endpoint, model string) (*LMStudioProvider, error) {
	if endpoint == "" {
		endpoint = "http://localhost:1234/v1"
	}
	if model == "" {
		model = "local-model" // LM Studio often ignores the model name if only one is loaded
	}

	// Configure the official OpenAI SDK to point to the local LM Studio instance
	config := openai.DefaultConfig("lm-studio")
	config.BaseURL = endpoint

	client := openai.NewClientWithConfig(config)

	baseProvider := &OpenAIProvider{
		client: client,
		model:  model,
		config: &ProviderConfig{
			Type:     ProviderLMStudio,
			Model:    model,
			APIKey:   "lm-studio",
			Endpoint: endpoint,
		},
	}

	return &LMStudioProvider{
		OpenAIProvider: baseProvider,
	}, nil
}

// GenerateStructuredOutput overrides OpenAIProvider to prevent using unsupported ResponseFormat
func (p *LMStudioProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schemaStruct interface{}) (interface{}, error) {
	return p.GenerateStructuredOutputWithOptions(ctx, prompt, schemaStruct, nil)
}

// GenerateStructuredOutputWithOptions overrides OpenAIProvider to prevent using unsupported json_object formatting
func (p *LMStudioProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *CompletionOptions) (interface{}, error) {
	req := openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "You are a data extractor. Output ONLY valid JSON strictly matching the specific schema requested by the user. Do not include markdown formatting or explanations. CRITICAL: The root of your output MUST be a JSON object {...}, NOT a JSON array [...]."},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		// Omit ResponseFormat because LM Studio with some local models (e.g. Qwen) 
		// rejects "json_object" format type.
	}
	
	if options != nil {
		if options.Temperature > 0 {
			req.Temperature = float32(options.Temperature)
		}
		if options.MaxTokens > 0 {
			req.MaxTokens = options.MaxTokens
		}
	}

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	jsonResponse := resp.Choices[0].Message.Content
	
	// Try to clean up markdown code blocks if the model wrapped it
	jsonResponse = cleanJSONResponse(jsonResponse)
	
	importJson := json.Unmarshal // need to handle import properly

	if err := importJson([]byte(jsonResponse), schemaStruct); err != nil {
		return nil, fmt.Errorf("schema unmarshal failed: %v\nResponse was: %s", err, jsonResponse)
	}

	return schemaStruct, nil
}

// cleanJSONResponse tries to remove markdown formatting like ```json ... ```
func cleanJSONResponse(text string) string {
	// Simple cleanup for common markdown JSON wrapping
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
	}
	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}
	return strings.TrimSpace(text)
}
