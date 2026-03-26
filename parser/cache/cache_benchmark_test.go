// Package parser - Benchmarks for caching system performance
package cache_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/cache"
	"github.com/NortonBen/ai-memory-go/parser/core"
)

// BenchmarkCacheOperations tests basic cache operations performance
func BenchmarkCacheOperations(b *testing.B) {
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	defer pc.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Benchmark test content", Type: schema.ChunkTypeText},
	}

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i)
			pc.Set(ctx, key, chunks, nil)
		}
	})

	// Pre-populate cache for Get benchmark
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("get-key-%d", i)
		pc.Set(ctx, key, chunks, nil)
	}

	b.Run("Get-Hit", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("get-key-%d", i%1000)
			pc.Get(ctx, key)
		}
	})

	b.Run("Get-Miss", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("miss-key-%d", i)
			pc.Get(ctx, key)
		}
	})
}

// BenchmarkCacheEvictionPolicies compares different eviction policies
func BenchmarkCacheEvictionPolicies(b *testing.B) {
	policies := []cache.CachePolicy{cache.PolicyLRU, cache.PolicyLFU, cache.PolicyTTL, cache.PolicyFIFO}

	for _, policy := range policies {
		b.Run(string(policy), func(b *testing.B) {
			config := cache.DefaultCacheConfig()
			config.MaxSize = 100 // Small cache to trigger evictions
			config.Policy = policy

			pc := cache.NewInMemoryParsingCache(config)
			defer pc.Close()

			ctx := context.Background()
			chunks := []*schema.Chunk{
				{ID: "1", Content: "Eviction test content", Type: schema.ChunkTypeText},
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i)
				pc.Set(ctx, key, chunks, nil)
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
	unifiedParser := core.NewUnifiedParser(config)
	defer unifiedParser.Close()

	cacheConfig := cache.DefaultCacheConfig()
	cachedParser := core.NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	b.Run("Uncached-ParseText", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			unifiedParser.ParseText(ctx, testContent)
		}
	})

	// Pre-populate cache
	cachedParser.ParseText(ctx, testContent)

	b.Run("Cached-ParseText-Hit", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cachedParser.ParseText(ctx, testContent)
		}
	})
}

// BenchmarkCacheScale tests cache performance at different scales
func BenchmarkCacheScale(b *testing.B) {
	scales := []int{100, 1000, 10000, 100000}

	for _, scale := range scales {
		b.Run(fmt.Sprintf("Scale-%d", scale), func(b *testing.B) {
			config := cache.DefaultCacheConfig()
			config.MaxSize = scale * 2
			pc := cache.NewInMemoryParsingCache(config)
			defer pc.Close()

			ctx := context.Background()
			chunks := []*schema.Chunk{
				{ID: "1", Content: "Scale test content", Type: schema.ChunkTypeText},
			}

			// Pre-populate
			for i := 0; i < scale; i++ {
				pc.Set(ctx, fmt.Sprintf("key-%d", i), chunks, nil)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i%scale)
				pc.Get(ctx, key)
			}
		})
	}
}

// BenchmarkCachePersistence tests persistence performance
func BenchmarkCachePersistence(b *testing.B) {
	tmpDir := b.TempDir()
	persistencePath := filepath.Join(tmpDir, "persist.json")

	config := cache.DefaultCacheConfig()
	config.EnablePersistence = true
	config.PersistencePath = persistencePath
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Persistence test content", Type: schema.ChunkTypeText},
	}

	// Populate cache
	for i := 0; i < 1000; i++ {
		pc.Set(ctx, fmt.Sprintf("key-%d", i), chunks, nil)
	}

	b.Run("Persist-1000", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pc.Persist(ctx)
		}
	})

	b.Run("Load-1000", func(b *testing.B) {
		pc.Persist(ctx)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pc.Load(ctx)
		}
	})
}

// BenchmarkCacheConcurrency tests cache performance under high concurrency
func BenchmarkCacheConcurrency(b *testing.B) {
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	defer pc.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Concurrency test content", Type: schema.ChunkTypeText},
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("parallel-key-%d", i)
			pc.Set(ctx, key, chunks, nil)
			pc.Get(ctx, key)
			i++
		}
	})
}

// BenchmarkCacheKeyGeneration tests cache key generation performance
func BenchmarkCacheKeyGeneration(b *testing.B) {
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	defer pc.Close()
	content := "This is some content to generate a key for."
	filePath := "/path/to/some/test/file/for/key/generation.txt"

	b.Run("GenerateCacheKey", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pc.GenerateCacheKey(content)
		}
	})

	b.Run("GenerateFileKey", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pc.GenerateFileKey(filePath)
		}
	})
}
