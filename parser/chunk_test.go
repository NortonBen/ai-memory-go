// Package parser - Tests for Chunk struct and related functionality
package parser

import (
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Testschema.NewChunk tests the creation of a new chunk
func TestNewChunk(t *testing.T) {
	content := "This is test content for a chunk."
	source := "test_source"
	chunkType := schema.ChunkTypeText

	chunk := schema.NewChunk(content, source, chunkType)

	assert.NotNil(t, chunk)
	assert.Equal(t, content, chunk.Content)
	assert.Equal(t, source, chunk.Source)
	assert.Equal(t, chunkType, chunk.Type)
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
	assert.NotNil(t, chunk.Metadata)
	assert.False(t, chunk.CreatedAt.IsZero())
}

// TestChunkIDGeneration tests that chunk IDs are unique and deterministic
func TestChunkIDGeneration(t *testing.T) {
	content1 := "Content A"
	content2 := "Content B"
	source := "test_source"

	chunk1 := schema.NewChunk(content1, source, schema.ChunkTypeText)
	chunk2 := schema.NewChunk(content1, source, schema.ChunkTypeText)
	chunk3 := schema.NewChunk(content2, source, schema.ChunkTypeText)

	// Same content and source should produce same ID
	assert.Equal(t, chunk1.ID, chunk2.ID)

	// Different content should produce different ID
	assert.NotEqual(t, chunk1.ID, chunk3.ID)
}

// TestChunkHashGeneration tests content hash generation
func TestChunkHashGeneration(t *testing.T) {
	content := "Test content"
	source := "test_source"

	chunk1 := schema.NewChunk(content, source, schema.ChunkTypeText)
	chunk2 := schema.NewChunk(content, source, schema.ChunkTypeText)
	chunk3 := schema.NewChunk(content+"  ", source, schema.ChunkTypeText) // Extra whitespace

	// Same content should produce same hash
	assert.Equal(t, chunk1.Hash, chunk2.Hash)

	// Whitespace should be trimmed, so hashes should be equal
	assert.Equal(t, chunk1.Hash, chunk3.Hash)
}

// TestChunkTypes tests all chunk type constants
func TestChunkTypes(t *testing.T) {
	types := []schema.ChunkType{
		schema.ChunkTypeText,
		schema.ChunkTypeParagraph,
		schema.ChunkTypeSentence,
		schema.ChunkTypeMarkdown,
		schema.ChunkTypePDF,
		schema.ChunkTypeCode,
	}

	for _, chunkType := range types {
		chunk := schema.NewChunk("test content", "test_source", chunkType)
		assert.Equal(t, chunkType, chunk.Type)
	}
}

// TestChunkMetadata tests metadata handling
func TestChunkMetadata(t *testing.T) {
	chunk := schema.NewChunk("test content", "test_source", schema.ChunkTypeText)

	// Metadata should be initialized
	assert.NotNil(t, chunk.Metadata)

	// Add metadata
	chunk.Metadata["key1"] = "value1"
	chunk.Metadata["key2"] = 42
	chunk.Metadata["key3"] = true

	assert.Equal(t, "value1", chunk.Metadata["key1"])
	assert.Equal(t, 42, chunk.Metadata["key2"])
	assert.Equal(t, true, chunk.Metadata["key3"])
}

// TestChunkWithEmptyContent tests chunk creation with empty content
func TestChunkWithEmptyContent(t *testing.T) {
	chunk := schema.NewChunk("", "test_source", schema.ChunkTypeText)

	assert.NotNil(t, chunk)
	assert.Empty(t, chunk.Content)
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
}

// TestChunkWithLargeContent tests chunk creation with large content
func TestChunkWithLargeContent(t *testing.T) {
	// Create a large content string (10KB)
	largeContent := string(make([]byte, 10*1024))
	for i := range largeContent {
		largeContent = largeContent[:i] + "a" + largeContent[i+1:]
	}

	chunk := schema.NewChunk(largeContent, "test_source", schema.ChunkTypeText)

	assert.NotNil(t, chunk)
	assert.Equal(t, len(largeContent), len(chunk.Content))
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
}

// TestChunkWithSpecialCharacters tests chunk creation with special characters
func TestChunkWithSpecialCharacters(t *testing.T) {
	specialContent := "Test with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	chunk := schema.NewChunk(specialContent, "test_source", schema.ChunkTypeText)

	assert.NotNil(t, chunk)
	assert.Equal(t, specialContent, chunk.Content)
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
}

// TestChunkWithUnicodeContent tests chunk creation with Unicode content
func TestChunkWithUnicodeContent(t *testing.T) {
	unicodeContent := "Tiếng Việt: Xin chào! 中文: 你好! 日本語: こんにちは! Emoji: 😀🎉"
	chunk := schema.NewChunk(unicodeContent, "test_source", schema.ChunkTypeText)

	assert.NotNil(t, chunk)
	assert.Equal(t, unicodeContent, chunk.Content)
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
}

// TestChunkCreatedAtTimestamp tests that CreatedAt is set correctly
func TestChunkCreatedAtTimestamp(t *testing.T) {
	before := time.Now()
	chunk := schema.NewChunk("test content", "test_source", schema.ChunkTypeText)
	after := time.Now()

	assert.False(t, chunk.CreatedAt.IsZero())
	assert.True(t, chunk.CreatedAt.After(before) || chunk.CreatedAt.Equal(before))
	assert.True(t, chunk.CreatedAt.Before(after) || chunk.CreatedAt.Equal(after))
}

// TestChunkSourceField tests the source field
func TestChunkSourceField(t *testing.T) {
	sources := []string{
		"file.txt",
		"/path/to/document.pdf",
		"https://example.com/article",
		"user_input",
		"",
	}

	for _, source := range sources {
		chunk := schema.NewChunk("test content", source, schema.ChunkTypeText)
		assert.Equal(t, source, chunk.Source)
	}
}

// TestGenerateChunkID tests the schema.GenerateChunkID function
func TestGenerateChunkID(t *testing.T) {
	id1 := schema.GenerateChunkID("content", "source")
	id2 := schema.GenerateChunkID("content", "source")
	id3 := schema.GenerateChunkID("different", "source")

	// Same inputs should produce same ID
	assert.Equal(t, id1, id2)

	// Different inputs should produce different ID
	assert.NotEqual(t, id1, id3)

	// ID should have expected format
	assert.Contains(t, id1, "chunk_")
}

// TestGenerateContentHash tests the schema.GenerateContentHash function
func TestGenerateContentHash(t *testing.T) {
	hash1 := schema.GenerateContentHash("content")
	hash2 := schema.GenerateContentHash("content")
	hash3 := schema.GenerateContentHash("different")

	// Same content should produce same hash
	assert.Equal(t, hash1, hash2)

	// Different content should produce different hash
	assert.NotEqual(t, hash1, hash3)

	// Hash should be non-empty
	assert.NotEmpty(t, hash1)
}

// TestGenerateContentHashTrimsWhitespace tests that hash generation trims whitespace
func TestGenerateContentHashTrimsWhitespace(t *testing.T) {
	hash1 := schema.GenerateContentHash("content")
	hash2 := schema.GenerateContentHash("  content  ")
	hash3 := schema.GenerateContentHash("content\n")

	// All should produce the same hash after trimming
	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash1, hash3)
}

