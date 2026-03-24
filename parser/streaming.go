// Package parser - Streaming parser for large files
package parser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// StreamingConfig configures streaming parser behavior
type StreamingConfig struct {
	// BufferSize is the size of each read buffer in bytes
	BufferSize int `json:"buffer_size"`

	// ChunkOverlap is the overlap between chunks in bytes
	ChunkOverlap int `json:"chunk_overlap"`

	// MaxChunkSize is the maximum size of a single chunk
	MaxChunkSize int `json:"max_chunk_size"`

	// MinChunkSize is the minimum size of a chunk to be processed
	MinChunkSize int `json:"min_chunk_size"`

	// ProgressCallback is called periodically with progress updates
	ProgressCallback func(bytesProcessed, totalBytes int64, chunksCreated int)

	// EnableProgressTracking enables progress tracking for long operations
	EnableProgressTracking bool `json:"enable_progress_tracking"`

	// FlushInterval determines how often to flush processed chunks
	FlushInterval time.Duration `json:"flush_interval"`
}

// DefaultStreamingConfig returns sensible defaults for streaming parsing
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		BufferSize:             64 * 1024, // 64KB buffer
		ChunkOverlap:           1024,      // 1KB overlap
		MaxChunkSize:           4 * 1024,  // 4KB max chunk
		MinChunkSize:           256,       // 256B min chunk
		EnableProgressTracking: true,
		FlushInterval:          100 * time.Millisecond,
	}
}

// StreamingParser handles memory-efficient parsing of large files
type StreamingParser struct {
	config       *StreamingConfig
	chunkingConf *ChunkingConfig
	mu           sync.RWMutex
}

// NewStreamingParser creates a new streaming parser
func NewStreamingParser(streamConfig *StreamingConfig, chunkConfig *ChunkingConfig) *StreamingParser {
	if streamConfig == nil {
		streamConfig = DefaultStreamingConfig()
	}
	if chunkConfig == nil {
		chunkConfig = DefaultChunkingConfig()
	}

	return &StreamingParser{
		config:       streamConfig,
		chunkingConf: chunkConfig,
	}
}

// StreamingResult represents the result of streaming parsing
type StreamingResult struct {
	Chunks          []Chunk
	TotalBytes      int64
	ProcessingTime  time.Duration
	ChunksCreated   int
	MemoryPeakUsage int64
}

