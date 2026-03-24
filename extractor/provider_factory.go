// Package extractor provides factory implementations for creating and managing
// LLM and embedding providers with configuration validation and capability discovery.
package extractor

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// DefaultProviderFactory implements the ProviderFactory interface
type DefaultProviderFactory struct {
	mu              sync.RWMutex
	customProviders map[ProviderType]func(*ProviderConfig) (LLMProvider, error)
	capabilities    map[ProviderType]*ProviderCapabilities
	supportedTypes  []ProviderType
}

// NewProviderFactory creates a new provider factory with default capabilities
func NewProviderFactory() ProviderFactory {
	factory := &DefaultProviderFactory{
		customProviders: make(map[ProviderType]func(*ProviderConfig) (LLMProvider, error)),
		capabilities:    GetProviderCapabilitiesMap(),
		supportedTypes: []ProviderType{
			ProviderOpenAI,
			ProviderAnthropic,
			ProviderGemini,
			ProviderOllama,
			ProviderDeepSeek,
			ProviderMistral,
			ProviderBedrock,
			ProviderAzure,
			ProviderCohere,
			ProviderHuggingFace,
			ProviderLocal,
			ProviderLMStudio,
			ProviderCustom,
		},
	}
	return factory
}

// CreateProvider creates a new provider instance from configuration
func (f *DefaultProviderFactory) CreateProvider(config *ProviderConfig) (LLMProvider, error) {
	if config == nil {
		return nil, NewExtractorError("validation", "provider config is nil", 400)
	}

	// Validate configuration
	if err := f.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check for custom provider
	if createFunc, exists := f.customProviders[config.Type]; exists {
		return createFunc(config)
	}

	// Create built-in provider based on type
	switch config.Type {
	case ProviderOpenAI:
		return f.createOpenAIProvider(config)
	case ProviderAnthropic:
		return f.createAnthropicProvider(config)
	case ProviderGemini:
		return f.createGeminiProvider(config)
	case ProviderOllama:
		return f.createOllamaProvider(config)
	case ProviderDeepSeek:
		return f.createDeepSeekProvider(config)
	case ProviderMistral:
		return f.createMistralProvider(config)
	case ProviderBedrock:
		return f.createBedrockProvider(config)
	case ProviderAzure:
		return f.createAzureProvider(config)
	case ProviderCohere:
		return f.createCohereProvider(config)
	case ProviderHuggingFace:
		return f.createHuggingFaceProvider(config)
	case ProviderLocal:
		return f.createLocalProvider(config)
	case ProviderLMStudio:
		return f.createLMStudioProvider(config)
	case ProviderOpenRouter:
		return f.createOpenRouterProvider(config)
	default:
		return nil, NewExtractorError("unsupported", fmt.Sprintf("unsupported provider type: %s", config.Type), 400)
	}
}

// CreateProviderWithDefaults creates a provider with sensible defaults
func (f *DefaultProviderFactory) CreateProviderWithDefaults(providerType ProviderType, apiKey, model string) (LLMProvider, error) {
	config := DefaultProviderConfig(providerType)
	config.APIKey = apiKey
	if model != "" {
		config.Model = model
	}
	return f.CreateProvider(config)
}

// ListSupportedProviders returns all supported provider types
func (f *DefaultProviderFactory) ListSupportedProviders() []ProviderType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]ProviderType, len(f.supportedTypes))
	copy(result, f.supportedTypes)
	return result
}

// GetProviderCapabilities returns capabilities for a provider type
func (f *DefaultProviderFactory) GetProviderCapabilities(providerType ProviderType) (*ProviderCapabilities, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	caps, exists := f.capabilities[providerType]
	if !exists {
		return nil, NewExtractorError("unsupported", fmt.Sprintf("unsupported provider type: %s", providerType), 400)
	}

	// Return a copy to prevent modification
	capsCopy := *caps
	return &capsCopy, nil
}

// ValidateConfig validates a provider configuration
func (f *DefaultProviderFactory) ValidateConfig(config *ProviderConfig) error {
	return ValidateProviderConfig(config)
}

