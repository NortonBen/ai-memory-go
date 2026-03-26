// Package parser - Content type detection and metadata extraction
package core

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// ContentTypeDetector provides advanced content type detection
type ContentTypeDetector struct {
	patterns map[schema.ChunkType]*regexp.Regexp
}

// NewContentTypeDetector creates a new content type detector
func NewContentTypeDetector() *ContentTypeDetector {
	return &ContentTypeDetector{
		patterns: map[schema.ChunkType]*regexp.Regexp{
			schema.ChunkTypeCode:     regexp.MustCompile(`(?m)(func|class|def|import|package|public|private|const|var|let|function)\s+\w+`),
			schema.ChunkTypeMarkdown: regexp.MustCompile(`(?m)^#{1,6}\s+.+|^\*\*.*\*\*|\[.*\]\(.*\)`),
			schema.ChunkTypePDF:      regexp.MustCompile(`%PDF-`),
		},
	}
}

// DetectContentType detects the type of content with advanced heuristics
func (ctd *ContentTypeDetector) DetectContentType(content string) schema.ChunkType {
	// Check for code patterns
	if ctd.patterns[schema.ChunkTypeCode].MatchString(content) {
		return schema.ChunkTypeCode
	}

	// Check for markdown patterns
	if ctd.patterns[schema.ChunkTypeMarkdown].MatchString(content) {
		return schema.ChunkTypeMarkdown
	}

	// Check for PDF header
	if ctd.patterns[schema.ChunkTypePDF].MatchString(content) {
		return schema.ChunkTypePDF
	}

	// Default to text
	return schema.ChunkTypeText
}

// DetectContentTypeFromFile detects content type from file extension
func DetectContentTypeFromFile(filePath string) schema.ChunkType {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".md", ".markdown":
		return schema.ChunkTypeMarkdown
	case ".pdf":
		return schema.ChunkTypePDF
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb":
		return schema.ChunkTypeCode
	case ".txt", ".text":
		return schema.ChunkTypeText
	default:
		return schema.ChunkTypeText
	}
}

// MetadataExtractor extracts metadata from content
type MetadataExtractor struct {
	titlePattern   *regexp.Regexp
	authorPattern  *regexp.Regexp
	datePattern    *regexp.Regexp
	keywordPattern *regexp.Regexp
}

// NewMetadataExtractor creates a new metadata extractor
func NewMetadataExtractor() *MetadataExtractor {
	return &MetadataExtractor{
		titlePattern:   regexp.MustCompile(`(?m)^#\s+(.+)$|^Title:\s*(.+)$`),
		authorPattern:  regexp.MustCompile(`(?m)^Author:\s*(.+)$|^By:\s*(.+)$`),
		datePattern:    regexp.MustCompile(`(?m)^Date:\s*(.+)$|(\d{4}-\d{2}-\d{2})`),
		keywordPattern: regexp.MustCompile(`(?m)^Keywords?:\s*(.+)$|^Tags?:\s*(.+)$`),
	}
}

// ExtractMetadata extracts metadata from content
func (me *MetadataExtractor) ExtractMetadata(content string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Extract title
	if matches := me.titlePattern.FindStringSubmatch(content); len(matches) > 0 {
		for _, match := range matches[1:] {
			if match != "" {
				metadata["title"] = strings.TrimSpace(match)
				break
			}
		}
	}

	// Extract author
	if matches := me.authorPattern.FindStringSubmatch(content); len(matches) > 0 {
		for _, match := range matches[1:] {
			if match != "" {
				metadata["author"] = strings.TrimSpace(match)
				break
			}
		}
	}

	// Extract date
	if matches := me.datePattern.FindStringSubmatch(content); len(matches) > 0 {
		for _, match := range matches[1:] {
			if match != "" {
				metadata["date"] = strings.TrimSpace(match)
				break
			}
		}
	}

	// Extract keywords
	if matches := me.keywordPattern.FindStringSubmatch(content); len(matches) > 0 {
		for _, match := range matches[1:] {
			if match != "" {
				keywords := strings.Split(match, ",")
				cleanKeywords := make([]string, 0)
				for _, kw := range keywords {
					cleanKeywords = append(cleanKeywords, strings.TrimSpace(kw))
				}
				metadata["keywords"] = cleanKeywords
				break
			}
		}
	}

	// Add basic statistics
	metadata["word_count"] = len(strings.Fields(content))
	metadata["char_count"] = len(content)
	metadata["line_count"] = strings.Count(content, "\n") + 1
	metadata["extracted_at"] = time.Now().Format(time.RFC3339)

	return metadata
}

