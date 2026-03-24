// Package parser - Enhanced file type detection and routing
package parser

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileTypeDetector provides enhanced file type detection capabilities
type FileTypeDetector struct {
	config *ChunkingConfig
}

// NewFileTypeDetector creates a new file type detector
func NewFileTypeDetector(config *ChunkingConfig) *FileTypeDetector {
	if config == nil {
		config = DefaultChunkingConfig()
	}
	return &FileTypeDetector{config: config}
}

// FileInfo contains detected file information
type FileInfo struct {
	Format      string                 `json:"format"`
	MimeType    string                 `json:"mime_type"`
	Encoding    string                 `json:"encoding"`
	Size        int64                  `json:"size"`
	Metadata    map[string]interface{} `json:"metadata"`
	IsText      bool                   `json:"is_text"`
	IsBinary    bool                   `json:"is_binary"`
	Confidence  float64                `json:"confidence"`
}

// DetectFileInfo performs comprehensive file type detection
func (ftd *FileTypeDetector) DetectFileInfo(filePath string) (*FileInfo, error) {
	// Get file stats
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	info := &FileInfo{
		Size:     stat.Size(),
		Metadata: make(map[string]interface{}),
	}

	// Extension-based detection
	info.Format = DetectFileFormat(filePath)
	info.Confidence = 0.7 // Base confidence for extension detection

	// Content-based detection for better accuracy
	if stat.Size() > 0 && stat.Size() < 10*1024*1024 { // Only for files < 10MB
		contentInfo, err := ftd.detectFromContent(filePath)
		if err == nil {
			// Merge content-based detection
			if contentInfo.Format != "unknown" && contentInfo.Confidence > info.Confidence {
				info.Format = contentInfo.Format
				info.Confidence = contentInfo.Confidence
			}
			info.MimeType = contentInfo.MimeType
			info.Encoding = contentInfo.Encoding
			info.IsText = contentInfo.IsText
			info.IsBinary = contentInfo.IsBinary
		}
	}

	// Add file metadata
	info.Metadata["file_name"] = filepath.Base(filePath)
	info.Metadata["file_path"] = filePath
	info.Metadata["file_size"] = info.Size
	info.Metadata["file_ext"] = filepath.Ext(filePath)
	info.Metadata["modified_time"] = stat.ModTime()

	return info, nil
}

// detectFromContent analyzes file content to determine type
func (ftd *FileTypeDetector) detectFromContent(filePath string) (*FileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read first 512 bytes for magic number detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}
	buffer = buffer[:n]

	info := &FileInfo{
		Format:   "unknown",
		Metadata: make(map[string]interface{}),
	}

	// Check for binary vs text
	info.IsBinary = ftd.isBinary(buffer)
	info.IsText = !info.IsBinary

	if info.IsBinary {
		// Binary file detection
		info.Format, info.MimeType, info.Confidence = ftd.detectBinaryFormat(buffer)
	} else {
		// Text file detection
		info.Format, info.MimeType, info.Confidence = ftd.detectTextFormat(filePath, buffer)
		info.Encoding = ftd.detectEncoding(buffer)
	}

	return info, nil
}

// isBinary checks if content appears to be binary
func (ftd *FileTypeDetector) isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Check for null bytes (common in binary files)
	nullCount := 0
	for _, b := range data {
		if b == 0 {
			nullCount++
		}
	}

	// If more than 1% null bytes, likely binary
	if float64(nullCount)/float64(len(data)) > 0.01 {
		return true
	}

	// Check for high percentage of non-printable characters
	nonPrintable := 0
	for _, b := range data {
		if b < 32 && b != 9 && b != 10 && b != 13 { // Not tab, LF, or CR
			nonPrintable++
		}
	}

	// If more than 30% non-printable, likely binary
	return float64(nonPrintable)/float64(len(data)) > 0.30
}

// detectBinaryFormat detects binary file formats
func (ftd *FileTypeDetector) detectBinaryFormat(data []byte) (string, string, float64) {
	if len(data) < 4 {
		return "binary", "application/octet-stream", 0.5
	}

	// PDF magic number
	if len(data) >= 4 && string(data[:4]) == "%PDF" {
		return "pdf", "application/pdf", 0.95
	}

	// PNG magic number
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "png", "image/png", 0.95
	}

	// JPEG magic number
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg", "image/jpeg", 0.95
	}

	// ZIP magic number (also used by DOCX, XLSX, etc.)
	if len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B && (data[2] == 0x03 || data[2] == 0x05) {
		return "zip", "application/zip", 0.90
	}

	return "binary", "application/octet-stream", 0.5
}

