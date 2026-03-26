// Package parser - PDF parsing implementation using unipdf
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"

	"github.com/NortonBen/ai-memory-go/schema"
)

// PDFPageInfo contains information about a specific page
type PDFPageInfo struct {
	PageNumber int    `json:"page_number"`
	Text       string `json:"text"`
	WordCount  int    `json:"word_count"`
	CharCount  int    `json:"char_count"`
	IsEmpty    bool   `json:"is_empty"`
}

// PDFParser handles PDF document parsing
type PDFParser struct {
	config *schema.ChunkingConfig
}

// NewPDFParser creates a new PDF parser with the given configuration
func NewPDFParser(config *schema.ChunkingConfig) *PDFParser {
	if config == nil {
		config = schema.DefaultChunkingConfig()
	}
	return &PDFParser{config: config}
}

// PDFMetadata contains extracted PDF document metadata
type PDFMetadata struct {
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Subject      string    `json:"subject"`
	Creator      string    `json:"creator"`
	Producer     string    `json:"producer"`
	CreationDate time.Time `json:"creation_date"`
	ModDate      time.Time `json:"modification_date"`
	PageCount    int       `json:"page_count"`
	FileSize     int64     `json:"file_size"`
	Keywords     string    `json:"keywords"`
	Language     string    `json:"language"`
	IsEncrypted  bool      `json:"is_encrypted"`
}

// ParsePDF extracts text content from a PDF file and converts it to chunks
func (pp *PDFParser) ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file not found: %s", filePath)
	}

	// Open the PDF file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer file.Close()

	// Create PDF reader
	pdfReader, err := model.NewPdfReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}

	// Check if PDF is encrypted
	isEncrypted, err := pdfReader.IsEncrypted()
	if err != nil {
		return nil, fmt.Errorf("failed to check PDF encryption: %w", err)
	}

	if isEncrypted {
		// Try to decrypt with empty password
		auth, err := pdfReader.Decrypt([]byte(""))
		if err != nil || !auth {
			return nil, fmt.Errorf("PDF is encrypted and requires a password")
		}
	}

	// Extract metadata
	metadata, err := pp.extractMetadata(pdfReader, filePath)
	if err != nil {
		// Log warning but continue processing
		fmt.Printf("Warning: failed to extract PDF metadata: %v\n", err)
		numPages, _ := pdfReader.GetNumPages()
		metadata = &PDFMetadata{
			PageCount:   numPages,
			IsEncrypted: isEncrypted,
		}
	} else {
		metadata.IsEncrypted = isEncrypted
	}

	// Extract text content with page information
	var allText strings.Builder
	pageInfos := make([]PDFPageInfo, 0, metadata.PageCount)

	for pageNum := 1; pageNum <= metadata.PageCount; pageNum++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pageText, err := pp.extractPageText(pdfReader, pageNum)
		if err != nil {
			fmt.Printf("Warning: failed to extract text from page %d: %v\n", pageNum, err)
			// Create empty page info for failed pages
			pageInfos = append(pageInfos, PDFPageInfo{
				PageNumber: pageNum,
				Text:       "",
				WordCount:  0,
				CharCount:  0,
				IsEmpty:    true,
			})
			continue
		}

		// Create page info
		pageInfo := PDFPageInfo{
			PageNumber: pageNum,
			Text:       pageText,
			WordCount:  len(strings.Fields(pageText)),
			CharCount:  len(pageText),
			IsEmpty:    strings.TrimSpace(pageText) == "",
		}
		pageInfos = append(pageInfos, pageInfo)

		if !pageInfo.IsEmpty {
			allText.WriteString(pageText)
			allText.WriteString("\n\n")
		}
	}

	// Convert extracted text to chunks with page information
	chunks, err := pp.textToChunksWithPages(allText.String(), filePath, metadata, pageInfos)
	if err != nil {
		return nil, fmt.Errorf("failed to convert PDF text to chunks: %w", err)
	}

	return chunks, nil
}

// extractMetadata extracts metadata from the PDF document
func (pp *PDFParser) extractMetadata(pdfReader *model.PdfReader, filePath string) (*PDFMetadata, error) {
	numPages, _ := pdfReader.GetNumPages()
	metadata := &PDFMetadata{
		PageCount: numPages,
	}

	// Get file size
	if fileInfo, err := os.Stat(filePath); err == nil {
		metadata.FileSize = fileInfo.Size()
	}

	// Extract document info
	pdfInfo, err := pdfReader.GetPdfInfo()
	if err != nil {
		return metadata, err
	}

	if pdfInfo.Title != nil {
		metadata.Title = pdfInfo.Title.String()
	}
	if pdfInfo.Author != nil {
		metadata.Author = pdfInfo.Author.String()
	}
	if pdfInfo.Subject != nil {
		metadata.Subject = pdfInfo.Subject.String()
	}
	if pdfInfo.Creator != nil {
		metadata.Creator = pdfInfo.Creator.String()
	}
	if pdfInfo.Producer != nil {
		metadata.Producer = pdfInfo.Producer.String()
	}
	if pdfInfo.Keywords != nil {
		metadata.Keywords = pdfInfo.Keywords.String()
	}

	// Parse dates
	if pdfInfo.CreationDate != nil {
		creationDate := pdfInfo.CreationDate.ToGoTime()
		metadata.CreationDate = creationDate
	}
	// Note: ModDate might not be available in this version of unipdf
	// We'll try to access it safely
	if modDate := pp.getModificationDate(pdfInfo); !modDate.IsZero() {
		metadata.ModDate = modDate
	}

	return metadata, nil
}