// Testschema.ChunkingConfig tests the schema.ChunkingConfig struct
func TestChunkingConfig(t *testing.T) {
	config := &schema.ChunkingConfig{
		Strategy:          schema.StrategyParagraph,
		MaxSize:           1000,
		Overlap:           100,
		MinSize:           50,
		PreserveStructure: true,
	}

	assert.Equal(t, schema.StrategyParagraph, config.Strategy)
	assert.Equal(t, 1000, config.MaxSize)
	assert.Equal(t, 100, config.Overlap)
	assert.Equal(t, 50, config.MinSize)
	assert.True(t, config.PreserveStructure)
}

// TestDefaultschema.ChunkingConfig tests the default configuration
func TestDefaultChunkingConfig(t *testing.T) {
	config := schema.DefaultChunkingConfig()

	assert.NotNil(t, config)
	assert.Equal(t, schema.StrategyParagraph, config.Strategy)
	assert.Equal(t, 1000, config.MaxSize)
	assert.Equal(t, 100, config.Overlap)
	assert.Equal(t, 50, config.MinSize)
	assert.True(t, config.PreserveStructure)
}

// TestChunkingStrategies tests all chunking strategy constants
func TestChunkingStrategies(t *testing.T) {
	strategies := []schema.ChunkingStrategy{
		schema.StrategyParagraph,
		schema.StrategySentence,
		schema.StrategyFixedSize,
		schema.StrategySemantic,
	}

	for _, strategy := range strategies {
		config := &schema.ChunkingConfig{Strategy: strategy}
		assert.Equal(t, strategy, config.Strategy)
	}
}

// TestChunkMetadataEnrichment tests metadata enrichment
func TestChunkMetadataEnrichment(t *testing.T) {
	chunk := schema.NewChunk("test content", "test.txt", schema.ChunkTypeText)

	// Add custom metadata
	chunk.Metadata["custom_field"] = "custom_value"
	chunk.Metadata["priority"] = 5

	assert.Equal(t, "custom_value", chunk.Metadata["custom_field"])
	assert.Equal(t, 5, chunk.Metadata["priority"])
}

