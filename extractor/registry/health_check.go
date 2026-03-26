// Package registry provides health check and failover logic for LLM and embedding providers
package registry

import (
	ext "github.com/NortonBen/ai-memory-go/extractor"
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// HealthChecker performs health checks on providers
type HealthChecker struct {
	mu                  sync.RWMutex
	checkInterval       time.Duration
	checkTimeout        time.Duration
	maxConsecutiveFails int
	enabled             bool
	stopChan            chan struct{}
	wg                  sync.WaitGroup
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(interval, timeout time.Duration, maxConsecutiveFails int) *HealthChecker {
	return &HealthChecker{
		checkInterval:       interval,
		checkTimeout:        timeout,
		maxConsecutiveFails: maxConsecutiveFails,
		enabled:             true,
		stopChan:            make(chan struct{}),
	}
}

// Start starts the health checker background worker
func (hc *HealthChecker) Start(ctx context.Context, manager ext.ProviderManager) {
	hc.mu.Lock()
	if !hc.enabled {
		hc.mu.Unlock()
		return
	}
	hc.mu.Unlock()

	hc.wg.Add(1)
	go hc.healthCheckWorker(ctx, manager)
}

// StartEmbedding starts the health checker for embedding providers
func (hc *HealthChecker) StartEmbedding(ctx context.Context, manager ext.EmbeddingProviderManager) {
	hc.mu.Lock()
	if !hc.enabled {
		hc.mu.Unlock()
		return
	}
	hc.mu.Unlock()

	hc.wg.Add(1)
	go hc.embeddingHealthCheckWorker(ctx, manager)
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.enabled {
		return
	}

	close(hc.stopChan)
	hc.enabled = false
	hc.wg.Wait()
}

// SetEnabled enables or disables health checking
func (hc *HealthChecker) SetEnabled(enabled bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.enabled = enabled
}

// healthCheckWorker performs periodic health checks on LLM providers
func (hc *HealthChecker) healthCheckWorker(ctx context.Context, manager ext.ProviderManager) {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.performHealthChecks(ctx, manager)
		case <-hc.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// embeddingHealthCheckWorker performs periodic health checks on embedding providers
func (hc *HealthChecker) embeddingHealthCheckWorker(ctx context.Context, manager ext.EmbeddingProviderManager) {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.performEmbeddingHealthChecks(ctx, manager)
		case <-hc.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performHealthChecks executes health checks on all providers
func (hc *HealthChecker) performHealthChecks(ctx context.Context, manager ext.ProviderManager) {
	checkCtx, cancel := context.WithTimeout(ctx, hc.checkTimeout)
	defer cancel()

	_ = manager.HealthCheck(checkCtx)
}

// performEmbeddingHealthChecks executes health checks on all embedding providers
func (hc *HealthChecker) performEmbeddingHealthChecks(ctx context.Context, manager ext.EmbeddingProviderManager) {
	checkCtx, cancel := context.WithTimeout(ctx, hc.checkTimeout)
	defer cancel()

	_ = manager.HealthCheck(checkCtx)
}

// FailoverManager handles automatic failover between providers
type FailoverManager struct {
	mu                      sync.RWMutex
	enabled                 bool
	maxRetries              int
	retryDelay              time.Duration
	backoffMultiplier       float64
	maxRetryDelay           time.Duration
	circuitBreakerThreshold int
	circuitBreakerTimeout   time.Duration
}

// NewFailoverManager creates a new failover manager
func NewFailoverManager() *FailoverManager {
	return &FailoverManager{
		enabled:                 true,
		maxRetries:              3,
		retryDelay:              1 * time.Second,
		backoffMultiplier:       2.0,
		maxRetryDelay:           30 * time.Second,
		circuitBreakerThreshold: 5,
		circuitBreakerTimeout:   60 * time.Second,
	}
}

// SetEnabled enables or disables failover
func (fm *FailoverManager) SetEnabled(enabled bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.enabled = enabled
}

// IsEnabled returns whether failover is enabled
func (fm *FailoverManager) IsEnabled() bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.enabled
}

// SetMaxRetries sets the maximum number of retry attempts
func (fm *FailoverManager) SetMaxRetries(maxRetries int) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.maxRetries = maxRetries
}

// SetRetryDelay sets the initial retry delay
func (fm *FailoverManager) SetRetryDelay(delay time.Duration) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.retryDelay = delay
}

// SetBackoffMultiplier sets the exponential backoff multiplier
func (fm *FailoverManager) SetBackoffMultiplier(multiplier float64) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.backoffMultiplier = multiplier
}

// SetMaxRetryDelay sets the maximum retry delay
func (fm *FailoverManager) SetMaxRetryDelay(delay time.Duration) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.maxRetryDelay = delay
}

