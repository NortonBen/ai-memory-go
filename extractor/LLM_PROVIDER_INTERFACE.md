# LLMProvider Interface Documentation

## Overview

The `LLMProvider` interface provides a standardized way to interact with multiple Large Language Model providers in the AI Memory Integration system. This interface supports text completion, structured output generation with JSON schema, entity extraction, relationship detection, context management, and comprehensive health monitoring.

## Supported Providers

The interface supports the following LLM providers:

- **OpenAI** (`openai`) - GPT-4, GPT-3.5-turbo with full feature support
- **Anthropic** (`anthropic`) - Claude models with conversation support
- **Google Gemini** (`gemini`) - Gemini Pro with multimodal capabilities
- **Ollama** (`ollama`) - Local models with streaming support
- **DeepSeek** (`deepseek`) - DeepSeek models with JSON schema mode
- **Mistral** (`mistral`) - Mistral models with function calling
- **AWS Bedrock** (`bedrock`) - Cloud-hosted models
- **Azure OpenAI** (`azure`) - Azure-hosted OpenAI models
- **Cohere** (`cohere`) - Cohere language models
- **Hugging Face** (`huggingface`) - Hugging Face models
- **Local** (`local`) - Custom local model implementations
- **Custom** (`custom`) - User-defined provider implementations

## Core Interface Methods

### Text Generation

```go
// Basic text completion
GenerateCompletion(ctx context.Context, prompt string) (string, error)

// Text completion with custom options
GenerateCompletionWithOptions(ctx context.Context, prompt string, options *CompletionOptions) (string, error)

// Structured output with JSON schema
GenerateStructuredOutput(ctx context.Context, prompt string, schema interface{}) (interface{}, error)

// Structured output with custom options
GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schema interface{}, options *CompletionOptions) (interface{}, error)
```

### Entity and Relationship Extraction

```go
// Extract entities from text
ExtractEntities(ctx context.Context, text string) ([]schema.Node, error)

// Detect relationships between entities
ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error)

// Extract data using custom JSON schema
ExtractWithCustomSchema(ctx context.Context, text string, jsonSchema map[string]interface{}) (interface{}, error)
```

### Context and Conversation Management

```go
// Generate completion with conversation context
GenerateWithContext(ctx context.Context, messages []Message, options *CompletionOptions) (string, error)

// Generate streaming text completion
GenerateStreamingCompletion(ctx context.Context, prompt string, callback StreamCallback) error
```

### Configuration and Metadata

```go
// Get/set model information
GetModel() string
SetModel(model string) error

// Provider information
GetProviderType() ProviderType
GetCapabilities() ProviderCapabilities

// Token management
GetTokenCount(text string) (int, error)
GetMaxTokens() int
```

### Health and Monitoring

```go
// Health check
Health(ctx context.Context) error

// Usage statistics
GetUsage(ctx context.Context) (*UsageStats, error)

// Rate limiting status
GetRateLimit(ctx context.Context) (*RateLimitStatus, error)
```

### Configuration Management

```go
// Configure provider
Configure(config *ProviderConfig) error
GetConfiguration() *ProviderConfig

// Lifecycle management
Close() error
```

## Configuration

### ProviderConfig Structure

```go
type ProviderConfig struct {
    // Basic provider information
    Type     ProviderType `json:"type"`
    Name     string       `json:"name,omitempty"`
    Model    string       `json:"model"`
    
    // Authentication
    APIKey   string `json:"api_key,omitempty"`
    APISecret string `json:"api_secret,omitempty"`
    Token    string `json:"token,omitempty"`
    
    // Endpoints and networking
    Endpoint    string        `json:"endpoint,omitempty"`
    BaseURL     string        `json:"base_url,omitempty"`
    Region      string        `json:"region,omitempty"`
    Timeout     time.Duration `json:"timeout,omitempty"`
    
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
        EnableStreaming      bool `json:"enable_streaming"`
        EnableJSONMode       bool `json:"enable_json_mode"`
        EnableFunctionCalls  bool `json:"enable_function_calls"`
        EnableUsageTracking  bool `json:"enable_usage_tracking"`
        EnableCaching        bool `json:"enable_caching"`
    } `json:"features,omitempty"`
    
    // Provider-specific options
    CustomOptions map[string]interface{} `json:"custom_options,omitempty"`
}
```

