// Package parser - Tests for caching system
package cache_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/cache"
	"github.com/NortonBen/ai-memory-go/parser/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryParsingCache_BasicOperations(t *testing.T) {
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	defer pc.Close()

	ctx := context.Background()

	// Test Set and Get
	chunks := []*schema.Chunk{
		{ID: "1", Content: "Test content 1", Type: schema.ChunkTypeText},
		{ID: "2", Content: "Test content 2", Type: schema.ChunkTypeText},
	}

	err := pc.Set(ctx, "test-key", chunks, map[string]interface{}{"test": "metadata"})
	require.NoError(t, err)

	retrievedChunks, found := pc.Get(ctx, "test-key")
	assert.True(t, found)
	assert.Len(t, retrievedChunks, 2)
	assert.Equal(t, "Test content 1", retrievedChunks[0].Content)
	assert.Equal(t, "Test content 2", retrievedChunks[1].Content)

	// Test non-existent key
	_, found = pc.Get(ctx, "non-existent")
	assert.False(t, found)
}

func TestInMemoryParsingCache_FileOperations(t *testing.T) {
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	defer pc.Close()

	ctx := context.Background()

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	chunks := []*schema.Chunk{
		{ID: "1", Content: "Test file content", Type: schema.ChunkTypeText},
	}

	// Test SetByFile and GetByFile
	err = pc.SetByFile(ctx, testFile, chunks, map[string]interface{}{"file": "metadata"})
	require.NoError(t, err)

	retrievedChunks, found := pc.GetByFile(ctx, testFile)
	assert.True(t, found)
	assert.Len(t, retrievedChunks, 1)
	assert.Equal(t, "Test file content", retrievedChunks[0].Content)
}

func TestInMemoryParsingCache_TTLExpiration(t *testing.T) {
	config := cache.DefaultCacheConfig()
	config.TTL = 100 * time.Millisecond
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()

	chunks := []*schema.Chunk{
		{ID: "1", Content: "Test content", Type: schema.ChunkTypeText},
	}

	// Set entry
	err := pc.Set(ctx, "test-key", chunks, nil)
	require.NoError(t, err)

	// Should be available immediately
	_, found := pc.Get(ctx, "test-key")
	assert.True(t, found)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found = pc.Get(ctx, "test-key")
	assert.False(t, found)
}

func TestInMemoryParsingCache_FileModificationTime(t *testing.T) {
	config := cache.DefaultCacheConfig()
	config.CheckFileModTime = true
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("original content"), 0644)
	require.NoError(t, err)

	chunks := []*schema.Chunk{
		{ID: "1", Content: "Original content", Type: schema.ChunkTypeText},
	}

	// Cache the file
	err = pc.SetByFile(ctx, testFile, chunks, nil)
	require.NoError(t, err)

	// Should be available
	_, found := pc.GetByFile(ctx, testFile)
	assert.True(t, found)

	// Modify the file
	time.Sleep(10 * time.Millisecond) // Ensure different mod time
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Should be invalidated due to file modification
	_, found = pc.GetByFile(ctx, testFile)
	assert.False(t, found)
}

func TestInMemoryParsingCache_LRUEviction(t *testing.T) {
	config := cache.DefaultCacheConfig()
	config.MaxSize = 2
	config.Policy = cache.PolicyLRU
	config.CleanupInterval = 0 // Disable automatic cleanup to avoid deadlock
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()

	// Add entries up to max size
	chunks1 := []*schema.Chunk{{ID: "1", Content: "Content 1", Type: schema.ChunkTypeText}}
	chunks2 := []*schema.Chunk{{ID: "2", Content: "Content 2", Type: schema.ChunkTypeText}}
	chunks3 := []*schema.Chunk{{ID: "3", Content: "Content 3", Type: schema.ChunkTypeText}}

	err := pc.Set(ctx, "key1", chunks1, nil)
	require.NoError(t, err)

	err = pc.Set(ctx, "key2", chunks2, nil)
	require.NoError(t, err)

	// Both should be available
	_, found := pc.Get(ctx, "key1")
	assert.True(t, found)
	_, found = pc.Get(ctx, "key2")
	assert.True(t, found)

	// Add third entry, should evict least recently used (key1)
	err = pc.Set(ctx, "key3", chunks3, nil)
	require.NoError(t, err)

	// key1 should be evicted
	_, found = pc.Get(ctx, "key1")
	assert.False(t, found)

	// key2 and key3 should still be available
	_, found = pc.Get(ctx, "key2")
	assert.True(t, found)
	_, found = pc.Get(ctx, "key3")
	assert.True(t, found)
}

