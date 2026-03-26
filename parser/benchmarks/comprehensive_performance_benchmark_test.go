// Package parser - Comprehensive performance benchmarks for all parser components
// This file implements Task 3.3.4: Create benchmarks and performance tests
package benchmarks

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
	"github.com/NortonBen/ai-memory-go/parser/core"
	"github.com/NortonBen/ai-memory-go/parser/cache"
	"github.com/NortonBen/ai-memory-go/parser/concurrency"
)

// BenchmarkUnifiedParserAllFormats benchmarks parsing performance across all supported formats
func BenchmarkUnifiedParserAllFormats(b *testing.B) {
	tempDir := b.TempDir()
	p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer p.Close()

	// Create test files for each supported format
	testFiles := createAllFormatTestFiles(b, tempDir)

	for format, filePath := range testFiles {
		b.Run(fmt.Sprintf("Format_%s", format), func(b *testing.B) {
			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				chunks, err := p.ParseFile(ctx, filePath)
				if err != nil {
					b.Fatal(err)
				}
				if len(chunks) == 0 {
					b.Fatal("No chunks produced")
				}
			}
		})
	}
}

// BenchmarkChunkingStrategiesPerformance compares all chunking strategies
func BenchmarkChunkingStrategiesPerformance(b *testing.B) {
	testContent := generateBenchmarkContent(5000) // ~50KB content

	strategies := []struct {
		name     string
		strategy schema.ChunkingStrategy
		config   *schema.ChunkingConfig
	}{
		{
			"Paragraph_Default",
			schema.StrategyParagraph,
			&schema.ChunkingConfig{
				Strategy: schema.StrategyParagraph,
				MaxSize:  1000,
				MinSize:  100,
				Overlap:  100,
			},
		},
		{
			"Paragraph_Large",
			schema.StrategyParagraph,
			&schema.ChunkingConfig{
				Strategy: schema.StrategyParagraph,
				MaxSize:  2000,
				MinSize:  200,
				Overlap:  200,
			},
		},
		{
			"Sentence_Default",
			schema.StrategySentence,
			&schema.ChunkingConfig{
				Strategy: schema.StrategySentence,
				MaxSize:  1000,
				MinSize:  100,
				Overlap:  100,
			},
		},
		{
			"FixedSize_1KB",
			schema.StrategyFixedSize,
			&schema.ChunkingConfig{
				Strategy: schema.StrategyFixedSize,
				MaxSize:  1024,
				MinSize:  100,
				Overlap:  100,
			},
		},
		{
			"FixedSize_2KB",
			schema.StrategyFixedSize,
			&schema.ChunkingConfig{
				Strategy: schema.StrategyFixedSize,
				MaxSize:  2048,
				MinSize:  200,
				Overlap:  200,
			},
		},
		{
			"Semantic_Default",
			schema.StrategySemantic,
			&schema.ChunkingConfig{
				Strategy: schema.StrategySemantic,
				MaxSize:  1500,
				MinSize:  150,
				Overlap:  150,
			},
		},
	}

	for _, strategy := range strategies {
		b.Run(strategy.name, func(b *testing.B) {
			p := core.NewUnifiedParser(strategy.config)
			defer p.Close()

			ctx := context.Background()
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				chunks, err := p.ParseText(ctx, testContent)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(chunks)), "chunks_produced")
			}
		})
	}
}

// BenchmarkParserScalability tests parser performance with increasing data sizes
func BenchmarkParserScalability(b *testing.B) {
	p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer p.Close()

	dataSizes := []struct {
		name      string
		wordCount int
		sizeDesc  string
	}{
		{"Small_1KB", 100, "~1KB"},
		{"Medium_10KB", 1000, "~10KB"},
		{"Large_100KB", 10000, "~100KB"},
		{"XLarge_1MB", 100000, "~1MB"},
		{"XXLarge_10MB", 1000000, "~10MB"},
	}

	for _, size := range dataSizes {
		b.Run(size.name, func(b *testing.B) {
			content := generateBenchmarkContent(size.wordCount)
			ctx := context.Background()

			b.SetBytes(int64(len(content)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				chunks, err := p.ParseText(ctx, content)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(chunks)), "chunks_produced")
				b.ReportMetric(float64(len(content))/1024, "content_kb")
			}
		})
	}
}

