// Package extractor provides provider manager implementations for handling
// multiple LLM and embedding providers with failover, load balancing, and health monitoring.
package extractor

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// DefaultProviderManager implements the ProviderManager interface
type DefaultProviderManager struct {
	mu                  sync.RWMutex
	providers           map[string]*ProviderEntry
	failoverEnabled     bool
	loadBalancing       LoadBalancingStrategy
	roundRobinCounter   int
	healthChecker       *HealthChecker
	failoverManager     *FailoverManager
	availabilityTracker *ProviderAvailabilityTracker
}

// ProviderEntry holds provider information with priority and health status
type ProviderEntry struct {
	Provider   LLMProvider
	Priority   int
	Health     *ProviderHealthStatus
	Metrics    *ProviderMetrics
	LastUsed   time.Time
	UsageCount int64
}

// NewProviderManager creates a new provider manager
func NewProviderManager() ProviderManager {
	return &DefaultProviderManager{
		providers:           make(map[string]*ProviderEntry),
		failoverEnabled:     true,
		loadBalancing:       LoadBalancePriority,
		healthChecker:       NewHealthChecker(5*time.Minute, 30*time.Second, 3),
		failoverManager:     NewFailoverManager(),
		availabilityTracker: NewProviderAvailabilityTracker(1*time.Hour, 0.8),
	}
}

// AddProvider adds a provider to the manager
func (m *DefaultProviderManager) AddProvider(name string, provider LLMProvider, priority int) error {
	if provider == nil {
		return NewExtractorError("validation", "provider is nil", 400)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[name] = &ProviderEntry{
		Provider: provider,
		Priority: priority,
		Health: &ProviderHealthStatus{
			IsHealthy:        true,
			LastCheck:        time.Now(),
			ConsecutiveFails: 0,
		},
		Metrics: &ProviderMetrics{
			FirstRequest: time.Now(),
		},
		LastUsed:   time.Now(),
		UsageCount: 0,
	}

	return nil
}

// RemoveProvider removes a provider from the manager
func (m *DefaultProviderManager) RemoveProvider(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.providers[name]
	if !exists {
		return NewExtractorError("not_found", fmt.Sprintf("provider %s not found", name), 404)
	}

	// Close the provider
	if err := entry.Provider.Close(); err != nil {
		return fmt.Errorf("failed to close provider %s: %w", name, err)
	}

	delete(m.providers, name)
	return nil
}

// GetProvider gets a specific provider by name
func (m *DefaultProviderManager) GetProvider(name string) (LLMProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.providers[name]
	if !exists {
		return nil, NewExtractorError("not_found", fmt.Sprintf("provider %s not found", name), 404)
	}

	return entry.Provider, nil
}

// GetBestProvider returns the best available provider based on health and priority
func (m *DefaultProviderManager) GetBestProvider(ctx context.Context) (LLMProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.providers) == 0 {
		return nil, NewExtractorError("no_providers", "no providers available", 503)
	}

	// Get healthy providers
	var healthyProviders []*ProviderEntry
	for _, entry := range m.providers {
		if entry.Health.IsHealthy {
			healthyProviders = append(healthyProviders, entry)
		}
	}

	if len(healthyProviders) == 0 {
		return nil, NewExtractorError("no_healthy_providers", "no healthy providers available", 503)
	}

	// Select provider based on load balancing strategy
	var selectedEntry *ProviderEntry
	switch m.loadBalancing {
	case LoadBalancePriority:
		selectedEntry = m.selectByPriority(healthyProviders)
	case LoadBalanceRoundRobin:
		selectedEntry = m.selectByRoundRobin(healthyProviders)
	case LoadBalanceRandom:
		selectedEntry = m.selectByRandom(healthyProviders)
	case LoadBalanceLeastUsed:
		selectedEntry = m.selectByLeastUsed(healthyProviders)
	default:
		selectedEntry = m.selectByPriority(healthyProviders)
	}

	if selectedEntry == nil {
		return nil, NewExtractorError("selection_failed", "failed to select provider", 500)
	}

	// Update usage statistics
	selectedEntry.LastUsed = time.Now()
	selectedEntry.UsageCount++

	return selectedEntry.Provider, nil
}

// ListProviders returns all registered providers
func (m *DefaultProviderManager) ListProviders() map[string]LLMProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]LLMProvider)
	for name, entry := range m.providers {
		result[name] = entry.Provider
	}
	return result
}

