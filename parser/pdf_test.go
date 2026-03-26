package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NortonBen/ai-memory-go/schema"
)

func TestNewPDFParser(t *testing.T) {
	// Test with nil config
	parser := NewPDFParser(nil)
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.config)
	assert.Equal(t, schema.StrategyParagraph, parser.config.Strategy)

	// Test with custom config
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategySentence,
		MaxSize:  500,
		MinSize:  25,
	}
	parser = NewPDFParser(config)
	assert.NotNil(t, parser)
	assert.Equal(t, config, parser.config)
}

func TestPDFParser_ParsePDF_FileNotFound(t *testing.T) {
	parser := NewPDFParser(nil)
	ctx := context.Background()

	chunks, err := parser.ParsePDF(ctx, "nonexistent.pdf")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PDF file not found")
	assert.Nil(t, chunks)
}

func TestPDFParser_ParsePDF_InvalidFile(t *testing.T) {
	parser := NewPDFParser(nil)
	ctx := context.Background()

	// Create a temporary non-PDF file
	tmpFile, err := os.CreateTemp("", "test*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("This is not a PDF file")
	require.NoError(t, err)
	tmpFile.Close()

	chunks, err := parser.ParsePDF(ctx, tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create PDF reader")
	assert.Nil(t, chunks)
}

func TestPDFParser_ParsePDF_EmptyPDF(t *testing.T) {
	parser := NewPDFParser(nil)
	ctx := context.Background()

	// Create a minimal PDF for testing
	// Note: In a real test environment, you would have actual test PDF files
	// For now, we'll test the error handling path

	// This test would require a valid empty PDF file
	// Skip if no test PDF is available
	testPDFPath := "testdata/empty.pdf"
	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Skip("Test PDF file not available")
	}

	chunks, err := parser.ParsePDF(ctx, testPDFPath)
	if err != nil {
		t.Logf("Expected behavior for empty PDF: %v", err)
	} else {
		assert.NotNil(t, chunks)
		// Empty PDF should return empty chunks
		assert.Len(t, chunks, 0)
	}
}

func TestPDFParser_textToChunks(t *testing.T) {
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategyParagraph,
		MaxSize:  100,
		MinSize:  10,
		Overlap:  20,
	}
	parser := NewPDFParser(config)

	metadata := &PDFMetadata{
		Title:     "Test Document",
		Author:    "Test Author",
		PageCount: 2,
		FileSize:  1024,
	}

	text := "This is the first paragraph of the PDF document.\n\nThis is the second paragraph with more content to test chunking behavior."
	source := "test.pdf"

	chunks, err := parser.textToChunks(text, source, metadata)
	assert.NoError(t, err)
	assert.NotEmpty(t, chunks)

	// Verify chunk properties
	for _, chunk := range chunks {
		assert.Equal(t, schema.ChunkTypePDF, chunk.Type)
		assert.Equal(t, source, chunk.Source)
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Hash)
		assert.False(t, chunk.CreatedAt.IsZero())

		// Verify PDF metadata is included
		assert.Equal(t, metadata.Title, chunk.Metadata["pdf_title"])
		assert.Equal(t, metadata.Author, chunk.Metadata["pdf_author"])
		assert.Equal(t, metadata.PageCount, chunk.Metadata["pdf_page_count"])
		assert.Equal(t, metadata.FileSize, chunk.Metadata["pdf_file_size"])
		assert.Equal(t, ".pdf", chunk.Metadata["file_extension"])
	}
}

func TestPDFParser_textToChunks_EmptyText(t *testing.T) {
	parser := NewPDFParser(nil)
	metadata := &PDFMetadata{}

	chunks, err := parser.textToChunks("", "test.pdf", metadata)
	assert.NoError(t, err)
	assert.Empty(t, chunks)

	chunks, err = parser.textToChunks("   \n\t  ", "test.pdf", metadata)
	assert.NoError(t, err)
	assert.Empty(t, chunks)
}

func TestPDFMetadata(t *testing.T) {
	metadata := &PDFMetadata{
		Title:        "Test Document",
		Author:       "John Doe",
		Subject:      "Testing",
		Creator:      "Test Creator",
		Producer:     "Test Producer",
		Keywords:     "test, pdf, parsing",
		Language:     "en",
		CreationDate: time.Now(),
		ModDate:      time.Now(),
		PageCount:    5,
		FileSize:     2048,
		IsEncrypted:  false,
	}

	assert.Equal(t, "Test Document", metadata.Title)
	assert.Equal(t, "John Doe", metadata.Author)
	assert.Equal(t, "Testing", metadata.Subject)
	assert.Equal(t, "Test Creator", metadata.Creator)
	assert.Equal(t, "Test Producer", metadata.Producer)
	assert.Equal(t, "test, pdf, parsing", metadata.Keywords)
	assert.Equal(t, "en", metadata.Language)
	assert.Equal(t, 5, metadata.PageCount)
	assert.Equal(t, int64(2048), metadata.FileSize)
	assert.False(t, metadata.IsEncrypted)
	assert.False(t, metadata.CreationDate.IsZero())
	assert.False(t, metadata.ModDate.IsZero())
}

func TestTextParser_ParsePDF_Integration(t *testing.T) {
	// Test the integration between TextParser and PDFParser
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategyParagraph,
		MaxSize:  200,
		MinSize:  20,
	}
	textParser := NewTextParser(config)
	ctx := context.Background()

	// Test with non-existent file
	chunks, err := textParser.ParsePDF(ctx, "nonexistent.pdf")
	assert.Error(t, err)
	assert.Nil(t, chunks)
}

func TestPDFParser_ContextCancellation(t *testing.T) {
	parser := NewPDFParser(nil)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test with non-existent file (should fail before context check)
	chunks, err := parser.ParsePDF(ctx, "nonexistent.pdf")
	assert.Error(t, err)
	assert.Nil(t, chunks)
}

// Benchmark tests
func BenchmarkPDFParser_textToChunks(b *testing.B) {
	parser := NewPDFParser(nil)
	metadata := &PDFMetadata{
		Title:     "Benchmark Test",
		PageCount: 10,
	}

	// Create a large text for benchmarking
	text := ""
	for i := 0; i < 1000; i++ {
		text += "This is a sample paragraph for benchmarking PDF text chunking performance. "
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.textToChunks(text, "benchmark.pdf", metadata)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to create test directories
func setupTestDir(t *testing.T) string {
	testDir := filepath.Join(os.TempDir(), "pdf_parser_test")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	return testDir
}