// GetDefaultConfig returns default configuration for a provider type
func (f *DefaultProviderFactory) GetDefaultConfig(providerType ProviderType) (*ProviderConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, exists := f.capabilities[providerType]; !exists {
		return nil, NewExtractorError("unsupported", fmt.Sprintf("unsupported provider type: %s", providerType), 400)
	}

	return DefaultProviderConfig(providerType), nil
}

// RegisterCustomProvider registers a custom provider implementation
func (f *DefaultProviderFactory) RegisterCustomProvider(providerType ProviderType, createFunc func(*ProviderConfig) (LLMProvider, error)) error {
	if createFunc == nil {
		return NewExtractorError("validation", "create function is nil", 400)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.customProviders[providerType] = createFunc

	// Add to supported types if not already present
	if !slices.Contains(f.supportedTypes, providerType) {
		f.supportedTypes = append(f.supportedTypes, providerType)
	}

	return nil
}

// Provider creation methods
// These create actual provider implementations or mock providers for testing

func (f *DefaultProviderFactory) createOpenAIProvider(config *ProviderConfig) (LLMProvider, error) {
	// Create actual OpenAI provider implementation
	provider, err := NewOpenAIProviderFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI provider: %w", err)
	}
	return provider, nil
}

func (f *DefaultProviderFactory) createAnthropicProvider(config *ProviderConfig) (LLMProvider, error) {
	// Create actual Anthropic provider implementation
	provider := NewAnthropicProvider(config.APIKey, config.Model)

	// Apply additional configuration if needed
	if config.Endpoint != "" {
		provider.SetEndpoint(config.Endpoint)
	}
	if config.Timeout > 0 {
		provider.SetTimeout(config.Timeout)
	}

	return provider, nil
}

func (f *DefaultProviderFactory) createGeminiProvider(config *ProviderConfig) (LLMProvider, error) {
	// Create actual Gemini provider implementation
	provider, err := NewGeminiProviderFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini provider: %w", err)
	}
	return provider, nil
}

func (f *DefaultProviderFactory) createOllamaProvider(config *ProviderConfig) (LLMProvider, error) {
	// Create actual Ollama provider implementation
	provider := NewOllamaProvider(config.Endpoint, config.Model)

	// Apply additional configuration if needed
	if config.Timeout > 0 {
		provider.SetTimeout(config.Timeout)
	}

	return provider, nil
}

func (f *DefaultProviderFactory) createDeepSeekProvider(config *ProviderConfig) (LLMProvider, error) {
	// Create actual DeepSeek provider implementation
	provider := NewDeepSeekProvider(config.APIKey, config.Model)

	// Apply additional configuration if needed
	if config.Endpoint != "" {
		provider.SetEndpoint(config.Endpoint)
	}
	if config.Timeout > 0 {
		provider.SetTimeout(config.Timeout)
	}

	return provider, nil
}

func (f *DefaultProviderFactory) createMistralProvider(config *ProviderConfig) (LLMProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Mistral provider implementation
	provider := newConfiguredMockLLMProvider(ProviderMistral, config)
	return provider, nil
}

func (f *DefaultProviderFactory) createBedrockProvider(config *ProviderConfig) (LLMProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Bedrock provider implementation
	provider := newConfiguredMockLLMProvider(ProviderBedrock, config)
	return provider, nil
}

func (f *DefaultProviderFactory) createAzureProvider(config *ProviderConfig) (LLMProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Azure provider implementation
	provider := newConfiguredMockLLMProvider(ProviderAzure, config)
	return provider, nil
}

func (f *DefaultProviderFactory) createCohereProvider(config *ProviderConfig) (LLMProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Cohere provider implementation
	provider := newConfiguredMockLLMProvider(ProviderCohere, config)
	return provider, nil
}

func (f *DefaultProviderFactory) createHuggingFaceProvider(config *ProviderConfig) (LLMProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual HuggingFace provider implementation
	provider := newConfiguredMockLLMProvider(ProviderHuggingFace, config)
	return provider, nil
}

func (f *DefaultProviderFactory) createLocalProvider(config *ProviderConfig) (LLMProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Local provider implementation
	provider := newConfiguredMockLLMProvider(ProviderLocal, config)
	return provider, nil
}

func (f *DefaultProviderFactory) createLMStudioProvider(config *ProviderConfig) (LLMProvider, error) {
	provider, err := NewLMStudioProvider(config.Endpoint, config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create LM Studio provider: %w", err)
	}
	return provider, nil
}