// HealthCheck performs health checks on all providers
func (m *DefaultProviderManager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make(map[string]error)

	for name, entry := range m.providers {
		start := time.Now()
		err := entry.Provider.Health(ctx)
		responseTime := time.Since(start)

		entry.Health.LastCheck = time.Now()
		entry.Health.ResponseTime = responseTime

		if err != nil {
			entry.Health.IsHealthy = false
			entry.Health.ConsecutiveFails++
			entry.Health.ErrorMessage = err.Error()
			results[name] = err
		} else {
			entry.Health.IsHealthy = true
			entry.Health.ConsecutiveFails = 0
			entry.Health.ErrorMessage = ""
		}
	}

	return results
}

// SetFailoverEnabled enables/disables automatic failover
func (m *DefaultProviderManager) SetFailoverEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failoverEnabled = enabled
}

// SetLoadBalancing configures load balancing strategy
func (m *DefaultProviderManager) SetLoadBalancing(strategy LoadBalancingStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadBalancing = strategy
}

// Provider selection methods

func (m *DefaultProviderManager) selectByPriority(providers []*ProviderEntry) *ProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by priority (lower number = higher priority)
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Priority < providers[j].Priority
	})

	return providers[0]
}

func (m *DefaultProviderManager) selectByRoundRobin(providers []*ProviderEntry) *ProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by name for consistent ordering
	sort.Slice(providers, func(i, j int) bool {
		return fmt.Sprintf("%p", providers[i]) < fmt.Sprintf("%p", providers[j])
	})

	selected := providers[m.roundRobinCounter%len(providers)]
	m.roundRobinCounter++
	return selected
}

func (m *DefaultProviderManager) selectByRandom(providers []*ProviderEntry) *ProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	return providers[rand.Intn(len(providers))]
}

func (m *DefaultProviderManager) selectByLeastUsed(providers []*ProviderEntry) *ProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by usage count (ascending)
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].UsageCount < providers[j].UsageCount
	})

	return providers[0]
}

// DefaultEmbeddingProviderManager implements the EmbeddingProviderManager interface
type DefaultEmbeddingProviderManager struct {
	mu                  sync.RWMutex
	providers           map[string]*EmbeddingProviderEntry
	failoverEnabled     bool
	loadBalancing       EmbeddingLoadBalancingStrategy
	roundRobinCounter   int
	healthChecker       *HealthChecker
	failoverManager     *FailoverManager
	availabilityTracker *ProviderAvailabilityTracker
}

// EmbeddingProviderEntry holds embedding provider information with priority and health status
type EmbeddingProviderEntry struct {
	Provider   EmbeddingProvider
	Priority   int
	Health     *EmbeddingProviderHealthStatus
	Metrics    *EmbeddingProviderMetrics
	LastUsed   time.Time
	UsageCount int64
}

// NewEmbeddingProviderManager creates a new embedding provider manager
func NewEmbeddingProviderManager() EmbeddingProviderManager {
	return &DefaultEmbeddingProviderManager{
		providers:           make(map[string]*EmbeddingProviderEntry),
		failoverEnabled:     true,
		loadBalancing:       EmbeddingLoadBalancePriority,
		healthChecker:       NewHealthChecker(5*time.Minute, 30*time.Second, 3),
		failoverManager:     NewFailoverManager(),
		availabilityTracker: NewProviderAvailabilityTracker(1*time.Hour, 0.8),
	}
}