func TestInMemoryParsingCache_MemoryLimit(t *testing.T) {
	config := cache.DefaultCacheConfig()
	config.MaxMemoryMB = 1 // 1MB limit
	config.Policy = cache.PolicyLRU
	config.CleanupInterval = 0 // Disable automatic cleanup
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()

	// Create large chunks that will exceed memory limit
	largeContent := make([]byte, 500*1024) // 500KB
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	chunks1 := []*schema.Chunk{{ID: "1", Content: string(largeContent), Type: schema.ChunkTypeText}}
	chunks2 := []*schema.Chunk{{ID: "2", Content: string(largeContent), Type: schema.ChunkTypeText}}
	chunks3 := []*schema.Chunk{{ID: "3", Content: string(largeContent), Type: schema.ChunkTypeText}}

	// Add first chunk
	err := pc.Set(ctx, "key1", chunks1, nil)
	require.NoError(t, err)

	// Add second chunk
	err = pc.Set(ctx, "key2", chunks2, nil)
	require.NoError(t, err)

	// Add third chunk - should trigger eviction
	err = pc.Set(ctx, "key3", chunks3, nil)
	require.NoError(t, err)

	// Check that some entries were evicted to stay under memory limit
	metrics := pc.GetMetrics()
	assert.True(t, metrics.MemoryUsageBytes < 1024*1024) // Under 1MB
}

func TestInMemoryParsingCache_Metrics(t *testing.T) {
	config := cache.DefaultCacheConfig()
	config.EnableMetrics = true
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()

	chunks := []*schema.Chunk{{ID: "1", Content: "Test content", Type: schema.ChunkTypeText}}

	// Set entry
	err := pc.Set(ctx, "test-key", chunks, nil)
	require.NoError(t, err)

	// Get entry (hit)
	_, found := pc.Get(ctx, "test-key")
	assert.True(t, found)

	// Get non-existent entry (miss)
	_, found = pc.Get(ctx, "non-existent")
	assert.False(t, found)

	// Check metrics
	metrics := pc.GetMetrics()
	assert.Equal(t, int64(1), metrics.Hits)
	assert.Equal(t, int64(1), metrics.Misses)
	assert.Equal(t, int64(1), metrics.TotalEntries)
	assert.Equal(t, 0.5, metrics.HitRate) // 1 hit out of 2 total requests
	assert.True(t, metrics.MemoryUsageBytes > 0)
}

func TestInMemoryParsingCache_Cleanup(t *testing.T) {
	config := cache.DefaultCacheConfig()
	config.TTL = 50 * time.Millisecond
	config.CleanupInterval = 0 // Disable automatic cleanup
	pc := cache.NewInMemoryParsingCache(config)
	defer pc.Close()

	ctx := context.Background()

	chunks := []*schema.Chunk{{ID: "1", Content: "Test content", Type: schema.ChunkTypeText}}

	// Set entry
	err := pc.Set(ctx, "test-key", chunks, nil)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Entry should still be in cache (no automatic cleanup)
	metrics := pc.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalEntries)

	// Manual cleanup should remove expired entries
	err = pc.Cleanup(ctx)
	require.NoError(t, err)

	metrics = pc.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalEntries)
}

func TestInMemoryParsingCache_Clear(t *testing.T) {
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	defer pc.Close()

	ctx := context.Background()

	// Add multiple entries
	for i := 0; i < 5; i++ {
		chunks := []*schema.Chunk{{ID: string(rune('1' + i)), Content: "Content", Type: schema.ChunkTypeText}}
		err := pc.Set(ctx, string(rune('a'+i)), chunks, nil)
		require.NoError(t, err)
	}

	// Verify entries exist
	metrics := pc.GetMetrics()
	assert.Equal(t, int64(5), metrics.TotalEntries)

	// Clear cache
	err := pc.Clear(ctx)
	require.NoError(t, err)

	// Verify cache is empty
	metrics = pc.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalEntries)
	assert.Equal(t, int64(0), metrics.MemoryUsageBytes)
}

