// Package parser - Streaming parser for large files
package stream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// StreamingConfig is an alias for schema.StreamingConfig
type StreamingConfig = schema.StreamingConfig

// DefaultStreamingConfig returns sensible defaults for streaming parsing
func DefaultStreamingConfig() *schema.StreamingConfig {
	return schema.DefaultStreamingConfig()
}

// StreamingParser handles memory-efficient parsing of large files
type StreamingParser struct {
	config       *schema.StreamingConfig
	chunkingConf *schema.ChunkingConfig
	mu           sync.RWMutex
}

// NewStreamingParser creates a new streaming parser
func NewStreamingParser(streamConfig *schema.StreamingConfig, chunkConfig *schema.ChunkingConfig) *StreamingParser {
	if streamConfig == nil {
		streamConfig = schema.DefaultStreamingConfig()
	}
	if chunkConfig == nil {
		chunkConfig = schema.DefaultChunkingConfig()
	}

	return &StreamingParser{
		config:       streamConfig,
		chunkingConf: chunkConfig,
	}
}

// StreamingResult is an alias for schema.StreamingResult
type StreamingResult = schema.StreamingResult

// ParseFileStream parses a large file using streaming approach
func (sp *StreamingParser) ParseFileStream(ctx context.Context, filePath string) (*schema.StreamingResult, error) {
	startTime := time.Now()

	// Open file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size for progress tracking
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}
	totalBytes := stat.Size()

	// Create buffered reader
	reader := bufio.NewReaderSize(file, sp.config.BufferSize)

	// Initialize result
	result := &schema.StreamingResult{
		Chunks:     make([]*schema.Chunk, 0),
		TotalBytes: totalBytes,
	}

	// Stream processing
	chunks, err := sp.streamProcess(ctx, reader, filePath, totalBytes)
	if err != nil {
		return nil, fmt.Errorf("streaming process failed: %w", err)
	}

	result.Chunks = chunks
	result.ChunksCreated = len(chunks)
	result.ProcessingTime = time.Since(startTime)

	return result, nil
}

// ParseReaderStream parses content from any io.Reader using streaming
func (sp *StreamingParser) ParseReaderStream(ctx context.Context, reader io.Reader, source string) (*schema.StreamingResult, error) {
	startTime := time.Now()

	// Wrap in buffered reader if not already
	var bufferedReader *bufio.Reader
	if br, ok := reader.(*bufio.Reader); ok {
		bufferedReader = br
	} else {
		bufferedReader = bufio.NewReaderSize(reader, sp.config.BufferSize)
	}

	// Stream processing (unknown total size)
	chunks, err := sp.streamProcess(ctx, bufferedReader, source, -1)
	if err != nil {
		return nil, fmt.Errorf("streaming process failed: %w", err)
	}

	return &schema.StreamingResult{
		Chunks:         chunks,
		ChunksCreated:  len(chunks),
		ProcessingTime: time.Since(startTime),
		TotalBytes:     -1, // Unknown for generic reader
	}, nil
}

