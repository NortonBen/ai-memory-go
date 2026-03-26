// Package parser - Multi-format parser implementations for TXT, CSV, JSON
package formats

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NortonBen/ai-memory-go/schema"
)

// FormatParser implements parsing for common file formats (TXT, CSV, JSON)
type FormatParser struct {
	config *schema.ChunkingConfig
}

// NewFormatParser creates a new format parser
func NewFormatParser(config *schema.ChunkingConfig) *FormatParser {
	if config == nil {
		config = schema.DefaultChunkingConfig()
	}
	return &FormatParser{config: config}
}

// ParseTXT parses plain text files
func (fp *FormatParser) ParseTXT(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TXT file: %w", err)
	}

	// Create metadata
	metadata := map[string]interface{}{
		"file_path":   filePath,
		"file_name":   filepath.Base(filePath),
		"file_type":   "txt",
		"file_size":   len(content),
		"encoding":    "utf-8",
	}

	// Use text parser for chunking
	textParser := NewTextParser(fp.config)
	chunks, err := textParser.ParseText(ctx, string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse TXT content: %w", err)
	}

	// Add metadata to chunks
	for i := range chunks {
		chunks[i].Source = filePath
		for k, v := range metadata {
			chunks[i].Metadata[k] = v
		}
	}

	return chunks, nil
}

// ParseCSV parses CSV files and converts each row to a chunk
func (fp *FormatParser) ParseCSV(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV records: %w", err)
	}

	if len(records) == 0 {
		return []*schema.Chunk{}, nil
	}

	// Create base metadata
	metadata := map[string]interface{}{
		"file_path":    filePath,
		"file_name":    filepath.Base(filePath),
		"file_type":    "csv",
		"total_rows":   len(records),
		"total_columns": len(records[0]),
	}

	// Use first row as headers if it looks like headers
	var headers []string
	startRow := 0
	if len(records) > 1 && fp.looksLikeHeaders(records[0]) {
		headers = records[0]
		startRow = 1
		metadata["headers"] = headers
	} else {
		// Generate generic headers
		for i := 0; i < len(records[0]); i++ {
			headers = append(headers, fmt.Sprintf("column_%d", i+1))
		}
	}

	chunks := make([]*schema.Chunk, 0)

	// Convert each data row to a chunk
	for i := startRow; i < len(records); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		record := records[i]
		
		// Create structured content for the row
		rowData := make(map[string]string)
		var contentParts []string
		
		for j, value := range record {
			if j < len(headers) {
				rowData[headers[j]] = value
				contentParts = append(contentParts, fmt.Sprintf("%s: %s", headers[j], value))
			}
		}

		content := strings.Join(contentParts, "\n")
		
		chunk := schema.NewChunk(content, filePath, schema.ChunkTypeText)
		
		// Add CSV-specific metadata
		for k, v := range metadata {
			chunk.Metadata[k] = v
		}
		chunk.Metadata["row_number"] = i + 1
		chunk.Metadata["row_data"] = rowData
		chunk.Metadata["chunk_type"] = "csv_row"

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// ParseJSON parses JSON files and converts them to chunks
func (fp *FormatParser) ParseJSON(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse JSON to determine structure
	var jsonData interface{}
	if err := json.Unmarshal(content, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Create base metadata
	metadata := map[string]interface{}{
		"file_path": filePath,
		"file_name": filepath.Base(filePath),
		"file_type": "json",
		"file_size": len(content),
	}

	chunks := make([]*schema.Chunk, 0)

	// Handle different JSON structures
	switch data := jsonData.(type) {
	case []interface{}:
		// JSON array - each element becomes a chunk
		metadata["json_type"] = "array"
		metadata["array_length"] = len(data)
		
		for i, item := range data {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			itemJSON, err := json.MarshalIndent(item, "", "  ")
			if err != nil {
				continue
			}

			chunk := schema.NewChunk(string(itemJSON), filePath, schema.ChunkTypeText)
			
			// Add JSON-specific metadata
			for k, v := range metadata {
				chunk.Metadata[k] = v
			}
			chunk.Metadata["array_index"] = i
			chunk.Metadata["chunk_type"] = "json_array_item"

			chunks = append(chunks, chunk)
		}

	case map[string]interface{}:
		// JSON object - create chunks for top-level properties
		metadata["json_type"] = "object"
		
		// If it's a simple object, create one chunk
		if fp.isSimpleObject(data) {
			content := string(content)
			chunk := schema.NewChunk(content, filePath, schema.ChunkTypeText)
			
			for k, v := range metadata {
				chunk.Metadata[k] = v
			}
			chunk.Metadata["chunk_type"] = "json_object"
			
			chunks = append(chunks, chunk)
		} else {
			// Complex object - chunk by top-level properties
			for key, value := range data {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}

				valueJSON, err := json.MarshalIndent(value, "", "  ")
				if err != nil {
					continue
				}

				content := fmt.Sprintf("{\n  \"%s\": %s\n}", key, string(valueJSON))
				chunk := schema.NewChunk(content, filePath, schema.ChunkTypeText)
				
				// Add JSON-specific metadata
				for k, v := range metadata {
					chunk.Metadata[k] = v
				}
				chunk.Metadata["property_name"] = key
				chunk.Metadata["chunk_type"] = "json_property"

				chunks = append(chunks, chunk)
			}
		}

	default:
		// Simple JSON value - create single chunk
		metadata["json_type"] = "primitive"
		chunk := schema.NewChunk(string(content), filePath, schema.ChunkTypeText)
		
		for k, v := range metadata {
			chunk.Metadata[k] = v
		}
		chunk.Metadata["chunk_type"] = "json_primitive"
		
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// looksLikeHeaders determines if a CSV row looks like headers
func (fp *FormatParser) looksLikeHeaders(row []string) bool {
	if len(row) == 0 {
		return false
	}

	// Check if all fields are non-numeric and contain letters
	for _, field := range row {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		
		// If it's purely numeric, probably not a header
		if fp.isNumeric(field) {
			return false
		}
		
		// Headers usually contain letters
		hasLetter := false
		for _, r := range field {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasLetter = true
				break
			}
		}
		if !hasLetter {
			return false
		}
	}
	
	return true
}

// isNumeric checks if a string is numeric
func (fp *FormatParser) isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	
	dotCount := 0
	for _, r := range s {
		if r == '.' {
			dotCount++
			if dotCount > 1 {
				return false // Multiple dots not allowed
			}
		} else if !((r >= '0' && r <= '9') || r == '-' || r == '+') {
			return false
		}
	}
	return true
}

