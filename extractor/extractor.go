// Package extractor provides LLM bridge implementation for entity and relationship extraction.
// It supports multiple LLM providers (OpenAI, Anthropic, Gemini, Ollama, DeepSeek) with
// JSON Schema Mode for structured output generation.
package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// LLMProvider defines the standardized interface for LLM interactions across multiple providers.
// This interface supports text completion, structured output generation with JSON schema,
// entity extraction, relationship detection, context management, and health monitoring.
type LLMProvider interface {
	// Core text generation methods

	// GenerateCompletion generates a text completion from a prompt
	GenerateCompletion(ctx context.Context, prompt string) (string, error)

	// GenerateCompletionWithOptions generates a text completion with custom options
	GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error)

	// GenerateStructuredOutput generates structured output using JSON schema mode
	GenerateStructuredOutput(ctx context.Context, prompt string, schema interface{}) (interface{}, error)

	// GenerateStructuredOutputWithOptions generates structured output with custom options
	GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schema interface{}, options *CompletionOptions) (interface{}, error)

	// Entity and relationship extraction methods

	// ExtractEntities extracts entities from text using the provider's capabilities
	ExtractEntities(ctx context.Context, text string) ([]schema.Node, error)

	// ExtractRelationships detects relationships between entities in text
	ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error)

	// ExtractWithCustomSchema extracts data using a custom JSON schema
	ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error)

	// Context and conversation management

	// GenerateWithContext generates completion with conversation context
	GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error)

	// GenerateStreamingCompletion generates streaming text completion
	GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error

	// Configuration and metadata methods

	// GetModel returns the current model name being used
	GetModel() string

	// SetModel sets the model to use (if supported by provider)
	SetModel(model string) error

	// GetProviderType returns the provider type (openai, anthropic, etc.)
	GetProviderType() ProviderType

	// GetCapabilities returns the capabilities supported by this provider
	GetCapabilities() ProviderCapabilities

	// GetTokenCount estimates token count for text (if supported)
	GetTokenCount(text string) (int, error)

	// GetMaxTokens returns the maximum token limit for this provider/model
	GetMaxTokens() int

	// Health and monitoring methods

	// Health checks if the provider is available and responsive
	Health(ctx context.Context) error

	// GetUsage returns usage statistics (if available)
	GetUsage(ctx context.Context) (*UsageStats, error)

	// GetRateLimit returns current rate limit status (if available)
	GetRateLimit(ctx context.Context) (*RateLimitStatus, error)

	// Configuration methods

	// Configure updates provider configuration
	Configure(config *ProviderConfig) error

	// GetConfiguration returns current provider configuration
	GetConfiguration() *ProviderConfig

	// Lifecycle methods

	// Close closes the provider and cleans up resources
	Close() error
}

// LLMExtractor defines the interface for entity and relationship extraction
type LLMExtractor interface {
	// ExtractEntities extracts entities from text content
	ExtractEntities(ctx context.Context, text string) ([]schema.Node, error)

	// ExtractRelationships detects relationships between entities
	ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error)

	// ExtractBridgingRelationship creates a direct relationship summarizing an LLM's multi-hop reasoning sequence
	ExtractBridgingRelationship(ctx context.Context, question string, answer string) (*schema.Edge, error)

	// CompareEntities compares a new entity against an existing similar entity and determines consistency action
	CompareEntities(ctx context.Context, existing schema.Node, newEntity schema.Node) (*schema.ConsistencyResult, error)

	// ExtractWithSchema extracts structured data using a Go struct schema
	ExtractWithSchema(ctx context.Context, text string, schemaStruct interface{}) (interface{}, error)

	// SetProvider sets the LLM provider to use
	SetProvider(provider LLMProvider)

	// GetProvider returns the current LLM provider
	GetProvider() LLMProvider
}

// CompletionOptions configures text generation behavior
type CompletionOptions struct {
	// Temperature controls randomness (0.0 to 2.0, typically 0.0-1.0)
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTokens limits the response length
	MaxTokens int `json:"max_tokens,omitempty"`

	// TopP controls nucleus sampling (0.0 to 1.0)
	TopP float64 `json:"top_p,omitempty"`

	// TopK controls top-k sampling (for providers that support it)
	TopK int `json:"top_k,omitempty"`

	// FrequencyPenalty reduces repetition (-2.0 to 2.0)
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`

	// PresencePenalty encourages new topics (-2.0 to 2.0)
	PresencePenalty float64 `json:"presence_penalty,omitempty"`

	// Stop sequences to halt generation
	Stop []string `json:"stop,omitempty"`

	// Seed for deterministic generation (if supported)
	Seed *int64 `json:"seed,omitempty"`

	// ResponseFormat specifies output format (json, text)
	ResponseFormat string `json:"response_format,omitempty"`

	// SystemPrompt for providers that support system messages
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Timeout for the request
	Timeout time.Duration `json:"timeout,omitempty"`

	// RetryAttempts for failed requests
	RetryAttempts int `json:"retry_attempts,omitempty"`

	// Custom provider-specific options
	CustomOptions map[string]interface{} `json:"custom_options,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role     MessageRole            `json:"role"`
	Content  string                 `json:"content"`
	Name     string                 `json:"name,omitempty"` // For function/tool messages
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MessageRole defines the role of a message in conversation
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleFunction  MessageRole = "function"
	RoleTool      MessageRole = "tool"
)

// StreamCallback is called for each chunk in streaming completion
type StreamCallback func(chunk string, done bool, err error)

// ProviderCapabilities describes what features a provider supports
type ProviderCapabilities struct {
	// Text generation capabilities
	SupportsCompletion      bool `json:"supports_completion"`
	SupportsChat            bool `json:"supports_chat"`
	SupportsStreaming       bool `json:"supports_streaming"`
	SupportsJSONMode        bool `json:"supports_json_mode"`
	SupportsJSONSchema      bool `json:"supports_json_schema"`
	SupportsFunctionCalling bool `json:"supports_function_calling"`

	// Context and memory capabilities
	SupportsSystemPrompts bool `json:"supports_system_prompts"`
	SupportsConversation  bool `json:"supports_conversation"`
	MaxContextLength      int  `json:"max_context_length"`

	// Advanced features
	SupportsEmbeddings     bool `json:"supports_embeddings"`
	SupportsImageInput     bool `json:"supports_image_input"`
	SupportsAudioInput     bool `json:"supports_audio_input"`
	SupportsCodeGeneration bool `json:"supports_code_generation"`

	// Reliability features
	SupportsRetries       bool `json:"supports_retries"`
	SupportsRateLimiting  bool `json:"supports_rate_limiting"`
	SupportsUsageTracking bool `json:"supports_usage_tracking"`

	// Model information
	AvailableModels []string `json:"available_models"`
	DefaultModel    string   `json:"default_model"`

	// Pricing and limits (if available)
	CostPerToken float64 `json:"cost_per_token,omitempty"`
	RateLimitRPM int     `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM int     `json:"rate_limit_tpm,omitempty"`
}

// UsageStats tracks provider usage statistics
type UsageStats struct {
	// Token usage
	TotalTokensUsed      int64 `json:"total_tokens_used"`
	PromptTokensUsed     int64 `json:"prompt_tokens_used"`
	CompletionTokensUsed int64 `json:"completion_tokens_used"`

	// Request statistics
	TotalRequests      int64 `json:"total_requests"`
	SuccessfulRequests int64 `json:"successful_requests"`
	FailedRequests     int64 `json:"failed_requests"`

	// Performance metrics
	AverageLatency      time.Duration `json:"average_latency"`
	AverageTokensPerSec float64       `json:"average_tokens_per_sec"`

	// Cost tracking (if available)
	EstimatedCost float64 `json:"estimated_cost,omitempty"`

	// Time period
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// Provider-specific metrics
	CustomMetrics map[string]interface{} `json:"custom_metrics,omitempty"`
}

// RateLimitStatus provides current rate limiting information
type RateLimitStatus struct {
	// Current limits
	RequestsPerMinute int `json:"requests_per_minute"`
	TokensPerMinute   int `json:"tokens_per_minute"`
	RequestsPerDay    int `json:"requests_per_day,omitempty"`

	// Current usage
	RequestsUsed      int `json:"requests_used"`
	TokensUsed        int `json:"tokens_used"`
	RequestsUsedToday int `json:"requests_used_today,omitempty"`

	// Remaining capacity
	RequestsRemaining int `json:"requests_remaining"`
	TokensRemaining   int `json:"tokens_remaining"`

	// Reset timing
	ResetTime          time.Time     `json:"reset_time"`
	ResetTimeRemaining time.Duration `json:"reset_time_remaining"`

	// Status flags
	IsLimited  bool          `json:"is_limited"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
}

