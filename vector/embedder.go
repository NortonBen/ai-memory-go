// Package vector - AutoEmbedder implementation with provider abstraction
package vector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// AutoEmbedder manages multiple embedding providers with caching and fallback
type AutoEmbedder struct {
	providers map[string]EmbeddingProvider
	primary   string
	fallbacks []string
	cache     EmbeddingCache
	mu        sync.RWMutex
}

// NewAutoEmbedder creates a new AutoEmbedder
func NewAutoEmbedder(primary string, cache EmbeddingCache) *AutoEmbedder {
	return &AutoEmbedder{
		providers: make(map[string]EmbeddingProvider),
		primary:   primary,
		fallbacks: make([]string, 0),
		cache:     cache,
	}
}

// AddProvider adds an embedding provider
func (ae *AutoEmbedder) AddProvider(name string, provider EmbeddingProvider) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	
	ae.providers[name] = provider
}

// SetPrimary sets the primary provider
func (ae *AutoEmbedder) SetPrimary(name string) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	
	if _, exists := ae.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}
	
	ae.primary = name
	return nil
}

// AddFallback adds a fallback provider
func (ae *AutoEmbedder) AddFallback(name string) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	
	if _, exists := ae.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}
	
	ae.fallbacks = append(ae.fallbacks, name)
	return nil
}

// GenerateEmbedding generates an embedding for a single text
func (ae *AutoEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Check cache first
	if ae.cache != nil {
		cacheKey := ae.generateCacheKey(text)
		if cached, found := ae.cache.Get(ctx, cacheKey); found {
			return cached, nil
		}
	}
	
	// Try primary provider
	embedding, primaryErr := ae.tryProvider(ctx, ae.primary, text)
	if primaryErr == nil {
		// Cache the result
		if ae.cache != nil {
			cacheKey := ae.generateCacheKey(text)
			_ = ae.cache.Set(ctx, cacheKey, embedding, 24*time.Hour)
		}
		return embedding, nil
	}
	
	lastErr := primaryErr
	// Try fallback providers
	for _, fallback := range ae.fallbacks {
		embedding, err := ae.tryProvider(ctx, fallback, text)
		if err == nil {
			// Cache the result
			if ae.cache != nil {
				cacheKey := ae.generateCacheKey(text)
				_ = ae.cache.Set(ctx, cacheKey, embedding, 24*time.Hour)
			}
			return embedding, nil
		}
		lastErr = err
	}
	
	return nil, fmt.Errorf("all embedding providers failed. last error: %v", lastErr)
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (ae *AutoEmbedder) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	uncachedIndices := make([]int, 0)
	uncachedTexts := make([]string, 0)
	
	// Check cache for each text
	for i, text := range texts {
		if ae.cache != nil {
			cacheKey := ae.generateCacheKey(text)
			if cached, found := ae.cache.Get(ctx, cacheKey); found {
				embeddings[i] = cached
				continue
			}
		}
		uncachedIndices = append(uncachedIndices, i)
		uncachedTexts = append(uncachedTexts, text)
	}
	
	// If all cached, return immediately
	if len(uncachedTexts) == 0 {
		return embeddings, nil
	}
	
	// Try primary provider for batch
	ae.mu.RLock()
	primaryProvider, exists := ae.providers[ae.primary]
	ae.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("primary provider %s not found", ae.primary)
	}
	
	batchEmbeddings, err := primaryProvider.GenerateBatchEmbeddings(ctx, uncachedTexts)
	if err == nil {
		// Fill in the embeddings and cache them
		for i, embedding := range batchEmbeddings {
			idx := uncachedIndices[i]
			embeddings[idx] = embedding
			
			// Cache the result
			if ae.cache != nil {
				cacheKey := ae.generateCacheKey(uncachedTexts[i])
				_ = ae.cache.Set(ctx, cacheKey, embedding, 24*time.Hour)
			}
		}
		return embeddings, nil
	}
	
	// Fallback to individual generation
	for i, text := range uncachedTexts {
		embedding, err := ae.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}
		idx := uncachedIndices[i]
		embeddings[idx] = embedding
	}
	
	return embeddings, nil
}

// tryProvider tries to generate embedding with a specific provider
func (ae *AutoEmbedder) tryProvider(ctx context.Context, providerName, text string) ([]float32, error) {
	ae.mu.RLock()
	provider, exists := ae.providers[providerName]
	ae.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("provider %s not found", providerName)
	}
	
	return provider.GenerateEmbedding(ctx, text)
}

// GetDimensions returns the embedding dimensions from the primary provider
func (ae *AutoEmbedder) GetDimensions() int {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	
	if provider, exists := ae.providers[ae.primary]; exists {
		return provider.GetDimensions()
	}
	
	return 0
}

// GetModel returns the model name from the primary provider
func (ae *AutoEmbedder) GetModel() string {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	
	if provider, exists := ae.providers[ae.primary]; exists {
		return provider.GetModel()
	}
	
	return ""
}

// Health checks the health of all providers
func (ae *AutoEmbedder) Health(ctx context.Context) error {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	
	// Check primary provider
	if provider, exists := ae.providers[ae.primary]; exists {
		if err := provider.Health(ctx); err != nil {
			return fmt.Errorf("primary provider %s unhealthy: %w", ae.primary, err)
		}
	} else {
		return fmt.Errorf("primary provider %s not found", ae.primary)
	}
	
	return nil
}

// generateCacheKey generates a cache key for a text
func (ae *AutoEmbedder) generateCacheKey(text string) string {
	hash := sha256.Sum256([]byte(text))
	return fmt.Sprintf("emb_%x", hash)
}

// InMemoryEmbeddingCache implements EmbeddingCache using in-memory storage
type InMemoryEmbeddingCache struct {
	cache map[string]*cacheEntry
	mu    sync.RWMutex
}

type cacheEntry struct {
	embedding []float32
	expiresAt time.Time
}

// NewInMemoryEmbeddingCache creates a new in-memory embedding cache
func NewInMemoryEmbeddingCache() *InMemoryEmbeddingCache {
	cache := &InMemoryEmbeddingCache{
		cache: make(map[string]*cacheEntry),
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves an embedding from cache
func (c *InMemoryEmbeddingCache) Get(ctx context.Context, key string) ([]float32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	
	return entry.embedding, true
}

// Set stores an embedding in cache
func (c *InMemoryEmbeddingCache) Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache[key] = &cacheEntry{
		embedding: embedding,
		expiresAt: time.Now().Add(ttl),
	}
	
	return nil
}

// Delete removes an embedding from cache
func (c *InMemoryEmbeddingCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.cache, key)
	return nil
}

// Clear removes all embeddings from cache
func (c *InMemoryEmbeddingCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache = make(map[string]*cacheEntry)
	return nil
}

// cleanup periodically removes expired entries
func (c *InMemoryEmbeddingCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.expiresAt) {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// GetSize returns the number of cached embeddings
func (c *InMemoryEmbeddingCache) GetSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.cache)
}
