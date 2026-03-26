# Provider Factory and Configuration System

## Overview

The Provider Factory and Configuration System provides a comprehensive framework for creating, managing, and configuring multiple LLM and embedding providers in the AI Memory Integration system. This system implements the factory pattern with configuration validation, capability discovery, provider registration, and health monitoring.

## Architecture

### Core Components

1. **Provider Factories**: Create and configure provider instances
2. **Provider Managers**: Handle multiple providers with failover and load balancing
3. **Configuration System**: Validate and manage provider configurations
4. **Capability Discovery**: Query provider features and limitations
5. **Health Monitoring**: Track provider health and performance

### Factory Pattern Implementation

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Provider Factory System                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐             │
│  │ LLM Provider    │    │ Embedding       │    │ Provider        │             │
│  │ Factory         │    │ Provider        │    │ Manager         │             │
│  │                 │    │ Factory         │    │                 │             │
│  │ • Create        │    │                 │    │ • Multi-provider│             │
│  │ • Validate      │    │ • Create        │    │ • Failover      │             │
│  │ • Configure     │    │ • Validate      │    │ • Load Balance  │             │
│  │ • Capabilities  │    │ • Configure     │    │ • Health Check  │             │
│  └─────────────────┘    │ • Capabilities  │    └─────────────────┘             │
│                         │ • Cost Estimate │                                    │
│                         └─────────────────┘                                    │
│                                                                                 │
├─────────────────────────────────────────────────────────────────────────────────┤
│                          Configuration System                                   │
│                                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐             │
│  │ Provider        │    │ Embedding       │    │ Validation      │             │
│  │ Config          │    │ Provider        │    │ System          │             │
│  │                 │    │ Config          │    │                 │             │
│  │ • Authentication│    │                 │    │ • Config Check  │             │
│  │ • Rate Limiting │    │ • Model Config  │    │ • Defaults      │             │
│  │ • Health Check  │    │ • Performance   │    │ • Error Handle  │             │
│  │ • Features      │    │ • Caching       │    │ • Type Safety   │             │
│  └─────────────────┘    │ • Features      │    └─────────────────┘             │
│                         └─────────────────┘                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Provider Factory Interfaces

### LLM Provider Factory

```go
type ProviderFactory interface {
    // Core factory methods
    CreateProvider(config *ProviderConfig) (LLMProvider, error)
    CreateProviderWithDefaults(providerType ProviderType, apiKey, model string) (LLMProvider, error)
    
    // Discovery and capabilities
    ListSupportedProviders() []ProviderType
    GetProviderCapabilities(providerType ProviderType) (*ProviderCapabilities, error)
    
    // Configuration management
    ValidateConfig(config *ProviderConfig) error
    GetDefaultConfig(providerType ProviderType) (*ProviderConfig, error)
    
    // Extensibility
    RegisterCustomProvider(providerType ProviderType, createFunc func(*ProviderConfig) (LLMProvider, error)) error
}
```

### Embedding Provider Factory

```go
type EmbeddingProviderFactory interface {
    // Core factory methods
    CreateProvider(config *EmbeddingProviderConfig) (EmbeddingProvider, error)
    CreateProviderWithDefaults(providerType EmbeddingProviderType, apiKey, model string) (EmbeddingProvider, error)
    
    // Discovery and capabilities
    ListSupportedProviders() []EmbeddingProviderType
    GetProviderCapabilities(providerType EmbeddingProviderType) (*EmbeddingProviderCapabilities, error)
    
    // Configuration management
    ValidateConfig(config *EmbeddingProviderConfig) error
    GetDefaultConfig(providerType EmbeddingProviderType) (*EmbeddingProviderConfig, error)
    
    // Extensibility
    RegisterCustomProvider(providerType EmbeddingProviderType, createFunc func(*EmbeddingProviderConfig) (EmbeddingProvider, error)) error
    
    // Additional embedding-specific methods
    GetSupportedModels(providerType EmbeddingProviderType) ([]string, error)
    EstimateProviderCost(providerType EmbeddingProviderType, tokenCount int) (float64, error)
}
```

## Configuration System

### Provider Configuration Structure

```go
type ProviderConfig struct {
    // Basic provider information
    Type  ProviderType `json:"type"`
    Name  string       `json:"name,omitempty"`
    Model string       `json:"model"`
    
    // Authentication
    APIKey    string `json:"api_key,omitempty"`
    APISecret string `json:"api_secret,omitempty"`
    Token     string `json:"token,omitempty"`
    
    // Endpoints and networking
    Endpoint string        `json:"endpoint,omitempty"`
    BaseURL  string        `json:"base_url,omitempty"`
    Region   string        `json:"region,omitempty"`
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
```