// ProviderType defines supported LLM provider types
type ProviderType string

const (
	ProviderOpenAI      ProviderType = "openai"
	ProviderAnthropic   ProviderType = "anthropic"
	ProviderGemini      ProviderType = "gemini"
	ProviderOllama      ProviderType = "ollama"
	ProviderDeepSeek    ProviderType = "deepseek"
	ProviderMistral     ProviderType = "mistral"
	ProviderBedrock     ProviderType = "bedrock"
	ProviderAzure       ProviderType = "azure"
	ProviderCohere      ProviderType = "cohere"
	ProviderHuggingFace ProviderType = "huggingface"
	ProviderLocal       ProviderType = "local"
	ProviderLMStudio    ProviderType = "lmstudio"
	ProviderOpenRouter  ProviderType = "openrouter"
	ProviderCustom      ProviderType = "custom"
)

// EmbeddingProviderType defines supported embedding provider types
type EmbeddingProviderType string

const (
	EmbeddingProviderOpenAI            EmbeddingProviderType = "openai"
	EmbeddingProviderOllama            EmbeddingProviderType = "ollama"
	EmbeddingProviderLocal             EmbeddingProviderType = "local"
	EmbeddingProviderSentenceTransform EmbeddingProviderType = "sentence_transformers"
	EmbeddingProviderHuggingFace       EmbeddingProviderType = "huggingface"
	EmbeddingProviderCohere            EmbeddingProviderType = "cohere"
	EmbeddingProviderAzure             EmbeddingProviderType = "azure"
	EmbeddingProviderBedrock           EmbeddingProviderType = "bedrock"
	EmbeddingProviderVertex            EmbeddingProviderType = "vertex"
	EmbeddingProviderGemini            EmbeddingProviderType = "gemini"
	EmbeddingProviderLMStudio          EmbeddingProviderType = "lmstudio"
	EmbeddingProviderCustom            EmbeddingProviderType = "custom"
)

// EmbeddingOptions configures embedding generation behavior
type EmbeddingOptions struct {
	// Model-specific options
	Model      string `json:"model,omitempty"`
	Dimensions int    `json:"dimensions,omitempty"`

	// Processing options
	Normalize      bool   `json:"normalize,omitempty"`       // Normalize embeddings to unit length
	Truncate       bool   `json:"truncate,omitempty"`        // Truncate input if too long
	EncodingFormat string `json:"encoding_format,omitempty"` // float, base64, etc.

	// Performance options
	BatchSize  int           `json:"batch_size,omitempty"`
	Timeout    time.Duration `json:"timeout,omitempty"`
	MaxRetries int           `json:"max_retries,omitempty"`
	RetryDelay time.Duration `json:"retry_delay,omitempty"`

	// Caching options
	EnableCaching bool          `json:"enable_caching,omitempty"`
	CacheTTL      time.Duration `json:"cache_ttl,omitempty"`

	// Provider-specific options
	CustomOptions map[string]interface{} `json:"custom_options,omitempty"`
}

// DefaultEmbeddingOptions returns sensible defaults for embedding options
func DefaultEmbeddingOptions() *EmbeddingOptions {
	return &EmbeddingOptions{
		Normalize:     true,
		Truncate:      true,
		BatchSize:     100,
		Timeout:       60 * time.Second,
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
		EnableCaching: true,
		CacheTTL:      24 * time.Hour,
		CustomOptions: make(map[string]interface{}),
	}
}

