package core_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedParserStreamingIntegration(t *testing.T) {
	parser := core.NewUnifiedParser(nil)
	defer parser.Close()

	// Verify streaming config access
	config := parser.GetStreamingConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 64*1024, config.BufferSize)
}

func TestUnifiedParserStreamingMethods(t *testing.T) {
	parser := core.NewUnifiedParser(nil)
	defer parser.Close()

	// Create test content
	content := `This is a test document with multiple paragraphs.
Each paragraph contains several sentences for testing.

This is the second paragraph. It also has multiple sentences.
The streaming parser should handle this content efficiently.

Final paragraph with concluding content. End of document.`

	// Test ParseReaderStream
	reader := strings.NewReader(content)
	result, err := parser.ParseReaderStream(context.Background(), reader, "test_unified")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Chunks), 0)
}

func TestUnifiedParserFileStreamParsing(t *testing.T) {
	parser := core.NewUnifiedParser(nil)
	defer parser.Close()

	// Create temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "stream_test.txt")

	content := generateStreamTestContent(500) // Medium-sized content
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Test ParseFileStream
	result, err := parser.ParseFileStream(context.Background(), testFile)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Chunks), 0)
	assert.Greater(t, result.TotalBytes, int64(0))

	// Verify chunks have proper metadata
	for _, chunk := range result.Chunks {
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Content)
		assert.Equal(t, testFile, chunk.Source)
		assert.NotEmpty(t, chunk.Hash)
	}
}

func TestUnifiedParserShouldUseStreaming(t *testing.T) {
	parser := core.NewUnifiedParser(nil)
	defer parser.Close()

	tmpDir := t.TempDir()

	// Create small file (should not use streaming)
	smallFile := filepath.Join(tmpDir, "small.txt")
	smallContent := "This is a small file."
	err := os.WriteFile(smallFile, []byte(smallContent), 0644)
	require.NoError(t, err)

	shouldStream, err := parser.ShouldUseStreaming(smallFile)
	require.NoError(t, err)
	assert.False(t, shouldStream)

	// Create large file (should use streaming)
	largeFile := filepath.Join(tmpDir, "large.txt")
	largeContent := strings.Repeat("This is a large file with repeated content. ", 250000) // ~12MB
	err = os.WriteFile(largeFile, []byte(largeContent), 0644)
	require.NoError(t, err)

	shouldStream, err = parser.ShouldUseStreaming(largeFile)
	require.NoError(t, err)
	assert.True(t, shouldStream)
}

func TestUnifiedParserParseFileAuto(t *testing.T) {
	parser := core.NewUnifiedParser(nil)
	defer parser.Close()

	tmpDir := t.TempDir()

	// Test with small file (should use regular parsing)
	smallFile := filepath.Join(tmpDir, "small_auto.txt")
	smallContent := generateStreamTestContent(50) // Small content
	err := os.WriteFile(smallFile, []byte(smallContent), 0644)
	require.NoError(t, err)

	chunks, err := parser.ParseFileAuto(context.Background(), smallFile)
	require.NoError(t, err)
	assert.Greater(t, len(chunks), 0)

	// Test with large file (should use streaming)
	largeFile := filepath.Join(tmpDir, "large_auto.txt")
	largeContent := strings.Repeat(generateStreamTestContent(100), 1000) // Very large
	err = os.WriteFile(largeFile, []byte(largeContent), 0644)
	require.NoError(t, err)

	chunks, err = parser.ParseFileAuto(context.Background(), largeFile)
	require.NoError(t, err)
	assert.Greater(t, len(chunks), 0)
}

func TestUnifiedParserStreamingConfigUpdate(t *testing.T) {
	parser := core.NewUnifiedParser(nil)
	defer parser.Close()

	// Get original config
	originalConfig := parser.GetStreamingConfig()
	assert.Equal(t, 64*1024, originalConfig.BufferSize)

	// Update config
	newConfig := &schema.StreamingConfig{
		BufferSize:   128 * 1024,
		ChunkOverlap: 2048,
		MaxChunkSize: 8 * 1024,
		MinChunkSize: 512,
	}

	parser.UpdateStreamingConfig(newConfig)

	// Verify update
	updatedConfig := parser.GetStreamingConfig()
	assert.Equal(t, 128*1024, updatedConfig.BufferSize)
	assert.Equal(t, 2048, updatedConfig.ChunkOverlap)
}