// ParseFileStream parses a large file using streaming approach
func (sp *StreamingParser) ParseFileStream(ctx context.Context, filePath string) (*StreamingResult, error) {
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
	result := &StreamingResult{
		Chunks:     make([]Chunk, 0),
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
func (sp *StreamingParser) ParseReaderStream(ctx context.Context, reader io.Reader, source string) (*StreamingResult, error) {
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

	return &StreamingResult{
		Chunks:         chunks,
		ChunksCreated:  len(chunks),
		ProcessingTime: time.Since(startTime),
		TotalBytes:     -1, // Unknown for generic reader
	}, nil
}

// streamProcess performs the core streaming processing logic
func (sp *StreamingParser) streamProcess(ctx context.Context, reader *bufio.Reader, source string, totalBytes int64) ([]Chunk, error) {
	var chunks []Chunk
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
func (sp *StreamingParser) processBuffer(content, source string) ([]Chunk, string) {
	if len(content) < sp.config.MinChunkSize {
		return nil, content // Return as overlap for next buffer
	}

	var chunks []Chunk
	var overlap string

	// Split content based on chunking strategy
	switch sp.chunkingConf.Strategy {
	case StrategyParagraph:
		chunks, overlap = sp.chunkBufferByParagraph(content, source)
	case StrategySentence:
		chunks, overlap = sp.chunkBufferBySentence(content, source)
	case StrategyFixedSize:
		chunks, overlap = sp.chunkBufferByFixedSize(content, source)
	default:
		chunks, overlap = sp.chunkBufferByParagraph(content, source)
	}

	return chunks, overlap
}

// processBufferFinal processes final buffer content, creating chunks even if below min size
func (sp *StreamingParser) processBufferFinal(content, source string) []Chunk {
	if len(content) == 0 {
		return nil
	}

	// For final processing, create chunk even if below min size
	var chunks []Chunk

	// Split content based on chunking strategy
	switch sp.chunkingConf.Strategy {
	case StrategyParagraph:
		chunks, _ = sp.chunkBufferByParagraph(content, source)
	case StrategySentence:
		chunks, _ = sp.chunkBufferBySentence(content, source)
	case StrategyFixedSize:
		chunks, _ = sp.chunkBufferByFixedSize(content, source)
	default:
		chunks, _ = sp.chunkBufferByParagraph(content, source)
	}

	// If no chunks were created but we have content, create a single chunk
	if len(chunks) == 0 && len(content) > 0 {
		chunk := NewChunk(content, source, ChunkTypeText)
		chunks = append(chunks, *chunk)
	}

	return chunks
}

// chunkBufferByParagraph splits buffer content by paragraphs with streaming overlap
func (sp *StreamingParser) chunkBufferByParagraph(content, source string) ([]Chunk, string) {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []Chunk
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
				chunk := NewChunk(currentChunk.String(), source, ChunkTypeParagraph)
				chunks = append(chunks, *chunk)
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
					chunk := NewChunk(currentChunk.String(), source, ChunkTypeParagraph)
					chunks = append(chunks, *chunk)
				}
				overlap = lastPara
			}
		}
	}

	// Create final chunk if we have content
	if currentChunk.Len() >= sp.config.MinChunkSize && overlap == "" {
		chunk := NewChunk(currentChunk.String(), source, ChunkTypeParagraph)
		chunks = append(chunks, *chunk)
	} else if overlap == "" && currentChunk.Len() > 0 {
		// Save remaining content as overlap
		overlap = currentChunk.String()
	}

	return chunks, overlap
}

// chunkBufferBySentence splits buffer content by sentences with streaming overlap
func (sp *StreamingParser) chunkBufferBySentence(content, source string) ([]Chunk, string) {
	// Simple sentence splitting - can be enhanced with NLP
	sentences := strings.FieldsFunc(content, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})

	var chunks []Chunk
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
			if currentChunk.Len() >= sp.config.MinChunkSize {
				chunk := NewChunk(currentChunk.String(), source, ChunkTypeSentence)
				chunks = append(chunks, *chunk)
			}
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
		chunk := NewChunk(currentChunk.String(), source, ChunkTypeSentence)
		chunks = append(chunks, *chunk)
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
func (sp *StreamingParser) chunkBufferByFixedSize(content, source string) ([]Chunk, string) {
	var chunks []Chunk
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
			chunk := NewChunk(chunkContent, source, ChunkTypeText)
			chunks = append(chunks, *chunk)
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
	// Estimate based on buffer sizes
	return int64(sp.config.BufferSize + sp.config.MaxChunkSize + sp.config.ChunkOverlap)
}

// UpdateConfig updates the streaming configuration
func (sp *StreamingParser) UpdateConfig(config *StreamingConfig) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if config != nil {
		sp.config = config
	}
}

// GetConfig returns a copy of the current configuration
func (sp *StreamingParser) GetConfig() *StreamingConfig {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Return a copy to prevent external modification
	return &StreamingConfig{
		BufferSize:             sp.config.BufferSize,
		ChunkOverlap:           sp.config.ChunkOverlap,
		MaxChunkSize:           sp.config.MaxChunkSize,
		MinChunkSize:           sp.config.MinChunkSize,
		ProgressCallback:       sp.config.ProgressCallback,
		EnableProgressTracking: sp.config.EnableProgressTracking,
		FlushInterval:          sp.config.FlushInterval,
	}
}