// BenchmarkWorkerPoolScalabilityComprehensive tests worker pool performance across different scenarios
func BenchmarkWorkerPoolScalabilityComprehensive(b *testing.B) {
	tempDir := b.TempDir()

	testScenarios := []struct {
		name      string
		fileCount int
		wordCount int
		workers   int
	}{
		{"SmallFiles_ManyWorkers", 100, 100, runtime.NumCPU() * 2},
		{"SmallFiles_FewWorkers", 100, 100, 2},
		{"MediumFiles_OptimalWorkers", 50, 1000, runtime.NumCPU()},
		{"LargeFiles_SingleWorker", 10, 10000, 1},
		{"LargeFiles_ManyWorkers", 10, 10000, runtime.NumCPU()},
		{"MixedLoad_AdaptiveWorkers", 30, 2000, runtime.NumCPU()},
	}

	for _, scenario := range testScenarios {
		b.Run(scenario.name, func(b *testing.B) {
			testFiles := createPerformanceBenchmarkTestFiles(b, tempDir, scenario.fileCount, scenario.wordCount)

			config := &concurrency.WorkerPoolConfig{
				NumWorkers:    scenario.workers,
				QueueSize:     scenario.fileCount * 2,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			}

			p := core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			defer p.Close()

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				results, err := p.BatchParseFiles(ctx, testFiles)
				if err != nil {
					b.Fatal(err)
				}

				totalChunks := 0
				for _, chunks := range results {
					totalChunks += len(chunks)
				}

				b.ReportMetric(float64(totalChunks), "total_chunks")
				b.ReportMetric(float64(scenario.workers), "worker_count")
				b.ReportMetric(float64(len(testFiles)), "files_processed")
			}
		})
	}
}

// BenchmarkMemoryEfficiencyComprehensive tests memory usage patterns
func BenchmarkMemoryEfficiencyComprehensive(b *testing.B) {
	testCases := []struct {
		name        string
		setupParser func() schema.Parser
		cleanup     func(schema.Parser)
	}{
		{
			"UnifiedParser_Default",
			func() schema.Parser {
				return core.NewUnifiedParser(schema.DefaultChunkingConfig())
			},
			func(p schema.Parser) {
				if up, ok := p.(*core.UnifiedParser); ok {
					up.Close()
				}
			},
		},
		{
			"CachedParser_LRU",
			func() schema.Parser {
				cacheConfig := &cache.CacheConfig{
					MaxSize:         1000,
					TTL:             10 * time.Minute,
					Policy:          cache.PolicyLRU,
					CleanupInterval: 2 * time.Minute,
				}
				return core.NewCachedUnifiedParser(schema.DefaultChunkingConfig(), cacheConfig)
			},
			func(p schema.Parser) {
				if cp, ok := p.(*core.CachedUnifiedParser); ok {
					cp.Close()
				}
			},
		},
	}

	testContent := generateBenchmarkContent(5000) // ~50KB content

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			p := tc.setupParser()
			defer tc.cleanup(p)

			ctx := context.Background()

			// Measure memory before
			runtime.GC()
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				chunks, err := p.ParseText(ctx, testContent)
				if err != nil {
					b.Fatal(err)
				}
				if len(chunks) == 0 {
					b.Fatal("No chunks produced")
				}
			}

			b.StopTimer()

			// Measure memory after
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			memoryUsed := m2.TotalAlloc - m1.TotalAlloc
			b.ReportMetric(float64(memoryUsed)/float64(b.N), "bytes_per_op")
			b.ReportMetric(float64(len(testContent)), "content_bytes")
			b.ReportMetric(float64(memoryUsed)/float64(len(testContent)*b.N)*100, "memory_efficiency_%")
		})
	}
}

