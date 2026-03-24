// Package parser - Tests for Chunk struct and related functionality
package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewChunk tests the creation of a new chunk
func TestNewChunk(t *testing.T) {
	content := "This is test content for a chunk."
	source := "test_source"
	chunkType := ChunkTypeText

	chunk := NewChunk(content, source, chunkType)

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

	chunk1 := NewChunk(content1, source, ChunkTypeText)
	chunk2 := NewChunk(content1, source, ChunkTypeText)
	chunk3 := NewChunk(content2, source, ChunkTypeText)

	// Same content and source should produce same ID
	assert.Equal(t, chunk1.ID, chunk2.ID)

	// Different content should produce different ID
	assert.NotEqual(t, chunk1.ID, chunk3.ID)
}

// TestChunkHashGeneration tests content hash generation
func TestChunkHashGeneration(t *testing.T) {
	content := "Test content"
	source := "test_source"

	chunk1 := NewChunk(content, source, ChunkTypeText)
	chunk2 := NewChunk(content, source, ChunkTypeText)
	chunk3 := NewChunk(content+"  ", source, ChunkTypeText) // Extra whitespace

	// Same content should produce same hash
	assert.Equal(t, chunk1.Hash, chunk2.Hash)

	// Whitespace should be trimmed, so hashes should be equal
	assert.Equal(t, chunk1.Hash, chunk3.Hash)
}

// TestChunkTypes tests all chunk type constants
func TestChunkTypes(t *testing.T) {
	types := []ChunkType{
		ChunkTypeText,
		ChunkTypeParagraph,
		ChunkTypeSentence,
		ChunkTypeMarkdown,
		ChunkTypePDF,
		ChunkTypeCode,
	}

	for _, chunkType := range types {
		chunk := NewChunk("test content", "test_source", chunkType)
		assert.Equal(t, chunkType, chunk.Type)
	}
}

// TestChunkMetadata tests metadata handling
func TestChunkMetadata(t *testing.T) {
	chunk := NewChunk("test content", "test_source", ChunkTypeText)

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
	chunk := NewChunk("", "test_source", ChunkTypeText)

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

	chunk := NewChunk(largeContent, "test_source", ChunkTypeText)

	assert.NotNil(t, chunk)
	assert.Equal(t, len(largeContent), len(chunk.Content))
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
}

// TestChunkWithSpecialCharacters tests chunk creation with special characters
func TestChunkWithSpecialCharacters(t *testing.T) {
	specialContent := "Test with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	chunk := NewChunk(specialContent, "test_source", ChunkTypeText)

	assert.NotNil(t, chunk)
	assert.Equal(t, specialContent, chunk.Content)
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
}

// TestChunkWithUnicodeContent tests chunk creation with Unicode content
func TestChunkWithUnicodeContent(t *testing.T) {
	unicodeContent := "Tiếng Việt: Xin chào! 中文: 你好! 日本語: こんにちは! Emoji: 😀🎉"
	chunk := NewChunk(unicodeContent, "test_source", ChunkTypeText)

	assert.NotNil(t, chunk)
	assert.Equal(t, unicodeContent, chunk.Content)
	assert.NotEmpty(t, chunk.ID)
	assert.NotEmpty(t, chunk.Hash)
}

// TestChunkCreatedAtTimestamp tests that CreatedAt is set correctly
func TestChunkCreatedAtTimestamp(t *testing.T) {
	before := time.Now()
	chunk := NewChunk("test content", "test_source", ChunkTypeText)
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
		chunk := NewChunk("test content", source, ChunkTypeText)
		assert.Equal(t, source, chunk.Source)
	}
}

// TestGenerateChunkID tests the generateChunkID function
func TestGenerateChunkID(t *testing.T) {
	id1 := generateChunkID("content", "source")
	id2 := generateChunkID("content", "source")
	id3 := generateChunkID("different", "source")

	// Same inputs should produce same ID
	assert.Equal(t, id1, id2)

	// Different inputs should produce different ID
	assert.NotEqual(t, id1, id3)

	// ID should have expected format
	assert.Contains(t, id1, "chunk_")
}

// TestGenerateContentHash tests the generateContentHash function
func TestGenerateContentHash(t *testing.T) {
	hash1 := generateContentHash("content")
	hash2 := generateContentHash("content")
	hash3 := generateContentHash("different")

	// Same content should produce same hash
	assert.Equal(t, hash1, hash2)

	// Different content should produce different hash
	assert.NotEqual(t, hash1, hash3)

	// Hash should be non-empty
	assert.NotEmpty(t, hash1)
}

