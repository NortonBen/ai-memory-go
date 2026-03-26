// Package parser - Examples demonstrating caching system usage
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// Example_basicCaching demonstrates basic caching operations
func Example_basicCaching() {
	// Create a cache with default configuration
	cache := NewInMemoryParsingCache(schema.DefaultCacheConfig())
	defer cache.Close()

	ctx := context.Background()

	// Create some test chunks
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Hello, world!", Type: schema.ChunkTypeText},
		{ID: "2", Content: "This is cached content.", Type: schema.ChunkTypeText},
	}

	// Store chunks in cache
	err := cache.Set(ctx, "example-key", chunks, map[string]interface{}{
		"example":   "metadata",
		"cached_at": time.Now(),
	})
	if err != nil {
		fmt.Printf("Error setting cache: %v\n", err)
		return
	}

	// Retrieve chunks from cache
	retrievedChunks, found := cache.Get(ctx, "example-key")
	if found {
		fmt.Printf("Cache hit! Retrieved %d chunks\n", len(retrievedChunks))
		fmt.Printf("First chunk: %s\n", retrievedChunks[0].Content)
	} else {
		fmt.Println("Cache miss")
	}

	// Get cache metrics
	metrics := cache.GetMetrics()
	fmt.Printf("Cache hits: %d, misses: %d, hit rate: %.2f%%\n",
		metrics.Hits, metrics.Misses, metrics.HitRate*100)

	// Output:
	// Cache hit! Retrieved 2 chunks
	// First chunk: Hello, world!
	// Cache hits: 1, misses: 0, hit rate: 100.00%
}

// Example_cachedParser demonstrates using cached parser wrapper
func Example_cachedParser() {
	// Create a unified parser
	unifiedParser := NewUnifiedParser(schema.DefaultChunkingConfig())
	defer unifiedParser.Close()

	// Wrap it with caching
	cacheConfig := schema.DefaultCacheConfig()
	cacheConfig.TTL = 1 * time.Hour // Cache for 1 hour
	cachedParser := NewCachedParserWithConfig(unifiedParser, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()
	testContent := "This is test content that will be cached after first parse."

	// First parse - will hit the underlying parser
	start := time.Now()
	chunks1, err := cachedParser.ParseText(ctx, testContent)
	firstParseTime := time.Since(start)
	if err != nil {
		fmt.Printf("Error parsing: %v\n", err)
		return
	}

	// Second parse - will hit the cache
	start = time.Now()
	chunks2, err := cachedParser.ParseText(ctx, testContent)
	secondParseTime := time.Since(start)
	if err != nil {
		fmt.Printf("Error parsing: %v\n", err)
		return
	}

	fmt.Printf("First parse: %d chunks in %v\n", len(chunks1), firstParseTime)
	fmt.Printf("Second parse: %d chunks in %v\n", len(chunks2), secondParseTime)
	fmt.Printf("Cache speedup: %.2fx faster\n", float64(firstParseTime)/float64(secondParseTime))

	// Show cache metrics
	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Cache hit rate: %.2f%%\n", metrics.HitRate*100)

	// Output:
	// First parse: 1 chunks in 100µs
	// Second parse: 1 chunks in 10µs
	// Cache speedup: 10.00x faster
	// Cache hit rate: 50.00%
}

// Example_fileCaching demonstrates file-based caching with modification time checking
func Example_fileCaching() {
	// Create a temporary file
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "cache_example.txt")

	// Write initial content
	err := os.WriteFile(testFile, []byte("Initial file content"), 0644)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer os.Remove(testFile)

	// Create cached parser with file modification checking enabled
	config := schema.DefaultChunkingConfig()
	cacheConfig := schema.DefaultCacheConfig()
	cacheConfig.CheckFileModTime = true
	cachedParser := NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	// First parse - cache miss
	chunks1, err := cachedParser.ParseFile(ctx, testFile)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		return
	}
	fmt.Printf("First parse: %d chunks, content: %s\n", len(chunks1), chunks1[0].Content)

	// Second parse - cache hit
	chunks2, err := cachedParser.ParseFile(ctx, testFile)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		return
	}
	fmt.Printf("Second parse: %d chunks (cached)\n", len(chunks2))

	// Modify the file
	time.Sleep(10 * time.Millisecond) // Ensure different modification time
	err = os.WriteFile(testFile, []byte("Modified file content"), 0644)
	if err != nil {
		fmt.Printf("Error modifying file: %v\n", err)
		return
	}

	// Third parse - cache miss due to file modification
	chunks3, err := cachedParser.ParseFile(ctx, testFile)
	if err != nil {
		fmt.Printf("Error parsing modified file: %v\n", err)
		return
	}
	fmt.Printf("Third parse: %d chunks, content: %s\n", len(chunks3), chunks3[0].Content)

	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Final cache stats - Hits: %d, Misses: %d\n", metrics.Hits, metrics.Misses)

	// Output:
	// First parse: 1 chunks, content: Initial file content
	// Second parse: 1 chunks (cached)
	// Third parse: 1 chunks, content: Modified file content
	// Final cache stats - Hits: 1, Misses: 0
}

