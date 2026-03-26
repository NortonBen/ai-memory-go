# Health Check and Failover Implementation

## Overview

This document describes the health check and failover logic implementation for LLM and embedding providers in the AI Memory Integration library. The implementation provides automatic provider health monitoring, intelligent failover between providers, retry logic with exponential backoff, and comprehensive availability tracking.

## Components

### 1. HealthChecker

The `HealthChecker` performs periodic health checks on all registered providers to ensure they are available and responsive.

**Features:**
- Configurable check interval and timeout
- Background worker for automatic health monitoring
- Support for both LLM and embedding providers
- Graceful start/stop with proper cleanup
- Thread-safe operations

**Configuration:**
```go
healthChecker := NewHealthChecker(
    5*time.Minute,  // Check interval
    30*time.Second, // Check timeout
    3,              // Max consecutive fails before marking unhealthy
)
```

**Usage:**
```go
// Start health checking
ctx := context.Background()
healthChecker.Start(ctx, providerManager)

// Stop health checking
defer healthChecker.Stop()
```

### 2. FailoverManager

The `FailoverManager` handles automatic failover between providers when operations fail, with intelligent retry logic and exponential backoff.

**Features:**
- Automatic failover to backup providers
- Exponential backoff with configurable multiplier
- Maximum retry delay cap
- Retryable error detection
- Circuit breaker support (planned)
- Thread-safe operations

**Configuration:**
```go
failoverManager := NewFailoverManager()
failoverManager.SetMaxRetries(3)
failoverManager.SetRetryDelay(1 * time.Second)
failoverManager.SetBackoffMultiplier(2.0)
failoverManager.SetMaxRetryDelay(30 * time.Second)
```

**Retry Logic:**
- Attempt 0: 1 second delay
- Attempt 1: 2 seconds delay (1s * 2^1)
- Attempt 2: 4 seconds delay (1s * 2^2)
- Attempt 3: 8 seconds delay (1s * 2^3)
- Maximum delay capped at configured max (default 30s)

**Retryable Errors:**
- Rate limit errors (429)
- Timeout errors (408)
- Server errors (500-599)
- Connection errors (connection refused, connection reset)
- Temporary failures
- Health check failures

**Usage:**
```go
// Execute operation with automatic failover
err := failoverManager.ExecuteWithFailover(
    ctx,
    []string{"provider1", "provider2", "provider3"},
    providerManager,
    func(provider LLMProvider) error {
        return provider.GenerateCompletion(ctx, prompt)
    },
)
```

### 3. ProviderAvailabilityTracker

The `ProviderAvailabilityTracker` tracks provider availability metrics and health status over time.

**Features:**
- Success/failure rate tracking
- Consecutive failure counting
- Average latency measurement
- Health threshold-based status
- Per-provider metrics
- Thread-safe operations

**Metrics Tracked:**
- Total requests
- Successful requests
- Failed requests
- Last success timestamp
- Last failure timestamp
- Consecutive failures
- Availability rate (success rate)
- Average latency
- Last health check timestamp
- Health status

**Configuration:**
```go
tracker := NewProviderAvailabilityTracker(
    1*time.Hour, // Metrics window size
    0.8,         // Health threshold (80% success rate)
)
```

**Usage:**
```go
// Record successful operation
tracker.RecordSuccess("provider1", 100*time.Millisecond)

// Record failed operation
tracker.RecordFailure("provider1")

// Record health check result
tracker.RecordHealthCheck("provider1", true)

// Get metrics
metrics := tracker.GetMetrics("provider1")
fmt.Printf("Availability: %.2f%%\n", metrics.AvailabilityRate * 100)
fmt.Printf("Average Latency: %v\n", metrics.AverageLatency)
fmt.Printf("Consecutive Fails: %d\n", metrics.ConsecutiveFails)
```

## Integration with Provider Managers

Both `DefaultProviderManager` and `DefaultEmbeddingProviderManager` are integrated with health check and failover components.

### Provider Manager Integration

```go
// Create provider manager (includes health checker, failover manager, and availability tracker)
manager := NewProviderManager()

// Add providers with priorities
manager.AddProvider("openai", openaiProvider, 1)    // Priority 1 (highest)
manager.AddProvider("anthropic", anthropicProvider, 2) // Priority 2
manager.AddProvider("gemini", geminiProvider, 3)    // Priority 3

// Start automatic health checking
ctx := context.Background()
manager.(*DefaultProviderManager).StartHealthChecks(ctx)
defer manager.(*DefaultProviderManager).StopHealthChecks()

// Execute operation with automatic failover
err := manager.(*DefaultProviderManager).ExecuteWithFailover(ctx, func(provider LLMProvider) error {
    return provider.GenerateCompletion(ctx, "Hello, world!")
})

// Get provider metrics
metrics := manager.(*DefaultProviderManager).GetProviderMetrics("openai")
fmt.Printf("OpenAI Availability: %.2f%%\n", metrics.AvailabilityRate * 100)

// Get all provider metrics
allMetrics := manager.(*DefaultProviderManager).GetAllProviderMetrics()
for name, metrics := range allMetrics {
    fmt.Printf("%s: %.2f%% available\n", name, metrics.AvailabilityRate * 100)
}
```

