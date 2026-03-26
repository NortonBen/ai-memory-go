// Package parser - Unified multi-format parser implementation
package parser

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"github.com/NortonBen/ai-memory-go/schema"
)

// UnifiedParser implements the Parser interface for all supported formats
type UnifiedParser struct {
	config          *schema.ChunkingConfig
	textParser      *TextParser
	pdfParser       *PDFParser
	formatParser    *FormatParser
	streamingParser *StreamingParser
	workerPool      *WorkerPool
}

// NewUnifiedParser creates a new unified parser that handles all formats
func NewUnifiedParser(config *schema.ChunkingConfig) *UnifiedParser {
	if config == nil {
		config = schema.DefaultChunkingConfig()
	}

	parser := &UnifiedParser{
		config:          config,
		textParser:      NewTextParser(config),
		pdfParser:       NewPDFParser(config),
		formatParser:    NewFormatParser(config),
		streamingParser: NewStreamingParser(DefaultStreamingConfig(), config),
	}

	// Initialize worker pool with default config
	parser.workerPool = NewWorkerPool(parser, DefaultWorkerPoolConfig())

	return parser
}

// NewUnifiedParserWithCache creates a unified parser with caching enabled
func NewUnifiedParserWithCache(config *schema.ChunkingConfig, cacheConfig *schema.CacheConfig) *CachedUnifiedParser {
	return NewCachedUnifiedParser(config, cacheConfig)
}

// ParseFile parses a file based on its extension and content type
func (up *UnifiedParser) ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Detect file format
	format := DetectFileFormat(filePath)

	switch format {
	case "txt":
		return up.formatParser.ParseTXT(ctx, filePath)
	case "csv":
		return up.formatParser.ParseCSV(ctx, filePath)
	case "json":
		return up.formatParser.ParseJSON(ctx, filePath)
	case "markdown":
		return up.ParseMarkdownFile(ctx, filePath)
	case "pdf":
		return up.pdfParser.ParsePDF(ctx, filePath)
	default:
		// Try to parse as text file
		return up.formatParser.ParseTXT(ctx, filePath)
	}
}

// ParseText parses raw text content into chunks
func (up *UnifiedParser) ParseText(ctx context.Context, content string) ([]*schema.Chunk, error) {
	return up.textParser.ParseText(ctx, content)
}

// ParseMarkdown parses markdown content with structure preservation
func (up *UnifiedParser) ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error) {
	// Use existing markdown parser if available, otherwise use text parser
	return up.textParser.ParseMarkdown(ctx, content)
}

// ParseMarkdownFile parses a markdown file
func (up *UnifiedParser) ParseMarkdownFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read markdown file: %w", err)
	}

	chunks, err := up.ParseMarkdown(ctx, string(content))
	if err != nil {
		return nil, err
	}

	// Add file metadata to chunks
	metadata := map[string]interface{}{
		"file_path": filePath,
		"file_name": filepath.Base(filePath),
		"file_type": "markdown",
		"file_size": len(content),
	}

	for i := range chunks {
		chunks[i].Source = filePath
		chunks[i].Type = schema.ChunkTypeMarkdown
		for k, v := range metadata {
			chunks[i].Metadata[k] = v
		}
	}

	return chunks, nil
}

// ParsePDF parses PDF files into text chunks
func (up *UnifiedParser) ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return up.pdfParser.ParsePDF(ctx, filePath)
}

// DetectContentType detects the type of content
func (up *UnifiedParser) DetectContentType(content string) schema.ChunkType {
	return up.textParser.DetectContentType(content)
}

// ParseCSV parses CSV files
func (up *UnifiedParser) ParseCSV(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return up.formatParser.ParseCSV(ctx, filePath)
}

// ParseJSON parses JSON files
func (up *UnifiedParser) ParseJSON(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return up.formatParser.ParseJSON(ctx, filePath)
}

// ParseTXT parses plain text files
func (up *UnifiedParser) ParseTXT(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return up.formatParser.ParseTXT(ctx, filePath)
}

// GetSupportedFormats returns a list of supported file formats
func (up *UnifiedParser) GetSupportedFormats() []string {
	return []string{
		"txt", "text", "csv", "json", "md", "markdown", "pdf",
	}
}

// IsFormatSupported checks if a file format is supported
func (up *UnifiedParser) IsFormatSupported(filePath string) bool {
	format := DetectFileFormat(filePath)
	supportedFormats := up.GetSupportedFormats()

	for _, supported := range supportedFormats {
		if format == supported {
			return true
		}
	}

	return format == "unknown" // Unknown formats are treated as text
}

// BatchParseFiles parses multiple files concurrently using the worker pool
func (up *UnifiedParser) BatchParseFiles(ctx context.Context, filePaths []string) (map[string][]*schema.Chunk, error) {
	if len(filePaths) == 0 {
		return make(map[string][]*schema.Chunk), nil
	}

	// Start worker pool if not already started
	if !up.workerPool.IsHealthy() {
		if err := up.workerPool.Start(); err != nil {
			return nil, fmt.Errorf("failed to start worker pool: %w", err)
		}
	}

	// Use worker pool for parallel processing
	return up.workerPool.ProcessFiles(ctx, filePaths)
}

// ValidateChunks validates that chunks meet quality requirements
func (up *UnifiedParser) ValidateChunks(chunks []*schema.Chunk) error {
	for i, chunk := range chunks {
		if chunk.Content == "" {
			return fmt.Errorf("chunk %d has empty content", i)
		}

		if len(chunk.Content) < up.config.MinSize {
			return fmt.Errorf("chunk %d is below minimum size (%d < %d)", i, len(chunk.Content), up.config.MinSize)
		}

		if chunk.ID == "" {
			return fmt.Errorf("chunk %d has empty ID", i)
		}

		if chunk.Hash == "" {
			return fmt.Errorf("chunk %d has empty hash", i)
		}
	}

	return nil
}

