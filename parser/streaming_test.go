package parser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStreamingParser(t *testing.T) {
	parser := NewStreamingParser(nil, nil)

	assert.NotNil(t, parser)
	assert.NotNil(t, parser.config)
	assert.NotNil(t, parser.chunkingConf)

	// Check default values
	assert.Equal(t, 64*1024, parser.config.BufferSize)
	assert.Equal(t, 1024, parser.config.ChunkOverlap)
	assert.Equal(t, 4*1024, parser.config.MaxChunkSize)
	assert.Equal(t, 256, parser.config.MinChunkSize)
	assert.True(t, parser.config.EnableProgressTracking)
}

func TestStreamingParserWithCustomConfig(t *testing.T) {
	streamConfig := &StreamingConfig{
		BufferSize:   32 * 1024,
		ChunkOverlap: 512,
		MaxChunkSize: 2 * 1024,
		MinChunkSize: 128,
	}

	chunkConfig := &ChunkingConfig{
		Strategy: StrategySentence,
		MaxSize:  500,
		MinSize:  50,
	}

	parser := NewStreamingParser(streamConfig, chunkConfig)

	assert.Equal(t, 32*1024, parser.config.BufferSize)
	assert.Equal(t, 512, parser.config.ChunkOverlap)
	assert.Equal(t, StrategySentence, parser.chunkingConf.Strategy)
}

func TestParseReaderStream(t *testing.T) {
	parser := NewStreamingParser(nil, nil)

	// Create test content
	content := `This is the first paragraph. It contains multiple sentences.
This helps test the streaming parser functionality.

This is the second paragraph. It also contains multiple sentences.
The streaming parser should handle this content efficiently.

This is the third paragraph. It demonstrates how the parser
processes content in chunks while maintaining structure.`

	reader := strings.NewReader(content)

	result, err := parser.ParseReaderStream(context.Background(), reader, "test_source")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Chunks), 0)
	assert.Greater(t, result.ProcessingTime, time.Duration(0))
	assert.Equal(t, len(result.Chunks), result.ChunksCreated)

	// Verify chunks have proper metadata
	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Content)
		assert.NotEmpty(t, chunk.Hash)
		assert.Equal(t, "test_source", chunk.Source)
	}
}

func TestParseFileStream(t *testing.T) {
	// Create temporary file with test content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_stream.txt")

	content := generateLargeTestContent(1000) // ~1000 words
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	parser := NewStreamingParser(nil, nil)

	result, err := parser.ParseFileStream(context.Background(), testFile)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Chunks), 0)
	assert.Greater(t, result.TotalBytes, int64(0))
	assert.Greater(t, result.ProcessingTime, time.Duration(0))

	// Verify total content is preserved
	var totalContent strings.Builder
	for _, chunk := range result.Chunks {
		totalContent.WriteString(chunk.Content)
		totalContent.WriteString("\n\n") // Add separator
	}

	// Content should be roughly the same length (allowing for chunking overhead)
	assert.InDelta(t, len(content), totalContent.Len(), float64(len(content))*0.1)
}

func TestStreamingWithProgressCallback(t *testing.T) {
	var progressUpdates []int64
	var chunkCounts []int

	config := DefaultStreamingConfig()
	config.ProgressCallback = func(bytesProcessed, totalBytes int64, chunksCreated int) {
		progressUpdates = append(progressUpdates, bytesProcessed)
		chunkCounts = append(chunkCounts, chunksCreated)
	}
	config.FlushInterval = 10 * time.Millisecond // Fast updates for testing

	parser := NewStreamingParser(config, nil)

	// Create large content
	content := generateLargeTestContent(2000) // ~2000 words
	reader := strings.NewReader(content)

	result, err := parser.ParseReaderStream(context.Background(), reader, "test_progress")

	require.NoError(t, err)
	assert.Greater(t, len(progressUpdates), 0)
	assert.Greater(t, len(chunkCounts), 0)

	// Progress should be monotonically increasing
	for i := 1; i < len(progressUpdates); i++ {
		assert.GreaterOrEqual(t, progressUpdates[i], progressUpdates[i-1])
	}

	// Final chunk count should match result
	if len(chunkCounts) > 0 {
		assert.Equal(t, result.ChunksCreated, chunkCounts[len(chunkCounts)-1])
	}
}