// extractPageText extracts text content from a specific page
func (pp *PDFParser) extractPageText(pdfReader *model.PdfReader, pageNum int) (string, error) {
	page, err := pdfReader.GetPage(pageNum)
	if err != nil {
		return "", fmt.Errorf("failed to get page %d: %w", pageNum, err)
	}

	textExtractor, err := extractor.New(page)
	if err != nil {
		return "", fmt.Errorf("failed to create text extractor for page %d: %w", pageNum, err)
	}

	text, err := textExtractor.ExtractText()
	if err != nil {
		return "", fmt.Errorf("failed to extract text from page %d: %w", pageNum, err)
	}

	// Clean up the extracted text
	text = pp.cleanExtractedText(text)

	return text, nil
}

// cleanExtractedText performs basic cleanup on extracted PDF text
func (pp *PDFParser) cleanExtractedText(text string) string {
	// Remove excessive whitespace while preserving paragraph breaks
	lines := strings.Split(text, "\n")
	var cleanedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		} else if len(cleanedLines) > 0 && cleanedLines[len(cleanedLines)-1] != "" {
			// Preserve paragraph breaks
			cleanedLines = append(cleanedLines, "")
		}
	}

	// Join lines back together
	result := strings.Join(cleanedLines, "\n")

	// Remove multiple consecutive empty lines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	// Remove trailing newlines
	result = strings.TrimRight(result, "\n")

	return result
}

// DetectContentType always returns PDF type for PDFParser
func (pp *PDFParser) DetectContentType(content string) schema.ChunkType {
	return schema.ChunkTypePDF
}

// ParseFile implements the Parser interface for PDF files
func (pp *PDFParser) ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return pp.ParsePDF(ctx, filePath)
}

// ParseText is not applicable for PDFParser as it works with files
func (pp *PDFParser) ParseText(ctx context.Context, content string) ([]*schema.Chunk, error) {
	return nil, fmt.Errorf("ParseText not supported for PDFParser, use ParseFile instead")
}

// ParseMarkdown is not applicable for PDFParser
func (pp *PDFParser) ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error) {
	return nil, fmt.Errorf("ParseMarkdown not supported for PDFParser, use ParseFile instead")
}

// textToChunks converts extracted PDF text into structured chunks
func (pp *PDFParser) textToChunks(text, source string, metadata *PDFMetadata) ([]*schema.Chunk, error) {
	return pp.textToChunksWithPages(text, source, metadata, nil)
}

// textToChunksWithPages converts extracted PDF text into structured chunks with page information
func (pp *PDFParser) textToChunksWithPages(text, source string, metadata *PDFMetadata, pageInfos []PDFPageInfo) ([]*schema.Chunk, error) {
	if strings.TrimSpace(text) == "" {
		return []*schema.Chunk{}, nil
	}

	// Create a text parser to handle the chunking
	textParser := NewTextParser(pp.config)
	chunks, err := textParser.ParseText(context.Background(), text)
	if err != nil {
		return nil, err
	}

	// Calculate total statistics
	totalWords := 0
	totalChars := 0
	nonEmptyPages := 0
	for _, pageInfo := range pageInfos {
		totalWords += pageInfo.WordCount
		totalChars += pageInfo.CharCount
		if !pageInfo.IsEmpty {
			nonEmptyPages++
		}
	}

	// Enhance chunks with PDF-specific metadata
	for i := range chunks {
		chunks[i].Type = schema.ChunkTypePDF
		chunks[i].Source = source

		// Add PDF metadata to chunk metadata
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = make(map[string]interface{})
		}

		chunks[i].Metadata["pdf_title"] = metadata.Title
		chunks[i].Metadata["pdf_author"] = metadata.Author
		chunks[i].Metadata["pdf_subject"] = metadata.Subject
		chunks[i].Metadata["pdf_creator"] = metadata.Creator
		chunks[i].Metadata["pdf_producer"] = metadata.Producer
		chunks[i].Metadata["pdf_keywords"] = metadata.Keywords
		chunks[i].Metadata["pdf_language"] = metadata.Language
		chunks[i].Metadata["pdf_page_count"] = metadata.PageCount
		chunks[i].Metadata["pdf_file_size"] = metadata.FileSize
		chunks[i].Metadata["pdf_is_encrypted"] = metadata.IsEncrypted
		chunks[i].Metadata["file_extension"] = filepath.Ext(source)

		// Add document statistics
		chunks[i].Metadata["pdf_total_words"] = totalWords
		chunks[i].Metadata["pdf_total_chars"] = totalChars
		chunks[i].Metadata["pdf_non_empty_pages"] = nonEmptyPages

		if !metadata.CreationDate.IsZero() {
			chunks[i].Metadata["pdf_creation_date"] = metadata.CreationDate
		}
		if !metadata.ModDate.IsZero() {
			chunks[i].Metadata["pdf_modification_date"] = metadata.ModDate
		}
	}

	return chunks, nil
}

// getModificationDate safely extracts modification date from PDF info
func (pp *PDFParser) getModificationDate(pdfInfo *model.PdfInfo) time.Time {
	// Try to access ModDate field using reflection or other safe methods
	// For now, return zero time if not available
	// This can be enhanced when the field becomes available in the library
	return time.Time{}
}