### Embedding Provider Manager Integration

```go
// Create embedding provider manager
embeddingManager := NewEmbeddingProviderManager()

// Add providers with priorities
embeddingManager.AddProvider("openai", openaiEmbedding, 1)
embeddingManager.AddProvider("ollama", ollamaEmbedding, 2)

// Start automatic health checking
embeddingManager.(*DefaultEmbeddingProviderManager).StartHealthChecks(ctx)
defer embeddingManager.(*DefaultEmbeddingProviderManager).StopHealthChecks()

// Execute operation with automatic failover
err := embeddingManager.(*DefaultEmbeddingProviderManager).ExecuteWithFailover(ctx, func(provider EmbeddingProvider) error {
    _, err := provider.GenerateEmbedding(ctx, "Sample text")
    return err
})
```

## Failover Workflow

When an operation fails, the failover process follows these steps:

1. **Initial Attempt**: Try the highest priority provider
2. **Retry Logic**: If the error is retryable, retry with exponential backoff
3. **Failover**: If all retries fail, move to the next priority provider
4. **Repeat**: Continue through all providers until success or exhaustion
5. **Metrics Update**: Record success/failure in availability tracker
6. **Health Update**: Update provider health status based on results

### Example Failover Scenario

```
Provider Priority: OpenAI (1) → Anthropic (2) → Gemini (3)

1. Try OpenAI
   - Attempt 1: Fail (timeout) → Wait 1s
   - Attempt 2: Fail (timeout) → Wait 2s
   - Attempt 3: Fail (timeout) → Wait 4s
   - Mark OpenAI as unhealthy

2. Try Anthropic
   - Attempt 1: Fail (rate limit) → Wait 1s
   - Attempt 2: Success ✓
   - Return result
   - Mark Anthropic as healthy
```

## Health Check Workflow

The health checker runs in the background and performs these steps:

1. **Periodic Check**: Every configured interval (default 5 minutes)
2. **Parallel Checks**: Check all providers concurrently
3. **Timeout**: Each check has a timeout (default 30 seconds)
4. **Status Update**: Update provider health status based on results
5. **Metrics Recording**: Record health check results in availability tracker
6. **Consecutive Failures**: Track consecutive failures for circuit breaker logic

### Health Check Example

```go
// Health check runs automatically every 5 minutes
// Manual health check can also be triggered:
results := manager.HealthCheck(ctx)
for providerName, err := range results {
    if err != nil {
        fmt.Printf("%s is unhealthy: %v\n", providerName, err)
    } else {
        fmt.Printf("%s is healthy\n", providerName)
    }
}
```

## Configuration Best Practices

### Production Settings

```go
// Health Checker
healthChecker := NewHealthChecker(
    5*time.Minute,   // Check every 5 minutes
    30*time.Second,  // 30 second timeout
    3,               // Mark unhealthy after 3 consecutive failures
)

// Failover Manager
failoverManager := NewFailoverManager()
failoverManager.SetMaxRetries(3)                    // Try each provider 3 times
failoverManager.SetRetryDelay(1 * time.Second)      // Start with 1 second delay
failoverManager.SetBackoffMultiplier(2.0)           // Double delay each retry
failoverManager.SetMaxRetryDelay(30 * time.Second)  // Cap at 30 seconds

// Availability Tracker
tracker := NewProviderAvailabilityTracker(
    1*time.Hour, // Track metrics over 1 hour window
    0.8,         // Require 80% success rate for healthy status
)
```

### Development Settings

```go
// Health Checker (more frequent checks)
healthChecker := NewHealthChecker(
    1*time.Minute,   // Check every minute
    10*time.Second,  // 10 second timeout
    2,               // Mark unhealthy after 2 consecutive failures
)

// Failover Manager (faster retries)
failoverManager := NewFailoverManager()
failoverManager.SetMaxRetries(2)                    // Try each provider 2 times
failoverManager.SetRetryDelay(500 * time.Millisecond) // Start with 500ms delay
failoverManager.SetBackoffMultiplier(1.5)           // 1.5x delay each retry
failoverManager.SetMaxRetryDelay(5 * time.Second)   // Cap at 5 seconds

// Availability Tracker (shorter window)
tracker := NewProviderAvailabilityTracker(
    15*time.Minute, // Track metrics over 15 minute window
    0.7,            // Require 70% success rate for healthy status
)
```