func (f *DefaultProviderFactory) createOpenRouterProvider(config *ProviderConfig) (LLMProvider, error) {
	siteURL := ""
	appName := ""
	if config.CustomOptions != nil {
		if s, ok := config.CustomOptions["site_url"].(string); ok {
			siteURL = s
		}
		if a, ok := config.CustomOptions["app_name"].(string); ok {
			appName = a
		}
	}
	provider, err := NewOpenRouterProvider(config.APIKey, config.Model, siteURL, appName)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenRouter provider: %w", err)
	}
	return provider, nil
}


// DefaultEmbeddingProviderFactory implements the EmbeddingProviderFactory interface
type DefaultEmbeddingProviderFactory struct {
	mu              sync.RWMutex
	customProviders map[EmbeddingProviderType]func(*EmbeddingProviderConfig) (EmbeddingProvider, error)
	capabilities    map[EmbeddingProviderType]*EmbeddingProviderCapabilities
	supportedTypes  []EmbeddingProviderType
}

// NewEmbeddingProviderFactory creates a new embedding provider factory
func NewEmbeddingProviderFactory() EmbeddingProviderFactory {
	factory := &DefaultEmbeddingProviderFactory{
		customProviders: make(map[EmbeddingProviderType]func(*EmbeddingProviderConfig) (EmbeddingProvider, error)),
		capabilities:    GetEmbeddingProviderCapabilitiesMap(),
		supportedTypes: []EmbeddingProviderType{
			EmbeddingProviderOpenAI,
			EmbeddingProviderOllama,
			EmbeddingProviderLocal,
			EmbeddingProviderSentenceTransform,
			EmbeddingProviderHuggingFace,
			EmbeddingProviderCohere,
			EmbeddingProviderAzure,
			EmbeddingProviderBedrock,
			EmbeddingProviderVertex,
			EmbeddingProviderGemini,
			EmbeddingProviderLMStudio,
			EmbeddingProviderCustom,
		},
	}
	return factory
}

// CreateProvider creates a new embedding provider instance from configuration
func (f *DefaultEmbeddingProviderFactory) CreateProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	if config == nil {
		return nil, NewExtractorError("validation", "embedding provider config is nil", 400)
	}

	// Validate configuration
	if err := f.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid embedding provider config: %w", err)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check for custom provider
	if createFunc, exists := f.customProviders[config.Type]; exists {
		return createFunc(config)
	}

	// Create built-in provider based on type
	switch config.Type {
	case EmbeddingProviderOpenAI:
		return f.createOpenAIEmbeddingProvider(config)
	case EmbeddingProviderOllama:
		return f.createOllamaEmbeddingProvider(config)
	case EmbeddingProviderLocal:
		return f.createLocalEmbeddingProvider(config)
	case EmbeddingProviderSentenceTransform:
		return f.createSentenceTransformProvider(config)
	case EmbeddingProviderHuggingFace:
		return f.createHuggingFaceEmbeddingProvider(config)
	case EmbeddingProviderCohere:
		return f.createCohereEmbeddingProvider(config)
	case EmbeddingProviderAzure:
		return f.createAzureEmbeddingProvider(config)
	case EmbeddingProviderBedrock:
		return f.createBedrockEmbeddingProvider(config)
	case EmbeddingProviderVertex:
		return f.createVertexEmbeddingProvider(config)
	case EmbeddingProviderGemini:
		return f.createGeminiEmbeddingProvider(config)
	case EmbeddingProviderLMStudio:
		return f.createLMStudioEmbeddingProvider(config)
	default:
		return nil, NewExtractorError("unsupported", fmt.Sprintf("unsupported embedding provider type: %s", config.Type), 400)
	}
}

// CreateProviderWithDefaults creates an embedding provider with sensible defaults
func (f *DefaultEmbeddingProviderFactory) CreateProviderWithDefaults(providerType EmbeddingProviderType, apiKey, model string) (EmbeddingProvider, error) {
	config := DefaultEmbeddingProviderConfig(providerType)
	config.APIKey = apiKey
	if model != "" {
		config.Model = model
	}
	return f.CreateProvider(config)
}

