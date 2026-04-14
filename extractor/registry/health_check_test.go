package registry

import (
	ext "github.com/NortonBen/ai-memory-go/extractor"
	"context"
	"errors"
	"testing"
	"time"
)

// TestHealthChecker tests the health checker functionality
func TestHealthChecker(t *testing.T) {
	t.Run("Create health checker", func(t *testing.T) {
		hc := NewHealthChecker(1*time.Minute, 10*time.Second, 3)
		if hc == nil {
			t.Fatal("Expected health checker to be created")
		}
		if hc.checkInterval != 1*time.Minute {
			t.Errorf("Expected check interval 1m, got %v", hc.checkInterval)
		}
		if hc.checkTimeout != 10*time.Second {
			t.Errorf("Expected check timeout 10s, got %v", hc.checkTimeout)
		}
		if hc.maxConsecutiveFails != 3 {
			t.Errorf("Expected max consecutive fails 3, got %d", hc.maxConsecutiveFails)
		}
	})

	t.Run("Enable and disable health checker", func(t *testing.T) {
		hc := NewHealthChecker(1*time.Minute, 10*time.Second, 3)

		if !hc.enabled {
			t.Error("Expected health checker to be enabled by default")
		}

		hc.SetEnabled(false)
		if hc.enabled {
			t.Error("Expected health checker to be disabled")
		}

		hc.SetEnabled(true)
		if !hc.enabled {
			t.Error("Expected health checker to be enabled")
		}
	})

	t.Run("Start and stop health checker", func(t *testing.T) {
		hc := NewHealthChecker(100*time.Millisecond, 10*time.Second, 3)
		manager := NewProviderManager()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// Start health checker
		hc.Start(ctx, manager)

		// Wait a bit for health checks to run
		time.Sleep(300 * time.Millisecond)

		// Stop health checker
		hc.Stop()
	})
}

// TestFailoverManager tests the failover manager functionality
func TestFailoverManager(t *testing.T) {
	t.Run("Create failover manager", func(t *testing.T) {
		fm := NewFailoverManager()
		if fm == nil {
			t.Fatal("Expected failover manager to be created")
		}
		if !fm.enabled {
			t.Error("Expected failover to be enabled by default")
		}
		if fm.maxRetries != 3 {
			t.Errorf("Expected max retries 3, got %d", fm.maxRetries)
		}
	})

	t.Run("Enable and disable failover", func(t *testing.T) {
		fm := NewFailoverManager()

		if !fm.IsEnabled() {
			t.Error("Expected failover to be enabled by default")
		}

		fm.SetEnabled(false)
		if fm.IsEnabled() {
			t.Error("Expected failover to be disabled")
		}

		fm.SetEnabled(true)
		if !fm.IsEnabled() {
			t.Error("Expected failover to be enabled")
		}
	})

	t.Run("Configure retry settings", func(t *testing.T) {
		fm := NewFailoverManager()

		fm.SetMaxRetries(5)
		if fm.maxRetries != 5 {
			t.Errorf("Expected max retries 5, got %d", fm.maxRetries)
		}

		fm.SetRetryDelay(2 * time.Second)
		if fm.retryDelay != 2*time.Second {
			t.Errorf("Expected retry delay 2s, got %v", fm.retryDelay)
		}

		fm.SetBackoffMultiplier(3.0)
		if fm.backoffMultiplier != 3.0 {
			t.Errorf("Expected backoff multiplier 3.0, got %f", fm.backoffMultiplier)
		}

		fm.SetMaxRetryDelay(60 * time.Second)
		if fm.maxRetryDelay != 60*time.Second {
			t.Errorf("Expected max retry delay 60s, got %v", fm.maxRetryDelay)
		}
	})

	t.Run("Calculate backoff", func(t *testing.T) {
		fm := NewFailoverManager()

		// Test exponential backoff
		delay0 := fm.calculateBackoff(0, 1*time.Second, 2.0, 30*time.Second)
		if delay0 != 1*time.Second {
			t.Errorf("Expected delay 1s for attempt 0, got %v", delay0)
		}

		delay1 := fm.calculateBackoff(1, 1*time.Second, 2.0, 30*time.Second)
		if delay1 != 2*time.Second {
			t.Errorf("Expected delay 2s for attempt 1, got %v", delay1)
		}

		delay2 := fm.calculateBackoff(2, 1*time.Second, 2.0, 30*time.Second)
		if delay2 != 4*time.Second {
			t.Errorf("Expected delay 4s for attempt 2, got %v", delay2)
		}

		// Test max delay cap
		delay10 := fm.calculateBackoff(10, 1*time.Second, 2.0, 30*time.Second)
		if delay10 > 30*time.Second {
			t.Errorf("Expected delay capped at 30s, got %v", delay10)
		}
	})

	t.Run("Identify retryable errors", func(t *testing.T) {
		fm := NewFailoverManager()

		// Test retryable errors
		retryableErrors := []error{
			ext.NewExtractorError("rate_limit", "rate limited", 429),
			ext.NewExtractorError("timeout", "request timeout", 408),
			ext.NewExtractorError("temporary_failure", "temporary failure", 503),
			ext.NewExtractorError("health_check", "health check failed", 503),
			errors.New("connection refused"),
			errors.New("connection reset"),
			errors.New("503 service unavailable"),
		}

		for _, err := range retryableErrors {
			if !fm.isRetryableError(err) {
				t.Errorf("Expected error to be retryable: %v", err)
			}
		}

		// Test non-retryable errors
		nonRetryableErrors := []error{
			ext.NewExtractorError("validation", "invalid input", 400),
			ext.NewExtractorError("not_found", "not found", 404),
			ext.NewExtractorError("unauthorized", "unauthorized", 401),
			errors.New("invalid configuration"),
		}

		for _, err := range nonRetryableErrors {
			if fm.isRetryableError(err) {
				t.Errorf("Expected error to be non-retryable: %v", err)
			}
		}
	})
}

