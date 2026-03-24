// Package parser - Tests for enhanced caching features
package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryParsingCache_EnhancedFeatures(t *testing.T) {
	config := DefaultCacheConfig()
	config.EnableCompression = true
	config.CompressionThreshold = 100
	config.EnablePersistence = true
	config.PersistencePath = filepath.Join(t.TempDir(), "test_cache.json")

	cache := NewInMemoryParsingCache(config)
	defer cache.Close()

	ctx := context.Background()

	// Test compression
	t.Run("Compression", func(t *testing.T) {
		largeContent := make([]byte, 2000)
		for i := range largeContent {
			largeContent[i] = 'A'
		}

		chunks := []Chunk{
			{ID: "1", Content: string(largeContent), Type: ChunkTypeText},
		}

		options := &CacheEntryOptions{
			Compress: true,
			Tags:     []string{"large", "test"},
			Priority: 5,
		}

		err := cache.SetWithOptions(ctx, "large-key", chunks, nil, options)
		require.NoError(t, err)

		// Verify compression worked
		retrievedChunks, found := cache.Get(ctx, "large-key")
		assert.True(t, found)
		assert.Len(t, retrievedChunks, 1)
		assert.Equal(t, string(largeContent), retrievedChunks[0].Content)

		// Check metrics for compression ratio
		metrics := cache.GetMetrics()
		assert.True(t, metrics.CompressionRatio > 0)
	})

	// Test tag-based operations
	t.Run("TagOperations", func(t *testing.T) {
		chunks1 := []Chunk{{ID: "1", Content: "Tagged content 1", Type: ChunkTypeText}}
		chunks2 := []Chunk{{ID: "2", Content: "Tagged content 2", Type: ChunkTypeText}}
		chunks3 := []Chunk{{ID: "3", Content: "Untagged content", Type: ChunkTypeText}}

		options1 := &CacheEntryOptions{Tags: []string{"category1", "important"}}
		options2 := &CacheEntryOptions{Tags: []string{"category1", "normal"}}

		err := cache.SetWithOptions(ctx, "tagged1", chunks1, nil, options1)
		require.NoError(t, err)

		err = cache.SetWithOptions(ctx, "tagged2", chunks2, nil, options2)
		require.NoError(t, err)

		err = cache.Set(ctx, "untagged", chunks3, nil)
		require.NoError(t, err)

		// Verify all entries exist
		_, found := cache.Get(ctx, "tagged1")
		assert.True(t, found)
		_, found = cache.Get(ctx, "tagged2")
		assert.True(t, found)
		_, found = cache.Get(ctx, "untagged")
		assert.True(t, found)

		// Delete by tag
		err = cache.DeleteByTag(ctx, "category1")
		require.NoError(t, err)

		// Verify tagged entries are gone
		_, found = cache.Get(ctx, "tagged1")
		assert.False(t, found)
		_, found = cache.Get(ctx, "tagged2")
		assert.False(t, found)

		// Verify untagged entry still exists
		_, found = cache.Get(ctx, "untagged")
		assert.True(t, found)
	})

	// Test persistence
	t.Run("Persistence", func(t *testing.T) {
		chunks := []Chunk{{ID: "1", Content: "Persistent content", Type: ChunkTypeText}}

		err := cache.Set(ctx, "persistent-key", chunks, nil)
		require.NoError(t, err)

		// Persist to disk
		err = cache.Persist(ctx)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(config.PersistencePath)
		assert.NoError(t, err)

		// Create new cache and load
		newCache := NewInMemoryParsingCache(config)
		defer newCache.Close()

		err = newCache.Load(ctx)
		require.NoError(t, err)

		// Verify data was loaded
		retrievedChunks, found := newCache.Get(ctx, "persistent-key")
		assert.True(t, found)
		assert.Len(t, retrievedChunks, 1)
		assert.Equal(t, "Persistent content", retrievedChunks[0].Content)
	})

	// Test custom TTL
	t.Run("CustomTTL", func(t *testing.T) {
		chunks := []Chunk{{ID: "1", Content: "Short TTL content", Type: ChunkTypeText}}

		shortTTL := 50 * time.Millisecond
		options := &CacheEntryOptions{
			TTL: &shortTTL,
		}

		err := cache.SetWithOptions(ctx, "short-ttl-key", chunks, nil, options)
		require.NoError(t, err)

		// Should be available immediately
		_, found := cache.Get(ctx, "short-ttl-key")
		assert.True(t, found)

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should be expired
		_, found = cache.Get(ctx, "short-ttl-key")
		assert.False(t, found)
	})

	// Test size information
	t.Run("SizeInformation", func(t *testing.T) {
		initialEntries, initialMemory := cache.GetSize()

		chunks := []Chunk{{ID: "1", Content: "Size test content", Type: ChunkTypeText}}
		err := cache.Set(ctx, "size-test-key", chunks, nil)
		require.NoError(t, err)

		newEntries, newMemory := cache.GetSize()
		assert.Equal(t, initialEntries+1, newEntries)
		assert.True(t, newMemory > initialMemory)
	})

	// Test enhanced metrics
	t.Run("EnhancedMetrics", func(t *testing.T) {
		metrics := cache.GetMetrics()

		// Check that new metrics fields are present
		assert.True(t, metrics.CompressionRatio >= 0)
		assert.True(t, metrics.PersistenceOperations >= 0)
		assert.True(t, metrics.ConcurrentOperations >= 0)
		assert.True(t, metrics.MaxConcurrentOperations >= 0)
		assert.True(t, metrics.ErrorCount >= 0)
	})
}