// detectTextFormat detects text file formats
func (ftd *FileTypeDetector) detectTextFormat(filePath string, data []byte) (string, string, float64) {
	content := string(data)

	// JSON detection
	if ftd.looksLikeJSON(content) {
		return "json", "application/json", 0.90
	}

	// CSV detection
	if ftd.looksLikeCSV(content) {
		return "csv", "text/csv", 0.85
	}

	// Markdown detection
	if ftd.looksLikeMarkdown(content) {
		return "markdown", "text/markdown", 0.80
	}

	// XML/HTML detection
	if ftd.looksLikeXML(content) {
		if strings.Contains(strings.ToLower(content), "<html") {
			return "html", "text/html", 0.85
		}
		return "xml", "application/xml", 0.80
	}

	// Code file detection based on extension and content
	if format := ftd.detectCodeFormat(filePath, content); format != "unknown" {
		return format, "text/plain", 0.75
	}

	// Default to text
	return "txt", "text/plain", 0.60
}

// looksLikeJSON checks if content appears to be JSON
func (ftd *FileTypeDetector) looksLikeJSON(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < 2 {
		return false
	}

	// JSON starts with { or [
	return (trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}') ||
		   (trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']')
}

// looksLikeCSV checks if content appears to be CSV
func (ftd *FileTypeDetector) looksLikeCSV(content string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return false
	}

	// Check first few lines for comma-separated values
	commaCount := 0
	for i, line := range lines {
		if i >= 3 { // Only check first 3 lines
			break
		}
		// Count commas, but handle quoted fields
		if ftd.hasCSVStructure(line) {
			commaCount++
		}
	}

	return commaCount >= 2
}

// hasCSVStructure checks if a line has CSV-like structure
func (ftd *FileTypeDetector) hasCSVStructure(line string) bool {
	// Simple check for commas, accounting for quoted fields
	if !strings.Contains(line, ",") {
		return false
	}
	
	// Count fields by splitting on commas (simplified)
	fields := strings.Split(line, ",")
	return len(fields) >= 2
}

// looksLikeMarkdown checks if content appears to be Markdown
func (ftd *FileTypeDetector) looksLikeMarkdown(content string) bool {
	// Look for common Markdown patterns
	patterns := []string{
		"# ", "## ", "### ", // Headers
		"* ", "- ", "+ ",    // Lists
		"```", "~~~",         // Code blocks
		"[]", "()",           // Links
		"**", "__", "*", "_", // Emphasis
	}

	matchCount := 0
	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			matchCount++
		}
	}

	return matchCount >= 2
}

// looksLikeXML checks if content appears to be XML/HTML
func (ftd *FileTypeDetector) looksLikeXML(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">")
}

// detectCodeFormat detects programming language based on file extension and content
func (ftd *FileTypeDetector) detectCodeFormat(filePath, content string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	codeExtensions := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".java": "java",
		".cpp":  "cpp",
		".c":    "c",
		".rs":   "rust",
		".rb":   "ruby",
		".php":  "php",
		".sh":   "shell",
		".sql":  "sql",
		".yaml": "yaml",
		".yml":  "yaml",
		".toml": "toml",
		".ini":  "ini",
	}

	if format, exists := codeExtensions[ext]; exists {
		return format
	}

	return "unknown"
}

// detectEncoding detects text encoding (simplified)
func (ftd *FileTypeDetector) detectEncoding(data []byte) string {
	// Check for BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return "utf-8-bom"
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return "utf-16le"
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return "utf-16be"
	}

	// Default to UTF-8 for text files
	return "utf-8"
}

// RouterConfig configures the file routing behavior
type RouterConfig struct {
	MaxFileSize          int64             `json:"max_file_size"`
	AllowedExtensions    []string          `json:"allowed_extensions"`
	BlockedExtensions    []string          `json:"blocked_extensions"`
	CustomParsers        map[string]Parser `json:"-"`
	FallbackToText       bool              `json:"fallback_to_text"`
	RequireConfidence    float64           `json:"require_confidence"`
}

// DefaultRouterConfig returns sensible defaults for file routing
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		MaxFileSize:       100 * 1024 * 1024, // 100MB
		AllowedExtensions: []string{}, // Empty means all allowed
		BlockedExtensions: []string{".exe", ".dll", ".so", ".dylib"},
		CustomParsers:     make(map[string]Parser),
		FallbackToText:    true,
		RequireConfidence: 0.5,
	}
}

// FileRouter handles intelligent routing of files to appropriate parsers
type FileRouter struct {
	config   *RouterConfig
	detector *FileTypeDetector
	parsers  map[string]Parser
}

