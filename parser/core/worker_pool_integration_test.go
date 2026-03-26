package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerPoolIntegration demonstrates the complete worker pool functionality
func TestWorkerPoolIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test directory with various file types
	tempDir := t.TempDir()
	testFiles := createIntegrationTestFiles(t, tempDir)

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// Create parser with custom worker pool configuration
		config := &schema.WorkerPoolConfig{
			NumWorkers:    4,
			QueueSize:     20,
			Timeout:       10 * time.Second,
			RetryAttempts: 2,
			RetryDelay:    100 * time.Millisecond,
		}

		parser := core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
		defer parser.Close()

		// Start worker pool
		require.NoError(t, parser.StartWorkerPool())
		assert.True(t, parser.IsWorkerPoolHealthy())

		// Process files in parallel
		ctx := context.Background()
		startTime := time.Now()
		results, err := parser.BatchParseFiles(ctx, testFiles)
		duration := time.Since(startTime)

		require.NoError(t, err)
		assert.Equal(t, len(testFiles), len(results))

		// Verify all files were processed
		totalChunks := 0
		for filePath, chunks := range results {
			assert.NotEmpty(t, chunks, "No chunks for file %s", filePath)
			totalChunks += len(chunks)

			// Verify chunk properties
			for _, chunk := range chunks {
				assert.NotEmpty(t, chunk.ID)
				assert.NotEmpty(t, chunk.Content)
				assert.NotEmpty(t, chunk.Hash)
				assert.Equal(t, filePath, chunk.Source)
				assert.NotZero(t, chunk.CreatedAt)
			}
		}

		// Get performance metrics
		metrics := parser.GetWorkerPoolMetrics()

		t.Logf("Integration Test Results:")
		t.Logf("  Files processed: %d", len(testFiles))
		t.Logf("  Total chunks: %d", totalChunks)
		t.Logf("  Processing time: %v", duration)
		t.Logf("  Tasks completed: %d", metrics.TasksCompleted)
		t.Logf("  Tasks failed: %d", metrics.TasksFailed)
		t.Logf("  Average processing time: %v", metrics.AverageProcessingTime)
		t.Logf("  Active workers: %d", metrics.ActiveWorkers)

		// Verify metrics
		assert.Equal(t, int64(len(testFiles)), metrics.TasksCompleted)
		assert.Equal(t, int64(0), metrics.TasksFailed)
		assert.Equal(t, 4, metrics.ActiveWorkers)
		assert.Greater(t, metrics.TotalProcessingTime, time.Duration(0))
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
		defer parser.Close()

		// Mix valid and invalid files
		mixedFiles := []string{testFiles[0], testFiles[1], "/nonexistent/file.txt"}

		ctx := context.Background()
		results, err := parser.BatchParseFiles(ctx, mixedFiles)

		// Should return partial results and error
		assert.Error(t, err)
		assert.Equal(t, 2, len(results)) // Only valid files processed

		// Verify valid files were processed correctly
		for _, filePath := range testFiles[:2] {
			chunks, exists := results[filePath]
			assert.True(t, exists)
			assert.NotEmpty(t, chunks)
		}
	})

	t.Run("PerformanceComparison", func(t *testing.T) {
		parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
		defer parser.Close()

		ctx := context.Background()

		// Use only the valid test files for performance comparison
		validFiles := testFiles

		// Sequential processing
		startTime := time.Now()
		sequentialResults := make(map[string][]*schema.Chunk)
		for _, filePath := range validFiles {
			chunks, err := parser.ParseFile(ctx, filePath)
			require.NoError(t, err)
			sequentialResults[filePath] = chunks
		}
		sequentialDuration := time.Since(startTime)

		// Parallel processing
		startTime = time.Now()
		parallelResults, err := parser.BatchParseFiles(ctx, validFiles)
		require.NoError(t, err)
		parallelDuration := time.Since(startTime)

		// Calculate speedup
		speedup := float64(sequentialDuration) / float64(parallelDuration)

		t.Logf("Performance Comparison:")
		t.Logf("  Sequential: %v", sequentialDuration)
		t.Logf("  Parallel: %v", parallelDuration)
		t.Logf("  Speedup: %.2fx", speedup)

		// Verify results are identical
		assert.Equal(t, len(sequentialResults), len(parallelResults))
		for filePath, seqChunks := range sequentialResults {
			parChunks, exists := parallelResults[filePath]
			assert.True(t, exists)
			assert.Equal(t, len(seqChunks), len(parChunks))
		}
	})
}