// TestProviderAvailabilityTracker tests the availability tracker functionality
func TestProviderAvailabilityTracker(t *testing.T) {
	t.Run("Create availability tracker", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)
		if tracker == nil {
			t.Fatal("Expected availability tracker to be created")
		}
		if tracker.windowSize != 1*time.Hour {
			t.Errorf("Expected window size 1h, got %v", tracker.windowSize)
		}
		if tracker.healthThreshold != 0.8 {
			t.Errorf("Expected health threshold 0.8, got %f", tracker.healthThreshold)
		}
	})

	t.Run("Record success", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		tracker.RecordSuccess("provider1", 100*time.Millisecond)

		metrics := tracker.GetMetrics("provider1")
		if metrics == nil {
			t.Fatal("Expected metrics to be created")
		}
		if metrics.TotalRequests != 1 {
			t.Errorf("Expected 1 total request, got %d", metrics.TotalRequests)
		}
		if metrics.SuccessfulReqs != 1 {
			t.Errorf("Expected 1 successful request, got %d", metrics.SuccessfulReqs)
		}
		if metrics.FailedRequests != 0 {
			t.Errorf("Expected 0 failed requests, got %d", metrics.FailedRequests)
		}
		if metrics.ConsecutiveFails != 0 {
			t.Errorf("Expected 0 consecutive fails, got %d", metrics.ConsecutiveFails)
		}
		if metrics.AvailabilityRate != 1.0 {
			t.Errorf("Expected availability rate 1.0, got %f", metrics.AvailabilityRate)
		}
		if !metrics.IsHealthy {
			t.Error("Expected provider to be healthy")
		}
	})

	t.Run("Record failure", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		tracker.RecordFailure("provider1")

		metrics := tracker.GetMetrics("provider1")
		if metrics == nil {
			t.Fatal("Expected metrics to be created")
		}
		if metrics.TotalRequests != 1 {
			t.Errorf("Expected 1 total request, got %d", metrics.TotalRequests)
		}
		if metrics.SuccessfulReqs != 0 {
			t.Errorf("Expected 0 successful requests, got %d", metrics.SuccessfulReqs)
		}
		if metrics.FailedRequests != 1 {
			t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
		}
		if metrics.ConsecutiveFails != 1 {
			t.Errorf("Expected 1 consecutive fail, got %d", metrics.ConsecutiveFails)
		}
		if metrics.AvailabilityRate != 0.0 {
			t.Errorf("Expected availability rate 0.0, got %f", metrics.AvailabilityRate)
		}
		if metrics.IsHealthy {
			t.Error("Expected provider to be unhealthy")
		}
	})

	t.Run("Mixed success and failure", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		// Record 8 successes and 2 failures (80% success rate)
		for i := 0; i < 8; i++ {
			tracker.RecordSuccess("provider1", 100*time.Millisecond)
		}
		for i := 0; i < 2; i++ {
			tracker.RecordFailure("provider1")
		}

		metrics := tracker.GetMetrics("provider1")
		if metrics == nil {
			t.Fatal("Expected metrics to be created")
		}
		if metrics.TotalRequests != 10 {
			t.Errorf("Expected 10 total requests, got %d", metrics.TotalRequests)
		}
		if metrics.SuccessfulReqs != 8 {
			t.Errorf("Expected 8 successful requests, got %d", metrics.SuccessfulReqs)
		}
		if metrics.FailedRequests != 2 {
			t.Errorf("Expected 2 failed requests, got %d", metrics.FailedRequests)
		}
		if metrics.AvailabilityRate != 0.8 {
			t.Errorf("Expected availability rate 0.8, got %f", metrics.AvailabilityRate)
		}
		if !metrics.IsHealthy {
			t.Error("Expected provider to be healthy at 80% threshold")
		}
	})

	t.Run("Below health threshold", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		// Record 7 successes and 3 failures (70% success rate, below 80% threshold)
		for i := 0; i < 7; i++ {
			tracker.RecordSuccess("provider1", 100*time.Millisecond)
		}
		for i := 0; i < 3; i++ {
			tracker.RecordFailure("provider1")
		}

		metrics := tracker.GetMetrics("provider1")
		if metrics == nil {
			t.Fatal("Expected metrics to be created")
		}
		if metrics.AvailabilityRate < 0.69 || metrics.AvailabilityRate > 0.71 {
			t.Errorf("Expected availability rate ~0.7, got %f", metrics.AvailabilityRate)
		}
		if metrics.IsHealthy {
			t.Error("Expected provider to be unhealthy below 80% threshold")
		}
	})

	t.Run("Record health check", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		tracker.RecordHealthCheck("provider1", true)

		metrics := tracker.GetMetrics("provider1")
		if metrics == nil {
			t.Fatal("Expected metrics to be created")
		}
		if !metrics.IsHealthy {
			t.Error("Expected provider to be healthy")
		}
		if metrics.ConsecutiveFails != 0 {
			t.Errorf("Expected 0 consecutive fails, got %d", metrics.ConsecutiveFails)
		}

		// Record failed health check
		tracker.RecordHealthCheck("provider1", false)

		metrics = tracker.GetMetrics("provider1")
		if metrics.IsHealthy {
			t.Error("Expected provider to be unhealthy")
		}
		if metrics.ConsecutiveFails != 1 {
			t.Errorf("Expected 1 consecutive fail, got %d", metrics.ConsecutiveFails)
		}
	})

	t.Run("Get all metrics", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		tracker.RecordSuccess("provider1", 100*time.Millisecond)
		tracker.RecordSuccess("provider2", 200*time.Millisecond)
		tracker.RecordFailure("provider3")

		allMetrics := tracker.GetAllMetrics()
		if len(allMetrics) != 3 {
			t.Errorf("Expected 3 providers, got %d", len(allMetrics))
		}

		if _, exists := allMetrics["provider1"]; !exists {
			t.Error("Expected provider1 in metrics")
		}
		if _, exists := allMetrics["provider2"]; !exists {
			t.Error("Expected provider2 in metrics")
		}
		if _, exists := allMetrics["provider3"]; !exists {
			t.Error("Expected provider3 in metrics")
		}
	})

	t.Run("Reset metrics", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		tracker.RecordSuccess("provider1", 100*time.Millisecond)
		tracker.RecordSuccess("provider2", 200*time.Millisecond)

		// Reset single provider
		tracker.ResetMetrics("provider1")

		metrics1 := tracker.GetMetrics("provider1")
		if metrics1 != nil {
			t.Error("Expected provider1 metrics to be reset")
		}

		metrics2 := tracker.GetMetrics("provider2")
		if metrics2 == nil {
			t.Error("Expected provider2 metrics to still exist")
		}

		// Reset all providers
		tracker.ResetAllMetrics()

		allMetrics := tracker.GetAllMetrics()
		if len(allMetrics) != 0 {
			t.Errorf("Expected 0 providers after reset, got %d", len(allMetrics))
		}
	})

	t.Run("Is healthy check", func(t *testing.T) {
		tracker := NewProviderAvailabilityTracker(1*time.Hour, 0.8)

		// New provider should be assumed healthy
		if !tracker.IsHealthy("new_provider") {
			t.Error("Expected new provider to be assumed healthy")
		}

		// Record success
		tracker.RecordSuccess("provider1", 100*time.Millisecond)
		if !tracker.IsHealthy("provider1") {
			t.Error("Expected provider1 to be healthy")
		}

		// Record failure
		tracker.RecordFailure("provider2")
		if tracker.IsHealthy("provider2") {
			t.Error("Expected provider2 to be unhealthy")
		}
	})
}