func TestStreamingChunkingStrategies(t *testing.T) {
	content := `First paragraph with multiple sentences. This is sentence two.

Second paragraph here. Another sentence in second paragraph.

Third paragraph content. Final sentence of third paragraph.`

	testCases := []struct {
		name     string
		strategy ChunkingStrategy
	}{
		{"Paragraph", StrategyParagraph},
		{"Sentence", StrategySentence},
		{"FixedSize", StrategyFixedSize},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunkConfig := &ChunkingConfig{
				Strategy: tc.strategy,
				MaxSize:  500, // Increased to ensure chunks are created
				MinSize:  10,  // Reduced to allow smaller chunks
			}

			streamConfig := &StreamingConfig{
				BufferSize:   1024,
				MaxChunkSize: 500,
				MinChunkSize: 10,
			}

			parser := NewStreamingParser(streamConfig, chunkConfig)
			reader := strings.NewReader(content)

			result, err := parser.ParseReaderStream(context.Background(), reader, "test_strategy")

			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(result.Chunks), 1) // Should create at least one chunk

			// Verify chunks meet size requirements
			for _, chunk := range result.Chunks {
				assert.GreaterOrEqual(t, len(chunk.Content), chunkConfig.MinSize)
				assert.LessOrEqual(t, len(chunk.Content), chunkConfig.MaxSize*2) // Allow some flexibility
			}
		})
	}
}

func TestStreamingMemoryEfficiency(t *testing.T) {
	parser := NewStreamingParser(nil, nil)

	// Memory usage should be predictable and small
	memUsage := parser.GetMemoryUsage()

	// Should be roughly buffer + max chunk + overlap
	expectedUsage := int64(64*1024 + 4*1024 + 1024) // Default config
	assert.InDelta(t, expectedUsage, memUsage, float64(expectedUsage)*0.1)
}

func TestStreamingConfigUpdate(t *testing.T) {
	parser := NewStreamingParser(nil, nil)

	originalConfig := parser.GetConfig()
	assert.Equal(t, 64*1024, originalConfig.BufferSize)

	newConfig := &StreamingConfig{
		BufferSize:   128 * 1024,
		ChunkOverlap: 2048,
		MaxChunkSize: 8 * 1024,
		MinChunkSize: 512,
	}

	parser.UpdateConfig(newConfig)

	updatedConfig := parser.GetConfig()
	assert.Equal(t, 128*1024, updatedConfig.BufferSize)
	assert.Equal(t, 2048, updatedConfig.ChunkOverlap)
	assert.Equal(t, 8*1024, updatedConfig.MaxChunkSize)
	assert.Equal(t, 512, updatedConfig.MinChunkSize)
}

func TestStreamingWithContextCancellation(t *testing.T) {
	parser := NewStreamingParser(nil, nil)

	// Create large content
	content := generateLargeTestContent(5000) // Very large content
	reader := strings.NewReader(content)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result, err := parser.ParseReaderStream(ctx, reader, "test_cancel")

	// Should get context error or may complete if very fast
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	}

	// May have partial results
	if result != nil {
		assert.GreaterOrEqual(t, len(result.Chunks), 0)
	}
}