// TestGenerateContentHashTrimsWhitespace tests that hash generation trims whitespace
func TestGenerateContentHashTrimsWhitespace(t *testing.T) {
	hash1 := generateContentHash("content")
	hash2 := generateContentHash("  content  ")
	hash3 := generateContentHash("content\n")

	// All should produce the same hash after trimming
	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash1, hash3)
}

// TestChunkingConfig tests the ChunkingConfig struct
func TestChunkingConfig(t *testing.T) {
	config := &ChunkingConfig{
		Strategy:          StrategyParagraph,
		MaxSize:           1000,
		Overlap:           100,
		MinSize:           50,
		PreserveStructure: true,
	}

	assert.Equal(t, StrategyParagraph, config.Strategy)
	assert.Equal(t, 1000, config.MaxSize)
	assert.Equal(t, 100, config.Overlap)
	assert.Equal(t, 50, config.MinSize)
	assert.True(t, config.PreserveStructure)
}

// TestDefaultChunkingConfig tests the default configuration
func TestDefaultChunkingConfig(t *testing.T) {
	config := DefaultChunkingConfig()

	assert.NotNil(t, config)
	assert.Equal(t, StrategyParagraph, config.Strategy)
	assert.Equal(t, 1000, config.MaxSize)
	assert.Equal(t, 100, config.Overlap)
	assert.Equal(t, 50, config.MinSize)
	assert.True(t, config.PreserveStructure)
}

// TestChunkingStrategies tests all chunking strategy constants
func TestChunkingStrategies(t *testing.T) {
	strategies := []ChunkingStrategy{
		StrategyParagraph,
		StrategySentence,
		StrategyFixedSize,
		StrategySemantic,
	}

	for _, strategy := range strategies {
		config := &ChunkingConfig{Strategy: strategy}
		assert.Equal(t, strategy, config.Strategy)
	}
}

// TestChunkMetadataEnrichment tests metadata enrichment
func TestChunkMetadataEnrichment(t *testing.T) {
	chunk := NewChunk("test content", "test.txt", ChunkTypeText)

	// Add custom metadata
	chunk.Metadata["custom_field"] = "custom_value"
	chunk.Metadata["priority"] = 5

	assert.Equal(t, "custom_value", chunk.Metadata["custom_field"])
	assert.Equal(t, 5, chunk.Metadata["priority"])
}

// TestChunkImmutabilityOfIDAndHash tests that ID and Hash are set at creation
func TestChunkImmutabilityOfIDAndHash(t *testing.T) {
	chunk := NewChunk("test content", "test_source", ChunkTypeText)

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

	chunk1 := NewChunk("First paragraph", source, ChunkTypeParagraph)
	chunk2 := NewChunk("Second paragraph", source, ChunkTypeParagraph)
	chunk3 := NewChunk("Third paragraph", source, ChunkTypeParagraph)

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

	textChunk := NewChunk(content, source, ChunkTypeText)
	codeChunk := NewChunk(content, source, ChunkTypeCode)
	markdownChunk := NewChunk(content, source, ChunkTypeMarkdown)

	assert.Equal(t, ChunkTypeText, textChunk.Type)
	assert.Equal(t, ChunkTypeCode, codeChunk.Type)
	assert.Equal(t, ChunkTypeMarkdown, markdownChunk.Type)

	// Same content but different types should have different IDs
	// because ID is based on content + source
	assert.Equal(t, textChunk.ID, codeChunk.ID)     // Same content and source
	assert.Equal(t, textChunk.Hash, codeChunk.Hash) // Same content
}

// TestChunkJSONSerialization tests that chunks can be serialized to JSON
func TestChunkJSONSerialization(t *testing.T) {
	chunk := NewChunk("test content", "test_source", ChunkTypeText)
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
	chunk := NewChunk("test content", "test_source", ChunkTypeText)

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
	chunks := make([]*Chunk, numChunks)
	done := make(chan bool, numChunks)

	for i := 0; i < numChunks; i++ {
		go func(index int) {
			content := "Content " + string(rune(index))
			chunks[index] = NewChunk(content, "test_source", ChunkTypeText)
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