// streamProcess performs the core streaming processing logic
func (sp *StreamingParser) streamProcess(ctx context.Context, reader *bufio.Reader, source string, totalBytes int64) ([]*schema.Chunk, error) {
	var chunks []*schema.Chunk
	var buffer strings.Builder
	var bytesProcessed int64
	var overlap string

	// Progress tracking
	lastProgressUpdate := time.Now()

	for {
		select {
		case <-ctx.Done():
			return chunks, ctx.Err()
		default:
		}

		// Read next buffer
		readBuffer := make([]byte, sp.config.BufferSize)
		n, err := reader.Read(readBuffer)
		if n > 0 {
			bytesProcessed += int64(n)

			// Add overlap from previous chunk
			if overlap != "" {
				buffer.WriteString(overlap)
				overlap = ""
			}

			// Add new content
			buffer.Write(readBuffer[:n])

			// Process buffer if it's large enough or we've reached EOF
			if buffer.Len() >= sp.config.MaxChunkSize || err == io.EOF {
				newChunks, newOverlap := sp.processBuffer(buffer.String(), source)
				chunks = append(chunks, newChunks...)

				// Save overlap for next iteration
				overlap = newOverlap
				buffer.Reset()
			}

			// Update progress if enabled
			if sp.config.EnableProgressTracking &&
				time.Since(lastProgressUpdate) > sp.config.FlushInterval {
				if sp.config.ProgressCallback != nil {
					sp.config.ProgressCallback(bytesProcessed, totalBytes, len(chunks))
				}
				lastProgressUpdate = time.Now()
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return chunks, fmt.Errorf("read error: %w", err)
		}
	}

	// Process any remaining content in buffer
	if buffer.Len() > 0 {
		finalChunks := sp.processBufferFinal(buffer.String(), source)
		chunks = append(chunks, finalChunks...)
	}

	// Process any remaining overlap
	if overlap != "" {
		finalChunks := sp.processBufferFinal(overlap, source)
		chunks = append(chunks, finalChunks...)
	}

	// Final progress update
	if sp.config.EnableProgressTracking && sp.config.ProgressCallback != nil {
		sp.config.ProgressCallback(bytesProcessed, totalBytes, len(chunks))
	}

	return chunks, nil
}

// processBuffer processes a buffer of text into chunks with overlap handling
func (sp *StreamingParser) processBuffer(content, source string) ([]*schema.Chunk, string) {
	if len(content) < sp.config.MinChunkSize {
		return nil, content // Return as overlap for next buffer
	}

	var chunks []*schema.Chunk
	var overlap string

	// Split content based on chunking strategy
	switch sp.chunkingConf.Strategy {
	case schema.StrategyParagraph:
		chunks, overlap = sp.chunkBufferByParagraph(content, source)
	case schema.StrategySentence:
		chunks, overlap = sp.chunkBufferBySentence(content, source)
	case schema.StrategyFixedSize:
		chunks, overlap = sp.chunkBufferByFixedSize(content, source)
	default:
		chunks, overlap = sp.chunkBufferByParagraph(content, source)
	}

	return chunks, overlap
}

// processBufferFinal processes final buffer content, creating chunks even if below min size
func (sp *StreamingParser) processBufferFinal(content, source string) []*schema.Chunk {
	if len(content) == 0 {
		return nil
	}

	// For final processing, create chunk even if below min size
	var chunks []*schema.Chunk

	// Split content based on chunking strategy
	switch sp.chunkingConf.Strategy {
	case schema.StrategyParagraph:
		chunks, _ = sp.chunkBufferByParagraph(content, source)
	case schema.StrategySentence:
		chunks, _ = sp.chunkBufferBySentence(content, source)
	case schema.StrategyFixedSize:
		chunks, _ = sp.chunkBufferByFixedSize(content, source)
	default:
		chunks, _ = sp.chunkBufferByParagraph(content, source)
	}

	// If no chunks were created but we have content, create a single chunk
	if len(chunks) == 0 && len(content) > 0 {
		chunk := schema.NewChunk(content, source, schema.ChunkTypeText)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// chunkBufferByParagraph splits buffer content by paragraphs with streaming overlap
func (sp *StreamingParser) chunkBufferByParagraph(content, source string) ([]*schema.Chunk, string) {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []*schema.Chunk
	var currentChunk strings.Builder

	// Process all but the last paragraph (which might be incomplete)
	for i := 0; i < len(paragraphs)-1; i++ {
		para := strings.TrimSpace(paragraphs[i])
		if para == "" {
			continue
		}

		// Check if adding this paragraph exceeds max size
		if currentChunk.Len()+len(para) > sp.config.MaxChunkSize && currentChunk.Len() > 0 {
			// Create chunk from current content
			if currentChunk.Len() >= sp.config.MinChunkSize {
				chunk := schema.NewChunk(currentChunk.String(), source, schema.ChunkTypeParagraph)
				chunks = append(chunks, chunk)
			}
			currentChunk.Reset()
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
	}

	// Handle the last paragraph (might be incomplete)
	var overlap string
	if len(paragraphs) > 0 {
		lastPara := strings.TrimSpace(paragraphs[len(paragraphs)-1])
		if lastPara != "" {
			if currentChunk.Len()+len(lastPara) <= sp.config.MaxChunkSize {
				// Add to current chunk
				if currentChunk.Len() > 0 {
					currentChunk.WriteString("\n\n")
				}
				currentChunk.WriteString(lastPara)
			} else {
				// Last paragraph is too big, save current chunk and use last para as overlap
				if currentChunk.Len() >= sp.config.MinChunkSize {
					chunk := schema.NewChunk(currentChunk.String(), source, schema.ChunkTypeParagraph)
					chunks = append(chunks, chunk)
				}
				overlap = lastPara
			}
		}
	}

	// Create final chunk if we have content
	if currentChunk.Len() >= sp.config.MinChunkSize && overlap == "" {
		chunk := schema.NewChunk(currentChunk.String(), source, schema.ChunkTypeParagraph)
		chunks = append(chunks, chunk)
	} else if overlap == "" && currentChunk.Len() > 0 {
		// Save remaining content as overlap
		overlap = currentChunk.String()
	}

	return chunks, overlap
}

// chunkBufferBySentence splits buffer content by sentences with streaming overlap
func (sp *StreamingParser) chunkBufferBySentence(content, source string) ([]*schema.Chunk, string) {
	// Simple sentence splitting - can be enhanced with NLP
	sentences := strings.FieldsFunc(content, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})

	var chunks []*schema.Chunk
	var currentChunk strings.Builder

	// Process all but the last sentence (which might be incomplete)
	for i := 0; i < len(sentences)-1; i++ {
		sentence := strings.TrimSpace(sentences[i])
		if sentence == "" {
			continue
		}

		// Add sentence terminator back
		sentence += "."

		// Check if adding this sentence exceeds max size
		if currentChunk.Len()+len(sentence) > sp.config.MaxChunkSize && currentChunk.Len() > 0 {
			// Create chunk from current content
				chunk := schema.NewChunk(currentChunk.String(), source, schema.ChunkTypeSentence)
				chunks = append(chunks, chunk)
			currentChunk.Reset()
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)
	}

	// Handle the last sentence (might be incomplete)
	var overlap string
	if len(sentences) > 0 {
		lastSentence := strings.TrimSpace(sentences[len(sentences)-1])
		if lastSentence != "" {
			// Last sentence might be incomplete, save as overlap
			overlap = lastSentence
		}
	}

	// Create final chunk if we have content
	if currentChunk.Len() >= sp.config.MinChunkSize {
		chunk := schema.NewChunk(currentChunk.String(), source, schema.ChunkTypeSentence)
		chunks = append(chunks, chunk)
	} else if currentChunk.Len() > 0 {
		// Combine with overlap
		if overlap != "" {
			overlap = currentChunk.String() + " " + overlap
		} else {
			overlap = currentChunk.String()
		}
	}

	return chunks, overlap
}

// chunkBufferByFixedSize splits buffer content by fixed size with streaming overlap
func (sp *StreamingParser) chunkBufferByFixedSize(content, source string) ([]*schema.Chunk, string) {
	var chunks []*schema.Chunk
	contentRunes := []rune(content)

	// Process content in fixed-size chunks, leaving some for overlap
	for i := 0; i < len(contentRunes)-sp.config.ChunkOverlap; i += sp.config.MaxChunkSize - sp.config.ChunkOverlap {
		end := i + sp.config.MaxChunkSize
		if end > len(contentRunes) {
			end = len(contentRunes)
		}

		chunkContent := string(contentRunes[i:end])
		chunkContent = strings.TrimSpace(chunkContent)

		if len(chunkContent) >= sp.config.MinChunkSize {
			chunk := schema.NewChunk(chunkContent, source, schema.ChunkTypeText)
			chunks = append(chunks, chunk)
		}

		// If we've processed most of the content, save remainder as overlap
		if end >= len(contentRunes)-sp.config.ChunkOverlap {
			break
		}
	}

	// Calculate overlap for next buffer
	var overlap string
	if len(contentRunes) > sp.config.ChunkOverlap {
		overlap = string(contentRunes[len(contentRunes)-sp.config.ChunkOverlap:])
	} else {
		overlap = content
	}

	return chunks, overlap
}

// GetMemoryUsage returns current memory usage estimate
func (sp *StreamingParser) GetMemoryUsage() int64 {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	// Estimate based on buffer sizes
	return int64(sp.config.BufferSize + sp.config.MaxChunkSize + sp.config.ChunkOverlap)
}

// UpdateConfig updates the streaming configuration
func (sp *StreamingParser) UpdateConfig(config *schema.StreamingConfig) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if config != nil {
		sp.config = config
	}
}

// GetConfig returns a copy of the current configuration
func (sp *StreamingParser) GetConfig() *schema.StreamingConfig {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Return a copy to prevent external modification
	conf := *sp.config
	return &conf
}
