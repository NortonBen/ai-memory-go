// Package parser - Caching system for frequently parsed content
package parser

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CachePolicy defines different cache eviction policies
type CachePolicy string

const (
	PolicyLRU  CachePolicy = "lru"  // Least Recently Used
	PolicyLFU  CachePolicy = "lfu"  // Least Frequently Used
	PolicyTTL  CachePolicy = "ttl"  // Time To Live based
	PolicyFIFO CachePolicy = "fifo" // First In First Out
)

// CacheConfig configures the parsing cache behavior
type CacheConfig struct {
	// MaxSize is the maximum number of entries to cache
	MaxSize int `json:"max_size"`

	// MaxMemoryMB is the maximum memory usage in megabytes
	MaxMemoryMB int64 `json:"max_memory_mb"`

	// TTL is the default time-to-live for cache entries
	TTL time.Duration `json:"ttl"`

	// Policy determines the eviction strategy
	Policy CachePolicy `json:"policy"`

	// EnablePersistence enables cache persistence across sessions
	EnablePersistence bool `json:"enable_persistence"`

	// PersistencePath is the file path for cache persistence
	PersistencePath string `json:"persistence_path"`

	// CheckFileModTime enables file modification time checking
	CheckFileModTime bool `json:"check_file_mod_time"`

	// EnableMetrics enables cache hit/miss metrics collection
	EnableMetrics bool `json:"enable_metrics"`

	// CleanupInterval determines how often to run cache cleanup
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// EnableCompression enables content compression to save memory
	EnableCompression bool `json:"enable_compression"`

	// CompressionThreshold minimum content size to trigger compression (bytes)
	CompressionThreshold int `json:"compression_threshold"`

	// EnableAsyncPersistence enables background persistence operations
	EnableAsyncPersistence bool `json:"enable_async_persistence"`

	// PersistenceInterval how often to persist cache to disk
	PersistenceInterval time.Duration `json:"persistence_interval"`

	// EnableWarmup enables cache warmup on startup
	EnableWarmup bool `json:"enable_warmup"`

	// WarmupFiles list of files to preload into cache
	WarmupFiles []string `json:"warmup_files"`

	// MaxConcurrentOperations limits concurrent cache operations
	MaxConcurrentOperations int `json:"max_concurrent_operations"`

	// EnableDistributedCache enables distributed caching (future extension)
	EnableDistributedCache bool `json:"enable_distributed_cache"`
}

// DefaultCacheConfig returns sensible defaults for cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:                 1000,
		MaxMemoryMB:             100, // 100MB default
		TTL:                     24 * time.Hour,
		Policy:                  PolicyLRU,
		EnablePersistence:       false,
		PersistencePath:         ".cache/parser_cache.json",
		CheckFileModTime:        true,
		EnableMetrics:           true,
		CleanupInterval:         5 * time.Minute,
		EnableCompression:       true,
		CompressionThreshold:    1024, // 1KB
		EnableAsyncPersistence:  true,
		PersistenceInterval:     10 * time.Minute,
		EnableWarmup:            false,
		WarmupFiles:             []string{},
		MaxConcurrentOperations: 100,
		EnableDistributedCache:  false,
	}
}

