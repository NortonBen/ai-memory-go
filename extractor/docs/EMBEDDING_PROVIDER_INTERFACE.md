# EmbeddingProvider Interface Documentation

## Overview

The `EmbeddingProvider` interface provides a comprehensive, standardized way to interact with multiple embedding providers in the AI Memory Integration system. This interface supports vector generation, batch processing, caching, deduplication, performance optimization, and health monitoring across various embedding providers including OpenAI, Ollama, local sentence-transformers, and more.

## Supported Providers

The interface supports the following embedding providers:

- **OpenAI** (`openai`) - text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002
- **Ollama** (`ollama`) - Local models like nomic-embed-text, mxbai-embed-large
- **Local** (`local`) - Local sentence-transformers models
- **Sentence Transformers** (`sentence_transformers`) - Direct sentence-transformers integration
- **Hugging Face** (`huggingface`) - Hugging Face Inference API
- **Cohere** (`cohere`) - Cohere embedding models
- **Azure** (`azure`) - Azure OpenAI embedding services
- **Bedrock** (`bedrock`) - AWS Bedrock embedding models
- **Vertex** (`vertex`) - Google Vertex AI embedding models
- **Custom** (`custom`) - User-defined provider implementations

## Core Interface Methods

### Basic Embedding Generation

```go
// Generate embedding for a single text
GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

// Generate embeddings for multiple texts with performance optimization
GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)

// Generate embedding with custom options
GenerateEmbeddingWithOptions(ctx context.Context, text string, options *EmbeddingOptions) ([]float32, error)

// Generate batch embeddings with custom options
GenerateBatchEmbeddingsWithOptions(ctx context.Context, texts []string, options *EmbeddingOptions) ([][]float32, error)
```

### Model and Configuration Information

```go
// Get embedding dimensions (768, 1536, 3072, etc.)
GetDimensions() int

// Get embedding model name
GetModel() string

// Set embedding model (if supported by provider)
SetModel(model string) error

// Get provider type (openai, ollama, local, etc.)
GetProviderType() EmbeddingProviderType

// Get list of models supported by this provider
GetSupportedModels() []string

// Get maximum batch size supported
GetMaxBatchSize() int

// Get maximum tokens per text input
GetMaxTokensPerText() int
```

### Performance and Optimization Methods

```go
// Generate embedding with caching support
GenerateEmbeddingCached(ctx context.Context, text string, ttl time.Duration) ([]float32, error)

// Generate batch embeddings with caching
GenerateBatchEmbeddingsCached(ctx context.Context, texts []string, ttl time.Duration) ([][]float32, error)

// Remove duplicate texts and generate embeddings efficiently
DeduplicateAndEmbed(ctx context.Context, texts []string) (map[string][]float32, error)

// Estimate token count for text (for cost/rate limiting)
EstimateTokenCount(text string) (int, error)

// Estimate the cost for embedding generation (if available)
EstimateCost(tokenCount int) (float64, error)
```

### Health and Monitoring Methods

```go
// Check if the embedding provider is available and responsive
Health(ctx context.Context) error

// Get usage statistics for the provider
GetUsage(ctx context.Context) (*EmbeddingUsageStats, error)

// Get current rate limit status
GetRateLimit(ctx context.Context) (*EmbeddingRateLimitStatus, error)
```

### Configuration and Lifecycle Methods

```go
// Update provider configuration
Configure(config *EmbeddingProviderConfig) error

// Get current provider configuration
GetConfiguration() *EmbeddingProviderConfig

// Validate the provider configuration
ValidateConfiguration(config *EmbeddingProviderConfig) error

// Close the provider and clean up resources
Close() error
```

### Advanced Features

```go
// Check if provider supports streaming embeddings
SupportsStreaming() bool

// Generate embedding with streaming callback (if supported)
GenerateStreamingEmbedding(ctx context.Context, text string, callback EmbeddingStreamCallback) error

// Check if provider supports custom embedding dimensions
SupportsCustomDimensions() bool

// Set custom embedding dimensions (if supported)
SetCustomDimensions(dimensions int) error

// Get the capabilities supported by this provider
GetCapabilities() *EmbeddingProviderCapabilities
```

## Configuration

### EmbeddingProviderConfig Structure

