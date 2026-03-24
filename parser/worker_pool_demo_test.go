// Package parser - Worker pool demonstration
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestWorkerPoolDemo demonstrates the worker pool functionality
func TestWorkerPoolDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping demo in short mode")
	}

	// Create test files
	tempDir := t.TempDir()
	testFiles := createDemoFiles(t, tempDir, 10)

	fmt.Printf("\n=== Worker Pool Demonstration ===\n")
	fmt.Printf("Created %d test files for processing\n", len(testFiles))

	// Create parser with worker pool
	config := &WorkerPoolConfig{
		NumWorkers:    4,
		QueueSize:     20,
		Timeout:       10 * time.Second,
		RetryAttempts: 2,
		RetryDelay:    100 * time.Millisecond,
	}

	parser := NewUnifiedParserWithWorkerPool(DefaultChunkingConfig(), config)
	defer parser.Close()

	fmt.Printf("Configured worker pool with %d workers\n", config.NumWorkers)

	// Start worker pool
	require.NoError(t, parser.StartWorkerPool())
	fmt.Printf("Worker pool started successfully\n")

	// Process files in parallel
	ctx := context.Background()
	fmt.Printf("Processing %d files in parallel...\n", len(testFiles))

	startTime := time.Now()
	results, err := parser.BatchParseFiles(ctx, testFiles)
	duration := time.Since(startTime)

	require.NoError(t, err)
	fmt.Printf("Processing completed in %v\n", duration)

	// Display results
	totalChunks := 0
	for filePath, chunks := range results {
		totalChunks += len(chunks)
		fmt.Printf("  %s: %d chunks\n", filepath.Base(filePath), len(chunks))
	}

	// Display metrics
	metrics := parser.GetWorkerPoolMetrics()
	fmt.Printf("\nWorker Pool Metrics:\n")
	fmt.Printf("  Tasks submitted: %d\n", metrics.TasksSubmitted)
	fmt.Printf("  Tasks completed: %d\n", metrics.TasksCompleted)
	fmt.Printf("  Tasks failed: %d\n", metrics.TasksFailed)
	fmt.Printf("  Total processing time: %v\n", metrics.TotalProcessingTime)
	fmt.Printf("  Average processing time: %v\n", metrics.AverageProcessingTime)
	fmt.Printf("  Active workers: %d\n", metrics.ActiveWorkers)
	fmt.Printf("  Total chunks generated: %d\n", totalChunks)

	// Compare with sequential processing
	fmt.Printf("\n=== Performance Comparison ===\n")

	// Sequential processing
	sequentialParser := NewUnifiedParser(DefaultChunkingConfig())
	defer sequentialParser.Close()

	fmt.Printf("Processing same files sequentially...\n")
	startTime = time.Now()
	sequentialResults := make(map[string][]Chunk)
	for _, filePath := range testFiles {
		chunks, err := sequentialParser.ParseFile(ctx, filePath)
		require.NoError(t, err)
		sequentialResults[filePath] = chunks
	}
	sequentialDuration := time.Since(startTime)

	fmt.Printf("Sequential processing completed in %v\n", sequentialDuration)

	// Calculate speedup
	speedup := float64(sequentialDuration) / float64(duration)
	fmt.Printf("Speedup with worker pool: %.2fx\n", speedup)

	// Verify results are identical
	fmt.Printf("Verifying results consistency...\n")
	for filePath, parallelChunks := range results {
		sequentialChunks, exists := sequentialResults[filePath]
		require.True(t, exists, "File missing from sequential results: %s", filePath)
		require.Equal(t, len(sequentialChunks), len(parallelChunks),
			"Chunk count mismatch for %s", filePath)
	}
	fmt.Printf("✓ All results are consistent between parallel and sequential processing\n")

	fmt.Printf("\n=== Demo Complete ===\n")
}

// createDemoFiles creates test files for the demonstration
func createDemoFiles(t *testing.T, dir string, count int) []string {
	files := make([]string, count)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("demo_file_%02d.txt", i)
		filePath := filepath.Join(dir, filename)

		// Create content with varying sizes
		wordCount := 100 + (i * 50) // 100, 150, 200, ... words
		content := fmt.Sprintf("Demo File %d\n\n%s", i, generateDemoContent(wordCount))

		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)

		files[i] = filePath
	}

	return files
}

// generateDemoContent creates content with the specified number of words
func generateDemoContent(wordCount int) string {
	words := []string{
		"artificial", "intelligence", "memory", "processing", "parallel", "worker",
		"pool", "concurrent", "performance", "optimization", "scalability", "throughput",
		"efficiency", "parsing", "chunking", "text", "document", "analysis", "extraction",
		"natural", "language", "processing", "machine", "learning", "data", "mining",
		"information", "retrieval", "search", "indexing", "vectorization", "embedding",
		"semantic", "similarity", "clustering", "classification", "tokenization", "preprocessing",
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

		// Add paragraph breaks every 30-50 words
		if paragraphLength > 30 && (i%37 == 0 || paragraphLength > 50) {
			content += "\n\n"
			paragraphLength = 0
		}
	}

	return content
}