// EmbeddingProviderConfig holds comprehensive configuration for embedding providers
type EmbeddingProviderConfig struct {
	// Basic provider information
	Type     EmbeddingProviderType `json:"type"`
	Name     string                `json:"name,omitempty"` // Custom name for this provider instance
	Model    string                `json:"model"`
	Endpoint string                `json:"endpoint,omitempty"`

	// Authentication
	APIKey    string `json:"api_key,omitempty"`
	APISecret string `json:"api_secret,omitempty"`
	Token     string `json:"token,omitempty"`

	// Model configuration
	Dimensions       int      `json:"dimensions,omitempty"`
	MaxTokensPerText int      `json:"max_tokens_per_text,omitempty"`
	MaxBatchSize     int      `json:"max_batch_size,omitempty"`
	SupportedModels  []string `json:"supported_models,omitempty"`

	// Performance settings
	DefaultOptions *EmbeddingOptions `json:"default_options,omitempty"`
	Timeout        time.Duration     `json:"timeout,omitempty"`

	// Rate limiting configuration
	RateLimit struct {
		RequestsPerMinute int           `json:"requests_per_minute,omitempty"`
		TokensPerMinute   int           `json:"tokens_per_minute,omitempty"`
		BurstSize         int           `json:"burst_size,omitempty"`
		RetryAttempts     int           `json:"retry_attempts,omitempty"`
		RetryDelay        time.Duration `json:"retry_delay,omitempty"`
		BackoffMultiplier float64       `json:"backoff_multiplier,omitempty"`
	} `json:"rate_limit,omitempty"`

	// Caching configuration
	Cache struct {
		Enabled    bool          `json:"enabled"`
		DefaultTTL time.Duration `json:"default_ttl,omitempty"`
		MaxSize    int           `json:"max_size,omitempty"`
	} `json:"cache,omitempty"`

	// Health check configuration
	HealthCheck struct {
		Enabled  bool          `json:"enabled"`
		Interval time.Duration `json:"interval,omitempty"`
		Timeout  time.Duration `json:"timeout,omitempty"`
	} `json:"health_check,omitempty"`

	// Feature flags
	Features struct {
		EnableBatching         bool `json:"enable_batching"`
		EnableCaching          bool `json:"enable_caching"`
		EnableDeduplication    bool `json:"enable_deduplication"`
		EnableStreaming        bool `json:"enable_streaming"`
		EnableCustomDimensions bool `json:"enable_custom_dimensions"`
		EnableUsageTracking    bool `json:"enable_usage_tracking"`
	} `json:"features,omitempty"`

	// Provider-specific options
	CustomOptions map[string]interface{} `json:"custom_options,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Version   string    `json:"version,omitempty"`
}

// DefaultEmbeddingProviderConfig returns default configuration for a provider type
func DefaultEmbeddingProviderConfig(providerType EmbeddingProviderType) *EmbeddingProviderConfig {
	config := &EmbeddingProviderConfig{
		Type:           providerType,
		Timeout:        60 * time.Second,
		DefaultOptions: DefaultEmbeddingOptions(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Version:        "1.0",
		CustomOptions:  make(map[string]interface{}),
	}

	// Set provider-specific defaults
	switch providerType {
	case EmbeddingProviderOpenAI:
		config.Model = "text-embedding-3-small"
		config.Endpoint = "https://api.openai.com/v1/embeddings"
		config.Dimensions = 1536
		config.MaxTokensPerText = 8192
		config.MaxBatchSize = 2048
		config.SupportedModels = []string{
			"text-embedding-3-small",
			"text-embedding-3-large",
			"text-embedding-ada-002",
		}
		config.Features.EnableBatching = true
		config.Features.EnableCaching = true
		config.Features.EnableDeduplication = true
		config.Features.EnableCustomDimensions = true
		config.Features.EnableUsageTracking = true

	case EmbeddingProviderOllama:
		config.Model = "nomic-embed-text"
		config.Endpoint = "http://localhost:11434"
		config.Dimensions = 768
		config.MaxTokensPerText = 2048
		config.MaxBatchSize = 32
		config.SupportedModels = []string{
			"nomic-embed-text",
			"mxbai-embed-large",
			"all-minilm",
		}
		config.Features.EnableBatching = true
		config.Features.EnableCaching = true
		config.Features.EnableDeduplication = true

	case EmbeddingProviderLocal:
		config.Model = "all-MiniLM-L6-v2"
		config.Dimensions = 384
		config.MaxTokensPerText = 512
		config.MaxBatchSize = 64
		config.SupportedModels = []string{
			"all-MiniLM-L6-v2",
			"all-mpnet-base-v2",
			"multi-qa-MiniLM-L6-cos-v1",
		}
		config.Features.EnableBatching = true
		config.Features.EnableCaching = true
		config.Features.EnableDeduplication = true

	case EmbeddingProviderSentenceTransform:
		config.Model = "all-MiniLM-L6-v2"
		config.Dimensions = 384
		config.MaxTokensPerText = 512
		config.MaxBatchSize = 64
		config.Features.EnableBatching = true
		config.Features.EnableCaching = true

	case EmbeddingProviderHuggingFace:
		config.Model = "sentence-transformers/all-MiniLM-L6-v2"
		config.Endpoint = "https://api-inference.huggingface.co/pipeline/feature-extraction"
		config.Dimensions = 384
		config.MaxTokensPerText = 512
		config.MaxBatchSize = 32
		config.Features.EnableBatching = true
		config.Features.EnableUsageTracking = true

	case EmbeddingProviderCohere:
		config.Model = "embed-english-v3.0"
		config.Endpoint = "https://api.cohere.ai/v1/embed"
		config.Dimensions = 1024
		config.MaxTokensPerText = 512
		config.MaxBatchSize = 96
		config.SupportedModels = []string{
			"embed-english-v3.0",
			"embed-multilingual-v3.0",
			"embed-english-light-v3.0",
		}
		config.Features.EnableBatching = true
		config.Features.EnableCaching = true
		config.Features.EnableDeduplication = true
		config.Features.EnableUsageTracking = true

	case EmbeddingProviderGemini:
		config.Model = "text-embedding-004"
		config.Endpoint = "https://generativelanguage.googleapis.com/v1beta"
		config.Dimensions = 768
		config.MaxTokensPerText = 2048
		config.MaxBatchSize = 100
		config.SupportedModels = []string{
			"text-embedding-004",
		}
		config.Features.EnableBatching = true
		config.Features.EnableCaching = true
		config.Features.EnableDeduplication = true
		config.Features.EnableCustomDimensions = true
		config.Features.EnableUsageTracking = true
	}

	// Set default rate limiting
	config.RateLimit.RequestsPerMinute = 3000
	config.RateLimit.TokensPerMinute = 1000000
	config.RateLimit.BurstSize = 10
	config.RateLimit.RetryAttempts = 3
	config.RateLimit.RetryDelay = 1 * time.Second
	config.RateLimit.BackoffMultiplier = 2.0

	// Set default caching
	config.Cache.Enabled = true
	config.Cache.DefaultTTL = 24 * time.Hour
	config.Cache.MaxSize = 10000

	// Set default health check
	config.HealthCheck.Enabled = true
	config.HealthCheck.Interval = 5 * time.Minute
	config.HealthCheck.Timeout = 30 * time.Second

	return config
}

// EmbeddingUsageStats tracks embedding provider usage statistics
type EmbeddingUsageStats struct {
	// Request statistics
	TotalRequests      int64 `json:"total_requests"`
	SuccessfulRequests int64 `json:"successful_requests"`
	FailedRequests     int64 `json:"failed_requests"`
	CachedRequests     int64 `json:"cached_requests"`

	// Token and text statistics
	TotalTextsProcessed int64 `json:"total_texts_processed"`
	TotalTokensUsed     int64 `json:"total_tokens_used"`
	TotalEmbeddings     int64 `json:"total_embeddings"`

	// Performance metrics
	AverageLatency      time.Duration `json:"average_latency"`
	AverageTextsPerSec  float64       `json:"average_texts_per_sec"`
	AverageTokensPerSec float64       `json:"average_tokens_per_sec"`

	// Batch statistics
	TotalBatchRequests int64   `json:"total_batch_requests"`
	AverageBatchSize   float64 `json:"average_batch_size"`
	BatchEfficiency    float64 `json:"batch_efficiency"` // Percentage of requests that were batched

	// Cache statistics
	CacheHitRate  float64 `json:"cache_hit_rate"`
	CacheMissRate float64 `json:"cache_miss_rate"`

	// Cost tracking (if available)
	EstimatedCost float64 `json:"estimated_cost,omitempty"`

	// Time period
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// Provider-specific metrics
	CustomMetrics map[string]interface{} `json:"custom_metrics,omitempty"`
}

// EmbeddingRateLimitStatus provides current rate limiting information for embeddings
type EmbeddingRateLimitStatus struct {
	// Current limits
	RequestsPerMinute int `json:"requests_per_minute"`
	TokensPerMinute   int `json:"tokens_per_minute"`
	RequestsPerDay    int `json:"requests_per_day,omitempty"`

	// Current usage
	RequestsUsed      int `json:"requests_used"`
	TokensUsed        int `json:"tokens_used"`
	RequestsUsedToday int `json:"requests_used_today,omitempty"`

	// Remaining capacity
	RequestsRemaining int `json:"requests_remaining"`
	TokensRemaining   int `json:"tokens_remaining"`

	// Reset timing
	ResetTime          time.Time     `json:"reset_time"`
	ResetTimeRemaining time.Duration `json:"reset_time_remaining"`

	// Status flags
	IsLimited  bool          `json:"is_limited"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
}

// EmbeddingProviderCapabilities describes what features an embedding provider supports
type EmbeddingProviderCapabilities struct {
	// Basic capabilities
	SupportsBatching      bool `json:"supports_batching"`
	SupportsStreaming     bool `json:"supports_streaming"`
	SupportsCustomDims    bool `json:"supports_custom_dimensions"`
	SupportsNormalization bool `json:"supports_normalization"`

	// Model capabilities
	MaxTokensPerText int      `json:"max_tokens_per_text"`
	MaxBatchSize     int      `json:"max_batch_size"`
	SupportedModels  []string `json:"supported_models"`
	DefaultModel     string   `json:"default_model"`

	// Dimension capabilities
	SupportedDimensions []int `json:"supported_dimensions"`
	DefaultDimension    int   `json:"default_dimension"`
	MinDimension        int   `json:"min_dimension,omitempty"`
	MaxDimension        int   `json:"max_dimension,omitempty"`

	// Performance capabilities
	SupportsRateLimiting  bool `json:"supports_rate_limiting"`
	SupportsUsageTracking bool `json:"supports_usage_tracking"`
	SupportsCaching       bool `json:"supports_caching"`
	SupportsDeduplication bool `json:"supports_deduplication"`

	// Advanced features
	SupportsMultiModal   bool     `json:"supports_multimodal"`
	SupportedInputTypes  []string `json:"supported_input_types"` // text, image, audio
	SupportsFineTuning   bool     `json:"supports_fine_tuning"`
	SupportsCustomModels bool     `json:"supports_custom_models"`

	// Pricing and limits (if available)
	CostPerToken float64 `json:"cost_per_token,omitempty"`
	RateLimitRPM int     `json:"rate_limit_rpm,omitempty"`
	RateLimitTPM int     `json:"rate_limit_tpm,omitempty"`

	// Quality metrics
	SupportsQualityMetrics bool               `json:"supports_quality_metrics"`
	AverageAccuracy        float64            `json:"average_accuracy,omitempty"`
	BenchmarkScores        map[string]float64 `json:"benchmark_scores,omitempty"`
}

// EmbeddingStreamCallback is called for each chunk in streaming embedding generation
type EmbeddingStreamCallback func(embedding []float32, done bool, err error)

