// Package extractor provides OpenAI provider implementation
package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NortonBen/ai-memory-go/schema"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements LLMProvider for OpenAI models
type OpenAIProvider struct {
	client *openai.Client
	model  string
	config *ProviderConfig
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, model string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, NewExtractorError("validation", "API key is required", 400)
	}
	if model == "" {
		model = "gpt-4"
	}

	client := openai.NewClient(apiKey)

	return &OpenAIProvider{
		client: client,
		model:  model,
		config: &ProviderConfig{
			Type:   ProviderOpenAI,
			Model:  model,
			APIKey: apiKey,
		},
	}, nil
}

// NewOpenAIProviderFromConfig creates a new OpenAI provider from configuration
func NewOpenAIProviderFromConfig(config *ProviderConfig) (*OpenAIProvider, error) {
	if config == nil {
		return nil, NewExtractorError("validation", "config is nil", 400)
	}
	return NewOpenAIProvider(config.APIKey, config.Model)
}

// GenerateCompletion generates a text completion
func (p *OpenAIProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	return p.GenerateCompletionWithOptions(ctx, prompt, nil)
}

// GenerateCompletionWithOptions generates a text completion with options
func (p *OpenAIProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
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
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateStructuredOutput generates JSON
func (p *OpenAIProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schemaStruct interface{}) (interface{}, error) {
	return p.GenerateStructuredOutputWithOptions(ctx, prompt, schemaStruct, nil)
}

// GenerateStructuredOutputWithOptions generates JSON with options
func (p *OpenAIProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *CompletionOptions) (interface{}, error) {
	req := openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "You are a data extractor. Output JSON strictly matching the schema."},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
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
	if err := json.Unmarshal([]byte(jsonResponse), schemaStruct); err != nil {
		return nil, fmt.Errorf("schema unmarshal failed: %v", err)
	}

	return schemaStruct, nil
}

func (p *OpenAIProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	return nil, fmt.Errorf("not natively implemented")
}

func (p *OpenAIProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return nil, fmt.Errorf("not natively implemented")
}

func (p *OpenAIProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *OpenAIProvider) GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error) {
	var prompt strings.Builder
	for _, msg := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return p.GenerateCompletionWithOptions(ctx, prompt.String(), options)
}

func (p *OpenAIProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error {
	return fmt.Errorf("not implemented")
}

func (p *OpenAIProvider) GetModel() string {
	return p.model
}

func (p *OpenAIProvider) SetModel(model string) error {
	p.model = model
	return nil
}

func (p *OpenAIProvider) GetProviderType() ProviderType {
	return ProviderOpenAI
}

func (p *OpenAIProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsCompletion: true,
		SupportsJSONMode:   true,
	}
}

func (p *OpenAIProvider) GetTokenCount(text string) (int, error) {
	return len(text) / 4, nil
}

func (p *OpenAIProvider) GetMaxTokens() int {
	return 8192
}

func (p *OpenAIProvider) Health(ctx context.Context) error {
	_, err := p.GenerateCompletionWithOptions(ctx, "Hello", &CompletionOptions{MaxTokens: 5})
	return err
}

func (p *OpenAIProvider) GetUsage(ctx context.Context) (*UsageStats, error) {
	return &UsageStats{}, nil
}

func (p *OpenAIProvider) GetRateLimit(ctx context.Context) (*RateLimitStatus, error) {
	return &RateLimitStatus{}, nil
}

func (p *OpenAIProvider) Configure(config *ProviderConfig) error {
	if config != nil {
		p.config = config
		if config.Model != "" {
			p.model = config.Model
		}
		if config.APIKey != "" {
			p.client = openai.NewClient(config.APIKey)
		}
	}
	return nil
}

func (p *OpenAIProvider) GetConfiguration() *ProviderConfig {
	return p.config
}

func (p *OpenAIProvider) Close() error {
	return nil
}
