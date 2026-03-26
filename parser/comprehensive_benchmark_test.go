// Package parser - Comprehensive benchmarks and performance tests for all parser components
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// BenchmarkAllParserTypes benchmarks all parser implementations
func BenchmarkAllParserTypes(b *testing.B) {
	tempDir := b.TempDir()

	// Create test files for different formats
	testFiles := createMultiFormatTestFiles(b, tempDir)

	// Test each parser type
	b.Run("UnifiedParser", func(b *testing.B) {
		parser := NewUnifiedParser(schema.DefaultChunkingConfig())
		defer parser.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			for _, filePath := range testFiles {
				_, err := parser.ParseFile(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("CachedParser", func(b *testing.B) {
		parser := NewCachedUnifiedParser(schema.DefaultChunkingConfig(), DefaultCacheConfig())
		defer parser.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			for _, filePath := range testFiles {
				_, err := parser.ParseFile(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("StreamingParser", func(b *testing.B) {
		parser := NewStreamingParser(DefaultStreamingConfig(), schema.DefaultChunkingConfig())

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			for _, filePath := range testFiles {
				_, err := parser.ParseFileStream(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

// BenchmarkFileSizeScaling tests performance with different file sizes
func BenchmarkFileSizeScaling(b *testing.B) {
	tempDir := b.TempDir()

	fileSizes := []struct {
		name      string
		wordCount int
	}{
		{"1KB", 100},
		{"10KB", 1000},
		{"100KB", 10000},
		{"1MB", 100000},
		{"10MB", 1000000},
	}

	for _, size := range fileSizes {
		b.Run(size.name, func(b *testing.B) {
			// Create test file
			content := generateBenchmarkContent(size.wordCount)
			filePath := filepath.Join(tempDir, fmt.Sprintf("test_%s.txt", size.name))
			err := os.WriteFile(filePath, []byte(content), 0644)
			if err != nil {
				b.Fatal(err)
			}

			parser := NewUnifiedParser(schema.DefaultChunkingConfig())
			defer parser.Close()

			b.SetBytes(int64(len(content)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				_, err := parser.ParseFile(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkConcurrentParsing tests concurrent parsing performance
func BenchmarkConcurrentParsing(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createComprehensiveBenchmarkTestFiles(b, tempDir, 20, "medium")

	concurrencyLevels := []int{1, 2, 4, 8, 16}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			config := &WorkerPoolConfig{
				NumWorkers:    concurrency,
				QueueSize:     concurrency * 5,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			}

			parser := NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			defer parser.Close()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				_, err := parser.BatchParseFiles(ctx, testFiles)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMemoryAllocation tests memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	tempDir := b.TempDir()

	testCases := []struct {
		name      string
		fileCount int
		fileSize  string
	}{
		{"SmallFiles_Many", 100, "small"},
		{"MediumFiles_Some", 20, "medium"},
		{"LargeFiles_Few", 5, "large"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			testFiles := createComprehensiveBenchmarkTestFiles(b, tempDir, tc.fileCount, tc.fileSize)
			parser := NewUnifiedParser(schema.DefaultChunkingConfig())
			defer parser.Close()

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()
			b.ReportAllocs()

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
			b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
		})
	}
}

// BenchmarkCachePerformance tests cache hit/miss performance
func BenchmarkCachePerformance(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createComprehensiveBenchmarkTestFiles(b, tempDir, 10, "medium")

	hitRates := []struct {
		name    string
		hitRate float64
	}{
		{"HighHit_90%", 0.9},
		{"MediumHit_50%", 0.5},
		{"LowHit_10%", 0.1},
	}

	for _, hr := range hitRates {
		b.Run(hr.name, func(b *testing.B) {
			parser := NewCachedUnifiedParser(schema.DefaultChunkingConfig(), DefaultCacheConfig())
			defer parser.Close()

			// Pre-populate cache
			ctx := context.Background()
			for _, filePath := range testFiles {
				parser.ParseFile(ctx, filePath)
			}

			b.ResetTimer()
			hits := 0

			for i := 0; i < b.N; i++ {
				if float64(i%100)/100.0 < hr.hitRate {
					// Cache hit
					filePath := testFiles[i%len(testFiles)]
					parser.ParseFile(ctx, filePath)
					hits++
				} else {
					// Cache miss - create unique content
					uniqueContent := fmt.Sprintf("unique content %d", i)
					parser.ParseText(ctx, uniqueContent)
				}
			}

			b.ReportMetric(float64(hits)/float64(b.N)*100, "hit-rate-%")
		})
	}
}

// BenchmarkStreamingMemoryEfficiency tests streaming parser memory usage
func BenchmarkStreamingMemoryEfficiency(b *testing.B) {
	tempDir := b.TempDir()

	// Create large files
	fileSizes := []int{1000000, 5000000, 10000000} // 1MB, 5MB, 10MB (word count)

	for _, size := range fileSizes {
		b.Run(fmt.Sprintf("Size_%dMB", size/1000000), func(b *testing.B) {
			content := generateBenchmarkContent(size)
			filePath := filepath.Join(tempDir, fmt.Sprintf("large_%d.txt", size))
			err := os.WriteFile(filePath, []byte(content), 0644)
			if err != nil {
				b.Fatal(err)
			}

			parser := NewStreamingParser(DefaultStreamingConfig(), schema.DefaultChunkingConfig())

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				_, err := parser.ParseFileStream(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
			}

			b.StopTimer()
			runtime.GC()
			runtime.ReadMemStats(&m2)

			memUsed := m2.TotalAlloc - m1.TotalAlloc
			b.ReportMetric(float64(memUsed)/float64(b.N), "bytes/op")
			b.ReportMetric(float64(len(content)), "content_bytes")
			b.ReportMetric(float64(memUsed)/float64(len(content)*b.N)*100, "memory_efficiency_%")
		})
	}
}

// BenchmarkWorkerPoolScalingComprehensive tests worker pool performance scaling
func BenchmarkWorkerPoolScalingComprehensive(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createComprehensiveBenchmarkTestFiles(b, tempDir, 50, "medium")

	workerCounts := []int{1, 2, 4, 8, 16, 32}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers_%d", workers), func(b *testing.B) {
			config := &WorkerPoolConfig{
				NumWorkers:    workers,
				QueueSize:     workers * 10,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    50 * time.Millisecond,
			}

			parser := NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			defer parser.Close()

			b.ResetTimer()

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

			// Report worker pool metrics
			metrics := parser.GetWorkerPoolMetrics()
			b.ReportMetric(float64(metrics.AverageProcessingTime.Nanoseconds()), "avg_processing_time_ns")
			b.ReportMetric(float64(workers), "worker_count")
		})
	}
}

// BenchmarkThroughputMeasurement measures parsing throughput
func BenchmarkThroughputMeasurement(b *testing.B) {
	tempDir := b.TempDir()

	// Create files of known sizes
	testCases := []struct {
		name      string
		wordCount int
		fileCount int
	}{
		{"SmallFiles", 100, 100},
		{"MediumFiles", 1000, 50},
		{"LargeFiles", 10000, 10},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Create test files
			testFiles := make([]string, tc.fileCount)
			totalBytes := 0

			for i := 0; i < tc.fileCount; i++ {
				content := generateBenchmarkContent(tc.wordCount)
				filePath := filepath.Join(tempDir, fmt.Sprintf("throughput_%s_%d.txt", tc.name, i))
				err := os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					b.Fatal(err)
				}
				testFiles[i] = filePath
				totalBytes += len(content)
			}

			parser := NewUnifiedParser(schema.DefaultChunkingConfig())
			defer parser.Close()

			b.SetBytes(int64(totalBytes))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				_, err := parser.BatchParseFiles(ctx, testFiles)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLatencyMeasurement measures parsing latency
func BenchmarkLatencyMeasurement(b *testing.B) {
	tempDir := b.TempDir()

	// Create single files of different sizes
	fileSizes := []struct {
		name      string
		wordCount int
	}{
		{"Small", 100},
		{"Medium", 1000},
		{"Large", 10000},
	}

	for _, size := range fileSizes {
		b.Run(size.name, func(b *testing.B) {
			content := generateBenchmarkContent(size.wordCount)
			filePath := filepath.Join(tempDir, fmt.Sprintf("latency_%s.txt", size.name))
			err := os.WriteFile(filePath, []byte(content), 0644)
			if err != nil {
				b.Fatal(err)
			}

			parser := NewUnifiedParser(schema.DefaultChunkingConfig())
			defer parser.Close()

			latencies := make([]time.Duration, b.N)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				ctx := context.Background()
				_, err := parser.ParseFile(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
				latencies[i] = time.Since(start)
			}

			// Calculate latency statistics
			var totalLatency time.Duration
			minLatency := latencies[0]
			maxLatency := latencies[0]

			for _, latency := range latencies {
				totalLatency += latency
				if latency < minLatency {
					minLatency = latency
				}
				if latency > maxLatency {
					maxLatency = latency
				}
			}

			avgLatency := totalLatency / time.Duration(b.N)

			b.ReportMetric(float64(avgLatency.Nanoseconds()), "avg_latency_ns")
			b.ReportMetric(float64(minLatency.Nanoseconds()), "min_latency_ns")
			b.ReportMetric(float64(maxLatency.Nanoseconds()), "max_latency_ns")
		})
	}
}

// BenchmarkConfigurationComparison compares different parser configurations
func BenchmarkConfigurationComparison(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createComprehensiveBenchmarkTestFiles(b, tempDir, 10, "medium")

	configs := []struct {
		name   string
		config *schema.ChunkingConfig
	}{
		{
			"Paragraph_Small",
			&schema.ChunkingConfig{
				Strategy: schema.StrategyParagraph,
				MaxSize:  500,
				MinSize:  50,
				Overlap:  50,
			},
		},
		{
			"Paragraph_Large",
			&schema.ChunkingConfig{
				Strategy: schema.StrategyParagraph,
				MaxSize:  2000,
				MinSize:  100,
				Overlap:  200,
			},
		},
		{
			"Sentence",
			&schema.ChunkingConfig{
				Strategy: schema.StrategySentence,
				MaxSize:  1000,
				MinSize:  50,
				Overlap:  100,
			},
		},
		{
			"FixedSize",
			&schema.ChunkingConfig{
				Strategy: schema.StrategyFixedSize,
				MaxSize:  1000,
				MinSize:  100,
				Overlap:  100,
			},
		},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			parser := NewUnifiedParser(cfg.config)
			defer parser.Close()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				_, err := parser.BatchParseFiles(ctx, testFiles)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Helper functions

func createMultiFormatTestFiles(b *testing.B, dir string) []string {
	files := make([]string, 0)

	// Text file
	txtFile := filepath.Join(dir, "test.txt")
	txtContent := "This is a plain text file.\n\nIt has multiple paragraphs.\n\nEach paragraph should be processed correctly."
	err := os.WriteFile(txtFile, []byte(txtContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files = append(files, txtFile)

	// Markdown file
	mdFile := filepath.Join(dir, "test.md")
	mdContent := "# Test Markdown\n\nThis is a **markdown** file.\n\n## Section 1\n\nSome content here.\n\n## Section 2\n\nMore content with `code` blocks."
	err = os.WriteFile(mdFile, []byte(mdContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files = append(files, mdFile)

	// CSV file
	csvFile := filepath.Join(dir, "test.csv")
	csvContent := "name,age,city\nJohn,30,New York\nJane,25,Los Angeles\nBob,35,Chicago"
	err = os.WriteFile(csvFile, []byte(csvContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files = append(files, csvFile)

	// JSON file
	jsonFile := filepath.Join(dir, "test.json")
	jsonContent := `{"users": [{"name": "John", "age": 30}, {"name": "Jane", "age": 25}], "total": 2}`
	err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files = append(files, jsonFile)

	return files
}

func createComprehensiveBenchmarkTestFiles(b *testing.B, dir string, count int, size string) []string {
	files := make([]string, count)

	var wordCount int
	switch size {
	case "small":
		wordCount = 100
	case "medium":
		wordCount = 500
	case "large":
		wordCount = 2000
	default:
		wordCount = 500
	}

	content := generateBenchmarkContent(wordCount)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("benchmark_%s_%d.txt", size, i)
		filePath := filepath.Join(dir, filename)

		fileContent := fmt.Sprintf("File %d - %s\n\n%s", i, size, content)

		err := os.WriteFile(filePath, []byte(fileContent), 0644)
		if err != nil {
			b.Fatal(err)
		}

		files[i] = filePath
	}

	return files
}

func generateBenchmarkContent(wordCount int) string {
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

	var builder strings.Builder
	paragraphLength := 0

	for i := 0; i < wordCount; i++ {
		word := words[i%len(words)]
		builder.WriteString(word)
		paragraphLength++

		if i < wordCount-1 {
			builder.WriteString(" ")
		}

		// Add paragraph breaks every 50-100 words
		if paragraphLength > 50 && (i%73 == 0 || paragraphLength > 100) {
			builder.WriteString("\n\n")
			paragraphLength = 0
		}
	}

	return builder.String()
}
