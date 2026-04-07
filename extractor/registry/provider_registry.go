// Package extractor provides a centralized registry for discovering and managing
// available LLM and embedding providers with runtime registration and discovery.
package registry

import (
	ext "github.com/NortonBen/ai-memory-go/extractor"

	"context"
	"fmt"
	"sync"
	"time"
)

// ProviderRegistry provides centralized provider discovery and management
type ProviderRegistry struct {
	mu                   sync.RWMutex
	llmFactory           ext.ProviderFactory
	embeddingFactory     ext.EmbeddingProviderFactory
	llmManager           ext.ProviderManager
	embeddingManager     ext.EmbeddingProviderManager
	configManager        *ext.ConfigManager
	registeredLLMs       map[string]*RegisteredLLMProvider
	registeredEmbeddings map[string]*RegisteredEmbeddingProvider
	healthCheckInterval  time.Duration
	healthCheckEnabled   bool
	healthCheckCancel    context.CancelFunc
	healthCheckRunning   bool
}

// RegisteredLLMProvider holds information about a registered LLM provider
type RegisteredLLMProvider struct {
	Name         string
	Provider     ext.LLMProvider
	Config       *ext.ProviderConfig
	Priority     int
	RegisteredAt time.Time
	LastUsed     time.Time
	UsageCount   int64
	Health       *ext.ProviderHealthStatus
}

// RegisteredEmbeddingProvider holds information about a registered embedding provider
type RegisteredEmbeddingProvider struct {
	Name         string
	Provider     ext.EmbeddingProvider
	Config       *ext.EmbeddingProviderConfig
	Priority     int
	RegisteredAt time.Time
	LastUsed     time.Time
	UsageCount   int64
	Health       *ext.EmbeddingProviderHealthStatus
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		llmFactory:           NewProviderFactory(),
		embeddingFactory:     NewEmbeddingProviderFactory(),
		llmManager:           NewProviderManager(),
		embeddingManager:     NewEmbeddingProviderManager(),
		configManager:        ext.NewConfigManager(),
		registeredLLMs:       make(map[string]*RegisteredLLMProvider),
		registeredEmbeddings: make(map[string]*RegisteredEmbeddingProvider),
		healthCheckInterval: 5 * time.Minute,
		healthCheckEnabled:  true,
	}
}

// NewProviderRegistryWithConfig creates a provider registry with configuration
func NewProviderRegistryWithConfig(configManager *ext.ConfigManager) *ProviderRegistry {
	registry := NewProviderRegistry()
	registry.configManager = configManager
	return registry
}

// RegisterLLMProvider registers an LLM provider with the registry
func (r *ProviderRegistry) RegisterLLMProvider(name string, config *ext.ProviderConfig, priority int) error {
	if config == nil {
		return ext.NewExtractorError("validation", "provider config is nil", 400)
	}

	// Validate configuration
	if err := r.llmFactory.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid provider config: %w", err)
	}

	// Create provider instance
	provider, err := r.llmFactory.CreateProvider(config)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Register with manager
	if err := r.llmManager.AddProvider(name, provider, priority); err != nil {
		return fmt.Errorf("failed to add provider to manager: %w", err)
	}

	// Store in registry
	r.registeredLLMs[name] = &RegisteredLLMProvider{
		Name:         name,
		Provider:     provider,
		Config:       config,
		Priority:     priority,
		RegisteredAt: time.Now(),
		LastUsed:     time.Now(),
		UsageCount:   0,
		Health: &ext.ProviderHealthStatus{
			IsHealthy:        true,
			LastCheck:        time.Now(),
			ConsecutiveFails: 0,
		},
	}

	// Store configuration
	if err := r.configManager.SetLLMConfig(name, config); err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	return nil
}