## Monitoring and Observability

### Metrics to Monitor

1. **Provider Availability Rate**: Percentage of successful requests
2. **Average Latency**: Response time for successful requests
3. **Consecutive Failures**: Number of consecutive failed requests
4. **Health Status**: Current health status (healthy/unhealthy)
5. **Failover Events**: Number of times failover occurred
6. **Retry Attempts**: Number of retry attempts per request

### Example Monitoring Code

```go
// Periodic metrics reporting
ticker := time.NewTicker(1 * time.Minute)
defer ticker.Stop()

for range ticker.C {
    allMetrics := manager.(*DefaultProviderManager).GetAllProviderMetrics()
    
    for name, metrics := range allMetrics {
        log.Printf("Provider: %s", name)
        log.Printf("  Availability: %.2f%%", metrics.AvailabilityRate * 100)
        log.Printf("  Avg Latency: %v", metrics.AverageLatency)
        log.Printf("  Consecutive Fails: %d", metrics.ConsecutiveFails)
        log.Printf("  Total Requests: %d", metrics.TotalRequests)
        log.Printf("  Successful: %d", metrics.SuccessfulReqs)
        log.Printf("  Failed: %d", metrics.FailedRequests)
        log.Printf("  Healthy: %v", metrics.IsHealthy)
    }
}
```

## Error Handling

### Retryable Errors

The following errors trigger automatic retry:
- HTTP 429 (Too Many Requests)
- HTTP 408 (Request Timeout)
- HTTP 500-599 (Server Errors)
- Connection refused
- Connection reset
- Temporary failures
- Rate limit errors
- Health check failures

### Non-Retryable Errors

The following errors do NOT trigger retry:
- HTTP 400 (Bad Request)
- HTTP 401 (Unauthorized)
- HTTP 403 (Forbidden)
- HTTP 404 (Not Found)
- Validation errors
- Configuration errors

## Thread Safety

All components are thread-safe and can be used concurrently:
- `HealthChecker`: Uses mutex for state management
- `FailoverManager`: Uses mutex for configuration access
- `ProviderAvailabilityTracker`: Uses RWMutex for metrics access
- Provider managers: Use RWMutex for provider map access

## Testing

Comprehensive tests are provided in `health_check_test.go`:

```bash
# Run all health check tests
go test -v -run TestHealthChecker ./extractor/

# Run failover manager tests
go test -v -run TestFailoverManager ./extractor/

# Run availability tracker tests
go test -v -run TestProviderAvailabilityTracker ./extractor/

# Run integration tests
go test -v -run TestFailoverIntegration ./extractor/
```

## Future Enhancements

1. **Circuit Breaker**: Automatically disable providers after threshold failures
2. **Adaptive Retry**: Adjust retry delays based on provider response patterns
3. **Cost-Based Routing**: Route requests based on provider costs
4. **Latency-Based Routing**: Route requests to fastest providers
5. **Geographic Routing**: Route requests based on geographic proximity
6. **Load Shedding**: Reject requests when all providers are unhealthy
7. **Metrics Export**: Export metrics to Prometheus/StatsD
8. **Alert Integration**: Send alerts when providers become unhealthy

## Requirements Validation

This implementation satisfies the following requirements:

### Requirement 2 (Multi-Provider LLM Support)
- ✅ Criterion 4: "WHEN a provider is unavailable, THE Memory_Engine SHALL gracefully fallback to alternative providers"
  - Implemented via `FailoverManager.ExecuteWithFailover()`
  - Automatic retry with exponential backoff
  - Priority-based provider selection

### Requirement 9 (Performance and Production Readiness)
- ✅ Criterion 5: "THE Memory_Engine SHALL provide metrics and monitoring capabilities for production deployment"
  - Implemented via `ProviderAvailabilityTracker`
  - Comprehensive metrics: availability rate, latency, failure counts
  - Health status tracking
  - Per-provider and aggregate metrics

## Summary

The health check and failover implementation provides:
- **Reliability**: Automatic failover ensures high availability
- **Observability**: Comprehensive metrics for monitoring
- **Performance**: Efficient health checking with minimal overhead
- **Flexibility**: Configurable retry logic and health thresholds
- **Production-Ready**: Thread-safe, well-tested, and documented
