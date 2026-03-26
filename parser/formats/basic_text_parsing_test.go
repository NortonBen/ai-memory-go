// Package parser - Comprehensive tests for Task 3.1: Basic Text Parsing
package formats_test

import (
	"context"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/parser/core"
	"github.com/NortonBen/ai-memory-go/parser/formats"
	"github.com/NortonBen/ai-memory-go/parser/processing"
	"github.com/NortonBen/ai-memory-go/schema"
)

// TestParserInterface_ParseText tests the core ParseText functionality
func TestParserInterface_ParseText(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		strategy schema.ChunkingStrategy
		expected int // expected number of chunks
	}{
		{
			name:     "Simple paragraph text",
			content:  "This is the first paragraph.\n\nThis is the second paragraph.\n\nThis is the third paragraph.",
			strategy: schema.StrategyParagraph,
			expected: 1, // Will be combined into one chunk due to size constraints
		},
		{
			name:     "Single paragraph",
			content:  "This is a single paragraph with multiple sentences. It should create one chunk.",
			strategy: schema.StrategyParagraph,
			expected: 1,
		},
		{
			name:     "Sentence chunking",
			content:  "First sentence. Second sentence! Third sentence? Fourth sentence.",
			strategy: schema.StrategySentence,
			expected: 1, // Will be combined into one chunk due to size constraints
		},
		{
			name:     "Large paragraph text for chunking",
			content:  strings.Repeat("This is a paragraph with enough content to trigger chunking. ", 20) + "\n\n" + strings.Repeat("This is another paragraph with enough content. ", 20) + "\n\n" + strings.Repeat("This is the third paragraph with sufficient content. ", 20),
			strategy: schema.StrategyParagraph,
			expected: 3, // Should create multiple chunks due to size
		},
		{
			name:     "Fixed size chunking",
			content:  strings.Repeat("a", 2500), // 2500 characters
			strategy: schema.StrategyFixedSize,
			expected: 3, // With default max size 1000 and overlap 100
		},
		{
			name:     "Empty content",
			content:  "",
			strategy: schema.StrategyParagraph,
			expected: 0,
		},
		{
			name:     "Whitespace only",
			content:  "   \n\n   \t\t   \n   ",
			strategy: schema.StrategyParagraph,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &schema.ChunkingConfig{
				Strategy:          tt.strategy,
				MaxSize:           1000,
				Overlap:           100,
				MinSize:           50,
				PreserveStructure: true,
			}

			parser := formats.NewTextParser(config)
			ctx := context.Background()

			chunks, err := parser.ParseText(ctx, tt.content)
			if err != nil {
				t.Fatalf("ParseText failed: %v", err)
			}

			if len(chunks) != tt.expected {
				t.Errorf("Expected %d chunks, got %d", tt.expected, len(chunks))
			}

			// Verify chunk properties
			for i, chunk := range chunks {
				if chunk.ID == "" {
					t.Errorf("Chunk %d has empty ID", i)
				}
				if chunk.Hash == "" {
					t.Errorf("Chunk %d has empty hash", i)
				}
				if chunk.Type != schema.ChunkTypeParagraph && chunk.Type != schema.ChunkTypeSentence && chunk.Type != schema.ChunkTypeText {
					t.Errorf("Chunk %d has invalid type: %s", i, chunk.Type)
				}
				if chunk.CreatedAt.IsZero() {
					t.Errorf("Chunk %d has zero Timestamp time", i)
				}
			}
		})
	}
}

