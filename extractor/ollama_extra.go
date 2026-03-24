package extractor

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
)

// GenerateCompletionWithOptions generates text with options
func (op *OllamaProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error) {
	return op.GenerateCompletion(ctx, prompt)
}

// GenerateStructuredOutputWithOptions generates JSON with options
func (op *OllamaProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *CompletionOptions) (interface{}, error) {
	return op.GenerateStructuredOutput(ctx, prompt, schemaStruct)
}

// ExtractEntities extracts entities using basic extractor fallback
func (op *OllamaProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	return nil, fmt.Errorf("not implemented natively - use basic extractor")
}

// ExtractRelationships extracts relationships using basic extractor fallback
func (op *OllamaProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return nil, fmt.Errorf("not implemented natively - use basic extractor")
}

// ExtractWithCustomSchema extracts custom data
func (op *OllamaProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// GenerateWithContext generates text from multiple messages
func (op *OllamaProvider) GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// GenerateStreamingCompletion generates streaming text
func (op *OllamaProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error {
	return fmt.Errorf("not implemented")
}

// GetProviderType returns provider type
func (op *OllamaProvider) GetProviderType() ProviderType {
	return ProviderOllama
}

// GetCapabilities returns provider capabilities
func (op *OllamaProvider) GetCapabilities() ProviderCapabilities {
	caps := GetProviderCapabilitiesMap()[ProviderOllama]
	if caps == nil {
		return ProviderCapabilities{
			SupportsCompletion: true,
			SupportsJSONMode:   true,
		}
	}
	return *caps
}

// GetTokenCount returns token count estimation
func (op *OllamaProvider) GetTokenCount(text string) (int, error) {
	return len(text) / 4, nil
}

// GetMaxTokens returns theoretical context limit
func (op *OllamaProvider) GetMaxTokens() int {
	return 4096
}

// GetUsage returns current usage
func (op *OllamaProvider) GetUsage(ctx context.Context) (*UsageStats, error) {
	return &UsageStats{}, nil
}

// GetRateLimit returns rate limit info
func (op *OllamaProvider) GetRateLimit(ctx context.Context) (*RateLimitStatus, error) {
	return &RateLimitStatus{}, nil
}

// Configure changes provider configuration
func (op *OllamaProvider) Configure(config *ProviderConfig) error {
	if config != nil {
		if config.Endpoint != "" {
			op.SetEndpoint(config.Endpoint)
		}
		if config.Model != "" {
			op.model = config.Model
		}
		if config.Timeout > 0 {
			op.SetTimeout(config.Timeout)
		}
	}
	return nil
}

// GetConfiguration returns current provider configuration
func (op *OllamaProvider) GetConfiguration() *ProviderConfig {
	return &ProviderConfig{
		Type:     ProviderOllama,
		Model:    op.model,
		Endpoint: op.endpoint,
		Timeout:  op.timeout,
	}
}

// Close cleans resources
func (op *OllamaProvider) Close() error {
	return nil
}
