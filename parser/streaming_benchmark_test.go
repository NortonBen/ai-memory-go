package parser

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// BenchmarkStreamingParserMemoryEfficiency tests memory usage with different file sizes
func BenchmarkStreamingParserMemoryEfficiency(b *testing.B) {
	testCases := []struct {
		name      string
		wordCount int
	}{
		{"Small_1KB", 100},
		{"Medium_10KB", 1000},
		{"Large_100KB", 10000},
		{"XLarge_1MB", 100000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			parser := NewStreamingParser(nil, nil)
			content := generateLargeTestContent(tc.wordCount)

			// Measure memory before
			var m1 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(content)
				_, err := parser.ParseReaderStream(context.Background(), reader, "bench_memory")
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			// Measure memory after
			var m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m2)

			// Report memory usage
			memUsed := m2.Alloc - m1.Alloc
			b.ReportMetric(float64(memUsed)/float64(b.N), "bytes/op")
			b.ReportMetric(float64(len(content)), "content_bytes")
		})
	}
}

// BenchmarkStreamingParserThroughput measures processing throughput
func BenchmarkStreamingParserThroughput(b *testing.B) {
	parser := NewStreamingParser(nil, nil)
	content := generateLargeTestContent(10000) // ~100KB content

	b.SetBytes(int64(len(content)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(content)
		_, err := parser.ParseReaderStream(context.Background(), reader, "bench_throughput")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStreamingParserBufferSizes tests different buffer sizes
func BenchmarkStreamingParserBufferSizes(b *testing.B) {
	content := generateLargeTestContent(5000) // Medium content

	bufferSizes := []int{
		4 * 1024,   // 4KB
		16 * 1024,  // 16KB
		64 * 1024,  // 64KB (default)
		256 * 1024, // 256KB
	}

	for _, bufferSize := range bufferSizes {
		b.Run(formatBytes(bufferSize), func(b *testing.B) {
			config := DefaultStreamingConfig()
			config.BufferSize = bufferSize

			parser := NewStreamingParser(config, nil)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(content)
				_, err := parser.ParseReaderStream(context.Background(), reader, "bench_buffer")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStreamingParserChunkSizes tests different chunk sizes
func BenchmarkStreamingParserChunkSizes(b *testing.B) {
	content := generateLargeTestContent(5000) // Medium content

	chunkSizes := []int{
		1 * 1024,  // 1KB
		4 * 1024,  // 4KB (default)
		8 * 1024,  // 8KB
		16 * 1024, // 16KB
	}

	for _, chunkSize := range chunkSizes {
		b.Run(formatBytes(chunkSize), func(b *testing.B) {
			config := DefaultStreamingConfig()
			config.MaxChunkSize = chunkSize

			parser := NewStreamingParser(config, nil)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(content)
				_, err := parser.ParseReaderStream(context.Background(), reader, "bench_chunk")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStreamingParserStrategies compares chunking strategies
func BenchmarkStreamingParserStrategies(b *testing.B) {
	content := generateLargeTestContent(2000)

	strategies := []ChunkingStrategy{
		StrategyParagraph,
		StrategySentence,
		StrategyFixedSize,
	}

	for _, strategy := range strategies {
		b.Run(string(strategy), func(b *testing.B) {
			chunkConfig := &ChunkingConfig{
				Strategy: strategy,
				MaxSize:  1000,
				MinSize:  100,
			}

			parser := NewStreamingParser(nil, chunkConfig)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(content)
				_, err := parser.ParseReaderStream(context.Background(), reader, "bench_strategy")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkStreamingParserFileVsReader compares file vs reader parsing
func BenchmarkStreamingParserFileVsReader(b *testing.B) {
	parser := NewStreamingParser(nil, nil)
	content := generateLargeTestContent(5000)

	// Create temporary file
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "bench_file.txt")
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("File", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := parser.ParseFileStream(context.Background(), testFile)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Reader", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := strings.NewReader(content)
			_, err := parser.ParseReaderStream(context.Background(), reader, "bench_reader")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkStreamingParserConcurrency tests concurrent streaming operations
func BenchmarkStreamingParserConcurrency(b *testing.B) {
	parser := NewStreamingParser(nil, nil)
	content := generateLargeTestContent(1000)

	concurrencyLevels := []int{1, 2, 4, 8}

	for _, concurrency := range concurrencyLevels {
		b.Run(formatConcurrency(concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					reader := strings.NewReader(content)
					_, err := parser.ParseReaderStream(context.Background(), reader, "bench_concurrent")
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkStreamingParserProgressTracking tests performance with progress tracking
func BenchmarkStreamingParserProgressTracking(b *testing.B) {
	content := generateLargeTestContent(5000)

	b.Run("WithProgress", func(b *testing.B) {
		config := DefaultStreamingConfig()
		config.EnableProgressTracking = true
		config.ProgressCallback = func(bytesProcessed, totalBytes int64, chunksCreated int) {
			// Minimal callback to simulate progress tracking overhead
		}
		config.FlushInterval = 10 * time.Millisecond

		parser := NewStreamingParser(config, nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := strings.NewReader(content)
			_, err := parser.ParseReaderStream(context.Background(), reader, "bench_progress")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithoutProgress", func(b *testing.B) {
		config := DefaultStreamingConfig()
		config.EnableProgressTracking = false

		parser := NewStreamingParser(config, nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := strings.NewReader(content)
			_, err := parser.ParseReaderStream(context.Background(), reader, "bench_no_progress")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkStreamingParserLargeFiles tests with very large files
func BenchmarkStreamingParserLargeFiles(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping large file benchmark in short mode")
	}

	parser := NewStreamingParser(nil, nil)

	// Create a very large temporary file (~10MB)
	tmpDir := b.TempDir()
	largeFile := filepath.Join(tmpDir, "very_large.txt")

	// Generate content in chunks to avoid memory issues
	file, err := os.Create(largeFile)
	if err != nil {
		b.Fatal(err)
	}

	baseContent := generateLargeTestContent(1000)
	for i := 0; i < 1000; i++ { // Write 1000 chunks
		_, err := file.WriteString(baseContent)
		if err != nil {
			b.Fatal(err)
		}
		if i%100 == 0 {
			file.WriteString("\n\n") // Add paragraph breaks
		}
	}
	file.Close()

	// Measure file size
	stat, err := os.Stat(largeFile)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(stat.Size())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := parser.ParseFileStream(context.Background(), largeFile)
		if err != nil {
			b.Fatal(err)
		}

		// Report metrics
		b.ReportMetric(float64(len(result.Chunks)), "chunks_created")
		b.ReportMetric(float64(result.ProcessingTime.Nanoseconds()), "processing_time_ns")
	}
}

// Helper functions for benchmark formatting
func formatBytes(bytes int) string {
	if bytes >= 1024*1024 {
		return string(rune(bytes/(1024*1024))) + "MB"
	} else if bytes >= 1024 {
		return string(rune(bytes/1024)) + "KB"
	}
	return string(rune(bytes)) + "B"
}

func formatConcurrency(level int) string {
	return string(rune(level)) + "_goroutines"
}

// Memory usage test (not a benchmark, but useful for profiling)
func TestStreamingParserMemoryProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profile test in short mode")
	}

	parser := NewStreamingParser(nil, nil)

	// Create progressively larger content and measure memory
	sizes := []int{1000, 5000, 10000}

	for _, size := range sizes {
		t.Run(formatBytes(size*10), func(t *testing.T) { // Rough size estimate
			content := generateLargeTestContent(size)

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			reader := strings.NewReader(content)
			result, err := parser.ParseReaderStream(context.Background(), reader, "memory_profile")
			if err != nil {
				t.Fatal(err)
			}

			runtime.GC()
			runtime.ReadMemStats(&m2)

			// Use TotalAlloc to avoid overflow issues
			memUsed := m2.TotalAlloc - m1.TotalAlloc
			t.Logf("Content size: %d bytes, Memory allocated: %d bytes, Chunks: %d",
				len(content), memUsed, len(result.Chunks))

			// Basic sanity check - memory shouldn't be excessively high
			if memUsed > uint64(len(content)*10) { // Allow 10x overhead for processing
				t.Logf("Warning: High memory usage detected: %d bytes for %d byte content", memUsed, len(content))
			}
		})
	}
}
