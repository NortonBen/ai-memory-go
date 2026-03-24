package parser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnifiedParser_PDFIntegration(t *testing.T) {
	parser := NewUnifiedParser(nil)
	ctx := context.Background()

	// Test PDF format detection
	assert.True(t, parser.IsFormatSupported("test.pdf"))
	assert.Contains(t, parser.GetSupportedFormats(), "pdf")

	// Test PDF parsing through unified parser (with non-existent file)
	chunks, err := parser.ParseFile(ctx, "nonexistent.pdf")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")
	assert.Nil(t, chunks)
}

func TestPDFParser_InterfaceCompliance(t *testing.T) {
	// Verify PDFParser implements Parser interface
	var _ Parser = (*PDFParser)(nil)

	parser := NewPDFParser(nil)
	ctx := context.Background()

	// Test all interface methods
	_, err := parser.ParseFile(ctx, "test.pdf")
	assert.Error(t, err) // Expected since file doesn't exist

	_, err = parser.ParseText(ctx, "some text")
	assert.Error(t, err) // Expected since PDFParser doesn't support text parsing
	assert.Contains(t, err.Error(), "ParseText not supported")

	_, err = parser.ParseMarkdown(ctx, "# Header")
	assert.Error(t, err) // Expected since PDFParser doesn't support markdown parsing
	assert.Contains(t, err.Error(), "ParseMarkdown not supported")

	_, err = parser.ParsePDF(ctx, "test.pdf")
	assert.Error(t, err) // Expected since file doesn't exist

	contentType := parser.DetectContentType("any content")
	assert.Equal(t, ChunkTypePDF, contentType)
}

func TestPDFParser_ErrorHandling(t *testing.T) {
	parser := NewPDFParser(nil)
	ctx := context.Background()

	// Test various error conditions
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "Non-existent file",
			filePath: "does_not_exist.pdf",
			wantErr:  "PDF file not found",
		},
		{
			name:     "Empty path",
			filePath: "",
			wantErr:  "PDF file not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunks, err := parser.ParsePDF(ctx, tc.filePath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Nil(t, chunks)
		})
	}
}

func TestPDFMetadata_Completeness(t *testing.T) {
	metadata := &PDFMetadata{
		Title:       "Test Document",
		Author:      "Test Author",
		Subject:     "Test Subject",
		Creator:     "Test Creator",
		Producer:    "Test Producer",
		Keywords:    "test, keywords",
		Language:    "en",
		PageCount:   10,
		FileSize:    2048,
		IsEncrypted: false,
	}

	// Verify all fields are accessible
	assert.Equal(t, "Test Document", metadata.Title)
	assert.Equal(t, "Test Author", metadata.Author)
	assert.Equal(t, "Test Subject", metadata.Subject)
	assert.Equal(t, "Test Creator", metadata.Creator)
	assert.Equal(t, "Test Producer", metadata.Producer)
	assert.Equal(t, "test, keywords", metadata.Keywords)
	assert.Equal(t, "en", metadata.Language)
	assert.Equal(t, 10, metadata.PageCount)
	assert.Equal(t, int64(2048), metadata.FileSize)
	assert.False(t, metadata.IsEncrypted)
}

func TestPDFParser_ConfigurationHandling(t *testing.T) {
	// Test with nil config
	parser1 := NewPDFParser(nil)
	assert.NotNil(t, parser1.config)
	assert.Equal(t, StrategyParagraph, parser1.config.Strategy)

	// Test with custom config
	customConfig := &ChunkingConfig{
		Strategy: StrategySentence,
		MaxSize:  500,
		MinSize:  25,
		Overlap:  50,
	}
	parser2 := NewPDFParser(customConfig)
	assert.Equal(t, customConfig, parser2.config)
	assert.Equal(t, StrategySentence, parser2.config.Strategy)
	assert.Equal(t, 500, parser2.config.MaxSize)
}

func TestPDFPageInfo(t *testing.T) {
	pageInfo := PDFPageInfo{
		PageNumber: 1,
		Text:       "This is a test page with some content.",
		WordCount:  8,
		CharCount:  39,
		IsEmpty:    false,
	}

	assert.Equal(t, 1, pageInfo.PageNumber)
	assert.Equal(t, "This is a test page with some content.", pageInfo.Text)
	assert.Equal(t, 8, pageInfo.WordCount)
	assert.Equal(t, 39, pageInfo.CharCount)
	assert.False(t, pageInfo.IsEmpty)

	// Test empty page
	emptyPageInfo := PDFPageInfo{
		PageNumber: 2,
		Text:       "",
		WordCount:  0,
		CharCount:  0,
		IsEmpty:    true,
	}

	assert.Equal(t, 2, emptyPageInfo.PageNumber)
	assert.Equal(t, "", emptyPageInfo.Text)
	assert.Equal(t, 0, emptyPageInfo.WordCount)
	assert.Equal(t, 0, emptyPageInfo.CharCount)
	assert.True(t, emptyPageInfo.IsEmpty)
}

func TestPDFParser_cleanExtractedText(t *testing.T) {
	parser := NewPDFParser(nil)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove excessive whitespace",
			input:    "Line 1\n\n\n\nLine 2\n   \n\nLine 3",
			expected: "Line 1\n\nLine 2\n\nLine 3",
		},
		{
			name:     "Preserve paragraph breaks",
			input:    "Paragraph 1\n\nParagraph 2\n\nParagraph 3",
			expected: "Paragraph 1\n\nParagraph 2\n\nParagraph 3",
		},
		{
			name:     "Clean up trailing spaces",
			input:    "Line with spaces   \n  Another line  \n",
			expected: "Line with spaces\nAnother line",
		},
		{
			name:     "Handle empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "Handle only whitespace",
			input:    "   \n\n\t\n   ",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.cleanExtractedText(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