// NewFileRouter creates a new file router
func NewFileRouter(config *RouterConfig, chunkingConfig *ChunkingConfig) *FileRouter {
	if config == nil {
		config = DefaultRouterConfig()
	}

	router := &FileRouter{
		config:   config,
		detector: NewFileTypeDetector(chunkingConfig),
		parsers:  make(map[string]Parser),
	}

	// Register default parsers
	router.RegisterParser("txt", NewFormatParser(chunkingConfig))
	router.RegisterParser("csv", NewFormatParser(chunkingConfig))
	router.RegisterParser("json", NewFormatParser(chunkingConfig))
	router.RegisterParser("markdown", NewTextParser(chunkingConfig))
	router.RegisterParser("pdf", NewPDFParser(chunkingConfig))

	// Register custom parsers
	for format, parser := range config.CustomParsers {
		router.RegisterParser(format, parser)
	}

	return router
}

// RegisterParser registers a parser for a specific format
func (fr *FileRouter) RegisterParser(format string, parser Parser) {
	fr.parsers[format] = parser
}

// RouteFile determines the appropriate parser and parses the file
func (fr *FileRouter) RouteFile(ctx context.Context, filePath string) ([]Chunk, error) {
	// Check file size
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.Size() > fr.config.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d)", stat.Size(), fr.config.MaxFileSize)
	}

	// Check extension restrictions
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// Check blocked extensions
	for _, blocked := range fr.config.BlockedExtensions {
		if ext == blocked {
			return nil, fmt.Errorf("file extension %s is blocked", ext)
		}
	}

	// Check allowed extensions (if specified)
	if len(fr.config.AllowedExtensions) > 0 {
		allowed := false
		for _, allowedExt := range fr.config.AllowedExtensions {
			if ext == allowedExt {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("file extension %s is not allowed", ext)
		}
	}

	// Detect file type
	fileInfo, err := fr.detector.DetectFileInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}

	// Check confidence threshold
	if fileInfo.Confidence < fr.config.RequireConfidence {
		if !fr.config.FallbackToText {
			return nil, fmt.Errorf("file type detection confidence too low: %.2f (required: %.2f)", 
				fileInfo.Confidence, fr.config.RequireConfidence)
		}
		fileInfo.Format = "txt" // Fallback to text
	}

	// Route to appropriate parser
	parser, exists := fr.parsers[fileInfo.Format]
	if !exists {
		if fr.config.FallbackToText {
			parser = fr.parsers["txt"]
		} else {
			return nil, fmt.Errorf("no parser available for format: %s", fileInfo.Format)
		}
	}

	// Parse the file
	chunks, err := fr.parseWithParser(ctx, parser, filePath, fileInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file with %s parser: %w", fileInfo.Format, err)
	}

	// Add detection metadata to chunks
	for i := range chunks {
		chunks[i].Metadata["detected_format"] = fileInfo.Format
		chunks[i].Metadata["detection_confidence"] = fileInfo.Confidence
		chunks[i].Metadata["mime_type"] = fileInfo.MimeType
		chunks[i].Metadata["encoding"] = fileInfo.Encoding
		chunks[i].Metadata["is_binary"] = fileInfo.IsBinary
	}

	return chunks, nil
}

// parseWithParser calls the appropriate parser method based on format
func (fr *FileRouter) parseWithParser(ctx context.Context, parser Parser, filePath string, fileInfo *FileInfo) ([]Chunk, error) {
	switch fileInfo.Format {
	case "pdf":
		if pdfParser, ok := parser.(*PDFParser); ok {
			return pdfParser.ParsePDF(ctx, filePath)
		}
	case "csv":
		if formatParser, ok := parser.(*FormatParser); ok {
			return formatParser.ParseCSV(ctx, filePath)
		}
	case "json":
		if formatParser, ok := parser.(*FormatParser); ok {
			return formatParser.ParseJSON(ctx, filePath)
		}
	case "txt":
		if formatParser, ok := parser.(*FormatParser); ok {
			return formatParser.ParseTXT(ctx, filePath)
		}
	case "markdown":
		if textParser, ok := parser.(*TextParser); ok {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			return textParser.ParseMarkdown(ctx, string(content))
		}
	}

	// Fallback to ParseFile method
	return parser.ParseFile(ctx, filePath)
}

// GetSupportedFormats returns all supported formats
func (fr *FileRouter) GetSupportedFormats() []string {
	formats := make([]string, 0, len(fr.parsers))
	for format := range fr.parsers {
		formats = append(formats, format)
	}
	return formats
}

// GetFileInfo returns detailed information about a file without parsing it
func (fr *FileRouter) GetFileInfo(filePath string) (*FileInfo, error) {
	return fr.detector.DetectFileInfo(filePath)
}