// AddProvider adds an embedding provider to the manager
func (m *DefaultEmbeddingProviderManager) AddProvider(name string, provider EmbeddingProvider, priority int) error {
	if provider == nil {
		return NewExtractorError("validation", "embedding provider is nil", 400)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[name] = &EmbeddingProviderEntry{
		Provider: provider,
		Priority: priority,
		Health: &EmbeddingProviderHealthStatus{
			IsHealthy:        true,
			LastCheck:        time.Now(),
			ConsecutiveFails: 0,
		},
		Metrics: &EmbeddingProviderMetrics{
			FirstRequest: time.Now(),
		},
		LastUsed:   time.Now(),
		UsageCount: 0,
	}

	return nil
}

// RemoveProvider removes an embedding provider from the manager
func (m *DefaultEmbeddingProviderManager) RemoveProvider(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.providers[name]
	if !exists {
		return NewExtractorError("not_found", fmt.Sprintf("embedding provider %s not found", name), 404)
	}

	// Close the provider
	if err := entry.Provider.Close(); err != nil {
		return fmt.Errorf("failed to close embedding provider %s: %w", name, err)
	}

	delete(m.providers, name)
	return nil
}

// GetProvider gets a specific embedding provider by name
func (m *DefaultEmbeddingProviderManager) GetProvider(name string) (EmbeddingProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.providers[name]
	if !exists {
		return nil, NewExtractorError("not_found", fmt.Sprintf("embedding provider %s not found", name), 404)
	}

	return entry.Provider, nil
}

// GetBestProvider returns the best available embedding provider based on health and priority
func (m *DefaultEmbeddingProviderManager) GetBestProvider(ctx context.Context) (EmbeddingProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.providers) == 0 {
		return nil, NewExtractorError("no_providers", "no embedding providers available", 503)
	}

	// Get healthy providers
	var healthyProviders []*EmbeddingProviderEntry
	for _, entry := range m.providers {
		if entry.Health.IsHealthy {
			healthyProviders = append(healthyProviders, entry)
		}
	}

	if len(healthyProviders) == 0 {
		return nil, NewExtractorError("no_healthy_providers", "no healthy embedding providers available", 503)
	}

	// Select provider based on load balancing strategy
	var selectedEntry *EmbeddingProviderEntry
	switch m.loadBalancing {
	case EmbeddingLoadBalancePriority:
		selectedEntry = m.selectEmbeddingByPriority(healthyProviders)
	case EmbeddingLoadBalanceRoundRobin:
		selectedEntry = m.selectEmbeddingByRoundRobin(healthyProviders)
	case EmbeddingLoadBalanceRandom:
		selectedEntry = m.selectEmbeddingByRandom(healthyProviders)
	case EmbeddingLoadBalanceLeastUsed:
		selectedEntry = m.selectEmbeddingByLeastUsed(healthyProviders)
	case EmbeddingLoadBalanceCostBased:
		selectedEntry = m.selectEmbeddingByCost(healthyProviders)
	case EmbeddingLoadBalanceLatency:
		selectedEntry = m.selectEmbeddingByLatency(healthyProviders)
	default:
		selectedEntry = m.selectEmbeddingByPriority(healthyProviders)
	}

	if selectedEntry == nil {
		return nil, NewExtractorError("selection_failed", "failed to select embedding provider", 500)
	}

	// Update usage statistics
	selectedEntry.LastUsed = time.Now()
	selectedEntry.UsageCount++

	return selectedEntry.Provider, nil
}

// ListProviders returns all registered embedding providers
func (m *DefaultEmbeddingProviderManager) ListProviders() map[string]EmbeddingProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]EmbeddingProvider)
	for name, entry := range m.providers {
		result[name] = entry.Provider
	}
	return result
}

// HealthCheck performs health checks on all embedding providers
func (m *DefaultEmbeddingProviderManager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make(map[string]error)

	for name, entry := range m.providers {
		start := time.Now()
		err := entry.Provider.Health(ctx)
		responseTime := time.Since(start)

		entry.Health.LastCheck = time.Now()
		entry.Health.ResponseTime = responseTime

		if err != nil {
			entry.Health.IsHealthy = false
			entry.Health.ConsecutiveFails++
			entry.Health.ErrorMessage = err.Error()
			results[name] = err
		} else {
			entry.Health.IsHealthy = true
			entry.Health.ConsecutiveFails = 0
			entry.Health.ErrorMessage = ""
		}
	}

	return results
}

// SetFailoverEnabled enables/disables automatic failover for embedding providers
func (m *DefaultEmbeddingProviderManager) SetFailoverEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failoverEnabled = enabled
}

// SetLoadBalancing configures load balancing strategy for embedding providers
func (m *DefaultEmbeddingProviderManager) SetLoadBalancing(strategy EmbeddingLoadBalancingStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadBalancing = strategy
}

// GenerateEmbeddingWithFailover generates embedding with automatic failover
func (m *DefaultEmbeddingProviderManager) GenerateEmbeddingWithFailover(ctx context.Context, text string) ([]float32, error) {
	if !m.failoverEnabled {
		provider, err := m.GetBestProvider(ctx)
		if err != nil {
			return nil, err
		}
		return provider.GenerateEmbedding(ctx, text)
	}

	// Try providers in order of priority until one succeeds
	m.mu.RLock()
	var providers []*EmbeddingProviderEntry
	for _, entry := range m.providers {
		providers = append(providers, entry)
	}
	m.mu.RUnlock()

	// Sort by priority
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Priority < providers[j].Priority
	})

	var lastErr error
	for _, entry := range providers {
		if !entry.Health.IsHealthy {
			continue
		}

		embedding, err := entry.Provider.GenerateEmbedding(ctx, text)
		if err == nil {
			return embedding, nil
		}
		lastErr = err

		// Mark provider as unhealthy if it fails
		m.mu.Lock()
		entry.Health.IsHealthy = false
		entry.Health.ConsecutiveFails++
		entry.Health.ErrorMessage = err.Error()
		m.mu.Unlock()
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, NewExtractorError("all_providers_failed", "all embedding providers failed", 503)
}

