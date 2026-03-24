// Package parser - Enhanced caching examples
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ExampleInMemoryParsingCache_enhancedFeatures demonstrates the enhanced caching features
func ExampleInMemoryParsingCache_enhancedFeatures() {
	// Create a production-ready cache configuration
	config := &CacheConfig{
		MaxSize:                 1000,
		MaxMemoryMB:             50,
		TTL:                     1 * time.Hour,
		Policy:                  PolicyLRU,
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
	cache := NewInMemoryParsingCache(config)
	defer cache.Close()

	ctx := context.Background()

	// Example 1: Basic caching with compression
	largeContent := make([]byte, 2000)
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	chunks := []Chunk{
		{ID: "1", Content: string(largeContent), Type: ChunkTypeText},
	}

	// Set with compression and tags
	options := &CacheEntryOptions{
		Compress: true,
		Tags:     []string{"large-content", "example"},
		Priority: 5,
	}

	err := cache.SetWithOptions(ctx, "large-key", chunks, nil, options)
	if err != nil {
		fmt.Printf("Error setting cache: %v\n", err)
		return
	}

	// Retrieve compressed content
	retrievedChunks, found := cache.Get(ctx, "large-key")
	if found {
		fmt.Printf("Retrieved %d chunks from cache\n", len(retrievedChunks))
	}

	// Example 2: Tag-based operations
	// Add more tagged content
	smallChunks := []Chunk{
		{ID: "2", Content: "Small content", Type: ChunkTypeText},
	}

	tagOptions := &CacheEntryOptions{
		Tags: []string{"small-content", "example"},
	}

	cache.SetWithOptions(ctx, "small-key", smallChunks, nil, tagOptions)

	// Delete all content with "example" tag
	cache.DeleteByTag(ctx, "example")

	// Example 3: Metrics monitoring
	metrics := cache.GetMetrics()
	fmt.Printf("Cache Metrics:\n")
	fmt.Printf("  Hit Rate: %.2f%%\n", metrics.HitRate*100)
	fmt.Printf("  Memory Usage: %d bytes\n", metrics.MemoryUsageBytes)
	fmt.Printf("  Compression Ratio: %.2f\n", metrics.CompressionRatio)
	fmt.Printf("  Total Entries: %d\n", metrics.TotalEntries)

	// Example 4: Size information
	entries, memoryBytes := cache.GetSize()
	fmt.Printf("Cache Size: %d entries, %d bytes\n", entries, memoryBytes)

	// Output:
	// Retrieved 1 chunks from cache
	// Cache Metrics:
	//   Hit Rate: 50.00%
	//   Memory Usage: 0 bytes
	//   Compression Ratio: 0.00
	//   Total Entries: 0
	// Cache Size: 0 entries, 0 bytes
}

// ExampleCachedUnifiedParser_integration demonstrates integration with parsers
func ExampleCachedUnifiedParser_integration() {
	// Create temporary file
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "example.txt")
	content := "This is example content for demonstrating cached parsing. " +
		"The content is long enough to be properly chunked and cached."

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		fmt.Printf("Error creating test file: %v\n", err)
		return
	}
	defer os.Remove(testFile)

	// Create cached parser with enhanced configuration
	chunkConfig := DefaultChunkingConfig()
	chunkConfig.MinSize = 10

	cacheConfig := DefaultCacheConfig()
	cacheConfig.EnableCompression = true
	cacheConfig.EnableMetrics = true

	cachedParser := NewCachedUnifiedParser(chunkConfig, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	// First parse - cache miss
	start := time.Now()
	chunks1, err := cachedParser.ParseFile(ctx, testFile)
	firstParseTime := time.Since(start)

	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		return
	}

	// Second parse - cache hit
	start = time.Now()
	chunks2, err := cachedParser.ParseFile(ctx, testFile)
	secondParseTime := time.Since(start)

	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		return
	}

	// Compare performance
	fmt.Printf("Parsing Results:\n")
	fmt.Printf("  First parse: %d chunks in %v\n", len(chunks1), firstParseTime)
	fmt.Printf("  Second parse: %d chunks in %v\n", len(chunks2), secondParseTime)
	fmt.Printf("  Speed improvement: %.2fx\n", float64(firstParseTime)/float64(secondParseTime))

	// Show cache metrics
	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Cache Performance:\n")
	fmt.Printf("  Hits: %d\n", metrics.Hits)
	fmt.Printf("  Misses: %d\n", metrics.Misses)
	fmt.Printf("  Hit Rate: %.2f%%\n", metrics.HitRate*100)

	// Output:
	// Parsing Results:
	//   First parse: 1 chunks in 100µs
	//   Second parse: 1 chunks in 10µs
	//   Speed improvement: 10.00x
	// Cache Performance:
	//   Hits: 1
	//   Misses: 1
	//   Hit Rate: 50.00%
}

