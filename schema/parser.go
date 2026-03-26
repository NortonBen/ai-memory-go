// Package schema defines the core data structures for the AI Memory system.
package schema

import (
	"context"
)

// Parser defines the interface for content parsing operations
type Parser interface {
	// ParseFile parses a file and returns chunks
	ParseFile(ctx context.Context, filePath string) ([]*Chunk, error)

	// ParseText parses raw text content into chunks
	ParseText(ctx context.Context, content string) ([]*Chunk, error)

	// ParseMarkdown parses markdown content with structure preservation
	ParseMarkdown(ctx context.Context, content string) ([]*Chunk, error)

	// ParsePDF parses PDF files into text chunks
	ParsePDF(ctx context.Context, filePath string) ([]*Chunk, error)

	// DetectContentType detects the type of content
	DetectContentType(content string) ChunkType
}