// GenerateBatchEmbeddingsWithFailover generates batch embeddings with automatic failover
func (m *DefaultEmbeddingProviderManager) GenerateBatchEmbeddingsWithFailover(ctx context.Context, texts []string) ([][]float32, error) {
	if !m.failoverEnabled {
		provider, err := m.GetBestProvider(ctx)
		if err != nil {
			return nil, err
		}
		return provider.GenerateBatchEmbeddings(ctx, texts)
	}

	// Try providers in order of priority until one succeeds
	m.mu.RLock()
	var providers []*EmbeddingProviderEntry
	for _, entry := range m.providers {
		providers = append(providers, entry)
	}
	m.mu.RUnlock()

	// Sort by priority
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Priority < providers[j].Priority
	})

	var lastErr error
	for _, entry := range providers {
		if !entry.Health.IsHealthy {
			continue
		}

		embeddings, err := entry.Provider.GenerateBatchEmbeddings(ctx, texts)
		if err == nil {
			return embeddings, nil
		}
		lastErr = err

		// Mark provider as unhealthy if it fails
		m.mu.Lock()
		entry.Health.IsHealthy = false
		entry.Health.ConsecutiveFails++
		entry.Health.ErrorMessage = err.Error()
		m.mu.Unlock()
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, NewExtractorError("all_providers_failed", "all embedding providers failed", 503)
}

// Embedding provider selection methods

func (m *DefaultEmbeddingProviderManager) selectEmbeddingByPriority(providers []*EmbeddingProviderEntry) *EmbeddingProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by priority (lower number = higher priority)
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Priority < providers[j].Priority
	})

	return providers[0]
}

func (m *DefaultEmbeddingProviderManager) selectEmbeddingByRoundRobin(providers []*EmbeddingProviderEntry) *EmbeddingProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by pointer address for consistent ordering
	sort.Slice(providers, func(i, j int) bool {
		return fmt.Sprintf("%p", providers[i]) < fmt.Sprintf("%p", providers[j])
	})

	selected := providers[m.roundRobinCounter%len(providers)]
	m.roundRobinCounter++
	return selected
}

func (m *DefaultEmbeddingProviderManager) selectEmbeddingByRandom(providers []*EmbeddingProviderEntry) *EmbeddingProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	return providers[rand.Intn(len(providers))]
}

func (m *DefaultEmbeddingProviderManager) selectEmbeddingByLeastUsed(providers []*EmbeddingProviderEntry) *EmbeddingProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by usage count (ascending)
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].UsageCount < providers[j].UsageCount
	})

	return providers[0]
}

func (m *DefaultEmbeddingProviderManager) selectEmbeddingByCost(providers []*EmbeddingProviderEntry) *EmbeddingProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by cost per token (ascending)
	sort.Slice(providers, func(i, j int) bool {
		caps1 := providers[i].Provider.GetCapabilities()
		caps2 := providers[j].Provider.GetCapabilities()
		return caps1.CostPerToken < caps2.CostPerToken
	})

	return providers[0]
}

func (m *DefaultEmbeddingProviderManager) selectEmbeddingByLatency(providers []*EmbeddingProviderEntry) *EmbeddingProviderEntry {
	if len(providers) == 0 {
		return nil
	}

	// Sort by average latency (ascending)
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Health.ResponseTime < providers[j].Health.ResponseTime
	})

	return providers[0]
}

// StartHealthChecks starts automatic health checking for all providers
func (m *DefaultProviderManager) StartHealthChecks(ctx context.Context) {
	m.mu.RLock()
	healthChecker := m.healthChecker
	m.mu.RUnlock()

	if healthChecker != nil {
		healthChecker.Start(ctx, m)
	}
}

// StopHealthChecks stops automatic health checking
func (m *DefaultProviderManager) StopHealthChecks() {
	m.mu.RLock()
	healthChecker := m.healthChecker
	m.mu.RUnlock()

	if healthChecker != nil {
		healthChecker.Stop()
	}
}

// GetFailoverManager returns the failover manager
func (m *DefaultProviderManager) GetFailoverManager() *FailoverManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failoverManager
}

// GetAvailabilityTracker returns the availability tracker
func (m *DefaultProviderManager) GetAvailabilityTracker() *ProviderAvailabilityTracker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.availabilityTracker
}