// EmbeddingResult represents the result of embedding generation with metadata
type EmbeddingResult struct {
	Embedding  []float32              `json:"embedding"`
	Text       string                 `json:"text"`
	TokenCount int                    `json:"token_count,omitempty"`
	Model      string                 `json:"model"`
	Cached     bool                   `json:"cached"`
	Latency    time.Duration          `json:"latency"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BatchEmbeddingResult represents the result of batch embedding generation
type BatchEmbeddingResult struct {
	Results      []*EmbeddingResult     `json:"results"`
	TotalTexts   int                    `json:"total_texts"`
	SuccessCount int                    `json:"success_count"`
	FailureCount int                    `json:"failure_count"`
	CachedCount  int                    `json:"cached_count"`
	TotalLatency time.Duration          `json:"total_latency"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ValidateEmbeddingProviderConfig validates an embedding provider configuration
func ValidateEmbeddingProviderConfig(config *EmbeddingProviderConfig) error {
	if config == nil {
		return NewExtractorError("validation", "embedding provider config is nil", 400)
	}

	if config.Type == "" {
		return NewExtractorError("validation", "embedding provider type is required", 400)
	}

	if config.Model == "" {
		return NewExtractorError("validation", "embedding model is required", 400)
	}

	// Validate provider-specific requirements
	switch config.Type {
	case EmbeddingProviderOpenAI, EmbeddingProviderCohere, EmbeddingProviderHuggingFace:
		if config.APIKey == "" {
			return NewExtractorError("validation", fmt.Sprintf("%s provider requires API key", config.Type), 400)
		}

	case EmbeddingProviderOllama:
		if config.Endpoint == "" {
			return NewExtractorError("validation", "Ollama provider requires endpoint", 400)
		}

	case EmbeddingProviderLocal, EmbeddingProviderSentenceTransform:
		// Local providers don't require API keys but may need model paths
		if config.CustomOptions != nil {
			if modelPath, exists := config.CustomOptions["model_path"]; exists {
				if modelPath == "" {
					return NewExtractorError("validation", "local provider model path cannot be empty", 400)
				}
			}
		}
	}

	// Validate dimensions
	if config.Dimensions <= 0 {
		return NewExtractorError("validation", "embedding dimensions must be positive", 400)
	}

	// Validate batch size
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 1
	}

	// Validate timeout
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}

	// Validate rate limiting
	if config.RateLimit.RetryAttempts < 0 {
		config.RateLimit.RetryAttempts = 0
	}
	if config.RateLimit.RetryAttempts > 10 {
		config.RateLimit.RetryAttempts = 10
	}

	return nil
}

// GetEmbeddingProviderCapabilitiesMap returns capabilities for all supported embedding providers
func GetEmbeddingProviderCapabilitiesMap() map[EmbeddingProviderType]*EmbeddingProviderCapabilities {
	return map[EmbeddingProviderType]*EmbeddingProviderCapabilities{
		EmbeddingProviderOpenAI: {
			SupportsBatching:      true,
			SupportsStreaming:     false,
			SupportsCustomDims:    true,
			SupportsNormalization: true,
			MaxTokensPerText:      8192,
			MaxBatchSize:          2048,
			SupportedModels:       []string{"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"},
			DefaultModel:          "text-embedding-3-small",
			SupportedDimensions:   []int{512, 1024, 1536, 3072},
			DefaultDimension:      1536,
			MinDimension:          1,
			MaxDimension:          3072,
			SupportsRateLimiting:  true,
			SupportsUsageTracking: true,
			SupportsCaching:       true,
			SupportsDeduplication: true,
			SupportedInputTypes:   []string{"text"},
			SupportsFineTuning:    false,
			SupportsCustomModels:  false,
			CostPerToken:          0.00001, // Approximate cost per token
			RateLimitRPM:          3000,
			RateLimitTPM:          1000000,
		},
		EmbeddingProviderOllama: {
			SupportsBatching:      true,
			SupportsStreaming:     false,
			SupportsCustomDims:    false,
			SupportsNormalization: true,
			MaxTokensPerText:      2048,
			MaxBatchSize:          32,
			SupportedModels:       []string{"nomic-embed-text", "mxbai-embed-large", "all-minilm"},
			DefaultModel:          "nomic-embed-text",
			SupportedDimensions:   []int{384, 768, 1024},
			DefaultDimension:      768,
			SupportsRateLimiting:  false,
			SupportsUsageTracking: false,
			SupportsCaching:       true,
			SupportsDeduplication: true,
			SupportedInputTypes:   []string{"text"},
			SupportsFineTuning:    true,
			SupportsCustomModels:  true,
		},
		EmbeddingProviderLocal: {
			SupportsBatching:      true,
			SupportsStreaming:     false,
			SupportsCustomDims:    false,
			SupportsNormalization: true,
			MaxTokensPerText:      512,
			MaxBatchSize:          64,
			SupportedModels:       []string{"all-MiniLM-L6-v2", "all-mpnet-base-v2", "multi-qa-MiniLM-L6-cos-v1"},
			DefaultModel:          "all-MiniLM-L6-v2",
			SupportedDimensions:   []int{384, 768},
			DefaultDimension:      384,
			SupportsRateLimiting:  false,
			SupportsUsageTracking: true,
			SupportsCaching:       true,
			SupportsDeduplication: true,
			SupportedInputTypes:   []string{"text"},
			SupportsFineTuning:    true,
			SupportsCustomModels:  true,
		},
		EmbeddingProviderCohere: {
			SupportsBatching:      true,
			SupportsStreaming:     false,
			SupportsCustomDims:    false,
			SupportsNormalization: true,
			MaxTokensPerText:      512,
			MaxBatchSize:          96,
			SupportedModels:       []string{"embed-english-v3.0", "embed-multilingual-v3.0", "embed-english-light-v3.0"},
			DefaultModel:          "embed-english-v3.0",
			SupportedDimensions:   []int{1024, 384},
			DefaultDimension:      1024,
			SupportsRateLimiting:  true,
			SupportsUsageTracking: true,
			SupportsCaching:       true,
			SupportsDeduplication: true,
			SupportedInputTypes:   []string{"text"},
			SupportsFineTuning:    false,
			SupportsCustomModels:  false,
			CostPerToken:          0.0001,
			RateLimitRPM:          1000,
			RateLimitTPM:          100000,
		},
		EmbeddingProviderGemini: {
			SupportsBatching:      true,
			SupportsStreaming:     false,
			SupportsCustomDims:    true,
			SupportsNormalization: true,
			MaxTokensPerText:      2048,
			MaxBatchSize:          100,
			SupportedModels:       []string{"text-embedding-004"},
			DefaultModel:          "text-embedding-004",
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
		},
	}
}

