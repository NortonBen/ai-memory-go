// Package cache - Cached parser wrapper that integrates caching with existing parsers
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// CachedParser wraps any Parser implementation with intelligent caching
type CachedParser struct {
	parser schema.Parser
	cache  ParsingCache
	config *schema.CacheConfig
}

// NewCachedParser creates a new cached parser wrapper
func NewCachedParser(p schema.Parser, cache ParsingCache) *CachedParser {
	return &CachedParser{
		parser: p,
		cache:  cache,
		config: schema.DefaultCacheConfig(),
	}
}

// NewCachedParserWithConfig creates a cached parser with custom cache configuration
func NewCachedParserWithConfig(p schema.Parser, config *schema.CacheConfig) *CachedParser {
	cache := NewInMemoryParsingCache(config)
	return &CachedParser{
		parser: p,
		cache:  cache,
		config: config,
	}
}

// ParseFile parses a file with caching support
func (cp *CachedParser) ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	// Try to get from cache first
	if chunks, found := cp.cache.GetByFile(ctx, filePath); found {
		return chunks, nil
	}

	// Cache miss - parse the file
	chunks, err := cp.parser.ParseFile(ctx, filePath)
	if err != nil {
		return nil, err
	}

	// Store in cache
	metadata := map[string]interface{}{
		"parser_type": fmt.Sprintf("%T", cp.parser),
		"parsed_at":   time.Now(),
		"file_path":   filePath,
	}

	if cacheErr := cp.cache.SetByFile(ctx, filePath, chunks, metadata); cacheErr != nil {
		fmt.Printf("Warning: Failed to cache parsing results for %s: %v\n", filePath, cacheErr)
	}

	return chunks, nil
}

// ParseText parses text content with caching support
func (cp *CachedParser) ParseText(ctx context.Context, content string) ([]*schema.Chunk, error) {
	key := cp.cache.GenerateCacheKey(content)

	if chunks, found := cp.cache.Get(ctx, key); found {
		return chunks, nil
	}

	chunks, err := cp.parser.ParseText(ctx, content)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"parser_type":  fmt.Sprintf("%T", cp.parser),
		"parsed_at":    time.Now(),
		"content_type": "text",
		"content_size": len(content),
	}

	if cacheErr := cp.cache.Set(ctx, key, chunks, metadata); cacheErr != nil {
		fmt.Printf("Warning: Failed to cache parsing results for text content: %v\n", cacheErr)
	}

	return chunks, nil
}

// ParseMarkdown parses markdown content with caching support
func (cp *CachedParser) ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error) {
	key := cp.generateContentKey("markdown:" + content)

	if chunks, found := cp.cache.Get(ctx, key); found {
		return chunks, nil
	}

	chunks, err := cp.parser.ParseMarkdown(ctx, content)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"parser_type":  fmt.Sprintf("%T", cp.parser),
		"parsed_at":    time.Now(),
		"content_type": "markdown",
		"content_size": len(content),
	}

	if cacheErr := cp.cache.Set(ctx, key, chunks, metadata); cacheErr != nil {
		fmt.Printf("Warning: Failed to cache parsing results for markdown content: %v\n", cacheErr)
	}

	return chunks, nil
}

// ParsePDF parses PDF files with caching support
func (cp *CachedParser) ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	if chunks, found := cp.cache.GetByFile(ctx, filePath); found {
		return chunks, nil
	}

	chunks, err := cp.parser.ParsePDF(ctx, filePath)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"parser_type":  fmt.Sprintf("%T", cp.parser),
		"parsed_at":    time.Now(),
		"content_type": "pdf",
		"file_path":    filePath,
	}

	if cacheErr := cp.cache.SetByFile(ctx, filePath, chunks, metadata); cacheErr != nil {
		fmt.Printf("Warning: Failed to cache parsing results for PDF %s: %v\n", filePath, cacheErr)
	}

	return chunks, nil
}

// DetectContentType delegates to the underlying parser
func (cp *CachedParser) DetectContentType(content string) schema.ChunkType {
	return cp.parser.DetectContentType(content)
}

// GetCache returns the underlying cache for direct access
func (cp *CachedParser) GetCache() ParsingCache {
	return cp.cache
}

// GetCacheMetrics returns current cache performance metrics
func (cp *CachedParser) GetCacheMetrics() *schema.CacheMetrics {
	return cp.cache.GetMetrics()
}

// InvalidateCache removes all cached entries
func (cp *CachedParser) InvalidateCache(ctx context.Context) error {
	return cp.cache.Clear(ctx)
}

// InvalidateFile removes cached entries for a specific file
func (cp *CachedParser) InvalidateFile(ctx context.Context, filePath string) error {
	key := cp.generateFileKey(filePath)
	return cp.cache.Delete(ctx, key)
}

// WarmupCache pre-populates the cache with frequently accessed files
func (cp *CachedParser) WarmupCache(ctx context.Context, filePaths []string) error {
	for _, filePath := range filePaths {
		if cp.cache.IsValid(ctx, cp.generateFileKey(filePath)) {
			continue
		}

		_, err := cp.ParseFile(ctx, filePath)
		if err != nil {
			fmt.Printf("Warning: Failed to warmup cache for %s: %v\n", filePath, err)
			continue
		}
	}
	return nil
}

// CleanupCache removes expired and invalid entries
func (cp *CachedParser) CleanupCache(ctx context.Context) error {
	return cp.cache.Cleanup(ctx)
}

// Close gracefully shuts down the cached parser
func (cp *CachedParser) Close() error {
	return cp.cache.Close()
}

// Helper methods

// generateContentKey creates a cache key from content
func (cp *CachedParser) generateContentKey(content string) string {
	return cp.cache.GenerateCacheKey(content)
}

// generateFileKey creates a cache key from file path
func (cp *CachedParser) generateFileKey(filePath string) string {
	return cp.cache.GenerateFileKey(filePath)
}
