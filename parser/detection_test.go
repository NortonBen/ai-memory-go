package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileTypeDetector_DetectFileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewFileTypeDetector(DefaultChunkingConfig())

	testCases := []struct {
		name           string
		content        string
		filename       string
		expectedFormat string
		minConfidence  float64
	}{
		{
			name:           "JSON file",
			content:        `{"name": "test", "value": 123}`,
			filename:       "test.json",
			expectedFormat: "json",
			minConfidence:  0.8,
		},
		{
			name:           "CSV file",
			content:        "Name,Age,City\nJohn,25,NYC\nJane,30,LA",
			filename:       "test.csv",
			expectedFormat: "csv",
			minConfidence:  0.7,
		},
		{
			name:           "Markdown file",
			content:        "# Header\n\nThis is **bold** text with a [link](http://example.com).",
			filename:       "test.md",
			expectedFormat: "markdown",
			minConfidence:  0.7,
		},
		{
			name:           "Text file",
			content:        "This is just plain text content without any special formatting.",
			filename:       "test.txt",
			expectedFormat: "txt",
			minConfidence:  0.5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tc.filename)
			err := os.WriteFile(filePath, []byte(tc.content), 0644)
			require.NoError(t, err)

			info, err := detector.DetectFileInfo(filePath)
			require.NoError(t, err)
			
			assert.Equal(t, tc.expectedFormat, info.Format)
			assert.GreaterOrEqual(t, info.Confidence, tc.minConfidence)
			assert.NotEmpty(t, info.Metadata["file_name"])
			assert.Equal(t, int64(len(tc.content)), info.Size)
		})
	}
}