// ListSupportedProviders returns all supported embedding provider types
func (f *DefaultEmbeddingProviderFactory) ListSupportedProviders() []EmbeddingProviderType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]EmbeddingProviderType, len(f.supportedTypes))
	copy(result, f.supportedTypes)
	return result
}

// GetProviderCapabilities returns capabilities for an embedding provider type
func (f *DefaultEmbeddingProviderFactory) GetProviderCapabilities(providerType EmbeddingProviderType) (*EmbeddingProviderCapabilities, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	caps, exists := f.capabilities[providerType]
	if !exists {
		return nil, NewExtractorError("unsupported", fmt.Sprintf("unsupported embedding provider type: %s", providerType), 400)
	}

	// Return a copy to prevent modification
	capsCopy := *caps
	return &capsCopy, nil
}

// ValidateConfig validates an embedding provider configuration
func (f *DefaultEmbeddingProviderFactory) ValidateConfig(config *EmbeddingProviderConfig) error {
	return ValidateEmbeddingProviderConfig(config)
}

// GetDefaultConfig returns default configuration for an embedding provider type
func (f *DefaultEmbeddingProviderFactory) GetDefaultConfig(providerType EmbeddingProviderType) (*EmbeddingProviderConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, exists := f.capabilities[providerType]; !exists {
		return nil, NewExtractorError("unsupported", fmt.Sprintf("unsupported embedding provider type: %s", providerType), 400)
	}

	return DefaultEmbeddingProviderConfig(providerType), nil
}

// RegisterCustomProvider registers a custom embedding provider implementation
func (f *DefaultEmbeddingProviderFactory) RegisterCustomProvider(providerType EmbeddingProviderType, createFunc func(*EmbeddingProviderConfig) (EmbeddingProvider, error)) error {
	if createFunc == nil {
		return NewExtractorError("validation", "create function is nil", 400)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.customProviders[providerType] = createFunc

	// Add to supported types if not already present
	if !slices.Contains(f.supportedTypes, providerType) {
		f.supportedTypes = append(f.supportedTypes, providerType)
	}

	return nil
}

// GetSupportedModels returns supported models for a provider type
func (f *DefaultEmbeddingProviderFactory) GetSupportedModels(providerType EmbeddingProviderType) ([]string, error) {
	caps, err := f.GetProviderCapabilities(providerType)
	if err != nil {
		return nil, err
	}
	return caps.SupportedModels, nil
}

// EstimateProviderCost estimates cost for embedding generation with a provider
func (f *DefaultEmbeddingProviderFactory) EstimateProviderCost(providerType EmbeddingProviderType, tokenCount int) (float64, error) {
	caps, err := f.GetProviderCapabilities(providerType)
	if err != nil {
		return 0, err
	}

	if caps.CostPerToken <= 0 {
		return 0, NewExtractorError("unsupported", fmt.Sprintf("cost estimation not available for provider: %s", providerType), 400)
	}

	return caps.CostPerToken * float64(tokenCount), nil
}

// Embedding provider creation methods
// These create actual embedding provider implementations or mock providers for testing

func (f *DefaultEmbeddingProviderFactory) createOpenAIEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// Create actual OpenAI embedding provider implementation
	provider, err := NewOpenAIEmbeddingProviderFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI embedding provider: %w", err)
	}
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createOllamaEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Ollama embedding provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createLocalEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Local embedding provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createSentenceTransformProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Sentence Transformers provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createHuggingFaceEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual HuggingFace embedding provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createCohereEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Cohere embedding provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createAzureEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Azure embedding provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createBedrockEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Bedrock embedding provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createVertexEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// For now, create a configured mock provider
	// TODO: Replace with actual Vertex embedding provider implementation
	provider := newConfiguredMockEmbeddingProvider(config)
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createGeminiEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	// Create actual Gemini embedding provider implementation
	provider, err := NewGeminiEmbeddingProviderFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini embedding provider: %w", err)
	}
	return provider, nil
}

func (f *DefaultEmbeddingProviderFactory) createLMStudioEmbeddingProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
	provider, err := NewLMStudioEmbeddingProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create LM Studio embedding provider: %w", err)
	}
	return provider, nil
}

