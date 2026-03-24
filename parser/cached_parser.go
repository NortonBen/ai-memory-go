// Package parser - Cached parser wrapper that integrates caching with existing parsers
package parser

import (
	"context"
	"fmt"
	"time"
)

// CachedParser wraps any Parser implementation with intelligent caching
type CachedParser struct {
	parser Parser
	cache  ParsingCache
	config *CacheConfig
}

// NewCachedParser creates a new cached parser wrapper
func NewCachedParser(parser Parser, cache ParsingCache) *CachedParser {
	return &CachedParser{
		parser: parser,
		cache:  cache,
		config: DefaultCacheConfig(),
	}
}

// NewCachedParserWithConfig creates a cached parser with custom cache configuration
func NewCachedParserWithConfig(parser Parser, config *CacheConfig) *CachedParser {
	cache := NewInMemoryParsingCache(config)
	return &CachedParser{
		parser: parser,
		cache:  cache,
		config: config,
	}
}

// ParseFile parses a file with caching support
func (cp *CachedParser) ParseFile(ctx context.Context, filePath string) ([]Chunk, error) {
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
		// Log cache error but don't fail the parsing operation
		// In a real implementation, you might want to use a proper logger
		fmt.Printf("Warning: Failed to cache parsing results for %s: %v\n", filePath, cacheErr)
	}

	return chunks, nil
}