### Embedding Provider Configuration

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
    Dimensions       int      `json:"dimensions,omitempty"`
    MaxTokensPerText int      `json:"max_tokens_per_text,omitempty"`
    MaxBatchSize     int      `json:"max_batch_size,omitempty"`
    SupportedModels  []string `json:"supported_models,omitempty"`
    
    // Performance settings
    DefaultOptions *EmbeddingOptions `json:"default_options,omitempty"`
    Timeout        time.Duration     `json:"timeout,omitempty"`
    
    // Rate limiting, caching, health check configurations...
    // (Similar structure to ProviderConfig)
}
```

## Provider Manager System

### LLM Provider Manager

```go
type ProviderManager interface {
    // Provider management
    AddProvider(name string, provider LLMProvider, priority int) error
    RemoveProvider(name string) error
    GetProvider(name string) (LLMProvider, error)
    ListProviders() map[string]LLMProvider
    
    // Selection and load balancing
    GetBestProvider(ctx context.Context) (LLMProvider, error)
    SetLoadBalancing(strategy LoadBalancingStrategy)
    
    // Health monitoring
    HealthCheck(ctx context.Context) map[string]error
    SetFailoverEnabled(enabled bool)
}
```

### Embedding Provider Manager

```go
type EmbeddingProviderManager interface {
    // Provider management
    AddProvider(name string, provider EmbeddingProvider, priority int) error
    RemoveProvider(name string) error
    GetProvider(name string) (EmbeddingProvider, error)
    ListProviders() map[string]EmbeddingProvider
    
    // Selection and load balancing
    GetBestProvider(ctx context.Context) (EmbeddingProvider, error)
    SetLoadBalancing(strategy EmbeddingLoadBalancingStrategy)
    
    // Health monitoring
    HealthCheck(ctx context.Context) map[string]error
    SetFailoverEnabled(enabled bool)
    
    // Failover operations
    GenerateEmbeddingWithFailover(ctx context.Context, text string) ([]float32, error)
    GenerateBatchEmbeddingsWithFailover(ctx context.Context, texts []string) ([][]float32, error)
}
```

## Usage Examples

### Basic Factory Usage

```go
// Create LLM provider factory
factory := NewProviderFactory()

// Create provider with default configuration
config := DefaultProviderConfig(ProviderOpenAI)
config.APIKey = "your-api-key"
config.Model = "gpt-4"

provider, err := factory.CreateProvider(config)
if err != nil {
    log.Fatal(err)
}

// Use provider
ctx := context.Background()
response, err := provider.GenerateCompletion(ctx, "Hello, world!")
if err != nil {
    log.Fatal(err)
}
fmt.Println(response)
```

### Embedding Provider Factory Usage

```go
// Create embedding provider factory
factory := NewEmbeddingProviderFactory()

// Create provider with defaults
provider, err := factory.CreateProviderWithDefaults(
    EmbeddingProviderOpenAI, 
    "your-api-key", 
    "text-embedding-3-small")
if err != nil {
    log.Fatal(err)
}

// Generate embedding
ctx := context.Background()
embedding, err := provider.GenerateEmbedding(ctx, "Hello, world!")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Generated embedding with %d dimensions\n", len(embedding))
```

### Provider Manager Usage

```go
// Create provider manager
manager := NewProviderManager()

// Add multiple providers with priorities
openaiProvider, _ := factory.CreateProviderWithDefaults(ProviderOpenAI, "openai-key", "gpt-4")
ollamaProvider, _ := factory.CreateProviderWithDefaults(ProviderOllama, "", "llama2")

manager.AddProvider("openai", openaiProvider, 1) // Higher priority
manager.AddProvider("ollama", ollamaProvider, 2) // Lower priority

// Configure load balancing
manager.SetLoadBalancing(LoadBalancePriority)
manager.SetFailoverEnabled(true)

// Get best available provider
ctx := context.Background()
provider, err := manager.GetBestProvider(ctx)
if err != nil {
    log.Fatal(err)
}

// Use provider
response, err := provider.GenerateCompletion(ctx, "Hello, world!")
if err != nil {
    log.Fatal(err)
}
```

### Embedding Provider Manager with Failover

```go
// Create embedding provider manager
manager := NewEmbeddingProviderManager()