// TestWorkerPoolResourceManagement tests resource cleanup and management
func TestWorkerPoolResourceManagement(t *testing.T) {
	tempDir := t.TempDir()
	testFiles := createIntegrationTestFiles(t, tempDir)

	t.Run("GracefulShutdown", func(t *testing.T) {
		config := &schema.WorkerPoolConfig{
			NumWorkers:    2,
			QueueSize:     10,
			Timeout:       5 * time.Second,
			RetryAttempts: 1,
			RetryDelay:    100 * time.Millisecond,
		}

		parser := core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)

		// Start and verify health
		require.NoError(t, parser.StartWorkerPool())
		assert.True(t, parser.IsWorkerPoolHealthy())

		// Process some files
		ctx := context.Background()
		results, err := parser.BatchParseFiles(ctx, testFiles[:2])
		require.NoError(t, err)
		assert.Equal(t, 2, len(results))

		// Shutdown gracefully
		require.NoError(t, parser.StopWorkerPool())
		assert.False(t, parser.IsWorkerPoolHealthy())

		// Verify metrics are preserved
		metrics := parser.GetWorkerPoolMetrics()
		assert.Equal(t, int64(2), metrics.TasksCompleted)
	})

	t.Run("MultipleStartStop", func(t *testing.T) {
		parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
		defer parser.Close()

		// Multiple start/stop cycles
		for i := 0; i < 3; i++ {
			require.NoError(t, parser.StartWorkerPool())
			assert.True(t, parser.IsWorkerPoolHealthy())

			// Process a file
			ctx := context.Background()
			results, err := parser.BatchParseFiles(ctx, testFiles[:1])
			require.NoError(t, err)
			assert.Equal(t, 1, len(results))

			require.NoError(t, parser.StopWorkerPool())
			assert.False(t, parser.IsWorkerPoolHealthy())
		}
	})
}

// createIntegrationTestFiles creates a variety of test files for integration testing
func createIntegrationTestFiles(t *testing.T, dir string) []string {
	files := make([]string, 0)

	// Create different types of content
	testCases := []struct {
		filename string
		content  string
	}{
		{
			"simple.txt",
			"This is a simple text file.\n\nIt has multiple paragraphs.\n\nEach paragraph should be chunked separately.",
		},
		{
			"markdown.md",
			"# Markdown File\n\nThis is a **markdown** file with formatting.\n\n## Section 1\n\nSome content here.\n\n## Section 2\n\nMore content with `code` blocks.",
		},
		{
			"large.txt",
			generateLargeContent(1000), // 1000 words
		},
		{
			"structured.txt",
			"Title: Test Document\nAuthor: Test User\nDate: 2024-01-01\n\nContent:\nThis is the main content of the document.\nIt spans multiple lines and paragraphs.\n\nConclusion:\nThis is the end of the document.",
		},
		{
			"code.txt",
			"package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}\n\n// This is a comment\nfunc helper() {\n    // Helper function\n}",
		},
	}

	for _, tc := range testCases {
		filePath := filepath.Join(dir, tc.filename)
		err := os.WriteFile(filePath, []byte(tc.content), 0644)
		require.NoError(t, err)
		files = append(files, filePath)
	}

	return files
}

// generateLargeContent creates content with approximately the specified number of words
func generateLargeContent(wordCount int) string {
	words := []string{
		"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog",
		"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing",
		"elit", "sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore",
		"et", "dolore", "magna", "aliqua", "enim", "ad", "minim", "veniam",
		"quis", "nostrud", "exercitation", "ullamco", "laboris", "nisi", "ut",
		"aliquip", "ex", "ea", "commodo", "consequat", "duis", "aute", "irure",
		"dolor", "in", "reprehenderit", "voluptate", "velit", "esse", "cillum",
		"fugiat", "nulla", "pariatur", "excepteur", "sint", "occaecat",
		"cupidatat", "non", "proident", "sunt", "in", "culpa", "qui", "officia",
		"deserunt", "mollit", "anim", "id", "est", "laborum",
	}

	content := ""
	paragraphLength := 0

	for i := 0; i < wordCount; i++ {
		word := words[i%len(words)]
		content += word
		paragraphLength++

		if i < wordCount-1 {
			content += " "
		}

		// Add paragraph breaks every 50-100 words
		if paragraphLength > 50 && (i%73 == 0 || paragraphLength > 100) {
			content += "\n\n"
			paragraphLength = 0
		}
	}

	return content
}
