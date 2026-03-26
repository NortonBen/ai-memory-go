// Package parser - Benchmarks for caching system performance
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// BenchmarkCacheOperations tests basic cache operations performance
func BenchmarkCacheOperations(b *testing.B) {
	cache := NewInMemoryParsingCache(DefaultCacheConfig())
	defer cache.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Benchmark test content", Type: schema.ChunkTypeText},
	}

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i)
			cache.Set(ctx, key, chunks, nil)
		}
	})

	// Pre-populate cache for Get benchmark
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("get-key-%d", i)
		cache.Set(ctx, key, chunks, nil)
	}

	b.Run("Get-Hit", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("get-key-%d", i%1000)
			cache.Get(ctx, key)
		}
	})

	b.Run("Get-Miss", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("miss-key-%d", i)
			cache.Get(ctx, key)
		}
	})
}

// BenchmarkCacheEvictionPolicies compares different eviction policies
func BenchmarkCacheEvictionPolicies(b *testing.B) {
	policies := []CachePolicy{PolicyLRU, PolicyLFU, PolicyTTL, PolicyFIFO}

	for _, policy := range policies {
		b.Run(string(policy), func(b *testing.B) {
			config := DefaultCacheConfig()
			config.MaxSize = 100 // Small cache to trigger evictions
			config.Policy = policy

			cache := NewInMemoryParsingCache(config)
			defer cache.Close()

			ctx := context.Background()
			chunks := []*schema.Chunk{
				{ID: "1", Content: "Eviction test content", Type: schema.ChunkTypeText},
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i)
				cache.Set(ctx, key, chunks, nil)
			}
		})
	}
}

// BenchmarkCachedVsUncachedParsing compares cached vs uncached parsing performance
func BenchmarkCachedVsUncachedParsing(b *testing.B) {
	// Create test content
	testContent := strings.Repeat("This is test content for parsing benchmarks. ", 100)

	// Setup parsers
	config := schema.DefaultChunkingConfig()
	unifiedParser := NewUnifiedParser(config)
	defer unifiedParser.Close()

	cacheConfig := DefaultCacheConfig()
	cachedParser := NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	b.Run("Uncached-ParseText", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			unifiedParser.ParseText(ctx, testContent)
		}
	})

	b.Run("Cached-ParseText-FirstTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			content := fmt.Sprintf("%s-%d", testContent, i) // Unique content each time
			cachedParser.ParseText(ctx, content)
		}
	})

	// Pre-populate cache for hit benchmark
	cachedParser.ParseText(ctx, testContent)

	b.Run("Cached-ParseText-Hit", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cachedParser.ParseText(ctx, testContent)
		}
	})
}

// BenchmarkFileParsing compares file parsing with and without caching
func BenchmarkFileParsing(b *testing.B) {
	// Create temporary test files
	tmpDir := b.TempDir()
	testFiles := make([]string, 10)

	for i := 0; i < 10; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("test%d.txt", i))
		content := strings.Repeat(fmt.Sprintf("File %d content. ", i), 200)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}
		testFiles[i] = filePath
	}

	// Setup parsers
	config := schema.DefaultChunkingConfig()
	unifiedParser := NewUnifiedParser(config)
	defer unifiedParser.Close()

	cacheConfig := DefaultCacheConfig()
	cachedParser := NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	b.Run("Uncached-ParseFile", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filePath := testFiles[i%len(testFiles)]
			unifiedParser.ParseFile(ctx, filePath)
		}
	})

	// Pre-populate cache
	for _, filePath := range testFiles {
		cachedParser.ParseFile(ctx, filePath)
	}

	b.Run("Cached-ParseFile-Hit", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filePath := testFiles[i%len(testFiles)]
			cachedParser.ParseFile(ctx, filePath)
		}
	})
}

// BenchmarkBatchParsing compares batch parsing with and without caching
func BenchmarkBatchParsing(b *testing.B) {
	// Create temporary test files
	tmpDir := b.TempDir()
	testFiles := make([]string, 50)

	for i := 0; i < 50; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("batch%d.txt", i))
		content := strings.Repeat(fmt.Sprintf("Batch file %d content. ", i), 100)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}
		testFiles[i] = filePath
	}

	// Setup parsers
	config := schema.DefaultChunkingConfig()
	unifiedParser := NewUnifiedParser(config)
	defer unifiedParser.Close()

	cacheConfig := DefaultCacheConfig()
	cachedParser := NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	b.Run("Uncached-BatchParse", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			unifiedParser.BatchParseFiles(ctx, testFiles)
		}
	})

	// Pre-populate cache
	cachedParser.BatchParseFiles(ctx, testFiles)

	b.Run("Cached-BatchParse-Hit", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cachedParser.BatchParseFiles(ctx, testFiles)
		}
	})
}

// BenchmarkCacheMemoryUsage tests memory efficiency of different cache sizes
func BenchmarkCacheMemoryUsage(b *testing.B) {
	cacheSizes := []int{100, 500, 1000, 5000}

	for _, size := range cacheSizes {
		b.Run(fmt.Sprintf("CacheSize-%d", size), func(b *testing.B) {
			config := DefaultCacheConfig()
			config.MaxSize = size

			cache := NewInMemoryParsingCache(config)
			defer cache.Close()

			ctx := context.Background()
			chunks := []*schema.Chunk{
				{ID: "1", Content: strings.Repeat("Memory test content ", 50), Type: schema.ChunkTypeText},
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i)
				cache.Set(ctx, key, chunks, nil)

				if i%100 == 0 {
					// Periodically check memory usage
					metrics := cache.GetMetrics()
					b.ReportMetric(float64(metrics.MemoryUsageBytes), "bytes/cache")
				}
			}
		})
	}
}

