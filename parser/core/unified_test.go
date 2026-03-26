package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedParser_ParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Test different file types
	testCases := []struct {
		filename string
		content  string
		format   string
	}{
		{"test.txt", "This is a test file with enough content to meet the minimum size requirement for chunking. It should be long enough to create at least one chunk.", "txt"},
		{"test.csv", "Name,Age\nJohn,25", "csv"},
		{"test.json", `{"name": "John", "age": 25}`, "json"},
		{"test.md", "# Header\n\nThis is markdown content with enough text to meet the minimum size requirement for proper chunking.", "markdown"},
	}
	
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	
	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tc.filename)
			err := os.WriteFile(filePath, []byte(tc.content), 0644)
			require.NoError(t, err)
			
			chunks, err := parser.ParseFile(context.Background(), filePath)
			require.NoError(t, err)
			assert.NotEmpty(t, chunks)
			
			// Check that all chunks have proper metadata
			for _, chunk := range chunks {
				assert.Equal(t, filePath, chunk.Source)
				assert.NotEmpty(t, chunk.Content)
				assert.NotEmpty(t, chunk.ID)
				assert.NotEmpty(t, chunk.Hash)
				assert.Equal(t, tc.format, chunk.Metadata["file_type"])
			}
		})
	}
}

func TestUnifiedParser_GetSupportedFormats(t *testing.T) {
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	formatsList := parser.GetSupportedFormats()
	
	expectedFormats := []string{"txt", "text", "csv", "json", "md", "markdown", "pdf"}
	assert.ElementsMatch(t, expectedFormats, formatsList)
}

func TestUnifiedParser_IsFormatSupported(t *testing.T) {
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	
	// Supported formats
	assert.True(t, parser.IsFormatSupported("test.txt"))
	assert.True(t, parser.IsFormatSupported("test.csv"))
	assert.True(t, parser.IsFormatSupported("test.json"))
	assert.True(t, parser.IsFormatSupported("test.md"))
	assert.True(t, parser.IsFormatSupported("test.pdf"))
	
	// Unknown formats are treated as text (supported)
	assert.True(t, parser.IsFormatSupported("test.unknown"))
}

func TestUnifiedParser_PDFIntegration(t *testing.T) {
	parserObj := core.NewUnifiedParser(nil)
	ctx := context.Background()

	// Test PDF format detection
	assert.True(t, parserObj.IsFormatSupported("test.pdf"))
	assert.Contains(t, parserObj.GetSupportedFormats(), "pdf")

	// Test PDF parsing through unified parser (with non-existent file)
	chunks, err := parserObj.ParseFile(ctx, "nonexistent.pdf")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")
	assert.Nil(t, chunks)
}