// RegisterEmbeddingProvider registers an embedding provider with the registry
func (r *ProviderRegistry) RegisterEmbeddingProvider(name string, config *ext.EmbeddingProviderConfig, priority int) error {
	if config == nil {
		return ext.NewExtractorError("validation", "embedding provider config is nil", 400)
	}

	// Validate configuration
	if err := r.embeddingFactory.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid embedding provider config: %w", err)
	}

	// Create provider instance
	provider, err := r.embeddingFactory.CreateProvider(config)
	if err != nil {
		return fmt.Errorf("failed to create embedding provider: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Register with manager
	if err := r.embeddingManager.AddProvider(name, provider, priority); err != nil {
		return fmt.Errorf("failed to add embedding provider to manager: %w", err)
	}

	// Store in registry
	r.registeredEmbeddings[name] = &RegisteredEmbeddingProvider{
		Name:         name,
		Provider:     provider,
		Config:       config,
		Priority:     priority,
		RegisteredAt: time.Now(),
		LastUsed:     time.Now(),
		UsageCount:   0,
		Health: &ext.EmbeddingProviderHealthStatus{
			IsHealthy:        true,
			LastCheck:        time.Now(),
			ConsecutiveFails: 0,
		},
	}

	// Store configuration
	if err := r.configManager.SetEmbeddingConfig(name, config); err != nil {
		return fmt.Errorf("failed to store embedding config: %w", err)
	}

	return nil
}

// UnregisterLLMProvider removes an LLM provider from the registry
func (r *ProviderRegistry) UnregisterLLMProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.registeredLLMs[name]; !exists {
		return ext.NewExtractorError("not_found", fmt.Sprintf("LLM provider %s not found", name), 404)
	}

	// Remove from manager
	if err := r.llmManager.RemoveProvider(name); err != nil {
		return fmt.Errorf("failed to remove provider from manager: %w", err)
	}

	// Remove from registry
	delete(r.registeredLLMs, name)

	// Remove configuration
	if err := r.configManager.RemoveLLMConfig(name); err != nil {
		return fmt.Errorf("failed to remove config: %w", err)
	}

	return nil
}

// UnregisterEmbeddingProvider removes an embedding provider from the registry
func (r *ProviderRegistry) UnregisterEmbeddingProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.registeredEmbeddings[name]; !exists {
		return ext.NewExtractorError("not_found", fmt.Sprintf("embedding provider %s not found", name), 404)
	}

	// Remove from manager
	if err := r.embeddingManager.RemoveProvider(name); err != nil {
		return fmt.Errorf("failed to remove embedding provider from manager: %w", err)
	}

	// Remove from registry
	delete(r.registeredEmbeddings, name)

	// Remove configuration
	if err := r.configManager.RemoveEmbeddingConfig(name); err != nil {
		return fmt.Errorf("failed to remove embedding config: %w", err)
	}

	return nil
}

// GetLLMProvider retrieves an LLM provider by name
func (r *ProviderRegistry) GetLLMProvider(name string) (ext.LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registered, exists := r.registeredLLMs[name]
	if !exists {
		return nil, ext.NewExtractorError("not_found", fmt.Sprintf("LLM provider %s not found", name), 404)
	}

	// Update usage statistics
	r.mu.RUnlock()
	r.mu.Lock()
	registered.LastUsed = time.Now()
	registered.UsageCount++
	r.mu.Unlock()
	r.mu.RLock()

	return registered.Provider, nil
}

// GetEmbeddingProvider retrieves an embedding provider by name
func (r *ProviderRegistry) GetEmbeddingProvider(name string) (ext.EmbeddingProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registered, exists := r.registeredEmbeddings[name]
	if !exists {
		return nil, ext.NewExtractorError("not_found", fmt.Sprintf("embedding provider %s not found", name), 404)
	}

	// Update usage statistics
	r.mu.RUnlock()
	r.mu.Lock()
	registered.LastUsed = time.Now()
	registered.UsageCount++
	r.mu.Unlock()
	r.mu.RLock()

	return registered.Provider, nil
}

// GetBestLLMProvider returns the best available LLM provider
func (r *ProviderRegistry) GetBestLLMProvider(ctx context.Context) (ext.LLMProvider, error) {
	return r.llmManager.GetBestProvider(ctx)
}

// GetBestEmbeddingProvider returns the best available embedding provider
func (r *ProviderRegistry) GetBestEmbeddingProvider(ctx context.Context) (ext.EmbeddingProvider, error) {
	return r.embeddingManager.GetBestProvider(ctx)
}

// ListLLMProviders returns all registered LLM providers
func (r *ProviderRegistry) ListLLMProviders() map[string]*RegisteredLLMProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*RegisteredLLMProvider)
	for name, provider := range r.registeredLLMs {
		providerCopy := *provider
		if provider.Health != nil {
			h := *provider.Health
			providerCopy.Health = &h
		}
		result[name] = &providerCopy
	}
	return result
}

// ListEmbeddingProviders returns all registered embedding providers
func (r *ProviderRegistry) ListEmbeddingProviders() map[string]*RegisteredEmbeddingProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*RegisteredEmbeddingProvider)
	for name, provider := range r.registeredEmbeddings {
		providerCopy := *provider
		if provider.Health != nil {
			h := *provider.Health
			providerCopy.Health = &h
		}
		result[name] = &providerCopy
	}
	return result
}

// UpdateLLMProviderConfig updates the configuration of an LLM provider
func (r *ProviderRegistry) UpdateLLMProviderConfig(name string, config *ext.ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	registered, exists := r.registeredLLMs[name]
	if !exists {
		return ext.NewExtractorError("not_found", fmt.Sprintf("LLM provider %s not found", name), 404)
	}

	// Validate new configuration
	if err := r.llmFactory.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid provider config: %w", err)
	}

	// Update provider configuration
	if err := registered.Provider.Configure(config); err != nil {
		return fmt.Errorf("failed to configure provider: %w", err)
	}

	// Update stored configuration
	registered.Config = config
	if err := r.configManager.SetLLMConfig(name, config); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	return nil
}