// Add providers with different priorities
openaiProvider, _ := embeddingFactory.CreateProviderWithDefaults(
    EmbeddingProviderOpenAI, "openai-key", "text-embedding-3-small")
ollamaProvider, _ := embeddingFactory.CreateProviderWithDefaults(
    EmbeddingProviderOllama, "", "nomic-embed-text")

manager.AddProvider("openai", openaiProvider, 1)
manager.AddProvider("ollama", ollamaProvider, 2)

// Configure for cost-based load balancing
manager.SetLoadBalancing(EmbeddingLoadBalanceCostBased)
manager.SetFailoverEnabled(true)

// Generate embedding with automatic failover
ctx := context.Background()
embedding, err := manager.GenerateEmbeddingWithFailover(ctx, "Hello, world!")
if err != nil {
    log.Fatal(err)
}

// Generate batch embeddings with failover
texts := []string{"Hello", "World", "Test"}
embeddings, err := manager.GenerateBatchEmbeddingsWithFailover(ctx, texts)
if err != nil {
    log.Fatal(err)
}
```

## Configuration Validation

### Validation Rules

The configuration system includes comprehensive validation:

1. **Required Fields**: Type, Model, and provider-specific authentication
2. **Provider-Specific Validation**: API keys for cloud providers, endpoints for local providers
3. **Range Validation**: Dimensions, batch sizes, timeouts within acceptable ranges
4. **Feature Compatibility**: Ensure requested features are supported by the provider
5. **Security Validation**: Secure handling of API keys and tokens

### Example Validation

```go
// Validate configuration before creating provider
config := DefaultProviderConfig(ProviderOpenAI)
config.APIKey = "your-api-key"

factory := NewProviderFactory()
if err := factory.ValidateConfig(config); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}

provider, err := factory.CreateProvider(config)
if err != nil {
    log.Fatal(err)
}
```

## Capability Discovery

### Provider Capabilities

Each provider exposes its capabilities through a structured interface:

```go
// Get provider capabilities
factory := NewProviderFactory()
caps, err := factory.GetProviderCapabilities(ProviderOpenAI)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Supports JSON Mode: %v\n", caps.SupportsJSONMode)
fmt.Printf("Supports Streaming: %v\n", caps.SupportsStreaming)
fmt.Printf("Max Context Length: %d\n", caps.MaxContextLength)
fmt.Printf("Available Models: %v\n", caps.AvailableModels)
```

### Embedding Provider Capabilities

```go
// Get embedding provider capabilities
factory := NewEmbeddingProviderFactory()
caps, err := factory.GetProviderCapabilities(EmbeddingProviderOpenAI)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Supports Batching: %v\n", caps.SupportsBatching)
fmt.Printf("Supports Custom Dimensions: %v\n", caps.SupportsCustomDims)
fmt.Printf("Max Batch Size: %d\n", caps.MaxBatchSize)
fmt.Printf("Cost Per Token: %f\n", caps.CostPerToken)
```

## Load Balancing Strategies

### LLM Provider Load Balancing

- **Priority**: Select provider with highest priority (lowest number)
- **Round Robin**: Cycle through providers in order
- **Random**: Randomly select from healthy providers
- **Least Used**: Select provider with lowest usage count

### Embedding Provider Load Balancing

- **Priority**: Select provider with highest priority
- **Round Robin**: Cycle through providers in order
- **Random**: Randomly select from healthy providers
- **Least Used**: Select provider with lowest usage count
- **Cost Based**: Select provider with lowest cost per token
- **Latency Based**: Select provider with lowest average latency

## Health Monitoring

### Health Check System

```go
// Perform health checks on all providers
manager := NewProviderManager()
// ... add providers ...

ctx := context.Background()
healthResults := manager.HealthCheck(ctx)

for providerName, err := range healthResults {
    if err != nil {
        log.Printf("Provider %s is unhealthy: %v", providerName, err)
    } else {
        log.Printf("Provider %s is healthy", providerName)
    }
}
```

### Automatic Failover

The system supports automatic failover when providers become unhealthy:

```go
// Enable automatic failover
manager.SetFailoverEnabled(true)

// The manager will automatically try the next available provider
// if the primary provider fails
provider, err := manager.GetBestProvider(ctx)
if err != nil {
    log.Fatal("All providers are unavailable")
}
```

## Custom Provider Registration

### Registering Custom LLM Providers

```go
// Define custom provider creation function
createCustomProvider := func(config *ProviderConfig) (LLMProvider, error) {
    return &MyCustomLLMProvider{
        apiKey: config.APIKey,
        model:  config.Model,
        // ... custom initialization
    }, nil
}