// ProviderConfig holds comprehensive configuration for LLM providers
type ProviderConfig struct {
	// Basic provider information
	Type  ProviderType `json:"type"`
	Name  string       `json:"name,omitempty"` // Custom name for this provider instance
	Model string       `json:"model"`

	// Authentication
	APIKey    string `json:"api_key,omitempty"`
	APISecret string `json:"api_secret,omitempty"` // For providers requiring secret
	Token     string `json:"token,omitempty"`      // Alternative to API key

	// Endpoints and networking
	Endpoint string        `json:"endpoint,omitempty"`
	BaseURL  string        `json:"base_url,omitempty"`
	Region   string        `json:"region,omitempty"` // For cloud providers
	Timeout  time.Duration `json:"timeout,omitempty"`

	// Default completion options
	DefaultOptions *CompletionOptions `json:"default_options,omitempty"`

	// Rate limiting and retry configuration
	RateLimit struct {
		RequestsPerMinute int           `json:"requests_per_minute,omitempty"`
		TokensPerMinute   int           `json:"tokens_per_minute,omitempty"`
		BurstSize         int           `json:"burst_size,omitempty"`
		RetryAttempts     int           `json:"retry_attempts,omitempty"`
		RetryDelay        time.Duration `json:"retry_delay,omitempty"`
		BackoffMultiplier float64       `json:"backoff_multiplier,omitempty"`
	} `json:"rate_limit,omitempty"`

	// Health check configuration
	HealthCheck struct {
		Enabled  bool          `json:"enabled"`
		Interval time.Duration `json:"interval,omitempty"`
		Timeout  time.Duration `json:"timeout,omitempty"`
	} `json:"health_check,omitempty"`

	// Feature flags
	Features struct {
		EnableStreaming     bool `json:"enable_streaming"`
		EnableJSONMode      bool `json:"enable_json_mode"`
		EnableFunctionCalls bool `json:"enable_function_calls"`
		EnableUsageTracking bool `json:"enable_usage_tracking"`
		EnableCaching       bool `json:"enable_caching"`
	} `json:"features,omitempty"`

	// Provider-specific options
	CustomOptions map[string]interface{} `json:"custom_options,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Version   string    `json:"version,omitempty"`
}

// ExtractionConfig configures entity and relationship extraction
type ExtractionConfig struct {
	// Domain-specific extraction templates
	Domain string `json:"domain,omitempty"`

	// Custom prompts for extraction
	EntityPrompt       string `json:"entity_prompt,omitempty"`
	RelationshipPrompt string `json:"relationship_prompt,omitempty"`

	// Extraction quality settings
	MinConfidence float64 `json:"min_confidence"`
	MaxEntities   int     `json:"max_entities"`
	MaxRetries    int     `json:"max_retries"`

	// JSON Schema mode settings
	UseJSONSchema bool `json:"use_json_schema"`
	StrictMode    bool `json:"strict_mode"`
}

// DefaultExtractionConfig returns sensible defaults
func DefaultExtractionConfig() *ExtractionConfig {
	return &ExtractionConfig{
		Domain:        "general",
		MinConfidence: 0.7,
		MaxEntities:   50,
		MaxRetries:    3,
		UseJSONSchema: true,
		StrictMode:    false,
	}
}

// DefaultCompletionOptions returns sensible defaults for completion options
func DefaultCompletionOptions() *CompletionOptions {
	return &CompletionOptions{
		Temperature:      0.7,
		MaxTokens:        2000,
		TopP:             1.0,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
		Timeout:          120 * time.Second,
		RetryAttempts:    3,
		ResponseFormat:   "text",
		CustomOptions:    make(map[string]interface{}),
	}
}

// DefaultProviderConfig returns default configuration for a provider type
func DefaultProviderConfig(providerType ProviderType) *ProviderConfig {
	config := &ProviderConfig{
		Type:           providerType,
		Timeout:        120 * time.Second,
		DefaultOptions: DefaultCompletionOptions(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Version:        "1.0",
		CustomOptions:  make(map[string]interface{}),
	}

	// Set provider-specific defaults
	switch providerType {
	case ProviderOpenAI:
		config.Model = "gpt-4"
		config.BaseURL = "https://api.openai.com/v1"
		config.Features.EnableStreaming = true
		config.Features.EnableJSONMode = true
		config.Features.EnableFunctionCalls = true
		config.Features.EnableUsageTracking = true

	case ProviderAnthropic:
		config.Model = "claude-3-haiku-20240307"
		config.BaseURL = "https://api.anthropic.com/v1"
		config.Features.EnableStreaming = true
		config.Features.EnableUsageTracking = true

	case ProviderGemini:
		config.Model = "gemini-1.5-flash"
		config.BaseURL = "https://generativelanguage.googleapis.com/v1"
		config.Features.EnableJSONMode = true
		config.Features.EnableUsageTracking = true

	case ProviderOllama:
		config.Model = "llama2"
		config.BaseURL = "http://localhost:11434"
		config.Features.EnableStreaming = true
		config.Features.EnableJSONMode = true

	case ProviderDeepSeek:
		config.Model = "deepseek-chat"
		config.BaseURL = "https://api.deepseek.com/v1"
		config.Features.EnableJSONMode = true
		config.Features.EnableUsageTracking = true

	case ProviderMistral:
		config.Model = "mistral-large-latest"
		config.BaseURL = "https://api.mistral.ai/v1"
		config.Features.EnableStreaming = true
		config.Features.EnableJSONMode = true
		config.Features.EnableFunctionCalls = true

	case ProviderBedrock:
		config.Model = "anthropic.claude-3-sonnet-20240229-v1:0"
		config.Region = "us-east-1"
		config.Features.EnableUsageTracking = true
	}

	// Set default rate limiting
	config.RateLimit.RequestsPerMinute = 60
	config.RateLimit.TokensPerMinute = 100000
	config.RateLimit.BurstSize = 10
	config.RateLimit.RetryAttempts = 3
	config.RateLimit.RetryDelay = 1 * time.Second
	config.RateLimit.BackoffMultiplier = 2.0

	// Set default health check
	config.HealthCheck.Enabled = true
	config.HealthCheck.Interval = 5 * time.Minute
	config.HealthCheck.Timeout = 30 * time.Second

	return config
}

// NewMessage creates a new conversation message
func NewMessage(role MessageRole, content string) Message {
	return Message{
		Role:     role,
		Content:  content,
		Metadata: make(map[string]interface{}),
	}
}

// NewSystemMessage creates a new system message
func NewSystemMessage(content string) Message {
	return NewMessage(RoleSystem, content)
}

// NewUserMessage creates a new user message
func NewUserMessage(content string) Message {
	return NewMessage(RoleUser, content)
}

// NewAssistantMessage creates a new assistant message
func NewAssistantMessage(content string) Message {
	return NewMessage(RoleAssistant, content)
}

// ValidateProviderConfig validates a provider configuration
func ValidateProviderConfig(config *ProviderConfig) error {
	if config == nil {
		return NewExtractorError("validation", "provider config is nil", 400)
	}

	if config.Type == "" {
		return NewExtractorError("validation", "provider type is required", 400)
	}

	if config.Model == "" {
		return NewExtractorError("validation", "model is required", 400)
	}

	// Validate provider-specific requirements
	switch config.Type {
	case ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderDeepSeek, ProviderMistral:
		if config.APIKey == "" {
			return NewExtractorError("validation", fmt.Sprintf("%s provider requires API key", config.Type), 400)
		}

	case ProviderOllama:
		if config.BaseURL == "" && config.Endpoint == "" {
			return NewExtractorError("validation", "Ollama provider requires endpoint or base URL", 400)
		}

	case ProviderBedrock:
		if config.Region == "" {
			return NewExtractorError("validation", "Bedrock provider requires region", 400)
		}
	}

	// Validate timeout
	if config.Timeout <= 0 {
		config.Timeout = 120 * time.Second
	}

	// Validate rate limiting
	if config.RateLimit.RetryAttempts < 0 {
		config.RateLimit.RetryAttempts = 0
	}
	if config.RateLimit.RetryAttempts > 10 {
		config.RateLimit.RetryAttempts = 10
	}

	return nil
}

// GetProviderCapabilitiesMap returns capabilities for all supported providers
func GetProviderCapabilitiesMap() map[ProviderType]*ProviderCapabilities {
	return map[ProviderType]*ProviderCapabilities{
		ProviderOpenAI: {
			SupportsCompletion:      true,
			SupportsChat:            true,
			SupportsStreaming:       true,
			SupportsJSONMode:        true,
			SupportsJSONSchema:      true,
			SupportsFunctionCalling: true,
			SupportsSystemPrompts:   true,
			SupportsConversation:    true,
			MaxContextLength:        128000,
			SupportsEmbeddings:      true,
			SupportsImageInput:      true,
			SupportsCodeGeneration:  true,
			SupportsRetries:         true,
			SupportsRateLimiting:    true,
			SupportsUsageTracking:   true,
			AvailableModels:         []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
			DefaultModel:            "gpt-4",
		},
		ProviderAnthropic: {
			SupportsCompletion:      true,
			SupportsChat:            true,
			SupportsStreaming:       true,
			SupportsJSONMode:        false,
			SupportsJSONSchema:      false,
			SupportsFunctionCalling: false,
			SupportsSystemPrompts:   true,
			SupportsConversation:    true,
			MaxContextLength:        200000,
			SupportsImageInput:      true,
			SupportsCodeGeneration:  true,
			SupportsRetries:         true,
			SupportsRateLimiting:    true,
			SupportsUsageTracking:   true,
			AvailableModels:         []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307", "claude-3-5-sonnet-20241022"},
			DefaultModel:            "claude-3-haiku-20240307",
		},
		ProviderGemini: {
			SupportsCompletion:      true,
			SupportsChat:            true,
			SupportsStreaming:       true,
			SupportsJSONMode:        true,
			SupportsJSONSchema:      true,
			SupportsFunctionCalling: true,
			SupportsSystemPrompts:   true,
			SupportsConversation:    true,
			MaxContextLength:        1000000, // Gemini 1.5 Pro has 1M token context
			SupportsImageInput:      true,
			SupportsCodeGeneration:  true,
			SupportsRetries:         true,
			SupportsRateLimiting:    true,
			SupportsUsageTracking:   true,
			AvailableModels:         []string{"gemini-pro", "gemini-pro-vision", "gemini-1.5-pro", "gemini-1.5-flash"},
			DefaultModel:            "gemini-1.5-flash",
		},
		ProviderOllama: {
			SupportsCompletion:      true,
			SupportsChat:            true,
			SupportsStreaming:       true,
			SupportsJSONMode:        true,
			SupportsJSONSchema:      false,
			SupportsFunctionCalling: false,
			SupportsSystemPrompts:   true,
			SupportsConversation:    true,
			MaxContextLength:        4096, // Varies by model
			SupportsCodeGeneration:  true,
			SupportsRetries:         true,
			SupportsRateLimiting:    false,
			SupportsUsageTracking:   false,
			AvailableModels:         []string{"llama2", "codellama", "mistral", "neural-chat"},
			DefaultModel:            "llama2",
		},
		ProviderDeepSeek: {
			SupportsCompletion:      true,
			SupportsChat:            true,
			SupportsStreaming:       false,
			SupportsJSONMode:        true,
			SupportsJSONSchema:      true,
			SupportsFunctionCalling: false,
			SupportsSystemPrompts:   true,
			SupportsConversation:    true,
			MaxContextLength:        32000,
			SupportsCodeGeneration:  true,
			SupportsRetries:         true,
			SupportsRateLimiting:    true,
			SupportsUsageTracking:   true,
			AvailableModels:         []string{"deepseek-chat", "deepseek-coder"},
			DefaultModel:            "deepseek-chat",
		},
		ProviderMistral: {
			SupportsCompletion:      true,
			SupportsChat:            true,
			SupportsStreaming:       true,
			SupportsJSONMode:        true,
			SupportsJSONSchema:      false,
			SupportsFunctionCalling: true,
			SupportsSystemPrompts:   true,
			SupportsConversation:    true,
			MaxContextLength:        32000,
			SupportsCodeGeneration:  true,
			SupportsRetries:         true,
			SupportsRateLimiting:    true,
			SupportsUsageTracking:   true,
			AvailableModels:         []string{"mistral-large-latest", "mistral-medium-latest", "mistral-small-latest"},
			DefaultModel:            "mistral-large-latest",
		},
	}
}

// ExtractedData represents the result of structured extraction
type ExtractedData struct {
	Entities      []schema.Node          `json:"entities"`
	Relationships []schema.Edge          `json:"relationships"`
	Confidence    float64                `json:"confidence"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ProviderFactory creates and manages LLM providers
type ProviderFactory interface {
	// CreateProvider creates a new provider instance from configuration
	CreateProvider(config *ProviderConfig) (LLMProvider, error)

	// CreateProviderWithDefaults creates a provider with sensible defaults
	CreateProviderWithDefaults(providerType ProviderType, apiKey, model string) (LLMProvider, error)

	// ListSupportedProviders returns all supported provider types
	ListSupportedProviders() []ProviderType

	// GetProviderCapabilities returns capabilities for a provider type
	GetProviderCapabilities(providerType ProviderType) (*ProviderCapabilities, error)

	// ValidateConfig validates a provider configuration
	ValidateConfig(config *ProviderConfig) error

	// GetDefaultConfig returns default configuration for a provider type
	GetDefaultConfig(providerType ProviderType) (*ProviderConfig, error)

	// RegisterCustomProvider registers a custom provider implementation
	RegisterCustomProvider(providerType ProviderType, createFunc func(*ProviderConfig) (LLMProvider, error)) error
}

// EmbeddingProviderFactory creates and manages embedding providers
type EmbeddingProviderFactory interface {
	// CreateProvider creates a new embedding provider instance from configuration
	CreateProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error)

	// CreateProviderWithDefaults creates a provider with sensible defaults
	CreateProviderWithDefaults(providerType EmbeddingProviderType, apiKey, model string) (EmbeddingProvider, error)

	// ListSupportedProviders returns all supported embedding provider types
	ListSupportedProviders() []EmbeddingProviderType

	// GetProviderCapabilities returns capabilities for an embedding provider type
	GetProviderCapabilities(providerType EmbeddingProviderType) (*EmbeddingProviderCapabilities, error)

	// ValidateConfig validates an embedding provider configuration
	ValidateConfig(config *EmbeddingProviderConfig) error

	// GetDefaultConfig returns default configuration for an embedding provider type
	GetDefaultConfig(providerType EmbeddingProviderType) (*EmbeddingProviderConfig, error)

	// RegisterCustomProvider registers a custom embedding provider implementation
	RegisterCustomProvider(providerType EmbeddingProviderType, createFunc func(*EmbeddingProviderConfig) (EmbeddingProvider, error)) error

	// GetSupportedModels returns supported models for a provider type
	GetSupportedModels(providerType EmbeddingProviderType) ([]string, error)

	// EstimateProviderCost estimates cost for embedding generation with a provider
	EstimateProviderCost(providerType EmbeddingProviderType, tokenCount int) (float64, error)
}