// BenchmarkConcurrentAccess tests cache performance under concurrent load
func BenchmarkConcurrentAccess(b *testing.B) {
	cache := NewInMemoryParsingCache(DefaultCacheConfig())
	defer cache.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Concurrent access test content", Type: schema.ChunkTypeText},
	}

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("concurrent-key-%d", i)
		cache.Set(ctx, key, chunks, nil)
	}

	b.Run("ConcurrentGet", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("concurrent-key-%d", i%1000)
				cache.Get(ctx, key)
				i++
			}
		})
	})

	b.Run("ConcurrentSet", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("new-key-%d", i)
				cache.Set(ctx, key, chunks, nil)
				i++
			}
		})
	})

	b.Run("ConcurrentMixed", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%2 == 0 {
					// Read operation
					key := fmt.Sprintf("concurrent-key-%d", i%1000)
					cache.Get(ctx, key)
				} else {
					// Write operation
					key := fmt.Sprintf("mixed-key-%d", i)
					cache.Set(ctx, key, chunks, nil)
				}
				i++
			}
		})
	})
}

// BenchmarkCacheCleanup tests cleanup operation performance
func BenchmarkCacheCleanup(b *testing.B) {
	config := DefaultCacheConfig()
	config.TTL = 1 * time.Millisecond // Very short TTL for quick expiration
	config.CleanupInterval = 0        // Disable automatic cleanup

	cache := NewInMemoryParsingCache(config)
	defer cache.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Cleanup test content", Type: schema.ChunkTypeText},
	}

	// Populate cache with entries that will expire
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("cleanup-key-%d", i)
		cache.Set(ctx, key, chunks, nil)
	}

	// Wait for entries to expire
	time.Sleep(10 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Cleanup(ctx)
	}
}

// BenchmarkCacheHitRates measures cache effectiveness with different access patterns
func BenchmarkCacheHitRates(b *testing.B) {
	cache := NewInMemoryParsingCache(DefaultCacheConfig())
	defer cache.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Hit rate test content", Type: schema.ChunkTypeText},
	}

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("hitrate-key-%d", i)
		cache.Set(ctx, key, chunks, nil)
	}

	b.Run("HighHitRate-90%", func(b *testing.B) {
		b.ResetTimer()
		hits := 0
		for i := 0; i < b.N; i++ {
			if i%10 < 9 {
				// 90% hits
				key := fmt.Sprintf("hitrate-key-%d", i%1000)
				if _, found := cache.Get(ctx, key); found {
					hits++
				}
			} else {
				// 10% misses
				key := fmt.Sprintf("miss-key-%d", i)
				cache.Get(ctx, key)
			}
		}
		b.ReportMetric(float64(hits)/float64(b.N)*100, "hit-rate-%")
	})

	b.Run("MediumHitRate-50%", func(b *testing.B) {
		b.ResetTimer()
		hits := 0
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				// 50% hits
				key := fmt.Sprintf("hitrate-key-%d", i%1000)
				if _, found := cache.Get(ctx, key); found {
					hits++
				}
			} else {
				// 50% misses
				key := fmt.Sprintf("miss-key-%d", i)
				cache.Get(ctx, key)
			}
		}
		b.ReportMetric(float64(hits)/float64(b.N)*100, "hit-rate-%")
	})

	b.Run("LowHitRate-10%", func(b *testing.B) {
		b.ResetTimer()
		hits := 0
		for i := 0; i < b.N; i++ {
			if i%10 == 0 {
				// 10% hits
				key := fmt.Sprintf("hitrate-key-%d", i%1000)
				if _, found := cache.Get(ctx, key); found {
					hits++
				}
			} else {
				// 90% misses
				key := fmt.Sprintf("miss-key-%d", i)
				cache.Get(ctx, key)
			}
		}
		b.ReportMetric(float64(hits)/float64(b.N)*100, "hit-rate-%")
	})
}

// BenchmarkStreamingWithCache tests streaming parser with caching
func BenchmarkStreamingWithCache(b *testing.B) {
	// Create a large test file
	tmpDir := b.TempDir()
	largeFile := filepath.Join(tmpDir, "large.txt")

	// Create 1MB file
	content := strings.Repeat("This is a large file for streaming benchmark testing. ", 20000)
	err := os.WriteFile(largeFile, []byte(content), 0644)
	if err != nil {
		b.Fatal(err)
	}

	config := schema.DefaultChunkingConfig()
	cacheConfig := DefaultCacheConfig()
	cachedParser := NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	b.Run("StreamingParse-FirstTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create unique file for each iteration to avoid cache hits
			uniqueFile := filepath.Join(tmpDir, fmt.Sprintf("unique%d.txt", i))
			os.WriteFile(uniqueFile, []byte(content), 0644)
			cachedParser.ParseFileStream(ctx, uniqueFile)
		}
	})

	// Pre-populate cache
	cachedParser.ParseFileStream(ctx, largeFile)

	b.Run("StreamingParse-CacheHit", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cachedParser.ParseFileStream(ctx, largeFile)
		}
	})
}

// BenchmarkCacheKeyGeneration tests cache key generation performance
func BenchmarkCacheKeyGeneration(b *testing.B) {
	cache := NewInMemoryParsingCache(DefaultCacheConfig())
	defer cache.Close()

	testContent := strings.Repeat("Key generation test content ", 100)
	testFile := "/path/to/test/file.txt"

	b.Run("ContentKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.generateCacheKey(testContent)
		}
	})

	b.Run("FileKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.generateFileKey(testFile)
		}
	})
}