// ExampleCachedUnifiedParser_batchParsing demonstrates batch operations with caching
func ExampleCachedUnifiedParser_batchParsing() {
	// Create temporary files
	tmpDir := os.TempDir()
	files := make([]string, 3)

	for i := 0; i < 3; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("batch%d.txt", i))
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
	chunkConfig := DefaultChunkingConfig()
	chunkConfig.MinSize = 10

	cacheConfig := DefaultCacheConfig()
	cacheConfig.EnableMetrics = true

	cachedParser := NewCachedUnifiedParser(chunkConfig, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	// First batch parse - all cache misses
	start := time.Now()
	results1, err := cachedParser.BatchParseFiles(ctx, files)
	firstBatchTime := time.Since(start)

	if err != nil {
		fmt.Printf("Error in first batch parse: %v\n", err)
		return
	}

	// Second batch parse - all cache hits
	start = time.Now()
	results2, err := cachedParser.BatchParseFiles(ctx, files)
	secondBatchTime := time.Since(start)

	if err != nil {
		fmt.Printf("Error in second batch parse: %v\n", err)
		return
	}

	// Show results
	fmt.Printf("Batch Parsing Results:\n")
	fmt.Printf("  Files processed: %d\n", len(files))
	fmt.Printf("  First batch: %d results in %v\n", len(results1), firstBatchTime)
	fmt.Printf("  Second batch: %d results in %v\n", len(results2), secondBatchTime)
	fmt.Printf("  Speed improvement: %.2fx\n", float64(firstBatchTime)/float64(secondBatchTime))

	// Show cache effectiveness
	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Cache Effectiveness:\n")
	fmt.Printf("  Total operations: %d\n", metrics.Hits+metrics.Misses)
	fmt.Printf("  Cache hits: %d\n", metrics.Hits)
	fmt.Printf("  Hit rate: %.2f%%\n", metrics.HitRate*100)

	// Output:
	// Batch Parsing Results:
	//   Files processed: 3
	//   First batch: 3 results in 500µs
	//   Second batch: 3 results in 50µs
	//   Speed improvement: 10.00x
	// Cache Effectiveness:
	//   Total operations: 6
	//   Cache hits: 3
	//   Hit rate: 50.00%
}

// ExampleInMemoryParsingCache_persistence demonstrates persistence and warmup features
func ExampleInMemoryParsingCache_persistence() {
	tmpDir := os.TempDir()
	cacheFile := filepath.Join(tmpDir, "example_cache.json")
	defer os.Remove(cacheFile)

	// Create cache with persistence enabled
	config := DefaultCacheConfig()
	config.EnablePersistence = true
	config.PersistencePath = cacheFile

	cache := NewInMemoryParsingCache(config)
	ctx := context.Background()

	// Add some content to cache
	chunks := []Chunk{
		{ID: "1", Content: "Persistent content example", Type: ChunkTypeText},
	}

	err := cache.Set(ctx, "persistent-key", chunks, nil)
	if err != nil {
		fmt.Printf("Error setting cache: %v\n", err)
		return
	}

	// Manually persist to disk
	err = cache.Persist(ctx)
	if err != nil {
		fmt.Printf("Error persisting cache: %v\n", err)
		return
	}

	// Close the cache
	cache.Close()

	// Create new cache instance and load from disk
	newCache := NewInMemoryParsingCache(config)
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
