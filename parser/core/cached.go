// Package core - Cached unified parser implementation
package core

import (
	"context"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/parser/cache"
	"github.com/NortonBen/ai-memory-go/schema"
)

// UnifiedParserInterface defines the methods that CachedUnifiedParser needs from the underlying parser.
// This allows for mocking in tests.
type UnifiedParserInterface interface {
	schema.Parser
	BatchParseFiles(ctx context.Context, filePaths []string) (map[string][]*schema.Chunk, error)
	ParseFileStream(ctx context.Context, filePath string) (*schema.StreamingResult, error)
	GetSupportedFormats() []string
	IsFormatSupported(filePath string) bool
	StartWorkerPool() error
	StopWorkerPool() error
	GetWorkerPoolMetrics() schema.WorkerPoolMetrics
	IsWorkerPoolHealthy() bool
	UpdateStreamingConfig(config *schema.StreamingConfig)
	GetStreamingConfig() *schema.StreamingConfig
	Close() error
}

// CachedUnifiedParser extends UnifiedParser with caching capabilities
type CachedUnifiedParser struct {
	*cache.CachedParser
	underlying UnifiedParserInterface
}

// NewCachedUnifiedParser creates a cached version of UnifiedParser with a new cache
func NewCachedUnifiedParser(config *schema.ChunkingConfig, cacheConfig *schema.CacheConfig) *CachedUnifiedParser {
	unifiedParser := NewUnifiedParser(config)
	pc := cache.NewInMemoryParsingCache(cacheConfig)

	return &CachedUnifiedParser{
		CachedParser: cache.NewCachedParser(unifiedParser, pc),
		underlying:   unifiedParser,
	}
}

// NewCachedUnifiedParserFromCache creates a cached version of UnifiedParser with an existing cache
func NewCachedUnifiedParserFromCache(underlying UnifiedParserInterface, pc *cache.InMemoryParsingCache) *CachedUnifiedParser {
	return &CachedUnifiedParser{
		CachedParser: cache.NewCachedParser(underlying, pc),
		underlying:   underlying,
	}
}

// BatchParseFiles parses multiple files with caching and parallel processing
func (cup *CachedUnifiedParser) BatchParseFiles(ctx context.Context, filePaths []string) (map[string][]*schema.Chunk, error) {
	results := make(map[string][]*schema.Chunk)
	uncachedFiles := make([]string, 0)

	// Check cache for each file first
	for _, filePath := range filePaths {
		if chunks, found := cup.GetCache().GetByFile(ctx, filePath); found {
			results[filePath] = chunks
		} else {
			uncachedFiles = append(uncachedFiles, filePath)
		}
	}

	if len(uncachedFiles) == 0 {
		return results, nil
	}

	// Parse uncached files using the underlying unified parser
	uncachedResults, err := cup.underlying.BatchParseFiles(ctx, uncachedFiles)
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

		if cacheErr := cup.GetCache().SetByFile(ctx, filePath, chunks, metadata); cacheErr != nil {
			fmt.Printf("Warning: Failed to cache batch parsing results for %s: %v\n", filePath, cacheErr)
		}
	}

	return results, nil
}

// GetSupportedFormats delegates to the underlying unified parser
func (cup *CachedUnifiedParser) GetSupportedFormats() []string {
	return cup.underlying.GetSupportedFormats()
}

// IsFormatSupported delegates to the underlying unified parser
func (cup *CachedUnifiedParser) IsFormatSupported(filePath string) bool {
	return cup.underlying.IsFormatSupported(filePath)
}

// StartWorkerPool starts the underlying worker pool
func (cup *CachedUnifiedParser) StartWorkerPool() error {
	return cup.underlying.StartWorkerPool()
}

// StopWorkerPool stops the underlying worker pool
func (cup *CachedUnifiedParser) StopWorkerPool() error {
	return cup.underlying.StopWorkerPool()
}

// GetWorkerPoolMetrics returns worker pool metrics
func (cup *CachedUnifiedParser) GetWorkerPoolMetrics() schema.WorkerPoolMetrics {
	return cup.underlying.GetWorkerPoolMetrics()
}

// IsWorkerPoolHealthy checks worker pool health
func (cup *CachedUnifiedParser) IsWorkerPoolHealthy() bool {
	return cup.underlying.IsWorkerPoolHealthy()
}

// ParseFileStream parses large files using streaming with caching for chunks
func (cup *CachedUnifiedParser) ParseFileStream(ctx context.Context, filePath string) (*schema.StreamingResult, error) {
	// Check if already cached
	if chunks, found := cup.GetCache().GetByFile(ctx, filePath); found {
		return &schema.StreamingResult{
			Chunks:         chunks,
			ChunksCreated:  len(chunks),
			ProcessingTime: 0,
		}, nil
	}

	// Stream parse the file
	result, err := cup.underlying.ParseFileStream(ctx, filePath)
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

	if cacheErr := cup.GetCache().SetByFile(ctx, filePath, result.Chunks, metadata); cacheErr != nil {
		fmt.Printf("Warning: Failed to cache streaming parsing results for %s: %v\n", filePath, cacheErr)
	}

	return result, nil
}

// UpdateStreamingConfig updates the streaming configuration
func (cup *CachedUnifiedParser) UpdateStreamingConfig(config *schema.StreamingConfig) {
	cup.underlying.UpdateStreamingConfig(config)
}

// GetStreamingConfig returns the current streaming configuration
func (cup *CachedUnifiedParser) GetStreamingConfig() *schema.StreamingConfig {
	return cup.underlying.GetStreamingConfig()
}

// Close gracefully shuts down the parser and its components
func (cup *CachedUnifiedParser) Close() error {
	if err := cup.underlying.Close(); err != nil {
		return err
	}
	return cup.CachedParser.Close()
}
