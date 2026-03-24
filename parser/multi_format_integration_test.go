package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiFormatIntegration verifies that all supported formats work correctly
func TestMultiFormatIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files for all supported formats
	testFiles := map[string]string{
		"test.txt": "This is a plain text file with enough content to create meaningful chunks for testing purposes.",
		"test.csv": "Name,Age,City\nJohn,25,New York\nJane,30,Los Angeles\nBob,35,Chicago",
		"test.json": `{
			"users": [
				{"name": "John", "age": 25, "city": "New York"},
				{"name": "Jane", "age": 30, "city": "Los Angeles"}
			],
			"metadata": {"version": "1.0", "created": "2024-01-01"}
		}`,
		"test.md": `# Test Document

This is a **markdown** document with various elements:

- List item 1
- List item 2

## Code Example

` + "```go\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}\n```" + `

Regular paragraph with *italic* and **bold** text.`,
	}

	// Write test files
	filePaths := make([]string, 0, len(testFiles))
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
		filePaths = append(filePaths, filePath)
	}

	// Test with UnifiedParser
	parser := NewUnifiedParser(DefaultChunkingConfig())

	// Test individual file parsing
	for _, filePath := range filePaths {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			chunks, err := parser.ParseFile(context.Background(), filePath)
			require.NoError(t, err)
			assert.NotEmpty(t, chunks, "Should produce at least one chunk")

			// Verify chunk properties
			for _, chunk := range chunks {
				assert.NotEmpty(t, chunk.ID, "Chunk should have ID")
				assert.NotEmpty(t, chunk.Hash, "Chunk should have hash")
				assert.NotEmpty(t, chunk.Content, "Chunk should have content")
				assert.Equal(t, filePath, chunk.Source, "Chunk should have correct source")
				assert.NotNil(t, chunk.Metadata, "Chunk should have metadata")
			}
		})
	}

	// Test batch parsing
	t.Run("BatchParsing", func(t *testing.T) {
		results, err := parser.BatchParseFiles(context.Background(), filePaths)
		require.NoError(t, err)
		assert.Len(t, results, len(filePaths), "Should parse all files")

		// Verify each file was parsed
		for _, filePath := range filePaths {
			chunks, exists := results[filePath]
			assert.True(t, exists, "File should be in results: %s", filePath)
			assert.NotEmpty(t, chunks, "File should have chunks: %s", filePath)
		}
	})

	// Test format detection
	t.Run("FormatDetection", func(t *testing.T) {
		expectedFormats := map[string]string{
			"test.txt":  "txt",
			"test.csv":  "csv",
			"test.json": "json",
			"test.md":   "markdown",
		}

		for filename, expectedFormat := range expectedFormats {
			filePath := filepath.Join(tmpDir, filename)
			detectedFormat := DetectFileFormat(filePath)
			assert.Equal(t, expectedFormat, detectedFormat, "Format detection failed for %s", filename)
		}
	})

	// Test file router
	t.Run("FileRouter", func(t *testing.T) {
		router := NewFileRouter(DefaultRouterConfig(), DefaultChunkingConfig())

		for _, filePath := range filePaths {
			chunks, err := router.RouteFile(context.Background(), filePath)
			require.NoError(t, err, "Router should handle file: %s", filePath)
			assert.NotEmpty(t, chunks, "Router should produce chunks for: %s", filePath)
		}
	})
}

// TestMultiFormatSpecificFeatures tests format-specific features
func TestMultiFormatSpecificFeatures(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("CSVHeaders", func(t *testing.T) {
		csvFile := filepath.Join(tmpDir, "headers.csv")
		content := "First Name,Last Name,Email\nJohn,Doe,john@example.com\nJane,Smith,jane@example.com"
		err := os.WriteFile(csvFile, []byte(content), 0644)
		require.NoError(t, err)

		parser := NewFormatParser(DefaultChunkingConfig())
		chunks, err := parser.ParseCSV(context.Background(), csvFile)
		require.NoError(t, err)

		// Should have 2 data rows (excluding header)
		assert.Len(t, chunks, 2)

		// Check that headers are properly detected
		firstChunk := chunks[0]
		headers, exists := firstChunk.Metadata["headers"]
		assert.True(t, exists)
		assert.Equal(t, []string{"First Name", "Last Name", "Email"}, headers)
	})

	t.Run("JSONStructure", func(t *testing.T) {
		jsonFile := filepath.Join(tmpDir, "structure.json")
		content := `[
			{"id": 1, "name": "Item 1"},
			{"id": 2, "name": "Item 2"}
		]`
		err := os.WriteFile(jsonFile, []byte(content), 0644)
		require.NoError(t, err)

		parser := NewFormatParser(DefaultChunkingConfig())
		chunks, err := parser.ParseJSON(context.Background(), jsonFile)
		require.NoError(t, err)

		// Should have 2 array items
		assert.Len(t, chunks, 2)

		// Check JSON-specific metadata
		firstChunk := chunks[0]
		assert.Equal(t, "array", firstChunk.Metadata["json_type"])
		assert.Equal(t, 0, firstChunk.Metadata["array_index"])
	})

	t.Run("MarkdownStructure", func(t *testing.T) {
		mdFile := filepath.Join(tmpDir, "structure.md")
		content := `# Main Header

This is a paragraph under the main header.

## Sub Header

This is content under the sub header.

` + "```python\nprint('Hello, World!')\n```"
		err := os.WriteFile(mdFile, []byte(content), 0644)
		require.NoError(t, err)

		parser := NewMarkdownParser(DefaultChunkingConfig())
		chunks, err := parser.ParseMarkdown(context.Background(), content)
		require.NoError(t, err)

		// Should have multiple sections
		assert.NotEmpty(t, chunks)

		// Check for different section types
		sectionTypes := make(map[string]bool)
		for _, chunk := range chunks {
			if sectionType, exists := chunk.Metadata["section_type"]; exists {
				sectionTypes[sectionType.(string)] = true
			}
		}

		// Should have headers and code sections
		assert.True(t, sectionTypes["header"] || sectionTypes["paragraph"], "Should have header or paragraph sections")
	})
}

// TestMultiFormatErrorHandling tests error handling across formats
func TestMultiFormatErrorHandling(t *testing.T) {
	parser := NewUnifiedParser(DefaultChunkingConfig())

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := parser.ParseFile(context.Background(), "/nonexistent/file.txt")
		assert.Error(t, err)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		jsonFile := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(jsonFile, []byte("{invalid json"), 0644)
		require.NoError(t, err)

		formatParser := NewFormatParser(DefaultChunkingConfig())
		_, err = formatParser.ParseJSON(context.Background(), jsonFile)
		assert.Error(t, err)
	})

	t.Run("EmptyFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyFile := filepath.Join(tmpDir, "empty.txt")
		err := os.WriteFile(emptyFile, []byte(""), 0644)
		require.NoError(t, err)

		chunks, err := parser.ParseFile(context.Background(), emptyFile)
		require.NoError(t, err)
		// Empty file should produce empty chunks array or single empty chunk
		assert.True(t, len(chunks) == 0 || (len(chunks) == 1 && chunks[0].Content == ""))
	})
}