// UpdateEmbeddingProviderConfig updates the configuration of an embedding provider
func (r *ProviderRegistry) UpdateEmbeddingProviderConfig(name string, config *ext.EmbeddingProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	registered, exists := r.registeredEmbeddings[name]
	if !exists {
		return ext.NewExtractorError("not_found", fmt.Sprintf("embedding provider %s not found", name), 404)
	}

	// Validate new configuration
	if err := r.embeddingFactory.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid embedding provider config: %w", err)
	}

	// Update provider configuration
	if err := registered.Provider.Configure(config); err != nil {
		return fmt.Errorf("failed to configure embedding provider: %w", err)
	}

	// Update stored configuration
	registered.Config = config
	if err := r.configManager.SetEmbeddingConfig(name, config); err != nil {
		return fmt.Errorf("failed to update embedding config: %w", err)
	}

	return nil
}

// HealthCheck performs health checks on all registered providers
func (r *ProviderRegistry) HealthCheck(ctx context.Context) (*RegistryHealthStatus, error) {
	r.mu.RLock()
	llmNames := make([]string, 0, len(r.registeredLLMs))
	llmProviders := make(map[string]ext.LLMProvider, len(r.registeredLLMs))
	for name, reg := range r.registeredLLMs {
		llmNames = append(llmNames, name)
		llmProviders[name] = reg.Provider
	}
	embNames := make([]string, 0, len(r.registeredEmbeddings))
	embProviders := make(map[string]ext.EmbeddingProvider, len(r.registeredEmbeddings))
	for name, reg := range r.registeredEmbeddings {
		embNames = append(embNames, name)
		embProviders[name] = reg.Provider
	}
	r.mu.RUnlock()

	status := &RegistryHealthStatus{
		Timestamp:          time.Now(),
		LLMProviders:       make(map[string]*ext.ProviderHealthStatus),
		EmbeddingProviders: make(map[string]*ext.EmbeddingProviderHealthStatus),
	}

	for _, name := range llmNames {
		p := llmProviders[name]
		start := time.Now()
		err := p.Health(ctx)
		responseTime := time.Since(start)

		r.mu.Lock()
		reg := r.registeredLLMs[name]
		if reg == nil {
			r.mu.Unlock()
			continue
		}
		prev := 0
		if reg.Health != nil {
			prev = reg.Health.ConsecutiveFails
		}
		healthStatus := &ext.ProviderHealthStatus{
			IsHealthy:        err == nil,
			LastCheck:        time.Now(),
			ResponseTime:     responseTime,
			ConsecutiveFails: prev,
		}
		if err != nil {
			healthStatus.ErrorMessage = err.Error()
			healthStatus.ConsecutiveFails = prev + 1
		} else {
			healthStatus.ConsecutiveFails = 0
		}
		reg.Health = healthStatus
		status.LLMProviders[name] = healthStatus
		r.mu.Unlock()
	}

	for _, name := range embNames {
		p := embProviders[name]
		start := time.Now()
		err := p.Health(ctx)
		responseTime := time.Since(start)

		r.mu.Lock()
		reg := r.registeredEmbeddings[name]
		if reg == nil {
			r.mu.Unlock()
			continue
		}
		prev := 0
		if reg.Health != nil {
			prev = reg.Health.ConsecutiveFails
		}
		healthStatus := &ext.EmbeddingProviderHealthStatus{
			IsHealthy:        err == nil,
			LastCheck:        time.Now(),
			ResponseTime:     responseTime,
			ConsecutiveFails: prev,
		}
		if err != nil {
			healthStatus.ErrorMessage = err.Error()
			healthStatus.ConsecutiveFails = prev + 1
		} else {
			healthStatus.ConsecutiveFails = 0
		}
		reg.Health = healthStatus
		status.EmbeddingProviders[name] = healthStatus
		r.mu.Unlock()
	}

	return status, nil
}

// StartHealthChecks starts periodic health checks
func (r *ProviderRegistry) StartHealthChecks(ctx context.Context) {
	r.mu.Lock()
	if r.healthCheckRunning {
		r.mu.Unlock()
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	r.healthCheckCancel = cancel
	r.healthCheckRunning = true
	r.mu.Unlock()

	go r.healthCheckWorker(workerCtx)
}

// StopHealthChecks stops periodic health checks
func (r *ProviderRegistry) StopHealthChecks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.healthCheckRunning {
		return
	}

	if r.healthCheckCancel != nil {
		r.healthCheckCancel()
		r.healthCheckCancel = nil
	}
	r.healthCheckRunning = false
}