// ExtractLanguage detects the language of the content (simplified)
func ExtractLanguage(content string) string {
	// Simple heuristic based on common words
	// In production, use a proper language detection library

	// Check for Vietnamese
	vietnamesePattern := regexp.MustCompile(`(?i)(được|không|của|và|có|này|trong|cho|với|là)`)
	if vietnamesePattern.MatchString(content) {
		return "vi"
	}

	// Check for common English words
	englishPattern := regexp.MustCompile(`(?i)\b(the|is|are|was|were|have|has|had|will|would|can|could)\b`)
	if englishPattern.MatchString(content) {
		return "en"
	}

	// Default to unknown
	return "unknown"
}

// ExtractCodeLanguage detects programming language from code content
func ExtractCodeLanguage(content string) string {
	patterns := map[string]*regexp.Regexp{
		"go":         regexp.MustCompile(`(?m)^package\s+\w+|func\s+\w+\(.*\)|import\s+\(`),
		"python":     regexp.MustCompile(`(?m)^def\s+\w+\(|^class\s+\w+:|^import\s+\w+|^from\s+\w+`),
		"javascript": regexp.MustCompile(`(?m)^function\s+\w+\(|^const\s+\w+\s*=|^let\s+\w+\s*=|^var\s+\w+\s*=`),
		"typescript": regexp.MustCompile(`(?m)^interface\s+\w+|^type\s+\w+\s*=|:\s*\w+\s*=>`),
		"java":       regexp.MustCompile(`(?m)^public\s+class|^private\s+\w+|^protected\s+\w+`),
		"rust":       regexp.MustCompile(`(?m)^fn\s+\w+\(|^impl\s+\w+|^struct\s+\w+`),
		"c":          regexp.MustCompile(`(?m)^#include\s+<|^int\s+main\(|^void\s+\w+\(`),
		"cpp":        regexp.MustCompile(`(?m)^#include\s+<|^class\s+\w+|^namespace\s+\w+`),
	}

	for lang, pattern := range patterns {
		if pattern.MatchString(content) {
			return lang
		}
	}

	return "unknown"
}

// EnrichChunkMetadata adds additional metadata to a chunk
func EnrichChunkMetadata(chunk *schema.Chunk, filePath string) {
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}

	// Add file information if available
	if filePath != "" {
		chunk.Metadata["file_path"] = filePath
		chunk.Metadata["file_name"] = filepath.Base(filePath)
		chunk.Metadata["file_ext"] = filepath.Ext(filePath)
	}

	// Detect language
	if chunk.Type == schema.ChunkTypeCode {
		chunk.Metadata["code_language"] = ExtractCodeLanguage(chunk.Content)
	} else {
		chunk.Metadata["language"] = ExtractLanguage(chunk.Content)
	}

	// Add content statistics
	chunk.Metadata["word_count"] = len(strings.Fields(chunk.Content))
	chunk.Metadata["char_count"] = len(chunk.Content)
	chunk.Metadata["line_count"] = strings.Count(chunk.Content, "\n") + 1

	// Extract metadata from content
	extractor := NewMetadataExtractor()
	contentMetadata := extractor.ExtractMetadata(chunk.Content)
	for k, v := range contentMetadata {
		if _, exists := chunk.Metadata[k]; !exists {
			chunk.Metadata[k] = v
		}
	}
}