// EmbeddingProviderManager manages multiple embedding providers with failover and load balancing
type EmbeddingProviderManager interface {
	// AddProvider adds an embedding provider to the manager
	AddProvider(name string, provider EmbeddingProvider, priority int) error

	// RemoveProvider removes an embedding provider from the manager
	RemoveProvider(name string) error

	// GetProvider gets a specific embedding provider by name
	GetProvider(name string) (EmbeddingProvider, error)

	// GetBestProvider returns the best available embedding provider based on health and priority
	GetBestProvider(ctx context.Context) (EmbeddingProvider, error)

	// ListProviders returns all registered embedding providers
	ListProviders() map[string]EmbeddingProvider

	// HealthCheck performs health checks on all embedding providers
	HealthCheck(ctx context.Context) map[string]error

	// SetFailoverEnabled enables/disables automatic failover for embedding providers
	SetFailoverEnabled(enabled bool)

	// SetLoadBalancing configures load balancing strategy for embedding providers
	SetLoadBalancing(strategy EmbeddingLoadBalancingStrategy)

	// GenerateEmbeddingWithFailover generates embedding with automatic failover
	GenerateEmbeddingWithFailover(ctx context.Context, text string) ([]float32, error)

	// GenerateBatchEmbeddingsWithFailover generates batch embeddings with automatic failover
	GenerateBatchEmbeddingsWithFailover(ctx context.Context, texts []string) ([][]float32, error)
}

// EmbeddingLoadBalancingStrategy defines how embedding requests are distributed across providers
type EmbeddingLoadBalancingStrategy string

const (
	EmbeddingLoadBalanceRoundRobin EmbeddingLoadBalancingStrategy = "round_robin"
	EmbeddingLoadBalancePriority   EmbeddingLoadBalancingStrategy = "priority"
	EmbeddingLoadBalanceRandom     EmbeddingLoadBalancingStrategy = "random"
	EmbeddingLoadBalanceLeastUsed  EmbeddingLoadBalancingStrategy = "least_used"
	EmbeddingLoadBalanceCostBased  EmbeddingLoadBalancingStrategy = "cost_based"
	EmbeddingLoadBalanceLatency    EmbeddingLoadBalancingStrategy = "latency_based"
)

