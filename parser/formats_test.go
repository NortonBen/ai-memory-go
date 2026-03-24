package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatParser_ParseTXT(t *testing.T) {
	// Create temporary TXT file
	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "test.txt")
	
	content := "This is a test file.\n\nIt has multiple paragraphs.\n\nEach paragraph should become a chunk."
	err := os.WriteFile(txtFile, []byte(content), 0644)
	require.NoError(t, err)
	
	// Parse the file
	parser := NewFormatParser(DefaultChunkingConfig())
	chunks, err := parser.ParseTXT(context.Background(), txtFile)
	
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
	
	// Check metadata
	for _, chunk := range chunks {
		assert.Equal(t, txtFile, chunk.Source)
		assert.Equal(t, "txt", chunk.Metadata["file_type"])
		assert.Equal(t, "test.txt", chunk.Metadata["file_name"])
		assert.NotEmpty(t, chunk.Content)
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Hash)
	}
}

func TestFormatParser_ParseCSV(t *testing.T) {
	// Create temporary CSV file
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	
	content := "Name,Age,City\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago"
	err := os.WriteFile(csvFile, []byte(content), 0644)
	require.NoError(t, err)
	
	// Parse the file
	parser := NewFormatParser(DefaultChunkingConfig())
	chunks, err := parser.ParseCSV(context.Background(), csvFile)
	
	require.NoError(t, err)
	assert.Len(t, chunks, 3) // 3 data rows
	
	// Check first chunk
	firstChunk := chunks[0]
	assert.Equal(t, csvFile, firstChunk.Source)
	assert.Equal(t, "csv", firstChunk.Metadata["file_type"])
	assert.Equal(t, 2, firstChunk.Metadata["row_number"]) // First data row (after header)
	assert.Contains(t, firstChunk.Content, "Name: John")
	assert.Contains(t, firstChunk.Content, "Age: 25")
	assert.Contains(t, firstChunk.Content, "City: New York")
	
	// Check row data
	rowData, ok := firstChunk.Metadata["row_data"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "John", rowData["Name"])
	assert.Equal(t, "25", rowData["Age"])
	assert.Equal(t, "New York", rowData["City"])
}

func TestFormatParser_ParseJSON_Array(t *testing.T) {
	// Create temporary JSON file with array
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")
	
	content := `[
		{"name": "John", "age": 25},
		{"name": "Jane", "age": 30},
		{"name": "Bob", "age": 35}
	]`
	err := os.WriteFile(jsonFile, []byte(content), 0644)
	require.NoError(t, err)
	
	// Parse the file
	parser := NewFormatParser(DefaultChunkingConfig())
	chunks, err := parser.ParseJSON(context.Background(), jsonFile)
	
	require.NoError(t, err)
	assert.Len(t, chunks, 3) // 3 array items
	
	// Check first chunk
	firstChunk := chunks[0]
	assert.Equal(t, jsonFile, firstChunk.Source)
	assert.Equal(t, "json", firstChunk.Metadata["file_type"])
	assert.Equal(t, "array", firstChunk.Metadata["json_type"])
	assert.Equal(t, 0, firstChunk.Metadata["array_index"])
	assert.Contains(t, firstChunk.Content, "John")
	assert.Contains(t, firstChunk.Content, "25")
}

func TestFormatParser_ParseJSON_Object(t *testing.T) {
	// Create temporary JSON file with object
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")
	
	content := `{
		"users": [{"name": "John"}, {"name": "Jane"}],
		"settings": {"theme": "dark", "language": "en"},
		"version": "1.0.0"
	}`
	err := os.WriteFile(jsonFile, []byte(content), 0644)
	require.NoError(t, err)
	
	// Parse the file
	parser := NewFormatParser(DefaultChunkingConfig())
	chunks, err := parser.ParseJSON(context.Background(), jsonFile)
	
	require.NoError(t, err)
	assert.Len(t, chunks, 3) // 3 top-level properties
	
	// Check that we have chunks for each property
	propertyNames := make([]string, len(chunks))
	for i, chunk := range chunks {
		propertyNames[i] = chunk.Metadata["property_name"].(string)
		assert.Equal(t, jsonFile, chunk.Source)
		assert.Equal(t, "json", chunk.Metadata["file_type"])
		assert.Equal(t, "object", chunk.Metadata["json_type"])
	}
	
	assert.Contains(t, propertyNames, "users")
	assert.Contains(t, propertyNames, "settings")
	assert.Contains(t, propertyNames, "version")
}

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
	
	parser := NewUnifiedParser(DefaultChunkingConfig())
	
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

func TestDetectFileFormat(t *testing.T) {
	testCases := []struct {
		filename string
		expected string
	}{
		{"test.txt", "txt"},
		{"test.text", "txt"},
		{"test.csv", "csv"},
		{"test.json", "json"},
		{"test.md", "markdown"},
		{"test.markdown", "markdown"},
		{"test.pdf", "pdf"},
		{"test.unknown", "unknown"},
		{"test", "unknown"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := DetectFileFormat(tc.filename)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUnifiedParser_GetSupportedFormats(t *testing.T) {
	parser := NewUnifiedParser(DefaultChunkingConfig())
	formats := parser.GetSupportedFormats()
	
	expectedFormats := []string{"txt", "text", "csv", "json", "md", "markdown", "pdf"}
	assert.ElementsMatch(t, expectedFormats, formats)
}

func TestUnifiedParser_IsFormatSupported(t *testing.T) {
	parser := NewUnifiedParser(DefaultChunkingConfig())
	
	// Supported formats
	assert.True(t, parser.IsFormatSupported("test.txt"))
	assert.True(t, parser.IsFormatSupported("test.csv"))
	assert.True(t, parser.IsFormatSupported("test.json"))
	assert.True(t, parser.IsFormatSupported("test.md"))
	assert.True(t, parser.IsFormatSupported("test.pdf"))
	
	// Unknown formats are treated as text (supported)
	assert.True(t, parser.IsFormatSupported("test.unknown"))
}

func TestFormatParser_looksLikeHeaders(t *testing.T) {
	parser := NewFormatParser(DefaultChunkingConfig())
	
	// Should be headers
	assert.True(t, parser.looksLikeHeaders([]string{"Name", "Age", "City"}))
	assert.True(t, parser.looksLikeHeaders([]string{"First Name", "Last Name", "Email"}))
	
	// Should not be headers (numeric data)
	assert.False(t, parser.looksLikeHeaders([]string{"123", "456", "789"}))
	assert.False(t, parser.looksLikeHeaders([]string{"1.5", "2.3", "4.7"}))
	
	// Mixed case
	assert.False(t, parser.looksLikeHeaders([]string{"Name", "123", "City"}))
}

func TestFormatParser_isNumeric(t *testing.T) {
	parser := NewFormatParser(DefaultChunkingConfig())
	
	// Numeric
	assert.True(t, parser.isNumeric("123"))
	assert.True(t, parser.isNumeric("123.45"))
	assert.True(t, parser.isNumeric("-123"))
	assert.True(t, parser.isNumeric("+123.45"))
	
	// Not numeric
	assert.False(t, parser.isNumeric("abc"))
	assert.False(t, parser.isNumeric("123abc"))
	assert.False(t, parser.isNumeric(""))
	assert.False(t, parser.isNumeric("12.34.56"))
}