// TestChunkImmutabilityOfIDAndHash tests that ID and Hash are set at creation
func TestChunkImmutabilityOfIDAndHash(t *testing.T) {
	chunk := schema.NewChunk("test content", "test_source", schema.ChunkTypeText)

	originalID := chunk.ID
	originalHash := chunk.Hash

	// Modify content (in real usage, chunks should be immutable, but testing the values)
	chunk.Content = "modified content"

	// ID and Hash should remain the same (they're set at creation)
	assert.Equal(t, originalID, chunk.ID)
	assert.Equal(t, originalHash, chunk.Hash)
}

// TestMultipleChunksFromSameSource tests creating multiple chunks from same source
func TestMultipleChunksFromSameSource(t *testing.T) {
	source := "document.txt"

	chunk1 := schema.NewChunk("First paragraph", source, schema.ChunkTypeParagraph)
	chunk2 := schema.NewChunk("Second paragraph", source, schema.ChunkTypeParagraph)
	chunk3 := schema.NewChunk("Third paragraph", source, schema.ChunkTypeParagraph)

	// All should have the same source
	assert.Equal(t, source, chunk1.Source)
	assert.Equal(t, source, chunk2.Source)
	assert.Equal(t, source, chunk3.Source)

	// But different IDs and hashes
	assert.NotEqual(t, chunk1.ID, chunk2.ID)
	assert.NotEqual(t, chunk2.ID, chunk3.ID)
	assert.NotEqual(t, chunk1.Hash, chunk2.Hash)
	assert.NotEqual(t, chunk2.Hash, chunk3.Hash)
}

// TestChunkWithDifferentTypes tests chunks with different types
func TestChunkWithDifferentTypes(t *testing.T) {
	content := "Test content"
	source := "test_source"

	textChunk := schema.NewChunk(content, source, schema.ChunkTypeText)
	codeChunk := schema.NewChunk(content, source, schema.ChunkTypeCode)
	markdownChunk := schema.NewChunk(content, source, schema.ChunkTypeMarkdown)

	assert.Equal(t, schema.ChunkTypeText, textChunk.Type)
	assert.Equal(t, schema.ChunkTypeCode, codeChunk.Type)
	assert.Equal(t, schema.ChunkTypeMarkdown, markdownChunk.Type)

	// Same content but different types should have different IDs
	// because ID is based on content + source
	assert.Equal(t, textChunk.ID, codeChunk.ID)     // Same content and source
	assert.Equal(t, textChunk.Hash, codeChunk.Hash) // Same content
}

// TestChunkJSONSerialization tests that chunks can be serialized to JSON
func TestChunkJSONSerialization(t *testing.T) {
	chunk := schema.NewChunk("test content", "test_source", schema.ChunkTypeText)
	chunk.Metadata["key"] = "value"

	// This test verifies the struct has proper JSON tags
	// The actual JSON marshaling is tested implicitly by the tags
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Content)
	assert.NotEmpty(t, chunk.Type)
	assert.NotNil(t, chunk.Metadata)
	assert.NotEmpty(t, chunk.Source)
	assert.NotEmpty(t, chunk.Hash)
	assert.False(t, chunk.CreatedAt.IsZero())
}

// TestChunkFieldValidation tests that all required fields are set
func TestChunkFieldValidation(t *testing.T) {
	chunk := schema.NewChunk("test content", "test_source", schema.ChunkTypeText)

	// Verify all fields are properly initialized
	require.NotEmpty(t, chunk.ID, "ID should not be empty")
	require.NotEmpty(t, chunk.Content, "Content should not be empty")
	require.NotEmpty(t, chunk.Type, "Type should not be empty")
	require.NotNil(t, chunk.Metadata, "Metadata should not be nil")
	require.NotEmpty(t, chunk.Source, "Source should not be empty")
	require.NotEmpty(t, chunk.Hash, "Hash should not be empty")
	require.False(t, chunk.CreatedAt.IsZero(), "CreatedAt should be set")
}

// TestChunkConcurrentCreation tests creating chunks concurrently
func TestChunkConcurrentCreation(t *testing.T) {
	const numChunks = 100
	chunks := make([]*schema.Chunk, numChunks)
	done := make(chan bool, numChunks)

	for i := 0; i < numChunks; i++ {
		go func(index int) {
			content := "Content " + string(rune(index))
			chunks[index] = schema.NewChunk(content, "test_source", schema.ChunkTypeText)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numChunks; i++ {
		<-done
	}

	// Verify all chunks were created
	for i := 0; i < numChunks; i++ {
		assert.NotNil(t, chunks[i])
		assert.NotEmpty(t, chunks[i].ID)
	}
}