// NewUnifiedParserWithWorkerPool creates a parser with custom worker pool configuration
func NewUnifiedParserWithWorkerPool(config *schema.ChunkingConfig, workerConfig *schema.WorkerPoolConfig) *UnifiedParser {
	if config == nil {
		config = schema.DefaultChunkingConfig()
	}
	if workerConfig == nil {
		workerConfig = schema.DefaultWorkerPoolConfig()
	}

	parser := &UnifiedParser{
		config:          config,
		textParser:      NewTextParser(config),
		pdfParser:       NewPDFParser(config),
		formatParser:    NewFormatParser(config),
		streamingParser: NewStreamingParser(DefaultStreamingConfig(), config),
	}

	// Initialize worker pool with custom config
	parser.workerPool = NewWorkerPool(parser, workerConfig)

	return parser
}

// StartWorkerPool starts the worker pool for parallel processing
func (up *UnifiedParser) StartWorkerPool() error {
	return up.workerPool.Start()
}

// StopWorkerPool gracefully shuts down the worker pool
func (up *UnifiedParser) StopWorkerPool() error {
	return up.workerPool.Stop()
}

// GetWorkerPoolMetrics returns current worker pool performance metrics
func (up *UnifiedParser) GetWorkerPoolMetrics() schema.WorkerPoolMetrics {
	return up.workerPool.GetMetrics()
}

// IsWorkerPoolHealthy checks if the worker pool is healthy and responsive
func (up *UnifiedParser) IsWorkerPoolHealthy() bool {
	return up.workerPool.IsHealthy()
}

// ProcessFilesParallel processes multiple files in parallel with custom options
func (up *UnifiedParser) ProcessFilesParallel(ctx context.Context, filePaths []string, metadata map[string]interface{}) (map[string][]*schema.Chunk, error) {
	if len(filePaths) == 0 {
		return make(map[string][]*schema.Chunk), nil
	}

	// Ensure worker pool is started
	if !up.workerPool.IsHealthy() {
		if err := up.workerPool.Start(); err != nil {
			return nil, fmt.Errorf("failed to start worker pool: %w", err)
		}
	}

	// Submit tasks with metadata
	results := make(map[string][]*schema.Chunk)
	errors := make(map[string]error)

	// Submit all tasks
	tasks := make([]*ProcessingTask, 0, len(filePaths))
	for _, filePath := range filePaths {
		task, err := up.workerPool.SubmitTask(filePath, metadata)
		if err != nil {
			errors[filePath] = fmt.Errorf("failed to submit task: %w", err)
			continue
		}
		tasks = append(tasks, task)
	}

	// Collect results
	completed := 0
	for completed < len(tasks) {
		select {
		case result := <-up.workerPool.resultQueue:
			completed++
			if result.Error != nil {
				errors[result.FilePath] = result.Error
			} else {
				results[result.FilePath] = result.Chunks
			}

		case <-ctx.Done():
			return results, fmt.Errorf("context cancelled, completed %d/%d tasks", completed, len(tasks))
		}
	}

	// Return partial results even if some files failed
	if len(errors) > 0 {
		var errorMsg string
		for filePath, err := range errors {
			errorMsg += fmt.Sprintf("%s: %v; ", filePath, err)
		}
		return results, fmt.Errorf("some files failed to process: %s", errorMsg)
	}

	return results, nil
}

// Close gracefully shuts down the parser and its worker pool
func (up *UnifiedParser) Close() error {
	if up.workerPool != nil {
		return up.workerPool.Stop()
	}
	return nil
}

// ParseFileStream parses large files using streaming approach for memory efficiency
func (up *UnifiedParser) ParseFileStream(ctx context.Context, filePath string) (*schema.StreamingResult, error) {
	return up.streamingParser.ParseFileStream(ctx, filePath)
}

// ParseReaderStream parses content from io.Reader using streaming approach
func (up *UnifiedParser) ParseReaderStream(ctx context.Context, reader io.Reader, source string) (*schema.StreamingResult, error) {
	return up.streamingParser.ParseReaderStream(ctx, reader, source)
}

// UpdateStreamingConfig updates the streaming parser configuration
func (up *UnifiedParser) UpdateStreamingConfig(config *schema.StreamingConfig) {
	up.streamingParser.UpdateConfig(config)
}

// GetStreamingConfig returns the current streaming configuration
func (up *UnifiedParser) GetStreamingConfig() *schema.StreamingConfig {
	return up.streamingParser.GetConfig()
}

// GetStreamingMemoryUsage returns estimated memory usage for streaming operations
func (up *UnifiedParser) GetStreamingMemoryUsage() int64 {
	return up.streamingParser.GetMemoryUsage()
}

// ShouldUseStreaming determines if streaming should be used based on file size
func (up *UnifiedParser) ShouldUseStreaming(filePath string) (bool, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	// Use streaming for files larger than 10MB
	const streamingThreshold = 10 * 1024 * 1024
	return stat.Size() > streamingThreshold, nil
}

// ParseFileAuto automatically chooses between regular and streaming parsing
func (up *UnifiedParser) ParseFileAuto(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	useStreaming, err := up.ShouldUseStreaming(filePath)
	if err != nil {
		return nil, err
	}

	if useStreaming {
		result, err := up.ParseFileStream(ctx, filePath)
		if err != nil {
			return nil, err
		}
		return result.Chunks, nil
	}

	return up.ParseFile(ctx, filePath)
}