// ExecuteWithFailover executes an operation with automatic failover
func (m *DefaultProviderManager) ExecuteWithFailover(
	ctx context.Context,
	operation func(provider LLMProvider) error,
) error {
	m.mu.RLock()
	failoverManager := m.failoverManager

	// Get provider names sorted by priority
	var providerNames []string
	var entries []*ProviderEntry
	for name, entry := range m.providers {
		providerNames = append(providerNames, name)
		entries = append(entries, entry)
	}
	m.mu.RUnlock()

	// Sort by priority
	sort.Slice(providerNames, func(i, j int) bool {
		return entries[i].Priority < entries[j].Priority
	})

	return failoverManager.ExecuteWithFailover(ctx, providerNames, m, operation)
}

// RecordProviderSuccess records a successful provider operation
func (m *DefaultProviderManager) RecordProviderSuccess(providerName string, latency time.Duration) {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		tracker.RecordSuccess(providerName, latency)
	}
}

// RecordProviderFailure records a failed provider operation
func (m *DefaultProviderManager) RecordProviderFailure(providerName string) {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		tracker.RecordFailure(providerName)
	}
}

// GetProviderMetrics returns availability metrics for a provider
func (m *DefaultProviderManager) GetProviderMetrics(providerName string) *ProviderAvailabilityMetrics {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		return tracker.GetMetrics(providerName)
	}
	return nil
}

// GetAllProviderMetrics returns availability metrics for all providers
func (m *DefaultProviderManager) GetAllProviderMetrics() map[string]*ProviderAvailabilityMetrics {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		return tracker.GetAllMetrics()
	}
	return make(map[string]*ProviderAvailabilityMetrics)
}

// StartEmbeddingHealthChecks starts automatic health checking for all embedding providers
func (m *DefaultEmbeddingProviderManager) StartHealthChecks(ctx context.Context) {
	m.mu.RLock()
	healthChecker := m.healthChecker
	m.mu.RUnlock()

	if healthChecker != nil {
		healthChecker.StartEmbedding(ctx, m)
	}
}

// StopEmbeddingHealthChecks stops automatic health checking
func (m *DefaultEmbeddingProviderManager) StopHealthChecks() {
	m.mu.RLock()
	healthChecker := m.healthChecker
	m.mu.RUnlock()

	if healthChecker != nil {
		healthChecker.Stop()
	}
}

// GetFailoverManager returns the failover manager
func (m *DefaultEmbeddingProviderManager) GetFailoverManager() *FailoverManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failoverManager
}

// GetAvailabilityTracker returns the availability tracker
func (m *DefaultEmbeddingProviderManager) GetAvailabilityTracker() *ProviderAvailabilityTracker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.availabilityTracker
}

// ExecuteWithFailover executes an embedding operation with automatic failover
func (m *DefaultEmbeddingProviderManager) ExecuteWithFailover(
	ctx context.Context,
	operation func(provider EmbeddingProvider) error,
) error {
	m.mu.RLock()
	failoverManager := m.failoverManager

	// Get provider names sorted by priority
	var providerNames []string
	var entries []*EmbeddingProviderEntry
	for name, entry := range m.providers {
		providerNames = append(providerNames, name)
		entries = append(entries, entry)
	}
	m.mu.RUnlock()

	// Sort by priority
	sort.Slice(providerNames, func(i, j int) bool {
		return entries[i].Priority < entries[j].Priority
	})

	return failoverManager.ExecuteEmbeddingWithFailover(ctx, providerNames, m, operation)
}

// RecordProviderSuccess records a successful embedding provider operation
func (m *DefaultEmbeddingProviderManager) RecordProviderSuccess(providerName string, latency time.Duration) {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		tracker.RecordSuccess(providerName, latency)
	}
}

// RecordProviderFailure records a failed embedding provider operation
func (m *DefaultEmbeddingProviderManager) RecordProviderFailure(providerName string) {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		tracker.RecordFailure(providerName)
	}
}

// GetProviderMetrics returns availability metrics for an embedding provider
func (m *DefaultEmbeddingProviderManager) GetProviderMetrics(providerName string) *ProviderAvailabilityMetrics {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		return tracker.GetMetrics(providerName)
	}
	return nil
}

// GetAllProviderMetrics returns availability metrics for all embedding providers
func (m *DefaultEmbeddingProviderManager) GetAllProviderMetrics() map[string]*ProviderAvailabilityMetrics {
	m.mu.RLock()
	tracker := m.availabilityTracker
	m.mu.RUnlock()

	if tracker != nil {
		return tracker.GetAllMetrics()
	}
	return make(map[string]*ProviderAvailabilityMetrics)
}
