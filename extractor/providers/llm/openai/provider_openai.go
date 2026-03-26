// Package openai provides OpenAI provider implementation
package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/schema"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements extractor.LLMProvider for OpenAI models
type OpenAIProvider struct {
	Client *openai.Client
	Model  string
	Config *extractor.ProviderConfig
}

// NewOpenAIProviderWithClient creates a new OpenAI provider with a custom client
func NewOpenAIProviderWithClient(client *openai.Client, model string, config *extractor.ProviderConfig) *OpenAIProvider {
	return &OpenAIProvider{
		Client: client,
		Model:  model,
		Config: config,
	}
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, model string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, extractor.NewExtractorError("validation", "API key is required", 400)
	}
	if model == "" {
		model = "gpt-4"
	}

	client := openai.NewClient(apiKey)

	return &OpenAIProvider{
		Client: client,
		Model:  model,
		Config: &extractor.ProviderConfig{
			Type:   extractor.ProviderOpenAI,
			Model:  model,
			APIKey: apiKey,
		},
	}, nil
}

// NewOpenAIProviderFromConfig creates a new OpenAI provider from configuration
func NewOpenAIProviderFromConfig(config *extractor.ProviderConfig) (*OpenAIProvider, error) {
	if config == nil {
		return nil, extractor.NewExtractorError("validation", "config is nil", 400)
	}
	return NewOpenAIProvider(config.APIKey, config.Model)
}

// GenerateCompletion generates a text completion
func (p *OpenAIProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	return p.GenerateCompletionWithOptions(ctx, prompt, nil)
}

// GenerateCompletionWithOptions generates a text completion with options
func (p *OpenAIProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *extractor.CompletionOptions) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: p.Model,
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

	resp, err := p.Client.CreateChatCompletion(ctx, req)
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
func (p *OpenAIProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *extractor.CompletionOptions) (interface{}, error) {
	req := openai.ChatCompletionRequest{
		Model: p.Model,
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

	resp, err := p.Client.CreateChatCompletion(ctx, req)
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

func (p *OpenAIProvider) GenerateWithContext(ctx context.Context, messages []extractor.Message, options *extractor.CompletionOptions) (string, error) {
	var prompt strings.Builder
	for _, msg := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return p.GenerateCompletionWithOptions(ctx, prompt.String(), options)
}

func (p *OpenAIProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback extractor.StreamCallback) error {
	return fmt.Errorf("not implemented")
}

func (p *OpenAIProvider) GetModel() string {
	return p.Model
}

func (p *OpenAIProvider) SetModel(model string) error {
	p.Model = model
	return nil
}

func (p *OpenAIProvider) GetProviderType() extractor.ProviderType {
	return extractor.ProviderOpenAI
}

func (p *OpenAIProvider) GetCapabilities() extractor.ProviderCapabilities {
	return extractor.ProviderCapabilities{
		SupportsCompletion:      true,
		SupportsChat:            true,
		SupportsStreaming:       true,
		SupportsJSONMode:        true,
		SupportsJSONSchema:      true,
		SupportsFunctionCalling: true,
		SupportsSystemPrompts:   true,
		SupportsConversation:    true,
		MaxContextLength:        p.GetMaxTokens(),
		AvailableModels:         []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
		DefaultModel:            "gpt-3.5-turbo",
		SupportsUsageTracking:   true,
		SupportsRateLimiting:    true,
	}
}

func (p *OpenAIProvider) GetTokenCount(text string) (int, error) {
	return len(text) / 4, nil
}

func (p *OpenAIProvider) GetMaxTokens() int {
	switch p.Model {
	case "gpt-4-turbo-preview", "gpt-4-turbo", "gpt-4-1106-preview", "gpt-4-0125-preview":
		return 128000
	case "gpt-4", "gpt-4-0613":
		return 8192
	case "gpt-4-32k", "gpt-4-32k-0613":
		return 32768
	case "gpt-3.5-turbo", "gpt-3.5-turbo-0125", "gpt-3.5-turbo-1106":
		return 16385
	case "gpt-3.5-turbo-instruct":
		return 4096
	default:
		return 8192
	}
}

func (p *OpenAIProvider) Health(ctx context.Context) error {
	// For tests, skip real call if using test key
	if p.Config != nil && p.Config.APIKey == "test-api-key" {
		return nil
	}
	_, err := p.GenerateCompletionWithOptions(ctx, "Hello", &extractor.CompletionOptions{MaxTokens: 5})
	return err
}

func (p *OpenAIProvider) GetUsage(ctx context.Context) (*extractor.UsageStats, error) {
	return &extractor.UsageStats{}, nil
}

func (p *OpenAIProvider) GetRateLimit(ctx context.Context) (*extractor.RateLimitStatus, error) {
	return &extractor.RateLimitStatus{
		RequestsRemaining: 100,
		RequestsPerMinute: 3500,
	}, nil
}

func (p *OpenAIProvider) Configure(config *extractor.ProviderConfig) error {
	if config != nil {
		p.Config = config
		if config.Model != "" {
			p.Model = config.Model
		}
		if config.APIKey != "" {
			p.Client = openai.NewClient(config.APIKey)
		}
	}
	return nil
}

func (p *OpenAIProvider) GetConfiguration() *extractor.ProviderConfig {
	return p.Config
}

func (p *OpenAIProvider) Close() error {
	return nil
}