// Helper functions for creating configured mock providers
// These will be replaced with actual provider implementations

// newConfiguredMockLLMProvider creates a mock LLM provider with proper configuration
func newConfiguredMockLLMProvider(providerType ProviderType, config *ProviderConfig) LLMProvider {
	// Create a basic mock provider that respects the configuration
	provider := &ConfiguredMockLLMProvider{
		providerType: providerType,
		model:        config.Model,
		config:       config,
		isHealthy:    true,
		metrics: &ProviderMetrics{
			FirstRequest: time.Now(),
		},
	}
	return provider
}

// newConfiguredMockEmbeddingProvider creates a mock embedding provider with proper configuration
func newConfiguredMockEmbeddingProvider(config *EmbeddingProviderConfig) EmbeddingProvider {
	// Create a basic mock provider that respects the configuration
	provider := &ConfiguredMockEmbeddingProvider{
		providerType: config.Type,
		model:        config.Model,
		dimensions:   config.Dimensions,
		config:       config,
		isHealthy:    true,
		metrics: &EmbeddingProviderMetrics{
			FirstRequest: time.Now(),
		},
	}
	return provider
}

// ConfiguredMockLLMProvider is a mock LLM provider that respects configuration
type ConfiguredMockLLMProvider struct {
	providerType ProviderType
	model        string
	config       *ProviderConfig
	isHealthy    bool
	metrics      *ProviderMetrics
	mu           sync.RWMutex
}

// Implement LLMProvider interface
func (p *ConfiguredMockLLMProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	p.mu.Lock()
	p.metrics.TotalRequests++
	p.mu.Unlock()

	// Simulate a basic completion
	return fmt.Sprintf("Mock completion from %s model %s for prompt: %s", p.providerType, p.model, prompt), nil
}

func (p *ConfiguredMockLLMProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error) {
	return p.GenerateCompletion(ctx, prompt)
}

func (p *ConfiguredMockLLMProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schema interface{}) (interface{}, error) {
	// Return mock structured output
	return map[string]interface{}{
		"entities": []map[string]interface{}{
			{"name": "MockEntity", "type": "Concept"},
		},
	}, nil
}

func (p *ConfiguredMockLLMProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schema interface{}, options *CompletionOptions) (interface{}, error) {
	return p.GenerateStructuredOutput(ctx, prompt, schema)
}

func (p *ConfiguredMockLLMProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	p.mu.Lock()
	p.metrics.TotalRequests++
	p.mu.Unlock()

	// Return mock entities
	return []schema.Node{
		{
			ID:   "mock-entity-1",
			Type: "Concept",
			Properties: map[string]interface{}{
				"name":       "MockEntity",
				"confidence": 0.95,
			},
		},
	}, nil
}

func (p *ConfiguredMockLLMProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	p.mu.Lock()
	p.metrics.TotalRequests++
	p.mu.Unlock()

	// Return mock relationships
	return []schema.Edge{
		{
			ID:   "mock-edge-1",
			From: "entity-1",
			To:   "entity-2",
			Type: "RELATED_TO",
			Properties: map[string]interface{}{
				"confidence": 0.9,
			},
		},
	}, nil
}

func (p *ConfiguredMockLLMProvider) ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error) {
	return p.GenerateStructuredOutput(ctx, text, jsonSchema)
}

func (p *ConfiguredMockLLMProvider) GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error) {
	// Combine messages into a single prompt
	var prompt strings.Builder
	for _, msg := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return p.GenerateCompletion(ctx, prompt.String())
}

func (p *ConfiguredMockLLMProvider) GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error {
	completion, err := p.GenerateCompletion(ctx, prompt)
	if err != nil {
		return err
	}

	// Simulate streaming by calling callback with chunks
	words := strings.Fields(completion)
	for _, word := range words {
		callback(word+" ", false, nil)
	}
	callback("", true, nil) // Signal completion
	return nil
}

func (p *ConfiguredMockLLMProvider) GetModel() string {
	return p.model
}

func (p *ConfiguredMockLLMProvider) SetModel(model string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.model = model
	return nil
}

func (p *ConfiguredMockLLMProvider) GetProviderType() ProviderType {
	return p.providerType
}