// TestFailoverIntegration tests failover with provider manager
func TestFailoverIntegration(t *testing.T) {
	t.Run("Failover with multiple providers", func(t *testing.T) {
		manager := NewProviderManager()

		// Create mock providers
		config1 := ext.DefaultProviderConfig(ext.ProviderOpenAI)
		config1.APIKey = "test-key-1"
		provider1 := NewConfiguredMockLLMProvider(ext.ProviderOpenAI, config1)

		config2 := ext.DefaultProviderConfig(ext.ProviderAnthropic)
		config2.APIKey = "test-key-2"
		provider2 := NewConfiguredMockLLMProvider(ext.ProviderAnthropic, config2)

		// Add providers with different priorities
		err := manager.AddProvider("provider1", provider1, 1)
		if err != nil {
			t.Fatalf("Failed to add provider1: %v", err)
		}

		err = manager.AddProvider("provider2", provider2, 2)
		if err != nil {
			t.Fatalf("Failed to add provider2: %v", err)
		}

		// Test successful operation
		ctx := context.Background()
		callCount := 0
		err = manager.(*DefaultProviderManager).ExecuteWithFailover(ctx, func(provider ext.LLMProvider) error {
			callCount++
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if callCount != 1 {
			t.Errorf("Expected 1 call, got %d", callCount)
		}
	})

	t.Run("Failover on provider failure", func(t *testing.T) {
		manager := NewProviderManager()

		// Create mock providers
		config1 := ext.DefaultProviderConfig(ext.ProviderOpenAI)
		config1.APIKey = "test-key-1"
		provider1 := NewConfiguredMockLLMProvider(ext.ProviderOpenAI, config1)

		config2 := ext.DefaultProviderConfig(ext.ProviderAnthropic)
		config2.APIKey = "test-key-2"
		provider2 := NewConfiguredMockLLMProvider(ext.ProviderAnthropic, config2)

		// Add providers
		_ = manager.AddProvider("provider1", provider1, 1)
		_ = manager.AddProvider("provider2", provider2, 2)

		// Test failover when first provider fails
		ctx := context.Background()
		callCount := 0
		err := manager.(*DefaultProviderManager).ExecuteWithFailover(ctx, func(provider ext.LLMProvider) error {
			callCount++
			if callCount == 1 {
				return ext.NewExtractorError("temporary_failure", "provider unavailable", 503)
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error after failover, got %v", err)
		}
		// Should try provider1 multiple times (with retries) then succeed with provider2
		if callCount < 2 {
			t.Errorf("Expected at least 2 calls (failover), got %d", callCount)
		}
	})
}

func TestFailoverManager_ContextCancelAndSubstringHelpers(t *testing.T) {
	t.Run("ExecuteWithFailover returns context canceled", func(t *testing.T) {
		fm := NewFailoverManager()
		fm.SetRetryDelay(200 * time.Millisecond)
		fm.SetMaxRetries(3)

		manager := NewProviderManager()
		cfg := ext.DefaultProviderConfig(ext.ProviderMistral)
		cfg.APIKey = "k"
		_ = manager.AddProvider("p1", NewConfiguredMockLLMProvider(ext.ProviderMistral, cfg), 1)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := fm.ExecuteWithFailover(ctx, []string{"p1"}, manager, func(provider ext.LLMProvider) error {
			return ext.NewExtractorError("timeout", "temporary timeout", 503)
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	})

	t.Run("ExecuteEmbeddingWithFailover returns context canceled", func(t *testing.T) {
		fm := NewFailoverManager()
		fm.SetRetryDelay(200 * time.Millisecond)
		fm.SetMaxRetries(2)

		manager := NewEmbeddingProviderManager()
		_ = manager.AddProvider("e1", ext.NewMockEmbeddingProvider("m", 8), 1)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := fm.ExecuteEmbeddingWithFailover(ctx, []string{"e1"}, manager, func(provider ext.EmbeddingProvider) error {
			return ext.NewExtractorError("temporary_failure", "retry", 503)
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	})

	t.Run("containsSubstring and finder branches", func(t *testing.T) {
		if !containsSubstring("abc", "abc") {
			t.Fatal("expected exact match")
		}
		if !containsSubstring("prefix-xyz", "prefix") {
			t.Fatal("expected prefix match")
		}
		if !containsSubstring("xyz-suffix", "suffix") {
			t.Fatal("expected suffix match")
		}
		if !containsSubstring("a-mid-b", "mid") {
			t.Fatal("expected middle match")
		}
		if containsSubstring("hello", "zz") {
			t.Fatal("expected no match")
		}
		if !findSubstringInString("hello world", "world") {
			t.Fatal("expected finder match")
		}
		if findSubstringInString("hello", "xyz") {
			t.Fatal("expected finder no match")
		}
	})
}