```go
type EmbeddingProviderConfig struct {
    // Basic provider information
    Type     EmbeddingProviderType `json:"type"`
    Name     string                `json:"name,omitempty"`
    Model    string                `json:"model"`
    Endpoint string                `json:"endpoint,omitempty"`
    
    // Authentication
    APIKey    string `json:"api_key,omitempty"`
    APISecret string `json:"api_secret,omitempty"`
    Token     string `json:"token,omitempty"`
    
    // Model configuration
    Dimensions         int      `json:"dimensions,omitempty"`
    MaxTokensPerText   int      `json:"max_tokens_per_text,omitempty"`
    MaxBatchSize       int      `json:"max_batch_size,omitempty"`
    SupportedModels    []string `json:"supported_models,omitempty"`
    
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
}
```

### EmbeddingOptions Structure

```go
type EmbeddingOptions struct {
    // Model-specific options
    Model      string `json:"model,omitempty"`
    Dimensions int    `json:"dimensions,omitempty"`
    
    // Processing options
    Normalize     bool   `json:"normalize,omitempty"`      // Normalize embeddings to unit length
    Truncate      bool   `json:"truncate,omitempty"`       // Truncate input if too long
    EncodingFormat string `json:"encoding_format,omitempty"` // float, base64, etc.
    
    // Performance options
    BatchSize    int           `json:"batch_size,omitempty"`
    Timeout      time.Duration `json:"timeout,omitempty"`
    MaxRetries   int           `json:"max_retries,omitempty"`
    RetryDelay   time.Duration `json:"retry_delay,omitempty"`
    
    // Caching options
    EnableCaching bool          `json:"enable_caching,omitempty"`
    CacheTTL      time.Duration `json:"cache_ttl,omitempty"`
    
    // Provider-specific options
    CustomOptions map[string]interface{} `json:"custom_options,omitempty"`
}
```

## Usage Examples

### Basic Embedding Generation

```go
// Create provider configuration
config := DefaultEmbeddingProviderConfig(EmbeddingProviderOpenAI)
config.APIKey = "your-openai-api-key"
config.Model = "text-embedding-3-small"

// Create provider (using factory)
provider, err := factory.CreateProvider(config)
if err != nil {
    log.Fatal(err)
}

// Generate single embedding
ctx := context.Background()
embedding, err := provider.GenerateEmbedding(ctx, "This is a test text")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Embedding dimensions: %d\n", len(embedding))
```

### Batch Embedding Generation

```go
texts := []string{
    "First text to embed",
    "Second text to embed",
    "Third text to embed",
}

embeddings, err := provider.GenerateBatchEmbeddings(ctx, texts)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Generated %d embeddings\n", len(embeddings))
for i, embedding := range embeddings {
    fmt.Printf("Text %d: %d dimensions\n", i+1, len(embedding))
}
```

### Embedding with Custom Options

```go
options := &EmbeddingOptions{
    Normalize:     true,
    Truncate:      true,
    BatchSize:     50,
    Timeout:       30 * time.Second,
    EnableCaching: true,
    CacheTTL:      1 * time.Hour,
}

embedding, err := provider.GenerateEmbeddingWithOptions(ctx, "Text with custom options", options)
if err != nil {
    log.Fatal(err)
}
```

### Deduplication and Embedding

```go
texts := []string{
    "Unique text 1",
    "Unique text 2",
    "Unique text 1", // Duplicate
    "Unique text 3",
    "Unique text 2", // Duplicate
}

// This will only generate embeddings for unique texts
uniqueEmbeddings, err := provider.DeduplicateAndEmbed(ctx, texts)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Generated embeddings for %d unique texts\n", len(uniqueEmbeddings))
```

### Cached Embedding Generation

```go
// Generate embedding with 24-hour cache
embedding, err := provider.GenerateEmbeddingCached(ctx, "Cached text", 24*time.Hour)
if err != nil {
    log.Fatal(err)
}

// Subsequent calls with the same text will return cached result
cachedEmbedding, err := provider.GenerateEmbeddingCached(ctx, "Cached text", 24*time.Hour)
if err != nil {
    log.Fatal(err)
}
```

### Health Monitoring