func (p *ConfiguredMockLLMProvider) GetCapabilities() ProviderCapabilities {
	caps := GetProviderCapabilitiesMap()[p.providerType]
	if caps == nil {
		return ProviderCapabilities{
			SupportsCompletion: true,
			SupportsStreaming:  true,
		}
	}
	return *caps
}

func (p *ConfiguredMockLLMProvider) GetTokenCount(text string) (int, error) {
	// Simple token estimation: ~4 characters per token
	return len(text) / 4, nil
}

func (p *ConfiguredMockLLMProvider) GetMaxTokens() int {
	return 4096 // Default max tokens
}

func (p *ConfiguredMockLLMProvider) Health(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.isHealthy {
		return NewExtractorError("health_check", "provider is unhealthy", 503)
	}
	return nil
}

func (p *ConfiguredMockLLMProvider) GetUsage(ctx context.Context) (*UsageStats, error) {
	return &UsageStats{
		TotalRequests: p.metrics.TotalRequests,
	}, nil
}

func (p *ConfiguredMockLLMProvider) GetRateLimit(ctx context.Context) (*RateLimitStatus, error) {
	return &RateLimitStatus{
		RequestsRemaining: 1000,
		RequestsPerMinute: 1000,
	}, nil
}

func (p *ConfiguredMockLLMProvider) Configure(config *ProviderConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := ValidateProviderConfig(config); err != nil {
		return err
	}

	p.config = config
	p.model = config.Model
	return nil
}

func (p *ConfiguredMockLLMProvider) GetConfiguration() *ProviderConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to prevent modification
	configCopy := *p.config
	return &configCopy
}

func (p *ConfiguredMockLLMProvider) Close() error {
	// Nothing to close for mock provider
	return nil
}

// ConfiguredMockEmbeddingProvider is a mock embedding provider that respects configuration
type ConfiguredMockEmbeddingProvider struct {
	providerType EmbeddingProviderType
	model        string
	dimensions   int
	config       *EmbeddingProviderConfig
	isHealthy    bool
	metrics      *EmbeddingProviderMetrics
	mu           sync.RWMutex
}

// Implement EmbeddingProvider interface
func (p *ConfiguredMockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	p.mu.Lock()
	p.metrics.TotalRequests++
	p.mu.Unlock()

	// Generate mock embedding with correct dimensions
	embedding := make([]float32, p.dimensions)
	for i := range embedding {
		embedding[i] = rand.Float32()*2 - 1 // Random values between -1 and 1
	}
	return embedding, nil
}