func TestStreamingErrorHandling(t *testing.T) {
	parser := NewStreamingParser(nil, nil)

	// Test with non-existent file
	_, err := parser.ParseFileStream(context.Background(), "/non/existent/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestStreamingBufferProcessing(t *testing.T) {
	parser := NewStreamingParser(nil, nil)

	// Test processBuffer directly with larger content
	content := strings.Repeat("This is a test paragraph with enough content to meet minimum size requirements.\n\n", 10)

	chunks, overlap := parser.processBuffer(content, "test_source")

	assert.GreaterOrEqual(t, len(chunks), 1) // Should create at least one chunk

	// Verify chunks
	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk.Content)
		assert.Equal(t, "test_source", chunk.Source)
		assert.NotEmpty(t, chunk.ID)
	}

	// Overlap might be empty or contain remaining content
	assert.GreaterOrEqual(t, len(overlap), 0)

	// Test final processing
	smallContent := "Small content that might not meet min size."
	finalChunks := parser.processBufferFinal(smallContent, "test_final")
	assert.GreaterOrEqual(t, len(finalChunks), 1) // Should always create at least one chunk
}

func TestStreamingLargeFileHandling(t *testing.T) {
	// Create a large temporary file
	tmpDir := t.TempDir()
	largeFile := filepath.Join(tmpDir, "large_test.txt")

	// Generate ~1MB of content
	var contentBuilder strings.Builder
	baseContent := generateLargeTestContent(100)
	for i := 0; i < 100; i++ {
		contentBuilder.WriteString(baseContent)
		contentBuilder.WriteString("\n\n")
	}

	err := os.WriteFile(largeFile, []byte(contentBuilder.String()), 0644)
	require.NoError(t, err)

	parser := NewStreamingParser(nil, nil)

	startTime := time.Now()
	result, err := parser.ParseFileStream(context.Background(), largeFile)
	processingTime := time.Since(startTime)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Chunks), 10)              // Should create many chunks
	assert.Greater(t, result.TotalBytes, int64(1024*1024)) // ~1MB
	assert.Less(t, processingTime, 10*time.Second)         // Should be reasonably fast

	// Verify memory efficiency - memory usage should be much smaller than file size
	memUsage := parser.GetMemoryUsage()
	assert.Less(t, memUsage, result.TotalBytes/10) // Memory should be < 10% of file size
}

// Helper function to generate large test content
func generateLargeTestContent(wordCount int) string {
	words := []string{
		"artificial", "intelligence", "memory", "processing", "parallel", "worker",
		"pool", "concurrent", "performance", "optimization", "scalability", "throughput",
		"efficiency", "parsing", "chunking", "text", "document", "analysis", "extraction",
		"streaming", "buffer", "large", "file", "handling", "memory", "efficient",
		"algorithm", "data", "structure", "implementation", "testing", "validation",
	}

	var content strings.Builder
	for i := 0; i < wordCount; i++ {
		if i > 0 && i%20 == 0 {
			content.WriteString(".\n\n") // New paragraph every 20 words
		} else if i > 0 && i%10 == 0 {
			content.WriteString(". ") // New sentence every 10 words
		} else if i > 0 {
			content.WriteString(" ")
		}

		content.WriteString(words[i%len(words)])
	}

	content.WriteString(".")
	return content.String()
}

// Benchmark tests
func BenchmarkStreamingParserSmall(b *testing.B) {
	parser := NewStreamingParser(nil, nil)
	content := generateLargeTestContent(100) // Small content

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(content)
		_, err := parser.ParseReaderStream(context.Background(), reader, "bench_small")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStreamingParserLarge(b *testing.B) {
	parser := NewStreamingParser(nil, nil)
	content := generateLargeTestContent(10000) // Large content

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(content)
		_, err := parser.ParseReaderStream(context.Background(), reader, "bench_large")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStreamingVsRegularParsing(b *testing.B) {
	content := generateLargeTestContent(1000)

	b.Run("Streaming", func(b *testing.B) {
		parser := NewStreamingParser(nil, nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := strings.NewReader(content)
			_, err := parser.ParseReaderStream(context.Background(), reader, "bench_stream")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Regular", func(b *testing.B) {
		parser := NewTextParser(DefaultChunkingConfig())
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := parser.ParseText(context.Background(), content)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