// CacheEntry represents a cached parsing result
type CacheEntry struct {
	Key            string                 `json:"key"`
	Chunks         []Chunk                `json:"chunks"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      time.Time              `json:"created_at"`
	LastAccessed   time.Time              `json:"last_accessed"`
	AccessCount    int64                  `json:"access_count"`
	ExpiresAt      time.Time              `json:"expires_at"`
	FileModTime    time.Time              `json:"file_mod_time,omitempty"`
	FilePath       string                 `json:"file_path,omitempty"`
	ContentHash    string                 `json:"content_hash"`
	EstimatedSize  int64                  `json:"estimated_size"`
	IsCompressed   bool                   `json:"is_compressed"`
	CompressedData []byte                 `json:"compressed_data,omitempty"`
	Priority       int                    `json:"priority"` // Higher priority = less likely to be evicted
	Tags           []string               `json:"tags"`     // For categorization and bulk operations
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	Hits                    int64         `json:"hits"`
	Misses                  int64         `json:"misses"`
	Evictions               int64         `json:"evictions"`
	TotalEntries            int64         `json:"total_entries"`
	MemoryUsageBytes        int64         `json:"memory_usage_bytes"`
	HitRate                 float64       `json:"hit_rate"`
	AverageAccessTime       time.Duration `json:"average_access_time"`
	LastCleanup             time.Time     `json:"last_cleanup"`
	CompressionRatio        float64       `json:"compression_ratio"`
	PersistenceOperations   int64         `json:"persistence_operations"`
	LastPersistence         time.Time     `json:"last_persistence"`
	ConcurrentOperations    int64         `json:"concurrent_operations"`
	MaxConcurrentOperations int64         `json:"max_concurrent_operations"`
	WarmupTime              time.Duration `json:"warmup_time"`
	ErrorCount              int64         `json:"error_count"`
	mu                      sync.RWMutex
}

// ParsingCache defines the interface for parsing result caching
type ParsingCache interface {
	// Get retrieves cached parsing results
	Get(ctx context.Context, key string) ([]Chunk, bool)

	// Set stores parsing results in cache
	Set(ctx context.Context, key string, chunks []Chunk, metadata map[string]interface{}) error

	// SetWithOptions stores parsing results with advanced options
	SetWithOptions(ctx context.Context, key string, chunks []Chunk, metadata map[string]interface{}, options *CacheEntryOptions) error

	// GetByFile retrieves cached results for a specific file
	GetByFile(ctx context.Context, filePath string) ([]Chunk, bool)

	// SetByFile stores parsing results for a specific file
	SetByFile(ctx context.Context, filePath string, chunks []Chunk, metadata map[string]interface{}) error

	// Delete removes an entry from cache
	Delete(ctx context.Context, key string) error

	// DeleteByTag removes all entries with a specific tag
	DeleteByTag(ctx context.Context, tag string) error

	// Clear removes all entries from cache
	Clear(ctx context.Context) error

	// IsValid checks if a cache entry is still valid (file mod time, TTL)
	IsValid(ctx context.Context, key string) bool

	// GetMetrics returns current cache performance metrics
	GetMetrics() *CacheMetrics

	// Cleanup removes expired and evicted entries
	Cleanup(ctx context.Context) error

	// Persist saves cache to persistent storage
	Persist(ctx context.Context) error

	// Load restores cache from persistent storage
	Load(ctx context.Context) error

	// Warmup preloads frequently accessed files
	Warmup(ctx context.Context, filePaths []string) error

	// GetKeys returns all cache keys (for debugging/monitoring)
	GetKeys() []string

	// GetSize returns current cache size information
	GetSize() (entries int, memoryBytes int64)

	// Close gracefully shuts down the cache
	Close() error
}

// CacheEntryOptions provides advanced options for cache entries
type CacheEntryOptions struct {
	TTL      *time.Duration `json:"ttl,omitempty"`
	Priority int            `json:"priority"`
	Tags     []string       `json:"tags"`
	Compress bool           `json:"compress"`
}

// Compressor interface for content compression
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// GzipCompressor implements compression using gzip
type GzipCompressor struct{}

func (gc *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (gc *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// InMemoryParsingCache implements ParsingCache using in-memory storage
type InMemoryParsingCache struct {
	config  *CacheConfig
	entries map[string]*CacheEntry
	metrics *CacheMetrics
	mu      sync.RWMutex

	// LRU tracking
	lruList *lruNode
	lruMap  map[string]*lruNode

	// Tag-based indexing
	tagIndex map[string]map[string]bool // tag -> set of keys

	// Cleanup ticker
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}

	// Persistence ticker
	persistenceTicker *time.Ticker
	stopPersistence   chan struct{}

	// Concurrency control
	semaphore chan struct{}

	// Compression support
	compressor Compressor
}

// lruNode represents a node in the LRU doubly-linked list
type lruNode struct {
	key  string
	prev *lruNode
	next *lruNode
}

// NewInMemoryParsingCache creates a new in-memory parsing cache
func NewInMemoryParsingCache(config *CacheConfig) *InMemoryParsingCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &InMemoryParsingCache{
		config:          config,
		entries:         make(map[string]*CacheEntry),
		metrics:         &CacheMetrics{},
		lruMap:          make(map[string]*lruNode),
		tagIndex:        make(map[string]map[string]bool),
		stopCleanup:     make(chan struct{}),
		stopPersistence: make(chan struct{}),
		semaphore:       make(chan struct{}, config.MaxConcurrentOperations),
		compressor:      &GzipCompressor{},
	}

	// Initialize LRU list with dummy head and tail
	cache.lruList = &lruNode{}
	cache.lruList.next = &lruNode{}
	cache.lruList.next.prev = cache.lruList

	// Start cleanup routine if enabled
	if config.CleanupInterval > 0 {
		cache.startCleanup()
	}

	// Start persistence routine if enabled
	if config.EnableAsyncPersistence && config.PersistenceInterval > 0 {
		cache.startPersistence()
	}

	// Load from persistent storage if enabled
	if config.EnablePersistence {
		cache.Load(context.Background())
	}

	// Warmup cache if enabled
	if config.EnableWarmup && len(config.WarmupFiles) > 0 {
		go cache.Warmup(context.Background(), config.WarmupFiles)
	}

	return cache
}

// generateCacheKey creates a cache key from content or file path
func (c *InMemoryParsingCache) generateCacheKey(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("parse_%x", hash[:16])
}

// generateFileKey creates a cache key specifically for file paths
func (c *InMemoryParsingCache) generateFileKey(filePath string) string {
	// Include file path and modification time in key
	stat, err := os.Stat(filePath)
	if err != nil {
		return c.generateCacheKey(filePath)
	}

	keyData := fmt.Sprintf("%s_%d", filePath, stat.ModTime().Unix())
	return c.generateCacheKey(keyData)
}

// Get retrieves cached parsing results
func (c *InMemoryParsingCache) Get(ctx context.Context, key string) ([]Chunk, bool) {
	startTime := time.Now()
	defer func() {
		c.metrics.mu.Lock()
		c.metrics.AverageAccessTime = (c.metrics.AverageAccessTime + time.Since(startTime)) / 2
		c.metrics.mu.Unlock()
	}()

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.recordMiss()
		return nil, false
	}

	// Check if entry is expired
	if time.Now().After(entry.ExpiresAt) {
		c.recordMiss()
		// Remove expired entry
		go func() {
			c.Delete(context.Background(), key)
		}()
		return nil, false
	}

	// Check file modification time if enabled and file path is available
	if c.config.CheckFileModTime && entry.FilePath != "" {
		if !c.isFileValid(entry) {
			c.recordMiss()
			// Remove invalidated entry
			go func() {
				c.Delete(context.Background(), key)
			}()
			return nil, false
		}
	}

	// Handle compressed data
	var chunks []Chunk
	if entry.IsCompressed && len(entry.CompressedData) > 0 {
		decompressed, err := c.compressor.Decompress(entry.CompressedData)
		if err != nil {
			c.recordMiss()
			c.metrics.mu.Lock()
			c.metrics.ErrorCount++
			c.metrics.mu.Unlock()
			return nil, false
		}

		if err := json.Unmarshal(decompressed, &chunks); err != nil {
			c.recordMiss()
			c.metrics.mu.Lock()
			c.metrics.ErrorCount++
			c.metrics.mu.Unlock()
			return nil, false
		}
	} else {
		chunks = entry.Chunks
	}

	// Update access information
	c.mu.Lock()
	entry.LastAccessed = time.Now()
	entry.AccessCount++
	c.mu.Unlock()

	// Update LRU position
	c.updateLRU(key)

	c.recordHit()
	return chunks, true
}

// Set stores parsing results in cache
func (c *InMemoryParsingCache) Set(ctx context.Context, key string, chunks []Chunk, metadata map[string]interface{}) error {
	return c.SetWithOptions(ctx, key, chunks, metadata, nil)
}

// SetWithOptions stores parsing results with advanced options
func (c *InMemoryParsingCache) SetWithOptions(ctx context.Context, key string, chunks []Chunk, metadata map[string]interface{}, options *CacheEntryOptions) error {
	if len(chunks) == 0 {
		return fmt.Errorf("cannot cache empty chunks")
	}

	// Acquire semaphore for concurrency control
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	c.metrics.mu.Lock()
	c.metrics.ConcurrentOperations++
	if c.metrics.ConcurrentOperations > c.metrics.MaxConcurrentOperations {
		c.metrics.MaxConcurrentOperations = c.metrics.ConcurrentOperations
	}
	c.metrics.mu.Unlock()

	defer func() {
		c.metrics.mu.Lock()
		c.metrics.ConcurrentOperations--
		c.metrics.mu.Unlock()
	}()

	// Apply default options if not provided
	if options == nil {
		options = &CacheEntryOptions{
			Priority: 1,
			Compress: c.config.EnableCompression,
		}
	}

	// Calculate estimated size
	estimatedSize := c.calculateSize(chunks, metadata)

	// Compress content if enabled and above threshold
	var compressedData []byte
	var isCompressed bool
	if options.Compress && estimatedSize > int64(c.config.CompressionThreshold) {
		chunksData, err := json.Marshal(chunks)
		if err == nil {
			if compressed, err := c.compressor.Compress(chunksData); err == nil {
				compressedData = compressed
				isCompressed = true
				// Update compression ratio metric
				c.metrics.mu.Lock()
				ratio := float64(len(compressed)) / float64(len(chunksData))
				c.metrics.CompressionRatio = (c.metrics.CompressionRatio + ratio) / 2
				c.metrics.mu.Unlock()
			}
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries
	c.evictIfNeeded(estimatedSize)

	// Determine TTL
	ttl := c.config.TTL
	if options.TTL != nil {
		ttl = *options.TTL
	}

	entry := &CacheEntry{
		Key:            key,
		Chunks:         chunks,
		Metadata:       metadata,
		CreatedAt:      time.Now(),
		LastAccessed:   time.Now(),
		AccessCount:    0,
		ExpiresAt:      time.Now().Add(ttl),
		ContentHash:    c.generateContentHash(chunks),
		EstimatedSize:  estimatedSize,
		IsCompressed:   isCompressed,
		CompressedData: compressedData,
		Priority:       options.Priority,
		Tags:           options.Tags,
	}

	c.entries[key] = entry
	c.metrics.TotalEntries++
	c.metrics.MemoryUsageBytes += estimatedSize

	// Add to LRU tracking
	c.addToLRUUnsafe(key)

	// Update tag index
	for _, tag := range options.Tags {
		if c.tagIndex[tag] == nil {
			c.tagIndex[tag] = make(map[string]bool)
		}
		c.tagIndex[tag][key] = true
	}

	return nil
}

// GetByFile retrieves cached results for a specific file
func (c *InMemoryParsingCache) GetByFile(ctx context.Context, filePath string) ([]Chunk, bool) {
	key := c.generateFileKey(filePath)
	return c.Get(ctx, key)
}

// SetByFile stores parsing results for a specific file
func (c *InMemoryParsingCache) SetByFile(ctx context.Context, filePath string, chunks []Chunk, metadata map[string]interface{}) error {
	key := c.generateFileKey(filePath)

	// Get file modification time
	stat, err := os.Stat(filePath)
	var modTime time.Time
	if err == nil {
		modTime = stat.ModTime()
	}

	// Add file-specific metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["file_path"] = filePath
	metadata["file_mod_time"] = modTime

	// Store with file-specific key
	err = c.Set(ctx, key, chunks, metadata)
	if err != nil {
		return err
	}

	// Update entry with file information
	c.mu.Lock()
	if entry, exists := c.entries[key]; exists {
		entry.FilePath = filePath
		entry.FileModTime = modTime
	}
	c.mu.Unlock()

	return nil
}

// Delete removes an entry from cache
func (c *InMemoryParsingCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil
	}

	delete(c.entries, key)
	c.metrics.TotalEntries--
	c.metrics.MemoryUsageBytes -= entry.EstimatedSize

	// Remove from LRU tracking
	c.removeFromLRUUnsafe(key)

	// Remove from tag indexes
	for _, tag := range entry.Tags {
		if c.tagIndex[tag] != nil {
			delete(c.tagIndex[tag], key)
			if len(c.tagIndex[tag]) == 0 {
				delete(c.tagIndex, tag)
			}
		}
	}

	return nil
}

// Clear removes all entries from cache
func (c *InMemoryParsingCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.lruMap = make(map[string]*lruNode)
	c.tagIndex = make(map[string]map[string]bool)
	c.metrics.TotalEntries = 0
	c.metrics.MemoryUsageBytes = 0

	// Reset LRU list
	c.lruList.next = &lruNode{}
	c.lruList.next.prev = c.lruList

	return nil
}

// IsValid checks if a cache entry is still valid
func (c *InMemoryParsingCache) IsValid(ctx context.Context, key string) bool {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return false
	}

	// Check TTL expiration
	if time.Now().After(entry.ExpiresAt) {
		return false
	}

	// Check file modification time if applicable
	if c.config.CheckFileModTime && entry.FilePath != "" {
		return c.isFileValid(entry)
	}

	return true
}

// GetMetrics returns current cache performance metrics
func (c *InMemoryParsingCache) GetMetrics() *CacheMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	// Calculate hit rate
	total := c.metrics.Hits + c.metrics.Misses
	if total > 0 {
		c.metrics.HitRate = float64(c.metrics.Hits) / float64(total)
	}

	// Return a copy to prevent external modification
	return &CacheMetrics{
		Hits:                    c.metrics.Hits,
		Misses:                  c.metrics.Misses,
		Evictions:               c.metrics.Evictions,
		TotalEntries:            c.metrics.TotalEntries,
		MemoryUsageBytes:        c.metrics.MemoryUsageBytes,
		HitRate:                 c.metrics.HitRate,
		AverageAccessTime:       c.metrics.AverageAccessTime,
		LastCleanup:             c.metrics.LastCleanup,
		CompressionRatio:        c.metrics.CompressionRatio,
		PersistenceOperations:   c.metrics.PersistenceOperations,
		LastPersistence:         c.metrics.LastPersistence,
		ConcurrentOperations:    c.metrics.ConcurrentOperations,
		MaxConcurrentOperations: c.metrics.MaxConcurrentOperations,
		WarmupTime:              c.metrics.WarmupTime,
		ErrorCount:              c.metrics.ErrorCount,
	}
}

// Cleanup removes expired and evicted entries
func (c *InMemoryParsingCache) Cleanup(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var keysToDelete []string

	// Find expired entries
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			keysToDelete = append(keysToDelete, key)
		} else if c.config.CheckFileModTime && entry.FilePath != "" && !c.isFileValid(entry) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Delete expired entries
	for _, key := range keysToDelete {
		if entry, exists := c.entries[key]; exists {
			delete(c.entries, key)
			c.metrics.TotalEntries--
			c.metrics.MemoryUsageBytes -= entry.EstimatedSize
			c.removeFromLRUUnsafe(key)

			// Remove from tag indexes
			for _, tag := range entry.Tags {
				if c.tagIndex[tag] != nil {
					delete(c.tagIndex[tag], key)
					if len(c.tagIndex[tag]) == 0 {
						delete(c.tagIndex, tag)
					}
				}
			}
		}
	}

	c.metrics.LastCleanup = now
	return nil
}

// DeleteByTag removes all entries with a specific tag
func (c *InMemoryParsingCache) DeleteByTag(ctx context.Context, tag string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys, exists := c.tagIndex[tag]
	if !exists {
		return nil
	}

	for key := range keys {
		if entry, exists := c.entries[key]; exists {
			delete(c.entries, key)
			c.metrics.TotalEntries--
			c.metrics.MemoryUsageBytes -= entry.EstimatedSize
			c.removeFromLRUUnsafe(key)

			// Remove from other tag indexes
			for _, entryTag := range entry.Tags {
				if c.tagIndex[entryTag] != nil {
					delete(c.tagIndex[entryTag], key)
					if len(c.tagIndex[entryTag]) == 0 {
						delete(c.tagIndex, entryTag)
					}
				}
			}
		}
	}

	delete(c.tagIndex, tag)
	return nil
}

// Persist saves cache to persistent storage
func (c *InMemoryParsingCache) Persist(ctx context.Context) error {
	if !c.config.EnablePersistence {
		return nil
	}

	c.mu.RLock()
	entries := make(map[string]*CacheEntry, len(c.entries))
	for k, v := range c.entries {
		entries[k] = v
	}
	c.mu.RUnlock()

	// Create directory if it doesn't exist
	dir := filepath.Dir(c.config.PersistencePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write to temporary file first
	tempFile := c.config.PersistencePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(entries); err != nil {
		return fmt.Errorf("failed to encode cache data: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, c.config.PersistencePath); err != nil {
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	c.metrics.mu.Lock()
	c.metrics.PersistenceOperations++
	c.metrics.LastPersistence = time.Now()
	c.metrics.mu.Unlock()

	return nil
}

// Load restores cache from persistent storage
func (c *InMemoryParsingCache) Load(ctx context.Context) error {
	if !c.config.EnablePersistence {
		return nil
	}

	file, err := os.Open(c.config.PersistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file exists yet
		}
		return fmt.Errorf("failed to open cache file: %w", err)
	}
	defer file.Close()

	var entries map[string]*CacheEntry
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&entries); err != nil {
		return fmt.Errorf("failed to decode cache data: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Restore entries and rebuild indexes
	now := time.Now()
	for key, entry := range entries {
		// Skip expired entries
		if now.After(entry.ExpiresAt) {
			continue
		}

		c.entries[key] = entry
		c.metrics.TotalEntries++
		c.metrics.MemoryUsageBytes += entry.EstimatedSize

		// Rebuild LRU
		c.addToLRUUnsafe(key)

		// Rebuild tag index
		for _, tag := range entry.Tags {
			if c.tagIndex[tag] == nil {
				c.tagIndex[tag] = make(map[string]bool)
			}
			c.tagIndex[tag][key] = true
		}
	}

	return nil
}

// Warmup preloads frequently accessed files
func (c *InMemoryParsingCache) Warmup(ctx context.Context, filePaths []string) error {
	startTime := time.Now()
	defer func() {
		c.metrics.mu.Lock()
		c.metrics.WarmupTime = time.Since(startTime)
		c.metrics.mu.Unlock()
	}()

	// This is a placeholder - in a real implementation, you would need
	// access to a parser to actually parse and cache the files
	// For now, we just validate that the files exist
	for _, filePath := range filePaths {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := os.Stat(filePath); err != nil {
			continue // Skip files that don't exist
		}

		// Check if already cached and valid
		key := c.generateFileKey(filePath)
		if c.IsValid(ctx, key) {
			continue
		}

		// In a real implementation, you would parse the file here
		// For now, we just mark it as a warmup attempt
	}

	return nil
}

// GetKeys returns all cache keys (for debugging/monitoring)
func (c *InMemoryParsingCache) GetKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}
	return keys
}

// GetSize returns current cache size information
func (c *InMemoryParsingCache) GetSize() (entries int, memoryBytes int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries), c.metrics.MemoryUsageBytes
}

// Close gracefully shuts down the cache
func (c *InMemoryParsingCache) Close() error {
	// Stop cleanup routine
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
		close(c.stopCleanup)
	}

	// Stop persistence routine
	if c.persistenceTicker != nil {
		c.persistenceTicker.Stop()
		close(c.stopPersistence)
	}

	// Final persistence if enabled
	if c.config.EnablePersistence {
		c.Persist(context.Background())
	}

	// Clear cache
	return c.Clear(context.Background())
}

// Helper methods

// isFileValid checks if a file-based cache entry is still valid
func (c *InMemoryParsingCache) isFileValid(entry *CacheEntry) bool {
	if entry.FilePath == "" {
		return true
	}

	stat, err := os.Stat(entry.FilePath)
	if err != nil {
		return false // File doesn't exist anymore
	}

	return stat.ModTime().Equal(entry.FileModTime) || stat.ModTime().Before(entry.FileModTime)
}

// calculateSize estimates the memory size of chunks and metadata
func (c *InMemoryParsingCache) calculateSize(chunks []Chunk, metadata map[string]interface{}) int64 {
	size := int64(0)

	// Estimate chunk sizes
	for _, chunk := range chunks {
		size += int64(len(chunk.Content))
		size += int64(len(chunk.ID))
		size += int64(len(chunk.Source))
		size += int64(len(chunk.Hash))
		size += 100 // Overhead for struct fields and metadata
	}

	// Estimate metadata size (rough approximation)
	if metadata != nil {
		size += int64(len(fmt.Sprintf("%v", metadata)))
	}

	return size
}

// generateContentHash creates a hash of the chunks for deduplication
func (c *InMemoryParsingCache) generateContentHash(chunks []Chunk) string {
	var content string
	for _, chunk := range chunks {
		content += chunk.Hash
	}
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash[:16])
}

// recordHit increments hit counter
func (c *InMemoryParsingCache) recordHit() {
	if c.config.EnableMetrics {
		c.metrics.mu.Lock()
		c.metrics.Hits++
		c.metrics.mu.Unlock()
	}
}

// recordMiss increments miss counter
func (c *InMemoryParsingCache) recordMiss() {
	if c.config.EnableMetrics {
		c.metrics.mu.Lock()
		c.metrics.Misses++
		c.metrics.mu.Unlock()
	}
}

// evictIfNeeded removes entries if cache limits are exceeded
func (c *InMemoryParsingCache) evictIfNeeded(newEntrySize int64) {
	// Check memory limit
	if c.config.MaxMemoryMB > 0 {
		maxBytes := c.config.MaxMemoryMB * 1024 * 1024
		for c.metrics.MemoryUsageBytes+newEntrySize > maxBytes && len(c.entries) > 0 {
			c.evictOne()
		}
	}

	// Check size limit
	if c.config.MaxSize > 0 {
		for len(c.entries) >= c.config.MaxSize {
			c.evictOne()
		}
	}
}

// evictOne removes one entry based on the configured policy
func (c *InMemoryParsingCache) evictOne() {
	if len(c.entries) == 0 {
		return
	}

	var keyToEvict string

	switch c.config.Policy {
	case PolicyLRU:
		keyToEvict = c.evictLRU()
	case PolicyLFU:
		keyToEvict = c.evictLFU()
	case PolicyTTL:
		keyToEvict = c.evictTTL()
	case PolicyFIFO:
		keyToEvict = c.evictFIFO()
	default:
		keyToEvict = c.evictLRU()
	}

	if keyToEvict != "" {
		if entry, exists := c.entries[keyToEvict]; exists {
			delete(c.entries, keyToEvict)
			c.metrics.TotalEntries--
			c.metrics.MemoryUsageBytes -= entry.EstimatedSize
			c.metrics.Evictions++
			c.removeFromLRUUnsafe(keyToEvict)

			// Remove from tag indexes
			for _, tag := range entry.Tags {
				if c.tagIndex[tag] != nil {
					delete(c.tagIndex[tag], keyToEvict)
					if len(c.tagIndex[tag]) == 0 {
						delete(c.tagIndex, tag)
					}
				}
			}
		}
	} else {
		// Safety fallback: if no key found to evict, just remove the first entry
		for key, entry := range c.entries {
			delete(c.entries, key)
			c.metrics.TotalEntries--
			c.metrics.MemoryUsageBytes -= entry.EstimatedSize
			c.metrics.Evictions++
			c.removeFromLRUUnsafe(key)

			// Remove from tag indexes
			for _, tag := range entry.Tags {
				if c.tagIndex[tag] != nil {
					delete(c.tagIndex[tag], key)
					if len(c.tagIndex[tag]) == 0 {
						delete(c.tagIndex, tag)
					}
				}
			}
			break // Only remove one entry
		}
	}
}

// LRU management methods

// updateLRU moves a key to the front of the LRU list
func (c *InMemoryParsingCache) updateLRU(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeFromLRUUnsafe(key)
	c.addToLRUUnsafe(key)
}

// addToLRU adds a key to the LRU list
func (c *InMemoryParsingCache) addToLRU(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addToLRUUnsafe(key)
}

// addToLRUUnsafe adds a key to the LRU list without locking
func (c *InMemoryParsingCache) addToLRUUnsafe(key string) {
	// Remove if already exists
	c.removeFromLRUUnsafe(key)

	// Add to front of list
	node := &lruNode{key: key}
	node.next = c.lruList.next
	node.prev = c.lruList
	c.lruList.next.prev = node
	c.lruList.next = node

	c.lruMap[key] = node
}

// removeFromLRU removes a key from the LRU list
func (c *InMemoryParsingCache) removeFromLRU(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeFromLRUUnsafe(key)
}

// removeFromLRUUnsafe removes a key from the LRU list without locking
func (c *InMemoryParsingCache) removeFromLRUUnsafe(key string) {
	if node, exists := c.lruMap[key]; exists {
		node.prev.next = node.next
		node.next.prev = node.prev
		delete(c.lruMap, key)
	}
}

// evictLRU returns the least recently used key
func (c *InMemoryParsingCache) evictLRU() string {
	// Get the last node (least recently used)
	// The LRU list structure: head -> most recent -> ... -> least recent -> tail
	if c.lruList.next != nil && c.lruList.next.prev != nil && c.lruList.next.prev != c.lruList {
		return c.lruList.next.prev.key
	}

	// Fallback: if LRU structure is broken, just pick any key
	for key := range c.entries {
		return key
	}

	return ""
}

// evictLFU returns the least frequently used key
func (c *InMemoryParsingCache) evictLFU() string {
	var minKey string
	var minCount int64 = -1

	for key, entry := range c.entries {
		if minCount == -1 || entry.AccessCount < minCount {
			minCount = entry.AccessCount
			minKey = key
		}
	}

	return minKey
}

// evictTTL returns the key with the earliest expiration time
func (c *InMemoryParsingCache) evictTTL() string {
	var earliestKey string
	var earliestTime time.Time

	for key, entry := range c.entries {
		if earliestTime.IsZero() || entry.ExpiresAt.Before(earliestTime) {
			earliestTime = entry.ExpiresAt
			earliestKey = key
		}
	}

	return earliestKey
}

// evictFIFO returns the oldest key (first in, first out)
func (c *InMemoryParsingCache) evictFIFO() string {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestTime.IsZero() || entry.CreatedAt.Before(oldestTime) {
			oldestTime = entry.CreatedAt
			oldestKey = key
		}
	}

	return oldestKey
}

// startCleanup starts the periodic cleanup routine
func (c *InMemoryParsingCache) startCleanup() {
	c.cleanupTicker = time.NewTicker(c.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-c.cleanupTicker.C:
				// Use a separate context to avoid blocking
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				c.Cleanup(ctx)
				cancel()
			case <-c.stopCleanup:
				return
			}
		}
	}()
}

// startPersistence starts the periodic persistence routine
func (c *InMemoryParsingCache) startPersistence() {
	c.persistenceTicker = time.NewTicker(c.config.PersistenceInterval)

	go func() {
		for {
			select {
			case <-c.persistenceTicker.C:
				// Use a separate context to avoid blocking
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				c.Persist(ctx)
				cancel()
			case <-c.stopPersistence:
				return
			}
		}
	}()
}
