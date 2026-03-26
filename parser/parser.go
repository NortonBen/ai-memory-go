// Package parser provides multi-format content processing capabilities.
// It converts various file types (text, PDF, markdown, etc.) into structured Chunk data.
package parser

import (
	"context"

	"github.com/NortonBen/ai-memory-go/schema"
)

// Parser defines the interface for content parsing operations
type Parser interface {
	// ParseFile parses a file and returns chunks
	ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error)

	// ParseText parses raw text content into chunks
	ParseText(ctx context.Context, content string) ([]*schema.Chunk, error)

	// ParseMarkdown parses markdown content with structure preservation
	ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error)

	// ParsePDF parses PDF files into text chunks
	ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error)

	// DetectContentType detects the type of content
	DetectContentType(content string) schema.ChunkType
}

// NewChunk creates a new chunk with proper initialization
func NewChunk(content, source string, chunkType schema.ChunkType) *schema.Chunk {
	return schema.NewChunk(content, source, chunkType)
}