// ParseText parses text content with caching support
func (cp *CachedParser) ParseText(ctx context.Context, content string) ([]Chunk, error) {
	// Generate cache key from content
	key := cp.generateContentKey(content)

	// Try to get from cache first
	if chunks, found := cp.cache.Get(ctx, key); found {
		return chunks, nil
	}

	// Cache miss - parse the content
	chunks, err := cp.parser.ParseText(ctx, content)
	if err != nil {
		return nil, err
	}

	// Store in cache
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
func (cp *CachedParser) ParseMarkdown(ctx context.Context, content string) ([]Chunk, error) {
	// Generate cache key from content with markdown prefix
	key := cp.generateContentKey("markdown:" + content)

	// Try to get from cache first
	if chunks, found := cp.cache.Get(ctx, key); found {
		return chunks, nil
	}

	// Cache miss - parse the content
	chunks, err := cp.parser.ParseMarkdown(ctx, content)
	if err != nil {
		return nil, err
	}

	// Store in cache
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
func (cp *CachedParser) ParsePDF(ctx context.Context, filePath string) ([]Chunk, error) {
	// Try to get from cache first
	if chunks, found := cp.cache.GetByFile(ctx, filePath); found {
		return chunks, nil
	}

	// Cache miss - parse the PDF
	chunks, err := cp.parser.ParsePDF(ctx, filePath)
	if err != nil {
		return nil, err
	}

	// Store in cache
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
func (cp *CachedParser) DetectContentType(content string) ChunkType {
	return cp.parser.DetectContentType(content)
}

// GetCache returns the underlying cache for direct access
func (cp *CachedParser) GetCache() ParsingCache {
	return cp.cache
}

// GetCacheMetrics returns current cache performance metrics
func (cp *CachedParser) GetCacheMetrics() *CacheMetrics {
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
		// Check if already cached and valid
		if cp.cache.IsValid(ctx, cp.generateFileKey(filePath)) {
			continue
		}

		// Parse and cache the file
		_, err := cp.ParseFile(ctx, filePath)
		if err != nil {
			// Continue with other files even if one fails
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
	cache := cp.cache.(*InMemoryParsingCache)
	return cache.generateCacheKey(content)
}

// generateFileKey creates a cache key from file path
func (cp *CachedParser) generateFileKey(filePath string) string {
	cache := cp.cache.(*InMemoryParsingCache)
	return cache.generateFileKey(filePath)
}

// CachedUnifiedParser extends UnifiedParser with caching capabilities
type CachedUnifiedParser struct {
	*CachedParser
	unifiedParser *UnifiedParser
}

// NewCachedUnifiedParser creates a cached version of UnifiedParser
func NewCachedUnifiedParser(config *ChunkingConfig, cacheConfig *CacheConfig) *CachedUnifiedParser {
	unifiedParser := NewUnifiedParser(config)
	cachedParser := NewCachedParserWithConfig(unifiedParser, cacheConfig)

	return &CachedUnifiedParser{
		CachedParser:  cachedParser,
		unifiedParser: unifiedParser,
	}
}

// BatchParseFiles parses multiple files with caching and parallel processing
func (cup *CachedUnifiedParser) BatchParseFiles(ctx context.Context, filePaths []string) (map[string][]Chunk, error) {
	results := make(map[string][]Chunk)
	uncachedFiles := make([]string, 0)

	// Check cache for each file first
	for _, filePath := range filePaths {
		if chunks, found := cup.cache.GetByFile(ctx, filePath); found {
			results[filePath] = chunks
		} else {
			uncachedFiles = append(uncachedFiles, filePath)
		}
	}

	// If all files were cached, return immediately
	if len(uncachedFiles) == 0 {
		return results, nil
	}

	// Parse uncached files using the underlying unified parser
	uncachedResults, err := cup.unifiedParser.BatchParseFiles(ctx, uncachedFiles)
	if err != nil {
		return results, err
	}

	// Cache the results and add to final results
	for filePath, chunks := range uncachedResults {
		results[filePath] = chunks

		// Cache the results
		metadata := map[string]interface{}{
			"parser_type": "CachedUnifiedParser",
			"parsed_at":   time.Now(),
			"file_path":   filePath,
			"batch_parse": true,
		}

		if cacheErr := cup.cache.SetByFile(ctx, filePath, chunks, metadata); cacheErr != nil {
			fmt.Printf("Warning: Failed to cache batch parsing results for %s: %v\n", filePath, cacheErr)
		}
	}

	return results, nil
}

// GetSupportedFormats delegates to the underlying unified parser
func (cup *CachedUnifiedParser) GetSupportedFormats() []string {
	return cup.unifiedParser.GetSupportedFormats()
}

// IsFormatSupported delegates to the underlying unified parser
func (cup *CachedUnifiedParser) IsFormatSupported(filePath string) bool {
	return cup.unifiedParser.IsFormatSupported(filePath)
}

// StartWorkerPool starts the underlying worker pool
func (cup *CachedUnifiedParser) StartWorkerPool() error {
	return cup.unifiedParser.StartWorkerPool()
}

// StopWorkerPool stops the underlying worker pool
func (cup *CachedUnifiedParser) StopWorkerPool() error {
	return cup.unifiedParser.StopWorkerPool()
}

// GetWorkerPoolMetrics returns worker pool metrics
func (cup *CachedUnifiedParser) GetWorkerPoolMetrics() WorkerPoolMetrics {
	return cup.unifiedParser.GetWorkerPoolMetrics()
}

// IsWorkerPoolHealthy checks worker pool health
func (cup *CachedUnifiedParser) IsWorkerPoolHealthy() bool {
	return cup.unifiedParser.IsWorkerPoolHealthy()
}

// ParseFileStream parses large files using streaming with caching for chunks
func (cup *CachedUnifiedParser) ParseFileStream(ctx context.Context, filePath string) (*StreamingResult, error) {
	// For streaming, we cache the final result but not intermediate chunks
	// This is because streaming is used for large files where caching might not be beneficial

	// Check if we have a cached result for this file
	if chunks, found := cup.cache.GetByFile(ctx, filePath); found {
		return &StreamingResult{
			Chunks:         chunks,
			ChunksCreated:  len(chunks),
			ProcessingTime: 0, // Cached result
		}, nil
	}

	// Stream parse the file
	result, err := cup.unifiedParser.ParseFileStream(ctx, filePath)
	if err != nil {
		return nil, err
	}

	// Cache the final result
	metadata := map[string]interface{}{
		"parser_type":     "CachedUnifiedParser",
		"parsed_at":       time.Now(),
		"file_path":       filePath,
		"streaming_parse": true,
		"processing_time": result.ProcessingTime,
		"total_bytes":     result.TotalBytes,
	}

	if cacheErr := cup.cache.SetByFile(ctx, filePath, result.Chunks, metadata); cacheErr != nil {
		fmt.Printf("Warning: Failed to cache streaming parsing results for %s: %v\n", filePath, cacheErr)
	}

	return result, nil
}

// UpdateStreamingConfig updates the streaming configuration
func (cup *CachedUnifiedParser) UpdateStreamingConfig(config *StreamingConfig) {
	cup.unifiedParser.UpdateStreamingConfig(config)
}

// GetStreamingConfig returns the current streaming configuration
func (cup *CachedUnifiedParser) GetStreamingConfig() *StreamingConfig {
	return cup.unifiedParser.GetStreamingConfig()
}