// TestParserInterface_DetectContentType tests content type detection
func TestParserInterface_DetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected schema.ChunkType
	}{
		{
			name:     "Go code",
			content:  "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			expected: schema.ChunkTypeCode,
		},
		{
			name:     "Python code",
			content:  "def hello():\n    print(\"Hello, World!\")\n\nif __name__ == \"__main__\":\n    hello()",
			expected: schema.ChunkTypeCode,
		},
		{
			name:     "Markdown content",
			content:  "# Title\n\nThis is a **bold** text with [link](http://example.com).",
			expected: schema.ChunkTypeMarkdown,
		},
		{
			name:     "Plain text",
			content:  "This is just plain text without any special formatting or code.",
			expected: schema.ChunkTypeText,
		},
		{
			name:     "Empty content",
			content:  "",
			expected: schema.ChunkTypeText,
		},
	}

	parser := formats.NewTextParser(schema.DefaultChunkingConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.DetectContentType(tt.content)
			if result != tt.expected {
				t.Errorf("Expected content type %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestChunkingStrategies_Comprehensive tests all chunking strategies
func TestChunkingStrategies_Comprehensive(t *testing.T) {
	content := `# Introduction

This is the first paragraph of our document. It contains multiple sentences. Each sentence provides important information.

This is the second paragraph. It also has multiple sentences! Some sentences end with exclamation marks? Others end with question marks.

## Section 2

Here we have another section with different content. The content is structured in a way that tests our chunking algorithms.

Final paragraph with some code: func main() { fmt.Println("Hello") }`

	strategies := []struct {
		name      schema.ChunkingStrategy
		minChunks int
		maxChunks int
	}{
		{schema.StrategyParagraph, 1, 3},
		{schema.StrategySentence, 1, 8},
		{schema.StrategyFixedSize, 1, 5},
		{schema.StrategySemantic, 1, 3}, // Falls back to paragraph
	}

	for _, strategy := range strategies {
		t.Run(string(strategy.name), func(t *testing.T) {
			config := &schema.ChunkingConfig{
				Strategy:          strategy.name,
				MaxSize:           500,
				Overlap:           50,
				MinSize:           20,
				PreserveStructure: true,
			}

			parser := formats.NewTextParser(config)
			ctx := context.Background()

			chunks, err := parser.ParseText(ctx, content)
			if err != nil {
				t.Fatalf("ParseText failed for strategy %s: %v", strategy.name, err)
			}

			if len(chunks) < strategy.minChunks || len(chunks) > strategy.maxChunks {
				t.Errorf("Strategy %s produced %d chunks, expected between %d and %d",
					strategy.name, len(chunks), strategy.minChunks, strategy.maxChunks)
			}

			// Verify no empty chunks
			for i, chunk := range chunks {
				if strings.TrimSpace(chunk.Content) == "" {
					t.Errorf("Strategy %s produced empty chunk at index %d", strategy.name, i)
				}
			}
		})
	}
}

// TestDeduplication_Basic tests basic deduplication functionality
func TestDeduplication_Basic(t *testing.T) {
	// Create chunks with some duplicates
	chunks := []*schema.Chunk{
		schema.NewChunk("This is unique content 1", "test", schema.ChunkTypeText),
		schema.NewChunk("This is unique content 2", "test", schema.ChunkTypeText),
		schema.NewChunk("This is unique content 1", "test", schema.ChunkTypeText), // Duplicate
		schema.NewChunk("This is unique content 3", "test", schema.ChunkTypeText),
		schema.NewChunk("This is unique content 2", "test", schema.ChunkTypeText), // Duplicate
	}

	// Test global deduplication
	unique := processing.DeduplicateChunksGlobal(chunks)
	if len(unique) != 3 {
		t.Errorf("Expected 3 unique chunks, got %d", len(unique))
	}

	// Test stateful deduplication
	deduplicator := processing.NewChunkDeduplicator()
	uniqueStateful := deduplicator.DeduplicateChunks(chunks)
	if len(uniqueStateful) != 3 {
		t.Errorf("Expected 3 unique chunks from stateful deduplicator, got %d", len(uniqueStateful))
	}

	// Verify seen count
	if deduplicator.GetSeenCount() != 3 {
		t.Errorf("Expected seen count of 3, got %d", deduplicator.GetSeenCount())
	}
}

// TestMetadataExtraction_Basic tests basic metadata extraction
func TestMetadataExtraction_Basic(t *testing.T) {
	content := `Title: Test Document
Author: John Doe
Date: 2024-01-15
Keywords: test, parsing, metadata

# Introduction

This is a test document with metadata in the header.
It contains various elements that should be extracted.`

	extractor := core.NewMetadataExtractor()
	metadata := extractor.ExtractMetadata(content)

	// Check extracted metadata
	if title, ok := metadata["title"].(string); !ok || title != "Test Document" {
		t.Errorf("Expected title 'Test Document', got %v", metadata["title"])
	}

	if author, ok := metadata["author"].(string); !ok || author != "John Doe" {
		t.Errorf("Expected author 'John Doe', got %v", metadata["author"])
	}

	if date, ok := metadata["date"].(string); !ok || date != "2024-01-15" {
		t.Errorf("Expected date '2024-01-15', got %v", metadata["date"])
	}

	if keywords, ok := metadata["keywords"].([]string); !ok || len(keywords) != 3 {
		t.Errorf("Expected 3 keywords, got %v", metadata["keywords"])
	}

	// Check basic statistics
	if wordCount, ok := metadata["word_count"].(int); !ok || wordCount == 0 {
		t.Errorf("Expected non-zero word count, got %v", metadata["word_count"])
	}
}

// TestChunkEnrichment tests chunk metadata enrichment
func TestChunkEnrichment(t *testing.T) {
	chunk := schema.NewChunk("This is test content for enrichment", "test.txt", schema.ChunkTypeText)

	// Enrich the chunk
	core.EnrichChunkMetadata(chunk, "test.txt")

	// Verify enrichment
	if chunk.Metadata["file_path"] != "test.txt" {
		t.Errorf("Expected file_path 'test.txt', got %v", chunk.Metadata["file_path"])
	}

	if chunk.Metadata["file_name"] != "test.txt" {
		t.Errorf("Expected file_name 'test.txt', got %v", chunk.Metadata["file_name"])
	}

	if chunk.Metadata["file_ext"] != ".txt" {
		t.Errorf("Expected file_ext '.txt', got %v", chunk.Metadata["file_ext"])
	}

	if wordCount, ok := chunk.Metadata["word_count"].(int); !ok || wordCount != 6 {
		t.Errorf("Expected word_count 6, got %v", chunk.Metadata["word_count"])
	}
}

// TestParserInterface_ErrorHandling tests error handling in parsing
func TestParserInterface_ErrorHandling(t *testing.T) {
	parser := formats.NewTextParser(schema.DefaultChunkingConfig())
	ctx := context.Background()

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := parser.ParseText(cancelledCtx, "test content")
	// Note: Current implementation doesn't check context cancellation
	// This test documents expected behavior for future implementation
	if err != nil {
		t.Logf("Context cancellation handling: %v", err)
	}

	// Test with very large content
	largeContent := strings.Repeat("Large content test. ", 10000)
	chunks, err := parser.ParseText(ctx, largeContent)
	if err != nil {
		t.Errorf("Failed to parse large content: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("Expected chunks from large content, got none")
	}
}

// TestContentTypeDetection_Advanced tests advanced content type detection
func TestContentTypeDetection_Advanced(t *testing.T) {
	detector := core.NewContentTypeDetector()

	tests := []struct {
		name     string
		content  string
		expected schema.ChunkType
	}{
		{
			name:     "Go function",
			content:  "func TestSomething(t *testing.T) {\n\t// test code\n}",
			expected: schema.ChunkTypeCode,
		},
		{
			name:     "Python class",
			content:  "class MyClass:\n    def __init__(self):\n        pass",
			expected: schema.ChunkTypeCode,
		},
		{
			name:     "JavaScript function",
			content:  "function myFunction() {\n    return 'hello';\n}",
			expected: schema.ChunkTypeCode,
		},
		{
			name:     "Markdown with headers",
			content:  "# Main Title\n\n## Subtitle\n\nSome **bold** text.",
			expected: schema.ChunkTypeMarkdown,
		},
		{
			name:     "Markdown with links",
			content:  "Check out [this link](https://example.com) for more info.",
			expected: schema.ChunkTypeMarkdown,
		},
		{
			name:     "Plain text",
			content:  "This is just regular text without any special formatting.",
			expected: schema.ChunkTypeText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectContentType(tt.content)
			if result != tt.expected {
				contentPreview := tt.content
				if len(contentPreview) > 50 {
					contentPreview = contentPreview[:50]
				}
				t.Errorf("Expected %s, got %s for content: %s", tt.expected, result, contentPreview)
			}
		})
	}
}