// ExecuteWithFailover executes a function with automatic failover and retry logic
func (fm *FailoverManager) ExecuteWithFailover(
	ctx context.Context,
	providers []string,
	manager ext.ProviderManager,
	operation func(provider ext.LLMProvider) error,
) error {
	fm.mu.RLock()
	enabled := fm.enabled
	maxRetries := fm.maxRetries
	retryDelay := fm.retryDelay
	backoffMultiplier := fm.backoffMultiplier
	maxRetryDelay := fm.maxRetryDelay
	fm.mu.RUnlock()

	if !enabled {
		// If failover is disabled, just try the first provider
		if len(providers) == 0 {
			return ext.NewExtractorError("no_providers", "no providers available", 503)
		}
		provider, err := manager.GetProvider(providers[0])
		if err != nil {
			return err
		}
		return operation(provider)
	}

	var lastErr error
	for _, providerName := range providers {
		provider, err := manager.GetProvider(providerName)
		if err != nil {
			lastErr = err
			continue
		}

		// Try the operation with exponential backoff
		for attempt := 0; attempt <= maxRetries; attempt++ {
			err = operation(provider)
			if err == nil {
				return nil
			}

			lastErr = err

			// Check if error is retryable
			if !fm.isRetryableError(err) {
				break
			}

			// Don't sleep after the last attempt
			if attempt < maxRetries {
				delay := fm.calculateBackoff(attempt, retryDelay, backoffMultiplier, maxRetryDelay)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	if lastErr != nil {
		return fmt.Errorf("all providers failed: %w", lastErr)
	}
	return ext.NewExtractorError("all_providers_failed", "all providers failed", 503)
}

// ExecuteEmbeddingWithFailover executes an embedding operation with automatic failover
func (fm *FailoverManager) ExecuteEmbeddingWithFailover(
	ctx context.Context,
	providers []string,
	manager ext.EmbeddingProviderManager,
	operation func(provider ext.EmbeddingProvider) error,
) error {
	fm.mu.RLock()
	enabled := fm.enabled
	maxRetries := fm.maxRetries
	retryDelay := fm.retryDelay
	backoffMultiplier := fm.backoffMultiplier
	maxRetryDelay := fm.maxRetryDelay
	fm.mu.RUnlock()

	if !enabled {
		// If failover is disabled, just try the first provider
		if len(providers) == 0 {
			return ext.NewExtractorError("no_providers", "no embedding providers available", 503)
		}
		provider, err := manager.GetProvider(providers[0])
		if err != nil {
			return err
		}
		return operation(provider)
	}

	var lastErr error
	for _, providerName := range providers {
		provider, err := manager.GetProvider(providerName)
		if err != nil {
			lastErr = err
			continue
		}

		// Try the operation with exponential backoff
		for attempt := 0; attempt <= maxRetries; attempt++ {
			err = operation(provider)
			if err == nil {
				return nil
			}

			lastErr = err

			// Check if error is retryable
			if !fm.isRetryableError(err) {
				break
			}

			// Don't sleep after the last attempt
			if attempt < maxRetries {
				delay := fm.calculateBackoff(attempt, retryDelay, backoffMultiplier, maxRetryDelay)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	if lastErr != nil {
		return fmt.Errorf("all embedding providers failed: %w", lastErr)
	}
	return ext.NewExtractorError("all_providers_failed", "all embedding providers failed", 503)
}

// calculateBackoff calculates the backoff delay for a given attempt
func (fm *FailoverManager) calculateBackoff(attempt int, initialDelay time.Duration, multiplier float64, maxDelay time.Duration) time.Duration {
	delay := float64(initialDelay) * math.Pow(multiplier, float64(attempt))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}
	return time.Duration(delay)
}

// isRetryableError determines if an error is retryable
func (fm *FailoverManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types that are retryable
	extractorErr, ok := err.(*ext.ExtractorError)
	if ok {
		switch extractorErr.Type {
		case "rate_limit", "timeout", "temporary_failure", "connection_error":
			return true
		case "health_check":
			return true
		}
		// Check HTTP status codes
		if extractorErr.Code >= 500 && extractorErr.Code < 600 {
			return true
		}
		if extractorErr.Code == 429 { // Too Many Requests
			return true
		}
	}

	// Check error message for common retryable patterns
	errMsg := err.Error()
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"rate limit",
		"503",
		"502",
		"500",
	}

	for _, pattern := range retryablePatterns {
		if containsSubstring(errMsg, pattern) {
			return true
		}
	}

	return false
}

// containsSubstring checks if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstringInString(s, substr)))
}

// findSubstringInString performs a simple substring search
func findSubstringInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ProviderAvailabilityTracker tracks provider availability and metrics
type ProviderAvailabilityTracker struct {
	mu              sync.RWMutex
	providerMetrics map[string]*ProviderAvailabilityMetrics
	windowSize      time.Duration
	healthThreshold float64
}

// ProviderAvailabilityMetrics tracks availability metrics for a provider
type ProviderAvailabilityMetrics struct {
	TotalRequests    int64
	SuccessfulReqs   int64
	FailedRequests   int64
	LastSuccess      time.Time
	LastFailure      time.Time
	ConsecutiveFails int
	AvailabilityRate float64
	AverageLatency   time.Duration
	LastHealthCheck  time.Time
	IsHealthy        bool
}

// NewProviderAvailabilityTracker creates a new availability tracker
func NewProviderAvailabilityTracker(windowSize time.Duration, healthThreshold float64) *ProviderAvailabilityTracker {
	return &ProviderAvailabilityTracker{
		providerMetrics: make(map[string]*ProviderAvailabilityMetrics),
		windowSize:      windowSize,
		healthThreshold: healthThreshold,
	}
}

// RecordSuccess records a successful request
func (pat *ProviderAvailabilityTracker) RecordSuccess(providerName string, latency time.Duration) {
	pat.mu.Lock()
	defer pat.mu.Unlock()

	metrics, exists := pat.providerMetrics[providerName]
	if !exists {
		metrics = &ProviderAvailabilityMetrics{
			IsHealthy: true,
		}
		pat.providerMetrics[providerName] = metrics
	}

	metrics.TotalRequests++
	metrics.SuccessfulReqs++
	metrics.LastSuccess = time.Now()
	metrics.ConsecutiveFails = 0

	// Update average latency
	if metrics.AverageLatency == 0 {
		metrics.AverageLatency = latency
	} else {
		metrics.AverageLatency = (metrics.AverageLatency + latency) / 2
	}

	// Update availability rate
	metrics.AvailabilityRate = float64(metrics.SuccessfulReqs) / float64(metrics.TotalRequests)
	metrics.IsHealthy = metrics.AvailabilityRate >= pat.healthThreshold
}

// RecordFailure records a failed request
func (pat *ProviderAvailabilityTracker) RecordFailure(providerName string) {
	pat.mu.Lock()
	defer pat.mu.Unlock()

	metrics, exists := pat.providerMetrics[providerName]
	if !exists {
		metrics = &ProviderAvailabilityMetrics{
			IsHealthy: true,
		}
		pat.providerMetrics[providerName] = metrics
	}

	metrics.TotalRequests++
	metrics.FailedRequests++
	metrics.LastFailure = time.Now()
	metrics.ConsecutiveFails++

	// Update availability rate
	if metrics.TotalRequests > 0 {
		metrics.AvailabilityRate = float64(metrics.SuccessfulReqs) / float64(metrics.TotalRequests)
	}
	metrics.IsHealthy = metrics.AvailabilityRate >= pat.healthThreshold
}

// RecordHealthCheck records a health check result
func (pat *ProviderAvailabilityTracker) RecordHealthCheck(providerName string, isHealthy bool) {
	pat.mu.Lock()
	defer pat.mu.Unlock()

	metrics, exists := pat.providerMetrics[providerName]
	if !exists {
		metrics = &ProviderAvailabilityMetrics{
			IsHealthy: true,
		}
		pat.providerMetrics[providerName] = metrics
	}

	metrics.LastHealthCheck = time.Now()
	metrics.IsHealthy = isHealthy

	if !isHealthy {
		metrics.ConsecutiveFails++
	} else {
		metrics.ConsecutiveFails = 0
	}
}

// GetMetrics returns metrics for a provider
func (pat *ProviderAvailabilityTracker) GetMetrics(providerName string) *ProviderAvailabilityMetrics {
	pat.mu.RLock()
	defer pat.mu.RUnlock()

	metrics, exists := pat.providerMetrics[providerName]
	if !exists {
		return nil
	}

	// Return a copy
	metricsCopy := *metrics
	return &metricsCopy
}

// GetAllMetrics returns metrics for all providers
func (pat *ProviderAvailabilityTracker) GetAllMetrics() map[string]*ProviderAvailabilityMetrics {
	pat.mu.RLock()
	defer pat.mu.RUnlock()

	result := make(map[string]*ProviderAvailabilityMetrics)
	for name, metrics := range pat.providerMetrics {
		metricsCopy := *metrics
		result[name] = &metricsCopy
	}
	return result
}

// IsHealthy checks if a provider is healthy
func (pat *ProviderAvailabilityTracker) IsHealthy(providerName string) bool {
	pat.mu.RLock()
	defer pat.mu.RUnlock()

	metrics, exists := pat.providerMetrics[providerName]
	if !exists {
		return true // Assume healthy if no metrics yet
	}

	return metrics.IsHealthy
}

// ResetMetrics resets metrics for a provider
func (pat *ProviderAvailabilityTracker) ResetMetrics(providerName string) {
	pat.mu.Lock()
	defer pat.mu.Unlock()

	delete(pat.providerMetrics, providerName)
}

// ResetAllMetrics resets metrics for all providers
func (pat *ProviderAvailabilityTracker) ResetAllMetrics() {
	pat.mu.Lock()
	defer pat.mu.Unlock()

	pat.providerMetrics = make(map[string]*ProviderAvailabilityMetrics)
}