// BenchmarkConcurrentParsingLoad tests parser performance under concurrent load
func BenchmarkConcurrentParsingLoad(b *testing.B) {
	p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer p.Close()

	testContent := generateBenchmarkContent(1000) // ~10KB content
	ctx := context.Background()

	concurrencyLevels := []int{1, 2, 4, 8, 16, 32}

	for _, c := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", c), func(b *testing.B) {
			b.SetParallelism(c)
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					chunks, err := p.ParseText(ctx, testContent)
					if err != nil {
						b.Fatal(err)
					}
					if len(chunks) == 0 {
						b.Fatal("No chunks produced")
					}
				}
			})

			b.ReportMetric(float64(c), "goroutines")
		})
	}
}

// BenchmarkFormatDetectionPerformance tests file format detection speed
func BenchmarkFormatDetectionPerformance(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createAllFormatTestFiles(b, tempDir)

	// Convert map to slice for consistent iteration
	filePaths := make([]string, 0, len(testFiles))
	for _, path := range testFiles {
		filePaths = append(filePaths, path)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, filePath := range filePaths {
			// This would normally be in formats/detector.go or similar
			// but for benchmark purposes we'll just check extension
			ext := filepath.Ext(filePath)
			if ext == "" {
				b.Fatal("Format detection failed")
			}
		}
	}

	b.ReportMetric(float64(len(filePaths)), "files_per_iteration")
}

// BenchmarkContentTypeDetection tests content type detection performance
func BenchmarkContentTypeDetection(b *testing.B) {
	p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer p.Close()

	testContents := map[string]string{
		"PlainText": "This is plain text content with multiple sentences. It should be detected as text.",
		"Markdown":  "# Markdown Content\n\nThis is **markdown** with *formatting*.\n\n## Section\n\nMore content here.",
		"Code":      "func main() {\n    fmt.Println(\"Hello, World!\")\n}\n\nclass Example {\n    public void method() {}\n}",
		"JSON":      `{"name": "test", "value": 123, "items": ["a", "b", "c"]}`,
		"CSV":       "name,age,city\nJohn,30,NYC\nJane,25,LA",
	}

	for contentType, content := range testContents {
		b.Run(contentType, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				detectedType := p.DetectContentType(content)
				if detectedType == "" {
					b.Fatal("Content type detection failed")
				}
			}
		})
	}
}

// BenchmarkChunkValidation tests chunk validation performance
func BenchmarkChunkValidation(b *testing.B) {
	p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer p.Close()

	// Create test chunks
	testChunks := make([]*schema.Chunk, 100)
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("Test chunk content %d. This is a valid chunk with sufficient content length.", i)
		chunk := schema.NewChunk(content, "test", schema.ChunkTypeText)
		testChunks[i] = chunk
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := p.ValidateChunks(testChunks)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(len(testChunks)), "chunks_validated")
}

// BenchmarkComprehensiveStreamingVsRegularParsing compares streaming vs regular parsing performance
func BenchmarkComprehensiveStreamingVsRegularParsing(b *testing.B) {
	tempDir := b.TempDir()

	fileSizes := []struct {
		name      string
		wordCount int
	}{
		{"Medium_10KB", 1000},
		{"Large_100KB", 10000},
		{"XLarge_1MB", 100000},
	}

	for _, size := range fileSizes {
		// Create test file
		content := generateBenchmarkContent(size.wordCount)
		testFile := filepath.Join(tempDir, fmt.Sprintf("streaming_test_%s.txt", size.name))
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("%s_Regular", size.name), func(b *testing.B) {
			p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
			defer p.Close()

			ctx := context.Background()
			b.SetBytes(int64(len(content)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				chunks, err := p.ParseFile(ctx, testFile)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(chunks)), "chunks_produced")
			}
		})

		b.Run(fmt.Sprintf("%s_Streaming", size.name), func(b *testing.B) {
			p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
			defer p.Close()

			ctx := context.Background()
			b.SetBytes(int64(len(content)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				result, err := p.ParseFileStream(ctx, testFile)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(result.Chunks)), "chunks_produced")
			}
		})
	}
}