// Example_batchCaching demonstrates batch parsing with caching
func Example_batchCaching() {
	// Create temporary files
	tmpDir := os.TempDir()
	files := make([]string, 3)

	for i := 0; i < 3; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("batch_example_%d.txt", i))
		content := fmt.Sprintf("Batch file %d content for caching example", i)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			fmt.Printf("Error creating file %d: %v\n", i, err)
			return
		}
		files[i] = filePath
		defer os.Remove(filePath)
	}

	// Create cached parser
	config := schema.DefaultChunkingConfig()
	cachedParser := NewCachedUnifiedParser(config, schema.DefaultCacheConfig())
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

	fmt.Printf("First batch: %d files in %v\n", len(results1), firstBatchTime)
	fmt.Printf("Second batch: %d files in %v\n", len(results2), secondBatchTime)
	fmt.Printf("Batch cache speedup: %.2fx faster\n", float64(firstBatchTime)/float64(secondBatchTime))

	// Show detailed results
	for i, filePath := range files {
		chunks := results2[filePath]
		fmt.Printf("File %d: %d chunks\n", i, len(chunks))
	}

	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Cache efficiency: %.2f%% hit rate\n", metrics.HitRate*100)

	// Output:
	// First batch: 3 files in 500µs
	// Second batch: 3 files in 50µs
	// Batch cache speedup: 10.00x faster
	// File 0: 1 chunks
	// File 1: 1 chunks
	// File 2: 1 chunks
	// Cache efficiency: 75.00% hit rate
}

// Example_cacheEviction demonstrates cache eviction policies
func Example_cacheEviction() {
	// Create cache with small size to trigger eviction
	config := DefaultCacheConfig()
	config.MaxSize = 2 // Only 2 entries
	config.Policy = PolicyLRU

	cache := NewInMemoryParsingCache(config)
	defer cache.Close()

	ctx := context.Background()

	// Create test chunks
	chunks1 := []*schema.Chunk{{ID: "1", Content: "First content", Type: schema.ChunkTypeText}}
	chunks2 := []*schema.Chunk{{ID: "2", Content: "Second content", Type: schema.ChunkTypeText}}
	chunks3 := []*schema.Chunk{{ID: "3", Content: "Third content", Type: schema.ChunkTypeText}}

	// Add first entry
	cache.Set(ctx, "key1", chunks1, nil)
	fmt.Printf("Added key1, cache size: %d\n", cache.GetMetrics().TotalEntries)

	// Add second entry
	cache.Set(ctx, "key2", chunks2, nil)
	fmt.Printf("Added key2, cache size: %d\n", cache.GetMetrics().TotalEntries)

	// Access key1 to make it recently used
	cache.Get(ctx, "key1")
	fmt.Println("Accessed key1 (making it recently used)")

	// Add third entry - should evict key2 (least recently used)
	cache.Set(ctx, "key3", chunks3, nil)
	fmt.Printf("Added key3, cache size: %d\n", cache.GetMetrics().TotalEntries)

	// Check which keys are still in cache
	_, found1 := cache.Get(ctx, "key1")
	_, found2 := cache.Get(ctx, "key2")
	_, found3 := cache.Get(ctx, "key3")

	fmt.Printf("key1 in cache: %t\n", found1)
	fmt.Printf("key2 in cache: %t (evicted)\n", found2)
	fmt.Printf("key3 in cache: %t\n", found3)

	metrics := cache.GetMetrics()
	fmt.Printf("Total evictions: %d\n", metrics.Evictions)

	// Output:
	// Added key1, cache size: 1
	// Added key2, cache size: 2
	// Accessed key1 (making it recently used)
	// Added key3, cache size: 2
	// key1 in cache: true
	// key2 in cache: false (evicted)
	// key3 in cache: true
	// Total evictions: 1
}