// isSimpleObject determines if a JSON object is simple enough to be one chunk
func (fp *FormatParser) isSimpleObject(obj map[string]interface{}) bool {
	// Consider it simple if it has few properties and they're mostly primitives
	if len(obj) > 10 {
		return false
	}
	
	complexCount := 0
	for _, value := range obj {
		switch value.(type) {
		case map[string]interface{}, []interface{}:
			complexCount++
		}
	}
	
	// If more than half are complex, split it
	return complexCount <= len(obj)/2
}

// DetectContentType detects the type of content
func (fp *FormatParser) DetectContentType(content string) schema.ChunkType {
	// Simple heuristics for content type detection
	if strings.Contains(content, "```") || strings.Contains(content, "func ") || strings.Contains(content, "class ") {
		return schema.ChunkTypeCode
	}
	if strings.Contains(content, "#") && strings.Contains(content, "\n") {
		return schema.ChunkTypeMarkdown
	}
	return schema.ChunkTypeText
}

// ParseFile parses a file based on its extension
func (fp *FormatParser) ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	format := DetectFileFormat(filePath)
	
	switch format {
	case "txt":
		return fp.ParseTXT(ctx, filePath)
	case "csv":
		return fp.ParseCSV(ctx, filePath)
	case "json":
		return fp.ParseJSON(ctx, filePath)
	default:
		// Try to parse as text file
		return fp.ParseTXT(ctx, filePath)
	}
}

// ParseText parses raw text content into chunks
func (fp *FormatParser) ParseText(ctx context.Context, content string) ([]*schema.Chunk, error) {
	// Use text parser for chunking
	textParser := NewTextParser(fp.config)
	return textParser.ParseText(ctx, content)
}

// ParseMarkdown parses markdown content with structure preservation
func (fp *FormatParser) ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error) {
	// Use text parser for markdown
	textParser := NewTextParser(fp.config)
	return textParser.ParseMarkdown(ctx, content)
}

// ParsePDF parses PDF files (not supported by FormatParser)
func (fp *FormatParser) ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return nil, fmt.Errorf("PDF parsing not supported by FormatParser, use PDFParser instead")
}
func DetectFileFormat(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".txt", ".text":
		return "txt"
	case ".csv":
		return "csv"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	case ".pdf":
		return "pdf"
	default:
		return "unknown"
	}
}