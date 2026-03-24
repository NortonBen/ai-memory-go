package extractor

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
)

// GenerateCompletionWithOptions generates text with options
func (dp *DeepSeekProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error) {
	request := DeepSeekRequest{
		Model:       dp.model,
		Messages:    []DeepSeekMessage{{Role: "user", Content: prompt}},
		Temperature: 0.7,
		MaxTokens:   2000,
		Stream:      false,
	}

	if options != nil {
		if options.Temperature > 0 {
			request.Temperature = options.Temperature
		}
		if options.MaxTokens > 0 {
			request.MaxTokens = options.MaxTokens
		}
	}

	return dp.sendRequest(ctx, request)
}

// GenerateStructuredOutputWithOptions generates JSON with options
func (dp *DeepSeekProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *CompletionOptions) (interface{}, error) {
	// Simple wrapper for now
	return dp.GenerateStructuredOutput(ctx, prompt, schemaStruct)
}

// ExtractEntities extracts entities using basic extractor fallback
func (dp *DeepSeekProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	return nil, fmt.Errorf("not implemented natively - use basic extractor")
}

// ExtractRelationships extracts relationships using basic extractor fallback
func (dp *DeepSeekProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return nil, fmt.Errorf("not implemented natively - use basic extractor")
}

// ExtractWithCustomSchema extracts custom data
func (dp *DeepSeekProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// GenerateWithContext generates text from multiple messages
func (dp *DeepSeekProvider) GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// GenerateStreamingCompletion generates streaming text
func (dp *DeepSeekProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error {
	return fmt.Errorf("not implemented")
}

// GetProviderType returns provider type
func (dp *DeepSeekProvider) GetProviderType() ProviderType {
	return ProviderDeepSeek
}

// GetCapabilities returns provider capabilities
func (dp *DeepSeekProvider) GetCapabilities() ProviderCapabilities {
	caps := GetProviderCapabilitiesMap()[ProviderDeepSeek]
	if caps == nil {
		return ProviderCapabilities{
			SupportsCompletion: true,
			SupportsJSONMode:   true,
			SupportsJSONSchema: true,
		}
	}
	return *caps
}

// GetTokenCount returns token count estimation
func (dp *DeepSeekProvider) GetTokenCount(text string) (int, error) {
	return len(text) / 4, nil
}

// GetMaxTokens returns theoretical context limit
func (dp *DeepSeekProvider) GetMaxTokens() int {
	return 8192
}

// GetUsage returns current usage
func (dp *DeepSeekProvider) GetUsage(ctx context.Context) (*UsageStats, error) {
	return &UsageStats{}, nil
}

// GetRateLimit returns rate limit info
func (dp *DeepSeekProvider) GetRateLimit(ctx context.Context) (*RateLimitStatus, error) {
	return &RateLimitStatus{}, nil
}

// Configure changes provider configuration
func (dp *DeepSeekProvider) Configure(config *ProviderConfig) error {
	if config != nil {
		if config.APIKey != "" {
			dp.SetAPIKey(config.APIKey)
		}
		if config.Endpoint != "" {
			dp.SetEndpoint(config.Endpoint)
		}
		if config.Model != "" {
			dp.model = config.Model
		}
		if config.Timeout > 0 {
			dp.SetTimeout(config.Timeout)
		}
	}
	return nil
}

// GetConfiguration returns current provider configuration
func (dp *DeepSeekProvider) GetConfiguration() *ProviderConfig {
	return &ProviderConfig{
		Type:     ProviderDeepSeek,
		Model:    dp.model,
		Endpoint: dp.endpoint,
		APIKey:   dp.apiKey,
		Timeout:  dp.timeout,
	}
}

// Close cuts potential streaming or cleans resources
func (dp *DeepSeekProvider) Close() error {
	return nil
}
