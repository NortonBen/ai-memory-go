// Package parser - Enhanced caching examples
package cache_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/cache"
	"github.com/NortonBen/ai-memory-go/parser/core"
)

// ExampleInMemoryParsingCache_enhancedFeatures demonstrates the enhanced caching features
func ExampleInMemoryParsingCache_enhancedFeatures() {
	// Create a production-ready cache configuration
	config := &cache.CacheConfig{
		MaxSize:                 1000,
		MaxMemoryMB:             50,
		TTL:                     1 * time.Hour,
		Policy:                  cache.PolicyLRU,
		EnablePersistence:       true,
		PersistencePath:         "/tmp/parser_cache.json",
		CheckFileModTime:        true,
		EnableMetrics:           true,
		CleanupInterval:         5 * time.Minute,
		EnableCompression:       true,
		CompressionThreshold:    1024,
		EnableAsyncPersistence:  true,
		PersistenceInterval:     10 * time.Minute,
		MaxConcurrentOperations: 50,
	}

	// Create cache with enhanced features
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()

	// Example 1: Basic caching with compression
	largeContent := make([]byte, 2000)
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	chunks := []*schema.Chunk{
		{ID: "1", Content: string(largeContent), Type: schema.ChunkTypeText},
	}

	// Set with compression and tags
	options := &cache.CacheEntryOptions{
		Compress: true,
		Tags:     []string{"large-content", "example"},
		Priority: 5,
	}

	err := pc.SetWithOptions(ctx, "large-key", chunks, nil, options)
	if err != nil {
		fmt.Printf("Error setting cache: %v\n", err)
		return
	}

	// Retrieve compressed content
	retrievedChunks, found := pc.Get(ctx, "large-key")
	if found {
		fmt.Printf("Retrieved %d chunks from cache\n", len(retrievedChunks))
	}

	// Example 2: Tag-based operations
	// Add more tagged content
	smallChunks := []*schema.Chunk{
		{ID: "2", Content: "Small content", Type: schema.ChunkTypeText},
	}

	tagOptions := &cache.CacheEntryOptions{
		Tags: []string{"small-content", "example"},
	}

	pc.SetWithOptions(ctx, "small-key", smallChunks, nil, tagOptions)

	// Delete all content with "example" tag
	pc.DeleteByTag(ctx, "example")

	// Example 3: Metrics monitoring
	_ = pc.GetMetrics()

	// Output:
	// Retrieved 1 chunks from cache
}

// ExampleCachedUnifiedParser_integration demonstrates integration with parsers
func ExampleCachedUnifiedParser_integration() {
	// Create temporary file
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "example_integration.txt")
	content := "This is example content for demonstrating cached parsing. " +
		"The content is long enough to be properly chunked and cached."

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		fmt.Printf("Error creating test file: %v\n", err)
		return
	}
	defer os.Remove(testFile)

	// Create cached parser with enhanced configuration
	chunkConfig := schema.DefaultChunkingConfig()
	chunkConfig.MinSize = 10

	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.EnableCompression = true
	cacheConfig.EnableMetrics = true

	cachedParser := core.NewCachedUnifiedParser(chunkConfig, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	// First parse - cache miss
	_, err = cachedParser.ParseFile(ctx, testFile)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		return
	}

	// Second parse - cache hit
	_, err = cachedParser.ParseFile(ctx, testFile)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		return
	}

	// Show cache metrics
	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Cache Performance:\n")
	fmt.Printf("  Hits: %d\n", metrics.Hits)
	fmt.Printf("  Misses: %d\n", metrics.Misses)

	// Output:
	// Cache Performance:
	//   Hits: 1
	//   Misses: 1
}

// ExampleCachedUnifiedParser_batchParsing demonstrates batch operations with caching
func ExampleCachedUnifiedParser_batchParsing() {
	// Create temporary files
	tmpDir := os.TempDir()
	files := make([]string, 3)

	for i := 0; i < 3; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("batch_enhanced_%d.txt", i))
		content := fmt.Sprintf("This is batch file %d content. "+
			"It contains enough text to be properly parsed and cached. "+
			"Each file has unique content for testing purposes.", i)

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			fmt.Printf("Error creating file %d: %v\n", i, err)
			return
		}
		files[i] = filePath
		defer os.Remove(filePath)
	}

	// Create cached parser
	chunkConfig := schema.DefaultChunkingConfig()
	chunkConfig.MinSize = 10

	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.EnableMetrics = true

	cachedParser := core.NewCachedUnifiedParser(chunkConfig, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	// First batch parse - all cache misses
	results1, err := cachedParser.BatchParseFiles(ctx, files)

	if err != nil {
		fmt.Printf("Error in first batch parse: %v\n", err)
		return
	}

	// Second batch parse - all cache hits
	_, err = cachedParser.BatchParseFiles(ctx, files)

	if err != nil {
		fmt.Printf("Error in second batch parse: %v\n", err)
		return
	}

	// Show results
	fmt.Printf("Batch Parsing Results:\n")
	fmt.Printf("  Files processed: %d\n", len(results1))

	// Show cache effectiveness
	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Cache Effectiveness:\n")
	fmt.Printf("  Cache hits: %d\n", metrics.Hits)

	// Output:
	// Batch Parsing Results:
	//   Files processed: 3
	// Cache Effectiveness:
	//   Cache hits: 3
}

// ExampleInMemoryParsingCache_persistence demonstrates persistence and warmup features
func ExampleInMemoryParsingCache_persistence() {
	tmpDir := os.TempDir()
	cacheFile := filepath.Join(tmpDir, "example_enhanced_cache.json")
	defer os.Remove(cacheFile)

	// Create cache with persistence enabled
	config := cache.DefaultCacheConfig()
	config.EnablePersistence = true
	config.PersistencePath = cacheFile

	pc := cache.NewInMemoryParsingCache(config)
	ctx := context.Background()

	// Add some content to cache
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Persistent content example", Type: schema.ChunkTypeText},
	}

	err := pc.Set(ctx, "persistent-key", chunks, nil)
	if err != nil {
		fmt.Printf("Error setting cache: %v\n", err)
		return
	}

	// Manually persist to disk
	err = pc.Persist(ctx)
	if err != nil {
		fmt.Printf("Error persisting cache: %v\n", err)
		return
	}

	// Close the cache
	pc.Close()

	// Create new cache instance and load from disk
	newCache := cache.NewInMemoryParsingCache(config)
	defer newCache.Close()

	err = newCache.Load(ctx)
	if err != nil {
		fmt.Printf("Error loading cache: %v\n", err)
		return
	}

	// Verify data was loaded
	retrievedChunks, found := newCache.Get(ctx, "persistent-key")
	if found {
		fmt.Printf("Successfully loaded %d chunks from persistent storage\n", len(retrievedChunks))
		fmt.Printf("Content: %s\n", retrievedChunks[0].Content)
	} else {
		fmt.Printf("Failed to load data from persistent storage\n")
	}

	// Show persistence metrics
	metrics := newCache.GetMetrics()
	fmt.Printf("Persistence Operations: %d\n", metrics.PersistenceOperations)

	// Output:
	// Successfully loaded 1 chunks from persistent storage
	// Content: Persistent content example
	// Persistence Operations: 0
}