### CompletionOptions Structure

```go
type CompletionOptions struct {
    Temperature      float64               `json:"temperature,omitempty"`       // 0.0-2.0
    MaxTokens        int                   `json:"max_tokens,omitempty"`        // Response length limit
    TopP             float64               `json:"top_p,omitempty"`             // 0.0-1.0
    TopK             int                   `json:"top_k,omitempty"`             // Top-k sampling
    FrequencyPenalty float64               `json:"frequency_penalty,omitempty"` // -2.0 to 2.0
    PresencePenalty  float64               `json:"presence_penalty,omitempty"`  // -2.0 to 2.0
    Stop             []string              `json:"stop,omitempty"`              // Stop sequences
    Seed             *int64                `json:"seed,omitempty"`              // Deterministic generation
    ResponseFormat   string                `json:"response_format,omitempty"`   // "json" or "text"
    SystemPrompt     string                `json:"system_prompt,omitempty"`     // System message
    Timeout          time.Duration         `json:"timeout,omitempty"`           // Request timeout
    RetryAttempts    int                   `json:"retry_attempts,omitempty"`    // Retry count
    CustomOptions    map[string]interface{} `json:"custom_options,omitempty"`   // Provider-specific
}
```

## Usage Examples

### Basic Text Completion

```go
provider := NewOpenAIProvider("your-api-key", "gpt-4")
ctx := context.Background()

response, err := provider.GenerateCompletion(ctx, "Explain quantum computing")
if err != nil {
    log.Fatal(err)
}
fmt.Println(response)
```

### Structured Output with JSON Schema

```go
type ExtractionResult struct {
    Entities []struct {
        Name string `json:"name"`
        Type string `json:"type"`
    } `json:"entities"`
}

var result ExtractionResult
_, err := provider.GenerateStructuredOutput(ctx, 
    "Extract entities from: 'John works at Google in California'", 
    &result)
if err != nil {
    log.Fatal(err)
}
```

### Entity Extraction

```go
entities, err := provider.ExtractEntities(ctx, "I'm learning Present Perfect tense")
if err != nil {
    log.Fatal(err)
}

for _, entity := range entities {
    fmt.Printf("Entity: %s, Type: %s\n", 
        entity.Properties["name"], entity.Type)
}
```

### Conversation with Context

```go
messages := []Message{
    NewSystemMessage("You are a helpful English tutor"),
    NewUserMessage("Explain the Present Perfect tense"),
    NewAssistantMessage("The Present Perfect tense is formed with have/has + past participle..."),
    NewUserMessage("Can you give me some examples?"),
}

response, err := provider.GenerateWithContext(ctx, messages, nil)
if err != nil {
    log.Fatal(err)
}
```

### Streaming Completion

```go
callback := func(chunk string, done bool, err error) {
    if err != nil {
        log.Printf("Streaming error: %v", err)
        return
    }
    fmt.Print(chunk)
    if done {
        fmt.Println("\n[Stream complete]")
    }
}

err := provider.GenerateStreamingCompletion(ctx, "Write a story about AI", callback)
if err != nil {
    log.Fatal(err)
}
```

### Provider Configuration

```go
config := DefaultProviderConfig(ProviderOpenAI)
config.APIKey = "your-api-key"
config.Model = "gpt-4-turbo"
config.DefaultOptions.Temperature = 0.3
config.DefaultOptions.MaxTokens = 1000
config.Features.EnableStreaming = true
config.Features.EnableJSONMode = true

provider, err := factory.CreateProvider(config)
if err != nil {
    log.Fatal(err)
}
```

## Provider Capabilities

Each provider has different capabilities that can be queried:

```go
caps := provider.GetCapabilities()

if caps.SupportsJSONMode {
    // Use JSON mode for structured output
}

if caps.SupportsStreaming {
    // Use streaming for real-time responses
}

if caps.SupportsFunctionCalling {
    // Use function calling features
}
```