// BenchmarkParserThroughputMeasurement measures parsing throughput in MB/s
func BenchmarkParserThroughputMeasurement(b *testing.B) {
	p := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer p.Close()

	testCases := []struct {
		name      string
		wordCount int
	}{
		{"Throughput_1KB", 100},
		{"Throughput_10KB", 1000},
		{"Throughput_100KB", 10000},
		{"Throughput_1MB", 100000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			content := generateBenchmarkContent(tc.wordCount)
			ctx := context.Background()

			b.SetBytes(int64(len(content)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				chunks, err := p.ParseText(ctx, content)
				if err != nil {
					b.Fatal(err)
				}
				if len(chunks) == 0 {
					b.Fatal("No chunks produced")
				}
			}
		})
	}
}

// Helper functions

func createAllFormatTestFiles(b *testing.B, dir string) map[string]string {
	files := make(map[string]string)

	// Text file
	txtContent := "This is a plain text file for benchmarking.\n\nIt contains multiple paragraphs with various content types.\n\nThis helps test the parser's ability to handle different text structures."
	txtFile := filepath.Join(dir, "test.txt")
	err := os.WriteFile(txtFile, []byte(txtContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files["txt"] = txtFile

	// Markdown file
	mdContent := "# Benchmark Test Document\n\nThis is a **markdown** file for testing.\n\n## Section 1\n\nSome content with `code` snippets.\n\n### Subsection\n\n- List item 1\n- List item 2\n- List item 3\n\n## Section 2\n\nMore content here with [links](http://example.com)."
	mdFile := filepath.Join(dir, "test.md")
	err = os.WriteFile(mdFile, []byte(mdContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files["markdown"] = mdFile

	// CSV file
	csvContent := "name,age,city,occupation\nJohn Doe,30,New York,Engineer\nJane Smith,25,Los Angeles,Designer\nBob Johnson,35,Chicago,Manager\nAlice Brown,28,Boston,Developer"
	csvFile := filepath.Join(dir, "test.csv")
	err = os.WriteFile(csvFile, []byte(csvContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files["csv"] = csvFile

	// JSON file
	jsonContent := `{
  "users": [
    {"name": "John", "age": 30, "skills": ["Go", "Python", "JavaScript"]},
    {"name": "Jane", "age": 25, "skills": ["React", "Node.js", "CSS"]},
    {"name": "Bob", "age": 35, "skills": ["Java", "Spring", "Docker"]}
  ],
  "metadata": {
    "version": "1.0",
    "created": "2024-01-01",
    "total_users": 3
  }
}`
	jsonFile := filepath.Join(dir, "test.json")
	err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		b.Fatal(err)
	}
	files["json"] = jsonFile

	return files
}

func createPerformanceBenchmarkTestFiles(b *testing.B, dir string, count int, wordCount int) []string {
	files := make([]string, count)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("comprehensive_benchmark_file_%d.txt", i)
		filePath := filepath.Join(dir, filename)

		content := generateComprehensiveBenchmarkContent(wordCount)
		fileContent := fmt.Sprintf("Comprehensive Benchmark File %d\n\n%s", i, content)

		err := os.WriteFile(filePath, []byte(fileContent), 0644)
		if err != nil {
			b.Fatal(err)
		}

		files[i] = filePath
	}

	return files
}

func generateComprehensiveBenchmarkContent(wordCount int) string {
	words := []string{
		"performance", "benchmark", "testing", "parser", "content",
		"processing", "analysis", "optimization", "efficiency", "throughput",
		"latency", "memory", "allocation", "concurrent", "parallel",
		"streaming", "caching", "chunking", "validation", "detection",
		"algorithm", "implementation", "architecture", "design", "system",
		"scalability", "reliability", "maintainability", "extensibility", "modularity",
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

		// Add paragraph breaks every 40-60 words
		if paragraphLength > 40 && (i%47 == 0 || paragraphLength > 60) {
			builder.WriteString("\n\n")
			paragraphLength = 0
		}
	}

	return builder.String()
}
