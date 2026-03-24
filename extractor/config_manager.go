// Package extractor provides configuration management for LLM and embedding providers
// with support for environment variables, configuration files, and runtime updates.
package extractor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigManager manages provider configurations with support for multiple sources
type ConfigManager struct {
	mu                   sync.RWMutex
	llmConfigs           map[string]*ProviderConfig
	embeddingConfigs     map[string]*EmbeddingProviderConfig
	globalConfig         *GlobalConfig
	configFile           string
	autoReload           bool
	environmentOverrides bool
	lastModified         time.Time
}

// GlobalConfig holds system-wide configuration settings
type GlobalConfig struct {
	// Default provider settings
	DefaultLLMProvider       string `json:"default_llm_provider" yaml:"default_llm_provider"`
	DefaultEmbeddingProvider string `json:"default_embedding_provider" yaml:"default_embedding_provider"`

	// Global timeouts and limits
	DefaultTimeout        time.Duration `json:"default_timeout" yaml:"default_timeout"`
	MaxConcurrentRequests int           `json:"max_concurrent_requests" yaml:"max_concurrent_requests"`

	// Logging and monitoring
	LogLevel        string `json:"log_level" yaml:"log_level"`
	EnableMetrics   bool   `json:"enable_metrics" yaml:"enable_metrics"`
	MetricsEndpoint string `json:"metrics_endpoint" yaml:"metrics_endpoint"`

	// Security settings
	EnableTLS        bool   `json:"enable_tls" yaml:"enable_tls"`
	TLSCertFile      string `json:"tls_cert_file" yaml:"tls_cert_file"`
	TLSKeyFile       string `json:"tls_key_file" yaml:"tls_key_file"`
	APIKeyEncryption bool   `json:"api_key_encryption" yaml:"api_key_encryption"`
	EncryptionKey    string `json:"encryption_key" yaml:"encryption_key"`

	// Cache settings
	EnableGlobalCache bool          `json:"enable_global_cache" yaml:"enable_global_cache"`
	CacheSize         int           `json:"cache_size" yaml:"cache_size"`
	CacheTTL          time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// Health check settings
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout" yaml:"health_check_timeout"`

	// Feature flags
	Features struct {
		EnableFailover      bool `json:"enable_failover" yaml:"enable_failover"`
		EnableLoadBalancing bool `json:"enable_load_balancing" yaml:"enable_load_balancing"`
		EnableAutoRetry     bool `json:"enable_auto_retry" yaml:"enable_auto_retry"`
		EnableRateLimiting  bool `json:"enable_rate_limiting" yaml:"enable_rate_limiting"`
	} `json:"features" yaml:"features"`
}