## Error Handling

The interface uses structured error handling with `ExtractorError`:

```go
response, err := provider.GenerateCompletion(ctx, prompt)
if err != nil {
    if extractorErr, ok := err.(*ExtractorError); ok {
        switch extractorErr.Type {
        case "rate_limit":
            // Handle rate limiting
            time.Sleep(time.Second * 10)
            // Retry logic
        case "authentication":
            // Handle auth errors
            log.Fatal("Invalid API key")
        case "validation":
            // Handle validation errors
            log.Printf("Validation error: %s", extractorErr.Message)
        }
    }
}
```

## Health Monitoring

```go
// Check provider health
err := provider.Health(ctx)
if err != nil {
    log.Printf("Provider unhealthy: %v", err)
}

// Get usage statistics
usage, err := provider.GetUsage(ctx)
if err == nil {
    fmt.Printf("Total tokens used: %d\n", usage.TotalTokensUsed)
    fmt.Printf("Success rate: %.2f%%\n", 
        float64(usage.SuccessfulRequests)/float64(usage.TotalRequests)*100)
}

// Check rate limits
rateLimit, err := provider.GetRateLimit(ctx)
if err == nil {
    fmt.Printf("Requests remaining: %d/%d\n", 
        rateLimit.RequestsRemaining, rateLimit.RequestsPerMinute)
}
```

## Advanced Features

### Provider Factory

```go
factory := NewProviderFactory()

// Create provider with defaults
provider, err := factory.CreateProviderWithDefaults(ProviderOpenAI, "api-key", "gpt-4")

// List supported providers
providers := factory.ListSupportedProviders()

// Get provider capabilities
caps, err := factory.GetProviderCapabilities(ProviderOpenAI)
```

### Provider Manager (Multi-Provider Support)

```go
manager := NewProviderManager()

// Add multiple providers with priorities
manager.AddProvider("primary", openaiProvider, 1)
manager.AddProvider("fallback", ollamaProvider, 2)

// Get best available provider
provider, err := manager.GetBestProvider(ctx)

// Enable automatic failover
manager.SetFailoverEnabled(true)
```

### Context Manager

```go
contextMgr := NewContextManager()

// Add messages to conversation
contextMgr.AddMessage("session-123", NewUserMessage("Hello"))
contextMgr.AddMessage("session-123", NewAssistantMessage("Hi there!"))

// Get conversation context
messages, err := contextMgr.GetContext("session-123")

// Trim context to fit token limits
contextMgr.TrimContext("session-123", 4000)
```

## Best Practices

1. **Always use context.Context** for cancellation and timeouts
2. **Handle rate limiting** gracefully with exponential backoff
3. **Validate configurations** before creating providers
4. **Monitor usage and health** in production environments
5. **Use structured output** for reliable data extraction
6. **Implement proper error handling** for different error types
7. **Cache responses** when appropriate to reduce API calls
8. **Use streaming** for long-form content generation
9. **Set appropriate timeouts** for your use case
10. **Test with mock providers** during development

## Integration with AI Memory System

The LLMProvider interface integrates seamlessly with the AI Memory system:

```go
// Create memory engine with LLM provider
config := DefaultProviderConfig(ProviderDeepSeek)
config.APIKey = "your-deepseek-key"
provider, _ := factory.CreateProvider(config)

extractor := NewBasicExtractor(provider, DefaultExtractionConfig())
memoryEngine := NewMemoryEngine(extractor, graphStore, vectorStore, relationalStore)

// Add and process memory
dataPoint, _ := memoryEngine.Add(ctx, "I struggle with Present Perfect tense", metadata)
enriched, _ := memoryEngine.Cognify(ctx, dataPoint)
memoryEngine.Memify(ctx, enriched)

// Search with context
query := &SearchQuery{
    Text: "How do I use Present Perfect?",
    Mode: ModeHybridSearch,
}
results, _ := memoryEngine.Search(ctx, query)
```

This interface provides a robust foundation for multi-provider LLM integration in the AI Memory system, supporting both current and future provider implementations while maintaining consistency and reliability.