```go
// Check provider health
err := provider.Health(ctx)
if err != nil {
    log.Printf("Provider unhealthy: %v", err)
}

// Get usage statistics
usage, err := provider.GetUsage(ctx)
if err == nil {
    fmt.Printf("Total requests: %d\n", usage.TotalRequests)
    fmt.Printf("Success rate: %.2f%%\n", 
        float64(usage.SuccessfulRequests)/float64(usage.TotalRequests)*100)
    fmt.Printf("Cache hit rate: %.2f%%\n", usage.CacheHitRate*100)
}

// Check rate limits
rateLimit, err := provider.GetRateLimit(ctx)
if err == nil {
    fmt.Printf("Requests remaining: %d/%d\n", 
        rateLimit.RequestsRemaining, rateLimit.RequestsPerMinute)
    fmt.Printf("Tokens remaining: %d/%d\n", 
        rateLimit.TokensRemaining, rateLimit.TokensPerMinute)
}
```

### Provider Capabilities

```go
caps := provider.GetCapabilities()

fmt.Printf("Supports batching: %v\n", caps.SupportsBatching)
fmt.Printf("Supports custom dimensions: %v\n", caps.SupportsCustomDims)
fmt.Printf("Max batch size: %d\n", caps.MaxBatchSize)
fmt.Printf("Max tokens per text: %d\n", caps.MaxTokensPerText)
fmt.Printf("Supported models: %v\n", caps.SupportedModels)
fmt.Printf("Supported dimensions: %v\n", caps.SupportedDimensions)

if caps.SupportsCustomDims {
    err := provider.SetCustomDimensions(512)
    if err != nil {
        log.Printf("Failed to set custom dimensions: %v", err)
    }
}
```

### Cost Estimation

```go
text := "This is a sample text for cost estimation"

// Estimate token count
tokenCount, err := provider.EstimateTokenCount(text)
if err != nil {
    log.Fatal(err)
}

// Estimate cost
cost, err := provider.EstimateCost(tokenCount)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Estimated tokens: %d\n", tokenCount)
fmt.Printf("Estimated cost: $%.6f\n", cost)
```

## Provider-Specific Examples

### OpenAI Provider

```go
config := DefaultEmbeddingProviderConfig(EmbeddingProviderOpenAI)
config.APIKey = "sk-..."
config.Model = "text-embedding-3-large"
config.Dimensions = 3072 // Custom dimensions for text-embedding-3-large

provider, err := factory.CreateProvider(config)
if err != nil {
    log.Fatal(err)
}

// OpenAI supports custom dimensions
if provider.SupportsCustomDimensions() {
    err = provider.SetCustomDimensions(1024)
    if err != nil {
        log.Printf("Failed to set custom dimensions: %v", err)
    }
}
```

### Ollama Provider

```go
config := DefaultEmbeddingProviderConfig(EmbeddingProviderOllama)
config.Endpoint = "http://localhost:11434"
config.Model = "nomic-embed-text"

provider, err := factory.CreateProvider(config)
if err != nil {
    log.Fatal(err)
}

// Ollama is local, so no API key needed
embedding, err := provider.GenerateEmbedding(ctx, "Local embedding generation")
if err != nil {
    log.Fatal(err)
}
```

### Local Sentence Transformers

```go
config := DefaultEmbeddingProviderConfig(EmbeddingProviderLocal)
config.Model = "all-MiniLM-L6-v2"
config.CustomOptions = map[string]interface{}{
    "model_path": "/path/to/local/models",
    "device":     "cpu", // or "cuda" for GPU
}

provider, err := factory.CreateProvider(config)
if err != nil {
    log.Fatal(err)
}
```

## Advanced Features

### Provider Factory

```go
factory := NewEmbeddingProviderFactory()

// Create provider with defaults
provider, err := factory.CreateProviderWithDefaults(
    EmbeddingProviderOpenAI, 
    "your-api-key", 
    "text-embedding-3-small")

// List supported providers
providers := factory.ListSupportedProviders()
fmt.Printf("Supported providers: %v\n", providers)

// Get provider capabilities
caps, err := factory.GetProviderCapabilities(EmbeddingProviderOpenAI)
if err == nil {
    fmt.Printf("OpenAI capabilities: %+v\n", caps)
}
```

### Provider Manager (Multi-Provider Support)

```go
manager := NewEmbeddingProviderManager()

// Add multiple providers with priorities
manager.AddProvider("primary", openaiProvider, 1)
manager.AddProvider("fallback", ollamaProvider, 2)
manager.AddProvider("local", localProvider, 3)

// Enable automatic failover
manager.SetFailoverEnabled(true)
manager.SetLoadBalancing(EmbeddingLoadBalancePriority)

// Get best available provider
provider, err := manager.GetBestProvider(ctx)
if err != nil {
    log.Fatal(err)
}

// Generate embedding with automatic failover
embedding, err := manager.GenerateEmbeddingWithFailover(ctx, "Text with failover")
if err != nil {
    log.Fatal(err)
}
```