func TestUnifiedParserStreamingWithCustomChunkingConfig(t *testing.T) {
	chunkConfig := &schema.ChunkingConfig{
		Strategy:          schema.StrategySentence,
		MaxSize:  300,
		MinSize:  30,
		Overlap:  50,
	}

	parser := core.NewUnifiedParser(chunkConfig)
	defer parser.Close()

	content := `First sentence here. Second sentence follows.
Third sentence in same paragraph. Fourth sentence concludes paragraph.

New paragraph starts here. Another sentence in new paragraph.
Final sentence of second paragraph. End of content.`

	reader := strings.NewReader(content)
	result, err := parser.ParseReaderStream(context.Background(), reader, "test_custom_chunking")

	require.NoError(t, err)
	assert.Greater(t, len(result.Chunks), 0)

	// Verify chunks respect the chunking configuration
	for _, chunk := range result.Chunks {
		// Content length should be reasonable for sentence strategy
		assert.GreaterOrEqual(t, len(chunk.Content), chunkConfig.MinSize)
		// Allow some flexibility for streaming overlap
		assert.LessOrEqual(t, len(chunk.Content), chunkConfig.MaxSize*3)
	}
}

func TestUnifiedParserStreamingErrorHandling(t *testing.T) {
	parser := core.NewUnifiedParser(nil)
	defer parser.Close()

	// Test with non-existent file
	_, err := parser.ParseFileStream(context.Background(), "/non/existent/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")

	// Test ShouldUseStreaming with non-existent file
	_, err = parser.ShouldUseStreaming("/non/existent/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat file")

	// Test ParseFileAuto with non-existent file
	_, err = parser.ParseFileAuto(context.Background(), "/non/existent/file.txt")
	assert.Error(t, err)
}

func TestUnifiedParserStreamingPerformanceComparison(t *testing.T) {
	// Use consistent configuration for both
	chunkConfig := schema.DefaultChunkingConfig()
	// Set explicit values to ensure they match
	chunkConfig.MaxSize = 2000
	chunkConfig.MinSize = 100
	
	streamConfig := schema.DefaultStreamingConfig()
	streamConfig.MaxChunkSize = 2000
	streamConfig.MinChunkSize = 100
	
	parser := core.NewUnifiedParserWithConfigs(chunkConfig, nil, streamConfig)
	defer parser.Close()

	// Create large content for comparison
	largeContent := generateStreamTestContent(500) // 500 paragraphs is enough

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "perf_test.txt")
	err := os.WriteFile(testFile, []byte(largeContent), 0644)
	require.NoError(t, err)

	// Test regular parsing
	regularChunks, err := parser.ParseFile(context.Background(), testFile)
	require.NoError(t, err)

	// Test streaming parsing
	streamResult, err := parser.ParseFileStream(context.Background(), testFile)
	require.NoError(t, err)

	// Both should produce same number of chunks if using same config
	// Allow for 10% difference due to how streaming handles boundaries
	assert.InDelta(t, len(regularChunks), len(streamResult.Chunks), float64(len(regularChunks))*0.1)
}

// Helper function to generate test content for streaming tests
func generateStreamTestContent(paragraphs int) string {
	sentences := []string{
		"This is a test sentence for streaming parser validation.",
		"The streaming parser should handle large files efficiently.",
		"Memory usage should remain constant regardless of file size.",
		"Chunking strategies should work correctly with streaming.",
		"Progress tracking provides visibility into long operations.",
		"Error handling ensures robust operation in production.",
		"Configuration options allow customization for different use cases.",
		"Integration with unified parser provides seamless experience.",
	}

	var content strings.Builder
	for i := 0; i < paragraphs; i++ {
		// Add 3-5 sentences per paragraph
		sentenceCount := 3 + (i % 3)
		for j := 0; j < sentenceCount; j++ {
			if j > 0 {
				content.WriteString(" ")
			}
			content.WriteString(sentences[(i*sentenceCount+j)%len(sentences)])
		}

		if i < paragraphs-1 {
			content.WriteString("\n\n") // Paragraph separator
		}
	}

	return content.String()
}