// Register custom provider
factory := NewProviderFactory()
err := factory.RegisterCustomProvider(ProviderType("my-custom"), createCustomProvider)
if err != nil {
    log.Fatal(err)
}

// Use custom provider
config := DefaultProviderConfig(ProviderType("my-custom"))
config.APIKey = "custom-api-key"
provider, err := factory.CreateProvider(config)
```

### Registering Custom Embedding Providers

```go
// Define custom embedding provider creation function
createCustomEmbeddingProvider := func(config *EmbeddingProviderConfig) (EmbeddingProvider, error) {
    return &MyCustomEmbeddingProvider{
        apiKey:     config.APIKey,
        model:      config.Model,
        dimensions: config.Dimensions,
        // ... custom initialization
    }, nil
}

// Register custom embedding provider
factory := NewEmbeddingProviderFactory()
err := factory.RegisterCustomProvider(EmbeddingProviderType("my-custom"), createCustomEmbeddingProvider)
if err != nil {
    log.Fatal(err)
}
```

## Error Handling

### Structured Error Handling

The system uses structured error handling with `ExtractorError`:

```go
provider, err := factory.CreateProvider(config)
if err != nil {
    if extractorErr, ok := err.(*ExtractorError); ok {
        switch extractorErr.Type {
        case "validation":
            log.Printf("Configuration validation error: %s", extractorErr.Message)
        case "authentication":
            log.Printf("Authentication error: %s", extractorErr.Message)
        case "unsupported":
            log.Printf("Unsupported provider: %s", extractorErr.Message)
        default:
            log.Printf("Unknown error: %s", extractorErr.Message)
        }
    }
}
```

## Best Practices

### Configuration Management

1. **Use Default Configurations**: Start with `DefaultProviderConfig()` and modify as needed
2. **Validate Early**: Always validate configurations before creating providers
3. **Secure API Keys**: Store API keys securely and never log them
4. **Environment Variables**: Use environment variables for sensitive configuration
5. **Configuration Files**: Support JSON/YAML configuration files for complex setups

### Provider Management

1. **Health Monitoring**: Regularly perform health checks on providers
2. **Failover Strategy**: Always configure failover for production systems
3. **Load Balancing**: Choose appropriate load balancing strategy for your use case
4. **Resource Cleanup**: Always close providers when done to free resources
5. **Monitoring**: Track provider usage and performance metrics

### Performance Optimization

1. **Connection Pooling**: Reuse provider instances when possible
2. **Batch Operations**: Use batch operations for embedding providers
3. **Caching**: Enable caching for frequently used operations
4. **Rate Limiting**: Respect provider rate limits to avoid throttling
5. **Timeout Configuration**: Set appropriate timeouts for your use case

## Integration with AI Memory System

The Provider Factory and Configuration System integrates seamlessly with the AI Memory system:

```go
// Create providers using factories
llmFactory := NewProviderFactory()
embeddingFactory := NewEmbeddingProviderFactory()

// Create LLM provider for entity extraction
llmConfig := DefaultProviderConfig(ProviderDeepSeek)
llmConfig.APIKey = "deepseek-api-key"
llmProvider, _ := llmFactory.CreateProvider(llmConfig)

// Create embedding provider for vector generation
embeddingConfig := DefaultEmbeddingProviderConfig(EmbeddingProviderOpenAI)
embeddingConfig.APIKey = "openai-api-key"
embeddingProvider, _ := embeddingFactory.CreateProvider(embeddingConfig)

// Create extractor with LLM provider
extractor := NewBasicExtractor(llmProvider, DefaultExtractionConfig())

// Create AutoEmbedder with embedding provider
autoEmbedder := NewAutoEmbedder("primary", cache)
autoEmbedder.AddProvider("primary", embeddingProvider)

// Create memory engine with providers
memoryEngine := NewMemoryEngine(extractor, graphStore, vectorStore, relationalStore)
memoryEngine.SetEmbeddingProvider(embeddingProvider)

// Use memory engine// Add data, kicking off the pipeline
dataPoint, _ := memoryEngine.Add(ctx, "I'm learning about AI memory systems", engine.WithMetadata(metadata))
enriched, _ := memoryEngine.Cognify(ctx, dataPoint)
memoryEngine.Memify(ctx, enriched)
```

This comprehensive factory and configuration system provides a robust foundation for managing multiple LLM and embedding providers in the AI Memory Integration system, supporting current requirements while being extensible for future enhancements.