### Caching Provider

```go
cache := NewEmbeddingCacheProvider()

// Generate cache key
key := cache.GenerateKey("sample text", options)

// Check cache first
if cached, err := cache.Get(ctx, key); err == nil {
    fmt.Println("Using cached embedding")
    return cached
}

// Generate and cache embedding
embedding, err := provider.GenerateEmbedding(ctx, "sample text")
if err == nil {
    cache.Set(ctx, key, embedding, 24*time.Hour)
}
```

### Event Handling

```go
// Create event handler
handler := &MyEmbeddingEventHandler{}

// Set event handler on provider (if supported)
if providerWithEvents, ok := provider.(EmbeddingProviderWithEvents); ok {
    providerWithEvents.SetEventHandler(handler)
    
    // Get metrics
    metrics := providerWithEvents.GetMetrics()
    fmt.Printf("Provider metrics: %+v\n", metrics)
}

type MyEmbeddingEventHandler struct{}

func (h *MyEmbeddingEventHandler) HandleEvent(event EmbeddingProviderEvent) {
    switch event.Type {
    case EmbeddingEventTypeRequest:
        log.Printf("Embedding request: %s", event.Message)
    case EmbeddingEventTypeError:
        log.Printf("Embedding error: %s", event.Error)
    case EmbeddingEventTypeCacheHit:
        log.Printf("Cache hit for embedding request")
    }
}
```

## Error Handling

The interface uses structured error handling with `ExtractorError`:

```go
embedding, err := provider.GenerateEmbedding(ctx, text)
if err != nil {
    if extractorErr, ok := err.(*ExtractorError); ok {
        switch extractorErr.Type {
        case "rate_limit":
            // Handle rate limiting
            log.Printf("Rate limited, retry after: %v", extractorErr.Message)
            time.Sleep(time.Second * 10)
            // Retry logic
        case "authentication":
            // Handle auth errors
            log.Fatal("Invalid API key")
        case "validation":
            // Handle validation errors
            log.Printf("Validation error: %s", extractorErr.Message)
        case "timeout":
            // Handle timeout errors
            log.Printf("Request timeout: %s", extractorErr.Message)
        }
    }
}
```

## Best Practices

1. **Always use context.Context** for cancellation and timeouts
2. **Handle rate limiting** gracefully with exponential backoff
3. **Validate configurations** before creating providers
4. **Monitor usage and health** in production environments
5. **Use batch processing** for multiple texts to improve performance
6. **Enable caching** to reduce API calls and improve response times
7. **Use deduplication** when processing large datasets with potential duplicates
8. **Set appropriate timeouts** for your use case
9. **Test with mock providers** during development
10. **Monitor costs** when using paid embedding services

## Integration with AI Memory System

The EmbeddingProvider interface integrates seamlessly with the AI Memory system:

```go
// Create embedding provider
config := DefaultEmbeddingProviderConfig(EmbeddingProviderOpenAI)
config.APIKey = "your-openai-key"
embeddingProvider, _ := factory.CreateProvider(config)

// Create AutoEmbedder with the provider
autoEmbedder := NewAutoEmbedder("primary", cache)
autoEmbedder.AddProvider("primary", embeddingProvider)

// Create memory engine with embedding provider
memoryEngine := NewMemoryEngine(llmExtractor, graphStore, vectorStore, relationalStore)
memoryEngine.SetEmbeddingProvider(embeddingProvider)

// Add data, which will eventually be embedded
dataPoint, _ := memoryEngine.Add(ctx, "I'm learning about embeddings", engine.WithMetadata(metadata))
enriched, _ := memoryEngine.Cognify(ctx, dataPoint) // This will generate embeddings
memoryEngine.Memify(ctx, enriched) // This will store embeddings in vector store

// Search with embedding-powered similarity
query := &SearchQuery{
    Text: "How do embeddings work?",
    Mode: ModeSemanticSearch, // Uses embedding similarity
}
results, _ := memoryEngine.Search(ctx, query)
```

This comprehensive interface provides a robust foundation for multi-provider embedding integration in the AI Memory system, supporting both current and future embedding provider implementations while maintaining consistency, performance, and reliability.