// Package parser - Performance benchmarks for worker pool
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// BenchmarkParsingPerformance compares sequential vs parallel processing performance
func BenchmarkParsingPerformance(b *testing.B) {
	// Create test files of different sizes
	tempDir := b.TempDir()

	testCases := []struct {
		name      string
		fileCount int
		fileSize  string
	}{
		{"Small_10files", 10, "small"},
		{"Medium_20files", 20, "medium"},
		{"Large_50files", 50, "large"},
	}

	for _, tc := range testCases {
		testFiles := createBenchmarkFiles(b, tempDir, tc.fileCount, tc.fileSize)

		b.Run(fmt.Sprintf("%s_Sequential", tc.name), func(b *testing.B) {
			parser := NewUnifiedParser(schema.DefaultChunkingConfig())
			defer parser.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				results := make(map[string][]*schema.Chunk)

				for _, filePath := range testFiles {
					chunks, err := parser.ParseFile(ctx, filePath)
					if err != nil {
						b.Fatal(err)
					}
					results[filePath] = chunks
				}

				if len(results) != len(testFiles) {
					b.Fatalf("Expected %d results, got %d", len(testFiles), len(results))
				}
			}
		})

		b.Run(fmt.Sprintf("%s_Parallel", tc.name), func(b *testing.B) {
			config := &WorkerPoolConfig{
				NumWorkers:    runtime.NumCPU(),
				QueueSize:     tc.fileCount * 2,
				Timeout:       30 * time.Second,
				RetryAttempts: 2,
				RetryDelay:    100 * time.Millisecond,
			}

			parser := NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			defer parser.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				results, err := parser.BatchParseFiles(ctx, testFiles)
				if err != nil {
					b.Fatal(err)
				}

				if len(results) != len(testFiles) {
					b.Fatalf("Expected %d results, got %d", len(testFiles), len(results))
				}
			}
		})
	}
}

// BenchmarkWorkerPoolScaling tests performance with different worker counts
func BenchmarkWorkerPoolScaling(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createBenchmarkFiles(b, tempDir, 30, "medium")

	workerCounts := []int{1, 2, 4, 8, runtime.NumCPU()}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers_%d", workers), func(b *testing.B) {
			config := &WorkerPoolConfig{
				NumWorkers:    workers,
				QueueSize:     50,
				Timeout:       30 * time.Second,
				RetryAttempts: 2,
				RetryDelay:    100 * time.Millisecond,
			}

			parser := NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			defer parser.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				results, err := parser.BatchParseFiles(ctx, testFiles)
				if err != nil {
					b.Fatal(err)
				}

				if len(results) != len(testFiles) {
					b.Fatalf("Expected %d results, got %d", len(testFiles), len(results))
				}
			}
		})
	}
}

// BenchmarkMemoryUsage tests memory efficiency of worker pool
func BenchmarkMemoryUsage(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createBenchmarkFiles(b, tempDir, 20, "large")

	b.Run("Sequential_Memory", func(b *testing.B) {
		parser := NewUnifiedParser(schema.DefaultChunkingConfig())
		defer parser.Close()

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			results := make(map[string][]*schema.Chunk)

			for _, filePath := range testFiles {
				chunks, err := parser.ParseFile(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
				results[filePath] = chunks
			}
		}

		b.StopTimer()
		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})

	b.Run("Parallel_Memory", func(b *testing.B) {
		config := &WorkerPoolConfig{
			NumWorkers:    4,
			QueueSize:     30,
			Timeout:       30 * time.Second,
			RetryAttempts: 2,
			RetryDelay:    100 * time.Millisecond,
		}

		parser := NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
		defer parser.Close()

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_, err := parser.BatchParseFiles(ctx, testFiles)
			if err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})
}

// TestPerformanceComparison provides a detailed performance comparison
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createBenchmarkFiles(t, tempDir, 20, "medium")

	// Sequential processing
	parser := NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()

	// Measure sequential processing
	startTime := time.Now()
	sequentialResults := make(map[string][]*schema.Chunk)
	for _, filePath := range testFiles {
		chunks, err := parser.ParseFile(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
		sequentialResults[filePath] = chunks
	}
	sequentialDuration := time.Since(startTime)

	// Measure parallel processing
	startTime = time.Now()
	parallelResults, err := parser.BatchParseFiles(ctx, testFiles)
	if err != nil {
		t.Fatal(err)
	}
	parallelDuration := time.Since(startTime)

	// Verify results are identical
	if len(sequentialResults) != len(parallelResults) {
		t.Fatalf("Result count mismatch: sequential=%d, parallel=%d",
			len(sequentialResults), len(parallelResults))
	}

	for filePath, seqChunks := range sequentialResults {
		parChunks, exists := parallelResults[filePath]
		if !exists {
			t.Fatalf("File %s missing from parallel results", filePath)
		}

		if len(seqChunks) != len(parChunks) {
			t.Fatalf("Chunk count mismatch for %s: sequential=%d, parallel=%d",
				filePath, len(seqChunks), len(parChunks))
		}

		// Verify chunk content matches (order might differ due to parallel processing)
		seqContent := make(map[string]bool)
		for _, chunk := range seqChunks {
			seqContent[chunk.Content] = true
		}

		for _, chunk := range parChunks {
			if !seqContent[chunk.Content] {
				t.Fatalf("Chunk content mismatch in file %s", filePath)
			}
		}
	}

	// Calculate performance improvement
	speedup := float64(sequentialDuration) / float64(parallelDuration)

	t.Logf("Performance Comparison:")
	t.Logf("  Files processed: %d", len(testFiles))
	t.Logf("  Sequential time: %v", sequentialDuration)
	t.Logf("  Parallel time: %v", parallelDuration)
	t.Logf("  Speedup: %.2fx", speedup)
	t.Logf("  Workers: %d", runtime.NumCPU())

	// Get worker pool metrics
	metrics := parser.GetWorkerPoolMetrics()
	t.Logf("Worker Pool Metrics:")
	t.Logf("  Tasks completed: %d", metrics.TasksCompleted)
	t.Logf("  Tasks failed: %d", metrics.TasksFailed)
	t.Logf("  Average processing time: %v", metrics.AverageProcessingTime)

	// Parallel processing should be faster for multiple files
	if speedup < 1.0 {
		t.Logf("Warning: Parallel processing was slower (%.2fx). This might be due to overhead or system load.", speedup)
	}
}

// createBenchmarkFiles creates test files of different sizes for benchmarking
func createBenchmarkFiles(tb testing.TB, dir string, count int, size string) []string {
	files := make([]string, count)

	var content string
	switch size {
	case "small":
		content = generateContent(100) // ~100 words
	case "medium":
		content = generateContent(500) // ~500 words
	case "large":
		content = generateContent(2000) // ~2000 words
	default:
		content = generateContent(500)
	}

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("benchmark_file_%s_%d.txt", size, i)
		filepath := filepath.Join(dir, filename)

		fileContent := fmt.Sprintf("File %d - %s\n\n%s", i, size, content)

		err := os.WriteFile(filepath, []byte(fileContent), 0644)
		if err != nil {
			tb.Fatal(err)
		}

		files[i] = filepath
	}

	return files
}

// generateContent creates content with approximately the specified number of words
func generateContent(wordCount int) string {
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