// EmbeddingProviderHealthStatus represents the health status of an embedding provider
type EmbeddingProviderHealthStatus struct {
	IsHealthy        bool          `json:"is_healthy"`
	LastCheck        time.Time     `json:"last_check"`
	ResponseTime     time.Duration `json:"response_time"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	ConsecutiveFails int           `json:"consecutive_fails"`
	SuccessRate      float64       `json:"success_rate"`
}

// EmbeddingProviderMetrics tracks performance metrics for an embedding provider
type EmbeddingProviderMetrics struct {
	// Request metrics
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	CachedRequests     int64         `json:"cached_requests"`
	AverageLatency     time.Duration `json:"average_latency"`

	// Embedding metrics
	TotalEmbeddings     int64   `json:"total_embeddings"`
	TotalTextsProcessed int64   `json:"total_texts_processed"`
	TotalTokensUsed     int64   `json:"total_tokens_used"`
	EmbeddingsPerSecond float64 `json:"embeddings_per_second"`
	TokensPerSecond     float64 `json:"tokens_per_second"`

	// Batch metrics
	TotalBatchRequests int64   `json:"total_batch_requests"`
	AverageBatchSize   float64 `json:"average_batch_size"`
	BatchEfficiency    float64 `json:"batch_efficiency"`

	// Cache metrics
	CacheHitRate  float64 `json:"cache_hit_rate"`
	CacheMissRate float64 `json:"cache_miss_rate"`

	// Cost metrics
	EstimatedCost    float64 `json:"estimated_cost"`
	CostPerEmbedding float64 `json:"cost_per_embedding"`
	CostPerToken     float64 `json:"cost_per_token"`

	// Error tracking
	LastError string  `json:"last_error,omitempty"`
	ErrorRate float64 `json:"error_rate"`

	// Time tracking
	FirstRequest time.Time `json:"first_request"`
	LastRequest  time.Time `json:"last_request"`

	// Health status
	Health EmbeddingProviderHealthStatus `json:"health"`
}

// EmbeddingCacheProvider defines the interface for caching embedding responses
type EmbeddingCacheProvider interface {
	// Get retrieves a cached embedding
	Get(ctx context.Context, key string) ([]float32, error)

	// Set stores an embedding in cache
	Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error

	// GetBatch retrieves multiple cached embeddings
	GetBatch(ctx context.Context, keys []string) (map[string][]float32, error)

	// SetBatch stores multiple embeddings in cache
	SetBatch(ctx context.Context, embeddings map[string][]float32, ttl time.Duration) error

	// Delete removes a cached embedding
	Delete(ctx context.Context, key string) error

	// Clear clears all cached embeddings
	Clear(ctx context.Context) error

	// GetStats returns cache statistics
	GetStats(ctx context.Context) (*EmbeddingCacheStats, error)

	// GenerateKey generates a cache key for text and options
	GenerateKey(text string, options *EmbeddingOptions) string
}

// EmbeddingCacheStats provides cache performance statistics for embeddings
type EmbeddingCacheStats struct {
	Hits             int64     `json:"hits"`
	Misses           int64     `json:"misses"`
	HitRate          float64   `json:"hit_rate"`
	Size             int64     `json:"size"`
	MaxSize          int64     `json:"max_size"`
	Evictions        int64     `json:"evictions"`
	LastCleared      time.Time `json:"last_cleared"`
	AverageKeySize   float64   `json:"average_key_size"`
	AverageValueSize float64   `json:"average_value_size"`
	TotalMemoryUsage int64     `json:"total_memory_usage"`
	CacheEfficiency  float64   `json:"cache_efficiency"`
}

// EmbeddingRetryConfig configures retry behavior for failed embedding requests
type EmbeddingRetryConfig struct {
	MaxAttempts       int           `json:"max_attempts"`
	InitialDelay      time.Duration `json:"initial_delay"`
	MaxDelay          time.Duration `json:"max_delay"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	RetryableErrors   []string      `json:"retryable_errors"`
	Jitter            bool          `json:"jitter"`
}

// DefaultEmbeddingRetryConfig returns default retry configuration for embeddings
func DefaultEmbeddingRetryConfig() *EmbeddingRetryConfig {
	return &EmbeddingRetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors: []string{
			"rate_limit",
			"timeout",
			"503",
			"502",
			"500",
			"connection_reset",
			"connection_refused",
			"temporary_failure",
		},
		Jitter: true,
	}
}

// EmbeddingProviderEvent represents events from embedding providers (for monitoring/logging)
type EmbeddingProviderEvent struct {
	Type       EmbeddingEventType     `json:"type"`
	ProviderID string                 `json:"provider_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Message    string                 `json:"message"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Error      string                 `json:"error,omitempty"`
	TextCount  int                    `json:"text_count,omitempty"`
	TokenCount int                    `json:"token_count,omitempty"`
	Latency    time.Duration          `json:"latency,omitempty"`
}

// EmbeddingEventType defines types of embedding provider events
type EmbeddingEventType string

const (
	EmbeddingEventTypeRequest     EmbeddingEventType = "request"
	EmbeddingEventTypeResponse    EmbeddingEventType = "response"
	EmbeddingEventTypeError       EmbeddingEventType = "error"
	EmbeddingEventTypeHealthCheck EmbeddingEventType = "health_check"
	EmbeddingEventTypeRateLimit   EmbeddingEventType = "rate_limit"
	EmbeddingEventTypeFailover    EmbeddingEventType = "failover"
	EmbeddingEventTypeRetry       EmbeddingEventType = "retry"
	EmbeddingEventTypeCacheHit    EmbeddingEventType = "cache_hit"
	EmbeddingEventTypeCacheMiss   EmbeddingEventType = "cache_miss"
	EmbeddingEventTypeBatch       EmbeddingEventType = "batch"
)

// EmbeddingEventHandler handles embedding provider events
type EmbeddingEventHandler interface {
	HandleEvent(event EmbeddingProviderEvent)
}

// EmbeddingProviderWithEvents extends EmbeddingProvider with event handling
type EmbeddingProviderWithEvents interface {
	EmbeddingProvider

	// SetEventHandler sets the event handler for this embedding provider
	SetEventHandler(handler EmbeddingEventHandler)

	// GetMetrics returns embedding provider metrics
	GetMetrics() *EmbeddingProviderMetrics

	// ResetMetrics resets embedding provider metrics
	ResetMetrics()
}

// EmbeddingProviderPool manages a pool of embedding provider instances for high-throughput scenarios
type EmbeddingProviderPool interface {
	// Get gets an embedding provider instance from the pool
	Get(ctx context.Context) (EmbeddingProvider, error)

	// Put returns an embedding provider instance to the pool
	Put(provider EmbeddingProvider) error

	// Size returns the current pool size
	Size() int

	// Close closes all embedding provider instances in the pool
	Close() error

	// Health checks the health of all embedding providers in the pool
	Health(ctx context.Context) []error

	// GetStats returns pool statistics
	GetStats() *EmbeddingProviderPoolStats
}

// EmbeddingProviderPoolStats provides statistics about the embedding provider pool
type EmbeddingProviderPoolStats struct {
	TotalProviders  int           `json:"total_providers"`
	ActiveProviders int           `json:"active_providers"`
	IdleProviders   int           `json:"idle_providers"`
	AverageWaitTime time.Duration `json:"average_wait_time"`
	TotalRequests   int64         `json:"total_requests"`
	SuccessfulGets  int64         `json:"successful_gets"`
	FailedGets      int64         `json:"failed_gets"`
	PoolUtilization float64       `json:"pool_utilization"`
	LastActivity    time.Time     `json:"last_activity"`
}

// JSONSchemaGenerator generates JSON schemas from Go structs
type JSONSchemaGenerator interface {
	GenerateSchema(structType interface{}) (map[string]interface{}, error)
	ValidateAgainstSchema(data interface{}, schema map[string]interface{}) error
}

// ExtractorError represents extraction-specific errors
type ExtractorError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *ExtractorError) Error() string {
	return fmt.Sprintf("extractor error [%s]: %s", e.Type, e.Message)
}

// NewExtractorError creates a new extractor error
func NewExtractorError(errorType, message string, code int) *ExtractorError {
	return &ExtractorError{
		Type:    errorType,
		Message: message,
		Code:    code,
	}
}

// MarshalJSON implements json.Marshaler for ExtractorError
func (e *ExtractorError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":    e.Type,
		"message": e.Message,
		"code":    e.Code,
	})
}

// ProviderManager manages multiple LLM providers with failover and load balancing
type ProviderManager interface {
	// AddProvider adds a provider to the manager
	AddProvider(name string, provider LLMProvider, priority int) error

	// RemoveProvider removes a provider from the manager
	RemoveProvider(name string) error

	// GetProvider gets a specific provider by name
	GetProvider(name string) (LLMProvider, error)

	// GetBestProvider returns the best available provider based on health and priority
	GetBestProvider(ctx context.Context) (LLMProvider, error)

	// ListProviders returns all registered providers
	ListProviders() map[string]LLMProvider

	// HealthCheck performs health checks on all providers
	HealthCheck(ctx context.Context) map[string]error

	// SetFailoverEnabled enables/disables automatic failover
	SetFailoverEnabled(enabled bool)

	// SetLoadBalancing configures load balancing strategy
	SetLoadBalancing(strategy LoadBalancingStrategy)
}

// LoadBalancingStrategy defines how requests are distributed across providers
type LoadBalancingStrategy string

const (
	LoadBalanceRoundRobin LoadBalancingStrategy = "round_robin"
	LoadBalancePriority   LoadBalancingStrategy = "priority"
	LoadBalanceRandom     LoadBalancingStrategy = "random"
	LoadBalanceLeastUsed  LoadBalancingStrategy = "least_used"
)