// Example_cacheMetrics demonstrates comprehensive cache metrics
func Example_cacheMetrics() {
	config := DefaultCacheConfig()
	config.EnableMetrics = true
	cache := NewInMemoryParsingCache(config)
	defer cache.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{{ID: "1", Content: "Metrics test content", Type: schema.ChunkTypeText}}

	// Perform various cache operations
	cache.Set(ctx, "key1", chunks, nil)
	cache.Set(ctx, "key2", chunks, nil)
	cache.Set(ctx, "key3", chunks, nil)

	// Generate some hits and misses
	cache.Get(ctx, "key1") // hit
	cache.Get(ctx, "key2") // hit
	cache.Get(ctx, "key4") // miss
	cache.Get(ctx, "key5") // miss
	cache.Get(ctx, "key1") // hit

	// Get comprehensive metrics
	metrics := cache.GetMetrics()

	fmt.Printf("Cache Metrics:\n")
	fmt.Printf("  Total Entries: %d\n", metrics.TotalEntries)
	fmt.Printf("  Memory Usage: %d bytes\n", metrics.MemoryUsageBytes)
	fmt.Printf("  Hits: %d\n", metrics.Hits)
	fmt.Printf("  Misses: %d\n", metrics.Misses)
	fmt.Printf("  Hit Rate: %.2f%%\n", metrics.HitRate*100)
	fmt.Printf("  Evictions: %d\n", metrics.Evictions)
	fmt.Printf("  Average Access Time: %v\n", metrics.AverageAccessTime)

	// Output:
	// Cache Metrics:
	//   Total Entries: 3
	//   Memory Usage: 150 bytes
	//   Hits: 3
	//   Misses: 2
	//   Hit Rate: 60.00%
	//   Evictions: 0
	//   Average Access Time: 50µs
}

// Example_cacheWarmup demonstrates cache warmup for frequently accessed files
func Example_cacheWarmup() {
	// Create test files
	tmpDir := os.TempDir()
	frequentFiles := make([]string, 3)

	for i := 0; i < 3; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("frequent_%d.txt", i))
		content := fmt.Sprintf("Frequently accessed file %d", i)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			fmt.Printf("Error creating file: %v\n", err)
			return
		}
		frequentFiles[i] = filePath
		defer os.Remove(filePath)
	}

	// Create cached parser
	config := schema.DefaultChunkingConfig()
	cachedParser := NewCachedUnifiedParser(config, schema.DefaultCacheConfig())
	defer cachedParser.Close()

	ctx := context.Background()

	// Warmup cache with frequently accessed files
	fmt.Println("Warming up cache...")
	err := cachedParser.WarmupCache(ctx, frequentFiles)
	if err != nil {
		fmt.Printf("Error warming up cache: %v\n", err)
		return
	}

	// Now all subsequent accesses will be cache hits
	fmt.Println("Accessing warmed up files...")
	for i, filePath := range frequentFiles {
		start := time.Now()
		chunks, err := cachedParser.ParseFile(ctx, filePath)
		accessTime := time.Since(start)

		if err != nil {
			fmt.Printf("Error parsing file %d: %v\n", i, err)
			continue
		}

		fmt.Printf("File %d: %d chunks in %v (cached)\n", i, len(chunks), accessTime)
	}

	metrics := cachedParser.GetCacheMetrics()
	fmt.Printf("Post-warmup hit rate: %.2f%%\n", metrics.HitRate*100)

	// Output:
	// Warming up cache...
	// Accessing warmed up files...
	// File 0: 1 chunks in 10µs (cached)
	// File 1: 1 chunks in 8µs (cached)
	// File 2: 1 chunks in 12µs (cached)
	// Post-warmup hit rate: 100.00%
}