// ConfigFile represents the structure of a configuration file
type ConfigFile struct {
	Global    *GlobalConfig                       `json:"global" yaml:"global"`
	LLM       map[string]*ProviderConfig          `json:"llm" yaml:"llm"`
	Embedding map[string]*EmbeddingProviderConfig `json:"embedding" yaml:"embedding"`
	Version   string                              `json:"version" yaml:"version"`
	CreatedAt time.Time                           `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time                           `json:"updated_at" yaml:"updated_at"`
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		llmConfigs:           make(map[string]*ProviderConfig),
		embeddingConfigs:     make(map[string]*EmbeddingProviderConfig),
		globalConfig:         DefaultGlobalConfig(),
		environmentOverrides: true,
		autoReload:           false,
	}
}

// NewConfigManagerFromFile creates a configuration manager from a file
func NewConfigManagerFromFile(configFile string) (*ConfigManager, error) {
	manager := NewConfigManager()
	manager.configFile = configFile

	if err := manager.LoadFromFile(configFile); err != nil {
		return nil, fmt.Errorf("failed to load config from file: %w", err)
	}

	return manager, nil
}

// NewConfigManagerFromEnv creates a configuration manager from environment variables
func NewConfigManagerFromEnv() (*ConfigManager, error) {
	manager := NewConfigManager()

	if err := manager.LoadFromEnvironment(); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	return manager, nil
}

// DefaultGlobalConfig returns default global configuration
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		DefaultLLMProvider:       "openai",
		DefaultEmbeddingProvider: "openai",
		DefaultTimeout:           60 * time.Second,
		MaxConcurrentRequests:    100,
		LogLevel:                 "info",
		EnableMetrics:            true,
		MetricsEndpoint:          ":8080/metrics",
		EnableTLS:                false,
		APIKeyEncryption:         false,
		EnableGlobalCache:        true,
		CacheSize:                1000,
		CacheTTL:                 1 * time.Hour,
		HealthCheckInterval:      5 * time.Minute,
		HealthCheckTimeout:       10 * time.Second,
		Features: struct {
			EnableFailover      bool `json:"enable_failover" yaml:"enable_failover"`
			EnableLoadBalancing bool `json:"enable_load_balancing" yaml:"enable_load_balancing"`
			EnableAutoRetry     bool `json:"enable_auto_retry" yaml:"enable_auto_retry"`
			EnableRateLimiting  bool `json:"enable_rate_limiting" yaml:"enable_rate_limiting"`
		}{
			EnableFailover:      true,
			EnableLoadBalancing: true,
			EnableAutoRetry:     true,
			EnableRateLimiting:  true,
		},
	}
}

// LoadFromFile loads configuration from a JSON or YAML file
func (cm *ConfigManager) LoadFromFile(filename string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var configFile ConfigFile

	// Determine file format by extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &configFile); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &configFile); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Apply configuration
	if configFile.Global != nil {
		cm.globalConfig = configFile.Global
	}

	if configFile.LLM != nil {
		for name, config := range configFile.LLM {
			cm.llmConfigs[name] = config
		}
	}

	if configFile.Embedding != nil {
		for name, config := range configFile.Embedding {
			cm.embeddingConfigs[name] = config
		}
	}

	// Apply environment overrides if enabled
	if cm.environmentOverrides {
		cm.applyEnvironmentOverrides()
	}

	// Update file tracking
	cm.configFile = filename
	if stat, err := os.Stat(filename); err == nil {
		cm.lastModified = stat.ModTime()
	}

	return nil
}

// LoadFromEnvironment loads configuration from environment variables
func (cm *ConfigManager) LoadFromEnvironment() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Load global configuration from environment
	cm.loadGlobalConfigFromEnv()

	// Load provider configurations from environment
	cm.loadProviderConfigsFromEnv()

	return nil
}

// SaveToFile saves current configuration to a file
func (cm *ConfigManager) SaveToFile(filename string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	configFile := ConfigFile{
		Global:    cm.globalConfig,
		LLM:       cm.llmConfigs,
		Embedding: cm.embeddingConfigs,
		Version:   "1.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	var data []byte
	var err error

	// Determine file format by extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(configFile, "", "  ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(configFile)
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetGlobalConfig returns the global configuration
func (cm *ConfigManager) GetGlobalConfig() *GlobalConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent modification
	config := *cm.globalConfig
	return &config
}

// SetGlobalConfig updates the global configuration
func (cm *ConfigManager) SetGlobalConfig(config *GlobalConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.globalConfig = config
}

// GetLLMConfig returns configuration for a specific LLM provider
func (cm *ConfigManager) GetLLMConfig(name string) (*ProviderConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	config, exists := cm.llmConfigs[name]
	if !exists {
		return nil, fmt.Errorf("LLM provider config not found: %s", name)
	}

	// Return a copy to prevent modification
	configCopy := *config
	return &configCopy, nil
}

// SetLLMConfig sets configuration for a specific LLM provider
func (cm *ConfigManager) SetLLMConfig(name string, config *ProviderConfig) error {
	if err := ValidateProviderConfig(config); err != nil {
		return fmt.Errorf("invalid provider config: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.llmConfigs[name] = config
	return nil
}

// GetEmbeddingConfig returns configuration for a specific embedding provider
func (cm *ConfigManager) GetEmbeddingConfig(name string) (*EmbeddingProviderConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	config, exists := cm.embeddingConfigs[name]
	if !exists {
		return nil, fmt.Errorf("embedding provider config not found: %s", name)
	}

	// Return a copy to prevent modification
	configCopy := *config
	return &configCopy, nil
}

// SetEmbeddingConfig sets configuration for a specific embedding provider
func (cm *ConfigManager) SetEmbeddingConfig(name string, config *EmbeddingProviderConfig) error {
	if err := ValidateEmbeddingProviderConfig(config); err != nil {
		return fmt.Errorf("invalid embedding provider config: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.embeddingConfigs[name] = config
	return nil
}

// ListLLMConfigs returns all LLM provider configurations
func (cm *ConfigManager) ListLLMConfigs() map[string]*ProviderConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*ProviderConfig)
	for name, config := range cm.llmConfigs {
		configCopy := *config
		result[name] = &configCopy
	}
	return result
}

// ListEmbeddingConfigs returns all embedding provider configurations
func (cm *ConfigManager) ListEmbeddingConfigs() map[string]*EmbeddingProviderConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*EmbeddingProviderConfig)
	for name, config := range cm.embeddingConfigs {
		configCopy := *config
		result[name] = &configCopy
	}
	return result
}

// RemoveLLMConfig removes a LLM provider configuration
func (cm *ConfigManager) RemoveLLMConfig(name string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.llmConfigs[name]; !exists {
		return fmt.Errorf("LLM provider config not found: %s", name)
	}

	delete(cm.llmConfigs, name)
	return nil
}

// RemoveEmbeddingConfig removes an embedding provider configuration
func (cm *ConfigManager) RemoveEmbeddingConfig(name string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.embeddingConfigs[name]; !exists {
		return fmt.Errorf("embedding provider config not found: %s", name)
	}

	delete(cm.embeddingConfigs, name)
	return nil
}

// EnableAutoReload enables automatic configuration reloading from file
func (cm *ConfigManager) EnableAutoReload(interval time.Duration) {
	cm.mu.Lock()
	cm.autoReload = true
	cm.mu.Unlock()

	if cm.configFile != "" {
		go cm.autoReloadWorker(interval)
	}
}

// DisableAutoReload disables automatic configuration reloading
func (cm *ConfigManager) DisableAutoReload() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.autoReload = false
}

// SetEnvironmentOverrides enables or disables environment variable overrides
func (cm *ConfigManager) SetEnvironmentOverrides(enabled bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.environmentOverrides = enabled

	if enabled {
		cm.applyEnvironmentOverrides()
	}
}

// Reload reloads configuration from the current source
func (cm *ConfigManager) Reload() error {
	if cm.configFile != "" {
		return cm.LoadFromFile(cm.configFile)
	}
	return cm.LoadFromEnvironment()
}

// Private helper methods

func (cm *ConfigManager) loadGlobalConfigFromEnv() {
	if val := os.Getenv("AI_MEMORY_DEFAULT_LLM_PROVIDER"); val != "" {
		cm.globalConfig.DefaultLLMProvider = val
	}

	if val := os.Getenv("AI_MEMORY_DEFAULT_EMBEDDING_PROVIDER"); val != "" {
		cm.globalConfig.DefaultEmbeddingProvider = val
	}

	if val := os.Getenv("AI_MEMORY_DEFAULT_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			cm.globalConfig.DefaultTimeout = duration
		}
	}

	if val := os.Getenv("AI_MEMORY_MAX_CONCURRENT_REQUESTS"); val != "" {
		if num, err := strconv.Atoi(val); err == nil {
			cm.globalConfig.MaxConcurrentRequests = num
		}
	}

	if val := os.Getenv("AI_MEMORY_LOG_LEVEL"); val != "" {
		cm.globalConfig.LogLevel = val
	}

	if val := os.Getenv("AI_MEMORY_ENABLE_METRICS"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			cm.globalConfig.EnableMetrics = enabled
		}
	}

	if val := os.Getenv("AI_MEMORY_METRICS_ENDPOINT"); val != "" {
		cm.globalConfig.MetricsEndpoint = val
	}

	// Load feature flags
	if val := os.Getenv("AI_MEMORY_ENABLE_FAILOVER"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			cm.globalConfig.Features.EnableFailover = enabled
		}
	}

	if val := os.Getenv("AI_MEMORY_ENABLE_LOAD_BALANCING"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			cm.globalConfig.Features.EnableLoadBalancing = enabled
		}
	}
}

func (cm *ConfigManager) loadProviderConfigsFromEnv() {
	// Load OpenAI configuration
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		config := DefaultProviderConfig(ProviderOpenAI)
		config.APIKey = apiKey

		if model := os.Getenv("OPENAI_MODEL"); model != "" {
			config.Model = model
		}

		if endpoint := os.Getenv("OPENAI_ENDPOINT"); endpoint != "" {
			config.Endpoint = endpoint
		}

		cm.llmConfigs["openai"] = config

		// Also create embedding config
		embeddingConfig := DefaultEmbeddingProviderConfig(EmbeddingProviderOpenAI)
		embeddingConfig.APIKey = apiKey

		if model := os.Getenv("OPENAI_EMBEDDING_MODEL"); model != "" {
			embeddingConfig.Model = model
		}

		cm.embeddingConfigs["openai"] = embeddingConfig
	}

	// Load Anthropic configuration
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		config := DefaultProviderConfig(ProviderAnthropic)
		config.APIKey = apiKey

		if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
			config.Model = model
		}

		cm.llmConfigs["anthropic"] = config
	}

	// Load Gemini configuration
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		config := DefaultProviderConfig(ProviderGemini)
		config.APIKey = apiKey

		if model := os.Getenv("GEMINI_MODEL"); model != "" {
			config.Model = model
		}

		cm.llmConfigs["gemini"] = config
	}

	// Load DeepSeek configuration
	if apiKey := os.Getenv("DEEPSEEK_API_KEY"); apiKey != "" {
		config := DefaultProviderConfig(ProviderDeepSeek)
		config.APIKey = apiKey

		if model := os.Getenv("DEEPSEEK_MODEL"); model != "" {
			config.Model = model
		}

		cm.llmConfigs["deepseek"] = config
	}

	// Load Ollama configuration
	if endpoint := os.Getenv("OLLAMA_ENDPOINT"); endpoint != "" {
		config := DefaultProviderConfig(ProviderOllama)
		config.Endpoint = endpoint

		if model := os.Getenv("OLLAMA_MODEL"); model != "" {
			config.Model = model
		}

		cm.llmConfigs["ollama"] = config

		// Also create embedding config
		embeddingConfig := DefaultEmbeddingProviderConfig(EmbeddingProviderOllama)
		embeddingConfig.Endpoint = endpoint

		if model := os.Getenv("OLLAMA_EMBEDDING_MODEL"); model != "" {
			embeddingConfig.Model = model
		}

		cm.embeddingConfigs["ollama"] = embeddingConfig
	}
}

func (cm *ConfigManager) applyEnvironmentOverrides() {
	cm.loadGlobalConfigFromEnv()
	cm.loadProviderConfigsFromEnv()
}

func (cm *ConfigManager) autoReloadWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		cm.mu.RLock()
		autoReload := cm.autoReload
		configFile := cm.configFile
		lastModified := cm.lastModified
		cm.mu.RUnlock()

		if !autoReload || configFile == "" {
			return
		}

		// Check if file has been modified
		if stat, err := os.Stat(configFile); err == nil {
			if stat.ModTime().After(lastModified) {
				if err := cm.LoadFromFile(configFile); err != nil {
					// Log error but continue
					fmt.Printf("Failed to reload config: %v\n", err)
				}
			}
		}
	}
}