// ProviderHealthStatus represents the health status of a provider
type ProviderHealthStatus struct {
	IsHealthy        bool          `json:"is_healthy"`
	LastCheck        time.Time     `json:"last_check"`
	ResponseTime     time.Duration `json:"response_time"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	ConsecutiveFails int           `json:"consecutive_fails"`
}

// ProviderMetrics tracks performance metrics for a provider
type ProviderMetrics struct {
	// Request metrics
	TotalRequests  int64         `json:"total_requests"`
	SuccessfulReqs int64         `json:"successful_requests"`
	FailedRequests int64         `json:"failed_requests"`
	AverageLatency time.Duration `json:"average_latency"`

	// Token metrics
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TokensPerSecond  float64 `json:"tokens_per_second"`

	// Error tracking
	LastError string  `json:"last_error,omitempty"`
	ErrorRate float64 `json:"error_rate"`

	// Time tracking
	FirstRequest time.Time `json:"first_request"`
	LastRequest  time.Time `json:"last_request"`

	// Health status
	Health ProviderHealthStatus `json:"health"`
}

// ContextManager handles conversation context and memory
type ContextManager interface {
	// AddMessage adds a message to the conversation context
	AddMessage(sessionID string, message Message) error

	// GetContext retrieves conversation context for a session
	GetContext(sessionID string) ([]Message, error)

	// ClearContext clears conversation context for a session
	ClearContext(sessionID string) error

	// SetMaxContextLength sets the maximum context length
	SetMaxContextLength(length int)

	// TrimContext trims context to fit within token limits
	TrimContext(sessionID string, maxTokens int) error

	// GetContextTokenCount returns the token count for a session's context
	GetContextTokenCount(sessionID string) (int, error)
}

// EmbeddingProvider defines the comprehensive interface for embedding generation
// supporting multiple providers (OpenAI, local sentence-transformers, Ollama, etc.)
// with batch processing, caching, deduplication, and performance optimization.
type EmbeddingProvider interface {
	// Core embedding generation methods

	// GenerateEmbedding generates an embedding for a single text
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

	// GenerateBatchEmbeddings generates embeddings for multiple texts with performance optimization
	GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)

	// GenerateEmbeddingWithOptions generates embedding with custom options
	GenerateEmbeddingWithOptions(ctx context.Context, text string, options *EmbeddingOptions) ([]float32, error)

	// GenerateBatchEmbeddingsWithOptions generates batch embeddings with custom options
	GenerateBatchEmbeddingsWithOptions(ctx context.Context, texts []string, options *EmbeddingOptions) ([][]float32, error)

	// Model and configuration information

	// GetDimensions returns the embedding dimensions (768, 1536, 3072, etc.)
	GetDimensions() int

	// GetModel returns the embedding model name
	GetModel() string

	// SetModel sets the embedding model to use (if supported by provider)
	SetModel(model string) error

	// GetProviderType returns the provider type (openai, ollama, local, etc.)
	GetProviderType() EmbeddingProviderType

	// GetSupportedModels returns list of models supported by this provider
	GetSupportedModels() []string

	// GetMaxBatchSize returns the maximum batch size supported
	GetMaxBatchSize() int

	// GetMaxTokensPerText returns the maximum tokens per text input
	GetMaxTokensPerText() int

	// Performance and optimization methods

	// GenerateEmbeddingCached generates embedding with caching support
	GenerateEmbeddingCached(ctx context.Context, text string, ttl time.Duration) ([]float32, error)

	// GenerateBatchEmbeddingsCached generates batch embeddings with caching
	GenerateBatchEmbeddingsCached(ctx context.Context, texts []string, ttl time.Duration) ([][]float32, error)

	// DeduplicateAndEmbed removes duplicate texts and generates embeddings efficiently
	DeduplicateAndEmbed(ctx context.Context, texts []string) (map[string][]float32, error)

	// EstimateTokenCount estimates token count for text (for cost/rate limiting)
	EstimateTokenCount(text string) (int, error)

	// EstimateCost estimates the cost for embedding generation (if available)
	EstimateCost(tokenCount int) (float64, error)

	// Health and monitoring methods

	// Health checks if the embedding provider is available and responsive
	Health(ctx context.Context) error

	// GetUsage returns usage statistics for the provider
	GetUsage(ctx context.Context) (*EmbeddingUsageStats, error)

	// GetRateLimit returns current rate limit status
	GetRateLimit(ctx context.Context) (*EmbeddingRateLimitStatus, error)

	// Configuration and lifecycle methods

	// Configure updates provider configuration
	Configure(config *EmbeddingProviderConfig) error

	// GetConfiguration returns current provider configuration
	GetConfiguration() *EmbeddingProviderConfig

	// ValidateConfiguration validates the provider configuration
	ValidateConfiguration(config *EmbeddingProviderConfig) error

	// Close closes the provider and cleans up resources
	Close() error

	// Advanced features

	// SupportsStreaming returns true if provider supports streaming embeddings
	SupportsStreaming() bool

	// GenerateStreamingEmbedding generates embedding with streaming callback (if supported)
	GenerateStreamingEmbedding(ctx context.Context, text string, callback EmbeddingStreamCallback) error

	// SupportsCustomDimensions returns true if provider supports custom embedding dimensions
	SupportsCustomDimensions() bool

	// SetCustomDimensions sets custom embedding dimensions (if supported)
	SetCustomDimensions(dimensions int) error

	// GetCapabilities returns the capabilities supported by this provider
	GetCapabilities() *EmbeddingProviderCapabilities
}

// CacheProvider defines the interface for caching LLM responses
type CacheProvider interface {
	// Get retrieves a cached response
	Get(ctx context.Context, key string) (interface{}, error)

	// Set stores a response in cache
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a cached response
	Delete(ctx context.Context, key string) error

	// Clear clears all cached responses
	Clear(ctx context.Context) error

	// GetStats returns cache statistics
	GetStats(ctx context.Context) (*CacheStats, error)
}

// CacheStats provides cache performance statistics
type CacheStats struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	HitRate     float64   `json:"hit_rate"`
	Size        int64     `json:"size"`
	MaxSize     int64     `json:"max_size"`
	Evictions   int64     `json:"evictions"`
	LastCleared time.Time `json:"last_cleared"`
}

// RetryConfig configures retry behavior for failed requests
type RetryConfig struct {
	MaxAttempts       int           `json:"max_attempts"`
	InitialDelay      time.Duration `json:"initial_delay"`
	MaxDelay          time.Duration `json:"max_delay"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	RetryableErrors   []string      `json:"retryable_errors"`
	Jitter            bool          `json:"jitter"`
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors: []string{
			"rate_limit",
			"timeout",
			"503",
			"502",
			"500",
			"connection_reset",
			"connection_refused",
		},
		Jitter: true,
	}
}

// ProviderEvent represents events from providers (for monitoring/logging)
type ProviderEvent struct {
	Type       EventType              `json:"type"`
	ProviderID string                 `json:"provider_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Message    string                 `json:"message"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// EventType defines types of provider events
type EventType string

const (
	EventTypeRequest     EventType = "request"
	EventTypeResponse    EventType = "response"
	EventTypeError       EventType = "error"
	EventTypeHealthCheck EventType = "health_check"
	EventTypeRateLimit   EventType = "rate_limit"
	EventTypeFailover    EventType = "failover"
	EventTypeRetry       EventType = "retry"
)

// EventHandler handles provider events
type EventHandler interface {
	HandleEvent(event ProviderEvent)
}

// LLMProviderWithEvents extends LLMProvider with event handling
type LLMProviderWithEvents interface {
	LLMProvider

	// SetEventHandler sets the event handler for this provider
	SetEventHandler(handler EventHandler)

	// GetMetrics returns provider metrics
	GetMetrics() *ProviderMetrics

	// ResetMetrics resets provider metrics
	ResetMetrics()
}

// ProviderPool manages a pool of provider instances for high-throughput scenarios
type ProviderPool interface {
	// Get gets a provider instance from the pool
	Get(ctx context.Context) (LLMProvider, error)

	// Put returns a provider instance to the pool
	Put(provider LLMProvider) error

	// Size returns the current pool size
	Size() int

	// Close closes all provider instances in the pool
	Close() error

	// Health checks the health of all providers in the pool
	Health(ctx context.Context) []error
}