func TestInMemoryParsingCache_ConcurrentAccess(t *testing.T) {
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	defer pc.Close()

	ctx := context.Background()
	chunks := []*schema.Chunk{{ID: "1", Content: "Test content", Type: schema.ChunkTypeText}}

	// Test concurrent reads and writes
	done := make(chan bool, 10)

	// Start multiple goroutines for concurrent access
	for i := 0; i < 5; i++ {
		go func(id int) {
			key := string(rune('a' + id))
			err := pc.Set(ctx, key, chunks, nil)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		go func(id int) {
			key := string(rune('a' + id))
			_, _ = pc.Get(ctx, key)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify cache state is consistent
	metrics := pc.GetMetrics()
	assert.True(t, metrics.TotalEntries <= 5) // Should have at most 5 entries
}

func TestCachedParser_Integration(t *testing.T) {
	// Create a mock parser
	mockParser := &MockParser{}
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	cachedParser := core.NewCachedUnifiedParserFromCache(mockParser, pc)
	defer cachedParser.Close()

	ctx := context.Background()

	// First call should hit the underlying parser
	chunks1, err := cachedParser.ParseText(ctx, "test content")
	require.NoError(t, err)
	assert.Len(t, chunks1, 1)
	assert.Equal(t, 1, mockParser.ParseTextCallCount)

	// Second call with same content should hit cache
	chunks2, err := cachedParser.ParseText(ctx, "test content")
	require.NoError(t, err)
	assert.Len(t, chunks2, 1)
	assert.Equal(t, 1, mockParser.ParseTextCallCount) // Should not increase

	// Verify cache metrics
	metrics := cachedParser.GetCacheMetrics()
	assert.Equal(t, int64(1), metrics.Hits)
	assert.Equal(t, int64(1), metrics.Misses) // One miss from the first call
}

func TestCachedParser_FileOperations(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test file content"), 0644)
	require.NoError(t, err)

	mockParser := &MockParser{}
	pc := cache.NewInMemoryParsingCache(cache.DefaultCacheConfig())
	cachedParser := core.NewCachedUnifiedParserFromCache(mockParser, pc)
	defer cachedParser.Close()

	ctx := context.Background()

	// First call should hit the underlying parser
	chunks1, err := cachedParser.ParseFile(ctx, testFile)
	require.NoError(t, err)
	assert.Len(t, chunks1, 1)
	assert.Equal(t, 1, mockParser.ParseFileCallCount)

	// Second call should hit cache
	chunks2, err := cachedParser.ParseFile(ctx, testFile)
	require.NoError(t, err)
	assert.Len(t, chunks2, 1)
	assert.Equal(t, 1, mockParser.ParseFileCallCount) // Should not increase

	// Modify file and verify cache invalidation
	time.Sleep(10 * time.Millisecond)
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Should hit parser again due to file modification
	chunks3, err := cachedParser.ParseFile(ctx, testFile)
	require.NoError(t, err)
	assert.Len(t, chunks3, 1)
	assert.Equal(t, 2, mockParser.ParseFileCallCount) // Should increase
}

// MockParser for testing
type MockParser struct {
	ParseFileCallCount     int
	ParseTextCallCount     int
	ParseMarkdownCallCount int
	ParsePDFCallCount      int
}

func (mp *MockParser) ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	mp.ParseFileCallCount++
	return []*schema.Chunk{
		{ID: "mock-1", Content: "Mock file content", Type: schema.ChunkTypeText, Source: filePath},
	}, nil
}

func (mp *MockParser) ParseText(ctx context.Context, content string) ([]*schema.Chunk, error) {
	mp.ParseTextCallCount++
	return []*schema.Chunk{
		{ID: "mock-1", Content: content, Type: schema.ChunkTypeText},
	}, nil
}

func (mp *MockParser) ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error) {
	mp.ParseMarkdownCallCount++
	return []*schema.Chunk{
		{ID: "mock-1", Content: content, Type: schema.ChunkTypeMarkdown},
	}, nil
}

func (mp *MockParser) ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	mp.ParsePDFCallCount++
	return []*schema.Chunk{
		{ID: "mock-1", Content: "Mock PDF content", Type: schema.ChunkTypePDF, Source: filePath},
	}, nil
}

