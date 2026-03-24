// Package parser provides multi-format content processing capabilities.
// It converts various file types (text, PDF, markdown, etc.) into structured Chunk data.
package parser

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

// ChunkType defines the type of content chunk
type ChunkType string

const (
	ChunkTypeText      ChunkType = "text"
	ChunkTypeParagraph ChunkType = "paragraph"
	ChunkTypeSentence  ChunkType = "sentence"
	ChunkTypeMarkdown  ChunkType = "markdown"
	ChunkTypePDF       ChunkType = "pdf"
	ChunkTypeCode      ChunkType = "code"
)

// Chunk represents a parsed piece of content with metadata
type Chunk struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Type      ChunkType              `json:"type"`
	Metadata  map[string]interface{} `json:"metadata"`
	Source    string                 `json:"source"`
	Hash      string                 `json:"hash"`
	CreatedAt time.Time              `json:"created_at"`
}

// Parser defines the interface for content parsing operations
type Parser interface {
	// ParseFile parses a file and returns chunks
	ParseFile(ctx context.Context, filePath string) ([]Chunk, error)

	// ParseText parses raw text content into chunks
	ParseText(ctx context.Context, content string) ([]Chunk, error)

	// ParseMarkdown parses markdown content with structure preservation
	ParseMarkdown(ctx context.Context, content string) ([]Chunk, error)

	// ParsePDF parses PDF files into text chunks
	ParsePDF(ctx context.Context, filePath string) ([]Chunk, error)

	// DetectContentType detects the type of content
	DetectContentType(content string) ChunkType
}

// ChunkingStrategy defines how content should be split into chunks
type ChunkingStrategy string

const (
	StrategyParagraph ChunkingStrategy = "paragraph"
	StrategySentence  ChunkingStrategy = "sentence"
	StrategyFixedSize ChunkingStrategy = "fixed_size"
	StrategySemantic  ChunkingStrategy = "semantic"
)

// ChunkingConfig configures how content is chunked
type ChunkingConfig struct {
	Strategy          ChunkingStrategy `json:"strategy"`
	MaxSize           int              `json:"max_size"`
	Overlap           int              `json:"overlap"`
	MinSize           int              `json:"min_size"`
	PreserveStructure bool             `json:"preserve_structure"`
}

// DefaultChunkingConfig returns a sensible default configuration
func DefaultChunkingConfig() *ChunkingConfig {
	return &ChunkingConfig{
		Strategy:          StrategyParagraph,
		MaxSize:           1000,
		Overlap:           100,
		MinSize:           50,
		PreserveStructure: true,
	}
}

// generateChunkID creates a unique ID for a chunk based on content and metadata
func generateChunkID(content, source string) string {
	hash := sha256.Sum256([]byte(content + source))
	return fmt.Sprintf("chunk_%x", hash[:8])
}

// generateContentHash creates a hash of the content for deduplication
func generateContentHash(content string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return fmt.Sprintf("%x", hash)
}

// NewChunk creates a new chunk with proper initialization
func NewChunk(content, source string, chunkType ChunkType) *Chunk {
	return &Chunk{
		ID:        generateChunkID(content, source),
		Content:   content,
		Type:      chunkType,
		Metadata:  make(map[string]interface{}),
		Source:    source,
		Hash:      generateContentHash(content),
		CreatedAt: time.Now(),
	}
}