func (p *ConfiguredMockEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	p.mu.Lock()
	p.metrics.TotalRequests++
	p.metrics.TotalTokensUsed += int64(len(texts) * 10) // Estimate 10 tokens per text
	p.mu.Unlock()

	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embedding, err := p.GenerateEmbedding(ctx, texts[i])
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (p *ConfiguredMockEmbeddingProvider) GenerateEmbeddingWithOptions(ctx context.Context, text string, options *EmbeddingOptions) ([]float32, error) {
	return p.GenerateEmbedding(ctx, text)
}

func (p *ConfiguredMockEmbeddingProvider) GenerateBatchEmbeddingsWithOptions(ctx context.Context, texts []string, options *EmbeddingOptions) ([][]float32, error) {
	return p.GenerateBatchEmbeddings(ctx, texts)
}

func (p *ConfiguredMockEmbeddingProvider) GetDimensions() int {
	return p.dimensions
}

func (p *ConfiguredMockEmbeddingProvider) GetModel() string {
	return p.model
}

func (p *ConfiguredMockEmbeddingProvider) SetModel(model string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.model = model
	return nil
}

func (p *ConfiguredMockEmbeddingProvider) GetProviderType() EmbeddingProviderType {
	return p.providerType
}

func (p *ConfiguredMockEmbeddingProvider) GetSupportedModels() []string {
	caps := p.GetCapabilities()
	return caps.SupportedModels
}

func (p *ConfiguredMockEmbeddingProvider) GetMaxBatchSize() int {
	return 100 // Default batch size
}

func (p *ConfiguredMockEmbeddingProvider) GetMaxTokensPerText() int {
	return 8192 // Default max tokens
}

func (p *ConfiguredMockEmbeddingProvider) GenerateEmbeddingCached(ctx context.Context, text string, ttl time.Duration) ([]float32, error) {
	// For mock, just generate without caching
	return p.GenerateEmbedding(ctx, text)
}

func (p *ConfiguredMockEmbeddingProvider) GenerateBatchEmbeddingsCached(ctx context.Context, texts []string, ttl time.Duration) ([][]float32, error) {
	// For mock, just generate without caching
	return p.GenerateBatchEmbeddings(ctx, texts)
}

func (p *ConfiguredMockEmbeddingProvider) DeduplicateAndEmbed(ctx context.Context, texts []string) (map[string][]float32, error) {
	// Remove duplicates and return map
	result := make(map[string][]float32)
	seen := make(map[string]bool)

	for _, text := range texts {
		if !seen[text] {
			embedding, err := p.GenerateEmbedding(ctx, text)
			if err != nil {
				return nil, err
			}
			result[text] = embedding
			seen[text] = true
		}
	}

	return result, nil
}

func (p *ConfiguredMockEmbeddingProvider) EstimateTokenCount(text string) (int, error) {
	// Simple token estimation: ~4 characters per token
	return len(text) / 4, nil
}

func (p *ConfiguredMockEmbeddingProvider) EstimateCost(tokenCount int) (float64, error) {
	caps := p.GetCapabilities()
	return caps.CostPerToken * float64(tokenCount), nil
}

func (p *ConfiguredMockEmbeddingProvider) Health(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.isHealthy {
		return NewExtractorError("health_check", "embedding provider is unhealthy", 503)
	}
	return nil
}

func (p *ConfiguredMockEmbeddingProvider) GetUsage(ctx context.Context) (*EmbeddingUsageStats, error) {
	return &EmbeddingUsageStats{
		TotalRequests:   p.metrics.TotalRequests,
		TotalTokensUsed: p.metrics.TotalTokensUsed,
	}, nil
}

func (p *ConfiguredMockEmbeddingProvider) GetRateLimit(ctx context.Context) (*EmbeddingRateLimitStatus, error) {
	return &EmbeddingRateLimitStatus{
		RequestsRemaining: 1000,
		RequestsPerMinute: 1000,
	}, nil
}

func (p *ConfiguredMockEmbeddingProvider) Configure(config *EmbeddingProviderConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := ValidateEmbeddingProviderConfig(config); err != nil {
		return err
	}

	p.config = config
	p.model = config.Model
	p.dimensions = config.Dimensions
	return nil
}

func (p *ConfiguredMockEmbeddingProvider) GetConfiguration() *EmbeddingProviderConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to prevent modification
	configCopy := *p.config
	return &configCopy
}

func (p *ConfiguredMockEmbeddingProvider) ValidateConfiguration(config *EmbeddingProviderConfig) error {
	return ValidateEmbeddingProviderConfig(config)
}

func (p *ConfiguredMockEmbeddingProvider) Close() error {
	// Nothing to close for mock provider
	return nil
}

func (p *ConfiguredMockEmbeddingProvider) SupportsStreaming() bool {
	return false // Mock provider doesn't support streaming
}

func (p *ConfiguredMockEmbeddingProvider) GenerateStreamingEmbedding(ctx context.Context, text string, callback EmbeddingStreamCallback) error {
	// Not supported by mock provider
	return NewExtractorError("unsupported", "streaming not supported by mock provider", 501)
}

func (p *ConfiguredMockEmbeddingProvider) GetCapabilities() *EmbeddingProviderCapabilities {
	caps := GetEmbeddingProviderCapabilitiesMap()[p.providerType]
	if caps == nil {
		return &EmbeddingProviderCapabilities{
			SupportsBatching:   true,
			SupportsCustomDims: true,
			MaxBatchSize:       100,
			MaxTokensPerText:   8192,
			DefaultDimension:   p.dimensions,
		}
	}
	return caps
}

func (p *ConfiguredMockEmbeddingProvider) SetCustomDimensions(dimensions int) error {
	if dimensions <= 0 {
		return NewExtractorError("validation", "dimensions must be positive", 400)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.dimensions = dimensions
	return nil
}

func (p *ConfiguredMockEmbeddingProvider) SupportsCustomDimensions() bool {
	return true // Mock provider supports custom dimensions
}