// SetHealthCheckInterval sets the health check interval
func (r *ProviderRegistry) SetHealthCheckInterval(interval time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.healthCheckInterval = interval
}

// LoadFromEnvironment loads provider configurations from environment variables
func (r *ProviderRegistry) LoadFromEnvironment() error {
	if err := r.configManager.LoadFromEnvironment(); err != nil {
		return fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Register providers from configuration
	return r.registerProvidersFromConfig()
}

// LoadFromFile loads provider configurations from a file
func (r *ProviderRegistry) LoadFromFile(filename string) error {
	if err := r.configManager.LoadFromFile(filename); err != nil {
		return fmt.Errorf("failed to load config from file: %w", err)
	}

	// Register providers from configuration
	return r.registerProvidersFromConfig()
}

// SaveToFile saves current provider configurations to a file
func (r *ProviderRegistry) SaveToFile(filename string) error {
	return r.configManager.SaveToFile(filename)
}

// GetConfigManager returns the configuration manager
func (r *ProviderRegistry) GetConfigManager() *ext.ConfigManager {
	return r.configManager
}

// GetLLMFactory returns the LLM provider factory
func (r *ProviderRegistry) GetLLMFactory() ext.ProviderFactory {
	return r.llmFactory
}

// GetEmbeddingFactory returns the embedding provider factory
func (r *ProviderRegistry) GetEmbeddingFactory() ext.EmbeddingProviderFactory {
	return r.embeddingFactory
}

// GetLLMManager returns the LLM provider manager
func (r *ProviderRegistry) GetLLMManager() ext.ProviderManager {
	return r.llmManager
}

// GetEmbeddingManager returns the embedding provider manager
func (r *ProviderRegistry) GetEmbeddingManager() ext.EmbeddingProviderManager {
	return r.embeddingManager
}

// SetLoadBalancing configures load balancing strategy for LLM providers
func (r *ProviderRegistry) SetLoadBalancing(strategy ext.LoadBalancingStrategy) {
	r.llmManager.SetLoadBalancing(strategy)
}

// SetEmbeddingLoadBalancing configures load balancing strategy for embedding providers
func (r *ProviderRegistry) SetEmbeddingLoadBalancing(strategy ext.EmbeddingLoadBalancingStrategy) {
	r.embeddingManager.SetLoadBalancing(strategy)
}

// SetFailoverEnabled enables/disables automatic failover
func (r *ProviderRegistry) SetFailoverEnabled(enabled bool) {
	r.llmManager.SetFailoverEnabled(enabled)
	r.embeddingManager.SetFailoverEnabled(enabled)
}

// Close closes all registered providers and stops health checks
func (r *ProviderRegistry) Close() error {
	r.StopHealthChecks()

	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []error

	// Close all LLM providers
	for name, registered := range r.registeredLLMs {
		if err := registered.Provider.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close LLM provider %s: %w", name, err))
		}
	}

	// Close all embedding providers
	for name, registered := range r.registeredEmbeddings {
		if err := registered.Provider.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close embedding provider %s: %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing providers: %v", errors)
	}

	return nil
}

// Private helper methods

func (r *ProviderRegistry) registerProvidersFromConfig() error {
	// Register LLM providers from configuration
	llmConfigs := r.configManager.ListLLMConfigs()
	for name, config := range llmConfigs {
		// Use priority from config or default to 100
		priority := 100
		if err := r.RegisterLLMProvider(name, config, priority); err != nil {
			return fmt.Errorf("failed to register LLM provider %s: %w", name, err)
		}
	}

	// Register embedding providers from configuration
	embeddingConfigs := r.configManager.ListEmbeddingConfigs()
	for name, config := range embeddingConfigs {
		// Use priority from config or default to 100
		priority := 100
		if err := r.RegisterEmbeddingProvider(name, config, priority); err != nil {
			return fmt.Errorf("failed to register embedding provider %s: %w", name, err)
		}
	}

	return nil
}

func (r *ProviderRegistry) healthCheckWorker(ctx context.Context) {
	r.mu.RLock()
	interval := r.healthCheckInterval
	r.mu.RUnlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.HealthCheck(ctx); err != nil {
				// Log error but continue
				fmt.Printf("Health check failed: %v\n", err)
			}
		}
	}
}

// RegistryHealthStatus represents the health status of all providers in the registry
type RegistryHealthStatus struct {
	Timestamp          time.Time                                 `json:"timestamp"`
	LLMProviders       map[string]*ext.ProviderHealthStatus          `json:"llm_providers"`
	EmbeddingProviders map[string]*ext.EmbeddingProviderHealthStatus `json:"embedding_providers"`
}