func TestCachedUnifiedParser_EnhancedIntegration(t *testing.T) {
	// Create temporary files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	// Create content that meets minimum chunk size requirements
	content := "Enhanced integration test content. This is a longer piece of text that should be properly chunked by the parser. It contains multiple sentences to ensure proper parsing."
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	config := DefaultChunkingConfig()
	config.MinSize = 10 // Lower minimum size for testing
	cacheConfig := DefaultCacheConfig()
	cacheConfig.EnableCompression = true
	cacheConfig.CompressionThreshold = 10 // Low threshold for testing

	cachedParser := NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	// Test file parsing with enhanced cache
	chunks1, err := cachedParser.ParseFile(ctx, testFile)
	require.NoError(t, err)
	require.True(t, len(chunks1) > 0, "Parser should return at least one chunk")

	// Second call should hit cache
	chunks2, err := cachedParser.ParseFile(ctx, testFile)
	require.NoError(t, err)
	require.True(t, len(chunks2) > 0, "Cached result should return at least one chunk")
	assert.Equal(t, chunks1[0].Content, chunks2[0].Content)

	// Verify cache metrics
	metrics := cachedParser.GetCacheMetrics()
	assert.True(t, metrics.Hits > 0)
	assert.True(t, metrics.HitRate > 0)

	// Test cache size
	entries, memoryBytes := cachedParser.GetCache().GetSize()
	assert.True(t, entries > 0)
	assert.True(t, memoryBytes > 0)
}

func TestCacheConfig_ProductionSettings(t *testing.T) {
	// Test production-ready configuration
	config := &CacheConfig{
		MaxSize:                 10000,
		MaxMemoryMB:             500,
		TTL:                     12 * time.Hour,
		Policy:                  PolicyLRU,
		EnablePersistence:       true,
		PersistencePath:         "/tmp/parser_cache.json",
		CheckFileModTime:        true,
		EnableMetrics:           true,
		CleanupInterval:         10 * time.Minute,
		EnableCompression:       true,
		CompressionThreshold:    2048,
		EnableAsyncPersistence:  true,
		PersistenceInterval:     30 * time.Minute,
		EnableWarmup:            true,
		WarmupFiles:             []string{"/path/to/important/file.txt"},
		MaxConcurrentOperations: 200,
		EnableDistributedCache:  false,
	}

	cache := NewInMemoryParsingCache(config)
	defer cache.Close()

	// Verify configuration is applied
	assert.Equal(t, config, cache.config)

	// Test that cache works with production settings
	ctx := context.Background()
	chunks := []Chunk{{ID: "1", Content: "Production test", Type: ChunkTypeText}}

	err := cache.Set(ctx, "prod-test", chunks, nil)
	assert.NoError(t, err)

	retrievedChunks, found := cache.Get(ctx, "prod-test")
	assert.True(t, found)
	assert.Equal(t, chunks[0].Content, retrievedChunks[0].Content)
}

func BenchmarkEnhancedCacheOperations(b *testing.B) {
	config := DefaultCacheConfig()
	config.EnableCompression = true
	config.CompressionThreshold = 500

	cache := NewInMemoryParsingCache(config)
	defer cache.Close()

	ctx := context.Background()
	chunks := []Chunk{
		{ID: "1", Content: "Benchmark enhanced cache content", Type: ChunkTypeText},
	}

	b.Run("SetWithOptions", func(b *testing.B) {
		options := &CacheEntryOptions{
			Compress: true,
			Tags:     []string{"benchmark", "test"},
			Priority: 3,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := "bench-key-" + string(rune('a'+i%26))
			cache.SetWithOptions(ctx, key, chunks, nil, options)
		}
	})

	b.Run("GetWithCompression", func(b *testing.B) {
		// Pre-populate with compressed entries
		options := &CacheEntryOptions{
			Compress: true,
			Tags:     []string{"benchmark"},
		}

		for i := 0; i < 100; i++ {
			key := "compressed-key-" + string(rune('a'+i%26))
			cache.SetWithOptions(ctx, key, chunks, nil, options)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := "compressed-key-" + string(rune('a'+i%26))
			cache.Get(ctx, key)
		}
	})

	b.Run("DeleteByTag", func(b *testing.B) {
		// Pre-populate with tagged entries
		options := &CacheEntryOptions{
			Tags: []string{"deletable"},
		}

		for i := 0; i < b.N; i++ {
			key := "deletable-key-" + string(rune('a'+i%26))
			cache.SetWithOptions(ctx, key, chunks, nil, options)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.DeleteByTag(ctx, "deletable")
			// Re-add for next iteration
			if i < b.N-1 {
				key := "deletable-key-" + string(rune('a'+i%26))
				cache.SetWithOptions(ctx, key, chunks, nil, options)
			}
		}
	})
}