func (mp *MockParser) DetectContentType(content string) schema.ChunkType {
	return schema.ChunkTypeText
}

func (mp *MockParser) BatchParseFiles(ctx context.Context, filePaths []string) (map[string][]*schema.Chunk, error) {
	results := make(map[string][]*schema.Chunk)
	for _, fp := range filePaths {
		chunks, _ := mp.ParseFile(ctx, fp)
		results[fp] = chunks
	}
	return results, nil
}

func (mp *MockParser) ParseFileStream(ctx context.Context, filePath string) (*schema.StreamingResult, error) {
	chunks, _ := mp.ParseFile(ctx, filePath)
	return &schema.StreamingResult{
		Chunks:        chunks,
		ChunksCreated: len(chunks),
	}, nil
}

func (mp *MockParser) GetSupportedFormats() []string {
	return []string{"txt", "md", "pdf"}
}

func (mp *MockParser) IsFormatSupported(filePath string) bool {
	return true
}

func (mp *MockParser) StartWorkerPool() error {
	return nil
}

func (mp *MockParser) StopWorkerPool() error {
	return nil
}

func (mp *MockParser) GetWorkerPoolMetrics() schema.WorkerPoolMetrics {
	return schema.WorkerPoolMetrics{}
}

func (mp *MockParser) IsWorkerPoolHealthy() bool {
	return true
}

func (mp *MockParser) UpdateStreamingConfig(config *schema.StreamingConfig) {}

func (mp *MockParser) GetStreamingConfig() *schema.StreamingConfig {
	return schema.DefaultStreamingConfig()
}

func (mp *MockParser) Close() error {
	return nil
}

func TestCachedUnifiedParser_BatchOperations(t *testing.T) {
	// Create temporary files
	tmpDir := t.TempDir()
	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("test%d.txt", i))
		// Create larger content that meets minimum chunk size requirements
		content := fmt.Sprintf("This is test content for file %d. It contains multiple sentences to ensure proper parsing and chunking. The content is long enough to meet minimum size requirements.", i)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
		files[i] = filePath
	}

	config := schema.DefaultChunkingConfig()
	config.MinSize = 10 // Lower minimum size for testing
	cacheConfig := cache.DefaultCacheConfig()
	cachedParser := core.NewCachedUnifiedParser(config, cacheConfig)
	defer cachedParser.Close()

	ctx := context.Background()

	// First batch parse - should parse all files
	results1, err := cachedParser.BatchParseFiles(ctx, files)
	require.NoError(t, err)
	assert.Len(t, results1, 3)

	// Second batch parse - should use cache for all files
	results2, err := cachedParser.BatchParseFiles(ctx, files)
	require.NoError(t, err)
	assert.Len(t, results2, 3)

	// Verify cache metrics show hits
	metrics := cachedParser.GetCacheMetrics()
	assert.True(t, metrics.Hits > 0)
	assert.True(t, metrics.HitRate > 0)
}

func TestCacheConfig_Validation(t *testing.T) {
	// Test default config
	config := cache.DefaultCacheConfig()
	assert.Equal(t, 1000, config.MaxSize)
	assert.Equal(t, int64(100), config.MaxMemoryMB)
	assert.Equal(t, 24*time.Hour, config.TTL)
	assert.Equal(t, cache.PolicyLRU, config.Policy)
	assert.True(t, config.CheckFileModTime)
	assert.True(t, config.EnableMetrics)

	// Test custom config
	customConfig := &cache.CacheConfig{
		MaxSize:           500,
		MaxMemoryMB:       50,
		TTL:               12 * time.Hour,
		Policy:            cache.PolicyLFU,
		EnablePersistence: true,
		CheckFileModTime:  false,
		EnableMetrics:     false,
	}

	pc := cache.NewInMemoryParsingCache(customConfig)
	defer pc.Close()

	assert.Equal(t, customConfig, pc.Config)
}
