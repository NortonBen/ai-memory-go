// Package parser provides multi-format content processing capabilities.
// It converts various file types (text, PDF, markdown, etc.) into structured Chunk data.
package parser

import (
	"github.com/NortonBen/ai-memory-go/schema"
)

// Parser defines the interface for content parsing operations.
// This is now an alias to schema.Parser to break import cycles.
type Parser = schema.Parser

// ChunkType is an alias to schema.ChunkType for backward compatibility.
type ChunkType = schema.ChunkType