func TestFileTypeDetector_isBinary(t *testing.T) {
	detector := NewFileTypeDetector(DefaultChunkingConfig())

	testCases := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "Text content",
			data:     []byte("This is plain text content"),
			expected: false,
		},
		{
			name:     "Binary with null bytes",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0x00, 0x00},
			expected: true,
		},
		{
			name:     "PDF header",
			data:     []byte("%PDF-1.4\n%âãÏÓ"),
			expected: false, // PDF header is text-like
		},
		{
			name:     "Empty data",
			data:     []byte{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.isBinary(tc.data)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFileTypeDetector_looksLikeJSON(t *testing.T) {
	detector := NewFileTypeDetector(DefaultChunkingConfig())

	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "Valid JSON object",
			content:  `{"name": "test", "value": 123}`,
			expected: true,
		},
		{
			name:     "Valid JSON array",
			content:  `[{"name": "test"}, {"name": "test2"}]`,
			expected: true,
		},
		{
			name:     "Invalid JSON",
			content:  `{name: "test"}`,
			expected: true, // Still looks like JSON structure
		},
		{
			name:     "Plain text",
			content:  "This is just text",
			expected: false,
		},
		{
			name:     "Empty content",
			content:  "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.looksLikeJSON(tc.content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFileTypeDetector_looksLikeCSV(t *testing.T) {
	detector := NewFileTypeDetector(DefaultChunkingConfig())

	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "Valid CSV",
			content:  "Name,Age,City\nJohn,25,NYC\nJane,30,LA",
			expected: true,
		},
		{
			name:     "CSV with quotes",
			content:  "\"Name\",\"Age\",\"City\"\n\"John Doe\",25,\"New York\"",
			expected: true,
		},
		{
			name:     "Single line",
			content:  "Name,Age,City",
			expected: false,
		},
		{
			name:     "No commas",
			content:  "Name Age City\nJohn 25 NYC",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.looksLikeCSV(tc.content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFileTypeDetector_looksLikeMarkdown(t *testing.T) {
	detector := NewFileTypeDetector(DefaultChunkingConfig())

	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "Markdown with headers",
			content:  "# Header 1\n## Header 2\nSome content",
			expected: true,
		},
		{
			name:     "Markdown with lists",
			content:  "* Item 1\n* Item 2\n- Item 3",
			expected: true,
		},
		{
			name:     "Markdown with code blocks",
			content:  "Some text\n```go\nfunc main() {}\n```",
			expected: true,
		},
		{
			name:     "Plain text",
			content:  "This is just plain text without markdown",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.looksLikeMarkdown(tc.content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFileRouter_RouteFile(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultRouterConfig()
	router := NewFileRouter(config, DefaultChunkingConfig())

	testCases := []struct {
		name     string
		content  string
		filename string
		wantErr  bool
	}{
		{
			name:     "JSON file",
			content:  `{"name": "test", "data": [1, 2, 3]}`,
			filename: "test.json",
			wantErr:  false,
		},
		{
			name:     "CSV file",
			content:  "Name,Age\nJohn,25\nJane,30",
			filename: "test.csv",
			wantErr:  false,
		},
		{
			name:     "Text file",
			content:  "This is a plain text file with enough content to meet minimum requirements for chunking.",
			filename: "test.txt",
			wantErr:  false,
		},
		{
			name:     "Blocked extension",
			content:  "binary content",
			filename: "test.exe",
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tc.filename)
			err := os.WriteFile(filePath, []byte(tc.content), 0644)
			require.NoError(t, err)

			chunks, err := router.RouteFile(context.Background(), filePath)
			
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			
			if tc.filename != "test.exe" { // Skip for blocked files
				assert.NotEmpty(t, chunks)
				
				// Check that detection metadata is added
				for _, chunk := range chunks {
					assert.Contains(t, chunk.Metadata, "detected_format")
					assert.Contains(t, chunk.Metadata, "detection_confidence")
					assert.Contains(t, chunk.Metadata, "mime_type")
				}
			}
		})
	}
}

func TestFileRouter_RegisterParser(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewFileRouter(config, DefaultChunkingConfig())

	// Create a custom parser
	customParser := NewTextParser(DefaultChunkingConfig())
	
	// Register it
	router.RegisterParser("custom", customParser)
	
	// Check it's registered
	formats := router.GetSupportedFormats()
	assert.Contains(t, formats, "custom")
}

func TestFileRouter_GetFileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultRouterConfig()
	router := NewFileRouter(config, DefaultChunkingConfig())

	// Create test file
	content := `{"test": "data"}`
	filePath := filepath.Join(tmpDir, "test.json")
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	info, err := router.GetFileInfo(filePath)
	require.NoError(t, err)
	
	assert.Equal(t, "json", info.Format)
	assert.Equal(t, "application/json", info.MimeType)
	assert.True(t, info.IsText)
	assert.False(t, info.IsBinary)
	assert.GreaterOrEqual(t, info.Confidence, 0.8)
}

func TestRouterConfig_Restrictions(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Test with restricted config
	config := &RouterConfig{
		MaxFileSize:       100, // Very small limit
		AllowedExtensions: []string{".txt"},
		BlockedExtensions: []string{".json"},
		FallbackToText:    false,
		RequireConfidence: 0.9, // High confidence required
	}
	
	router := NewFileRouter(config, DefaultChunkingConfig())

	// Test file too large
	largeContent := make([]byte, 200)
	for i := range largeContent {
		largeContent[i] = 'a'
	}
	largeFile := filepath.Join(tmpDir, "large.txt")
	err := os.WriteFile(largeFile, largeContent, 0644)
	require.NoError(t, err)

	_, err = router.RouteFile(context.Background(), largeFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")

	// Test blocked extension
	jsonFile := filepath.Join(tmpDir, "test.json")
	err = os.WriteFile(jsonFile, []byte(`{"test": "data"}`), 0644)
	require.NoError(t, err)

	_, err = router.RouteFile(context.Background(), jsonFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is blocked")

	// Test not allowed extension
	csvFile := filepath.Join(tmpDir, "test.csv")
	err = os.WriteFile(csvFile, []byte("a,b\n1,2"), 0644)
	require.NoError(t, err)

	_, err = router.RouteFile(context.Background(), csvFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not allowed")
}

func TestDetectCodeFormat(t *testing.T) {
	detector := NewFileTypeDetector(DefaultChunkingConfig())

	testCases := []struct {
		filename string
		content  string
		expected string
	}{
		{"test.go", "package main\nfunc main() {}", "go"},
		{"test.py", "def hello():\n    print('hello')", "python"},
		{"test.js", "function hello() { console.log('hello'); }", "javascript"},
		{"test.unknown", "some content", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := detector.detectCodeFormat(tc.filename, tc.content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDetectEncoding(t *testing.T) {
	detector := NewFileTypeDetector(DefaultChunkingConfig())

	testCases := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "UTF-8 BOM",
			data:     []byte{0xEF, 0xBB, 0xBF, 'h', 'e', 'l', 'l', 'o'},
			expected: "utf-8-bom",
		},
		{
			name:     "UTF-16 LE",
			data:     []byte{0xFF, 0xFE, 'h', 0x00, 'e', 0x00},
			expected: "utf-16le",
		},
		{
			name:     "UTF-16 BE",
			data:     []byte{0xFE, 0xFF, 0x00, 'h', 0x00, 'e'},
			expected: "utf-16be",
		},
		{
			name:     "Plain UTF-8",
			data:     []byte("hello world"),
			expected: "utf-8",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.detectEncoding(tc.data)
			assert.Equal(t, tc.expected, result)
		})
	}
}