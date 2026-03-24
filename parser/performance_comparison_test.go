// Package parser - Performance comparison tests between different configurations
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ComparisonResult holds performance comparison data
type ComparisonResult struct {
	Name             string        `json:"name"`
	Duration         time.Duration `json:"duration"`
	ThroughputMBps   float64       `json:"throughput_mbps"`
	MemoryUsageMB    float64       `json:"memory_usage_mb"`
	AllocationsPerOp int64         `json:"allocations_per_op"`
	ChunksProduced   int           `json:"chunks_produced"`
	FilesProcessed   int           `json:"files_processed"`
	SpeedupFactor    float64       `json:"speedup_factor"`
	MemoryEfficiency float64       `json:"memory_efficiency"`
}

// TestParserTypeComparison compares different parser implementations
func TestParserTypeComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping parser comparison tests in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createComparisonTestFiles(t, tempDir, 15, 1000)

	// Define parser configurations to compare
	parserConfigs := []struct {
		name  string
		setup func() (Parser, func())
	}{
		{
			"UnifiedParser_Default",
			func() (Parser, func()) {
				parser := NewUnifiedParser(DefaultChunkingConfig())
				return parser, func() { parser.Close() }
			},
		},
		{
			"UnifiedParser_SmallChunks",
			func() (Parser, func()) {
				config := &ChunkingConfig{
					Strategy: StrategyParagraph,
					MaxSize:  500,
					MinSize:  50,
					Overlap:  50,
				}
				parser := NewUnifiedParser(config)
				return parser, func() { parser.Close() }
			},
		},
		{
			"UnifiedParser_LargeChunks",
			func() (Parser, func()) {
				config := &ChunkingConfig{
					Strategy: StrategyParagraph,
					MaxSize:  2000,
					MinSize:  200,
					Overlap:  200,
				}
				parser := NewUnifiedParser(config)
				return parser, func() { parser.Close() }
			},
		},
		{
			"CachedParser_Default",
			func() (Parser, func()) {
				parser := NewCachedUnifiedParser(DefaultChunkingConfig(), DefaultCacheConfig())
				return parser, func() { parser.Close() }
			},
		},
	}

	results := make([]ComparisonResult, 0)
	baselineResult := ComparisonResult{}

	for i, config := range parserConfigs {
		t.Run(config.name, func(t *testing.T) {
			parser, cleanup := config.setup()
			defer cleanup()

			result := measureParserPerformance(t, parser, testFiles, config.name)
			results = append(results, result)

			if i == 0 {
				baselineResult = result
				result.SpeedupFactor = 1.0
			} else {
				result.SpeedupFactor = float64(baselineResult.Duration) / float64(result.Duration)
			}

			t.Logf("Performance for %s:", config.name)
			t.Logf("  Duration: %v", result.Duration)
			t.Logf("  Throughput: %.2f MB/s", result.ThroughputMBps)
			t.Logf("  Memory: %.2f MB", result.MemoryUsageMB)
			t.Logf("  Speedup: %.2fx", result.SpeedupFactor)
		})
	}

	// Generate comparison report
	generateComparisonReport(t, results)
}

// TestChunkingStrategyComparison compares different chunking strategies
func TestChunkingStrategyComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chunking strategy comparison in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createComparisonTestFiles(t, tempDir, 10, 2000)

	strategies := []struct {
		name   string
		config *ChunkingConfig
	}{
		{
			"Paragraph_Strategy",
			&ChunkingConfig{
				Strategy: StrategyParagraph,
				MaxSize:  1000,
				MinSize:  100,
				Overlap:  100,
			},
		},
		{
			"Sentence_Strategy",
			&ChunkingConfig{
				Strategy: StrategySentence,
				MaxSize:  1000,
				MinSize:  100,
				Overlap:  100,
			},
		},
		{
			"FixedSize_Strategy",
			&ChunkingConfig{
				Strategy: StrategyFixedSize,
				MaxSize:  1000,
				MinSize:  100,
				Overlap:  100,
			},
		},
	}

	results := make([]ComparisonResult, 0)

	for _, strategy := range strategies {
		t.Run(strategy.name, func(t *testing.T) {
			parser := NewUnifiedParser(strategy.config)
			defer parser.Close()

			result := measureParserPerformance(t, parser, testFiles, strategy.name)
			results = append(results, result)

			t.Logf("Strategy %s:", strategy.name)
			t.Logf("  Duration: %v", result.Duration)
			t.Logf("  Chunks produced: %d", result.ChunksProduced)
			t.Logf("  Throughput: %.2f MB/s", result.ThroughputMBps)
		})
	}

	// Find most efficient strategy
	mostEfficient := results[0]
	for _, result := range results[1:] {
		if result.ThroughputMBps > mostEfficient.ThroughputMBps {
			mostEfficient = result
		}
	}

	t.Logf("Most efficient chunking strategy: %s (%.2f MB/s)",
		mostEfficient.Name, mostEfficient.ThroughputMBps)
}

// TestWorkerPoolConfigComparison compares different worker pool configurations
func TestWorkerPoolConfigComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping worker pool comparison in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createComparisonTestFiles(t, tempDir, 30, 500)

	workerConfigs := []struct {
		name   string
		config *WorkerPoolConfig
	}{
		{
			"Sequential_1Worker",
			&WorkerPoolConfig{
				NumWorkers:    1,
				QueueSize:     10,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			},
		},
		{
			"Parallel_2Workers",
			&WorkerPoolConfig{
				NumWorkers:    2,
				QueueSize:     20,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			},
		},
		{
			"Parallel_4Workers",
			&WorkerPoolConfig{
				NumWorkers:    4,
				QueueSize:     40,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			},
		},
		{
			"Parallel_8Workers",
			&WorkerPoolConfig{
				NumWorkers:    8,
				QueueSize:     80,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			},
		},
		{
			"Parallel_MaxCPU",
			&WorkerPoolConfig{
				NumWorkers:    runtime.NumCPU(),
				QueueSize:     runtime.NumCPU() * 10,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			},
		},
	}

	results := make([]ComparisonResult, 0)
	baselineResult := ComparisonResult{}

	for i, config := range workerConfigs {
		t.Run(config.name, func(t *testing.T) {
			parser := NewUnifiedParserWithWorkerPool(DefaultChunkingConfig(), config.config)
			defer parser.Close()

			result := measureWorkerPoolPerformance(t, parser, testFiles, config.name)
			results = append(results, result)

			if i == 0 {
				baselineResult = result
				result.SpeedupFactor = 1.0
			} else {
				result.SpeedupFactor = float64(baselineResult.Duration) / float64(result.Duration)
			}

			t.Logf("Worker pool %s:", config.name)
			t.Logf("  Duration: %v", result.Duration)
			t.Logf("  Speedup: %.2fx", result.SpeedupFactor)
			t.Logf("  Workers: %d", config.config.NumWorkers)
		})
	}

	// Find optimal worker count
	optimalResult := results[0]
	for _, result := range results[1:] {
		if result.SpeedupFactor > optimalResult.SpeedupFactor {
			optimalResult = result
		}
	}

	t.Logf("Optimal worker configuration: %s (%.2fx speedup)",
		optimalResult.Name, optimalResult.SpeedupFactor)
}

// TestCacheConfigComparison compares different cache configurations
func TestCacheConfigComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cache configuration comparison in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createComparisonTestFiles(t, tempDir, 20, 800)

	cacheConfigs := []struct {
		name   string
		config *CacheConfig
	}{
		{
			"NoCache",
			nil, // Will use uncached parser
		},
		{
			"SmallCache_LRU",
			&CacheConfig{
				MaxSize:         100,
				TTL:             5 * time.Minute,
				Policy:          PolicyLRU,
				CleanupInterval: 1 * time.Minute,
			},
		},
		{
			"MediumCache_LRU",
			&CacheConfig{
				MaxSize:         500,
				TTL:             10 * time.Minute,
				Policy:          PolicyLRU,
				CleanupInterval: 2 * time.Minute,
			},
		},
		{
			"LargeCache_LRU",
			&CacheConfig{
				MaxSize:         1000,
				TTL:             15 * time.Minute,
				Policy:          PolicyLRU,
				CleanupInterval: 3 * time.Minute,
			},
		},
		{
			"MediumCache_LFU",
			&CacheConfig{
				MaxSize:         500,
				TTL:             10 * time.Minute,
				Policy:          PolicyLFU,
				CleanupInterval: 2 * time.Minute,
			},
		},
	}

	results := make([]ComparisonResult, 0)

	for _, config := range cacheConfigs {
		t.Run(config.name, func(t *testing.T) {
			var parser Parser
			var cleanup func()

			if config.config == nil {
				// No cache
				uncachedParser := NewUnifiedParser(DefaultChunkingConfig())
				parser = uncachedParser
				cleanup = func() { uncachedParser.Close() }
			} else {
				// With cache
				cachedParser := NewCachedUnifiedParser(DefaultChunkingConfig(), config.config)
				parser = cachedParser
				cleanup = func() { cachedParser.Close() }
			}
			defer cleanup()

			result := measureCachePerformance(t, parser, testFiles, config.name)
			results = append(results, result)

			t.Logf("Cache config %s:", config.name)
			t.Logf("  Duration: %v", result.Duration)
			t.Logf("  Throughput: %.2f MB/s", result.ThroughputMBps)
			t.Logf("  Memory efficiency: %.2f%%", result.MemoryEfficiency)
		})
	}

	// Find best cache configuration
	bestResult := results[0]
	for _, result := range results[1:] {
		if result.ThroughputMBps > bestResult.ThroughputMBps {
			bestResult = result
		}
	}

	t.Logf("Best cache configuration: %s (%.2f MB/s)",
		bestResult.Name, bestResult.ThroughputMBps)
}

// TestStreamingVsRegularComparison compares streaming vs regular parsing
func TestStreamingVsRegularComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming comparison in short mode")
	}

	tempDir := t.TempDir()

	// Create files of different sizes
	fileSizes := []struct {
		name      string
		wordCount int
	}{
		{"Small_1KB", 100},
		{"Medium_10KB", 1000},
		{"Large_100KB", 10000},
		{"XLarge_1MB", 100000},
	}

	for _, size := range fileSizes {
		t.Run(size.name, func(t *testing.T) {
			// Create test file
			testFile := createLargeTestFile(t, tempDir, size.name, size.wordCount)

			// Test regular parsing
			regularResult := measureRegularParsingPerformance(t, testFile, fmt.Sprintf("Regular_%s", size.name))

			// Test streaming parsing
			streamingResult := measureStreamingParsingPerformance(t, testFile, fmt.Sprintf("Streaming_%s", size.name))

			// Compare results
			speedup := float64(regularResult.Duration) / float64(streamingResult.Duration)
			memoryEfficiency := regularResult.MemoryUsageMB / streamingResult.MemoryUsageMB

			t.Logf("Comparison for %s:", size.name)
			t.Logf("  Regular parsing: %v (%.2f MB memory)", regularResult.Duration, regularResult.MemoryUsageMB)
			t.Logf("  Streaming parsing: %v (%.2f MB memory)", streamingResult.Duration, streamingResult.MemoryUsageMB)
			t.Logf("  Streaming speedup: %.2fx", speedup)
			t.Logf("  Memory efficiency: %.2fx", memoryEfficiency)

			// For large files, streaming should be more memory efficient
			if size.wordCount >= 10000 && memoryEfficiency < 2.0 {
				t.Logf("Warning: Streaming not significantly more memory efficient for large files")
			}
		})
	}
}

// Helper functions

func measureParserPerformance(t *testing.T, parser Parser, testFiles []string, name string) ComparisonResult {
	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run test
	start := time.Now()
	totalChunks := 0
	totalBytes := int64(0)

	for _, filePath := range testFiles {
		chunks, err := parser.ParseFile(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
		totalChunks += len(chunks)
		for _, chunk := range chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}
	duration := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	memoryUsed := m2.TotalAlloc - m1.TotalAlloc

	return ComparisonResult{
		Name:             name,
		Duration:         duration,
		ThroughputMBps:   float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:    float64(memoryUsed) / (1024 * 1024),
		AllocationsPerOp: int64(m2.Mallocs - m1.Mallocs),
		ChunksProduced:   totalChunks,
		FilesProcessed:   len(testFiles),
		MemoryEfficiency: float64(totalBytes) / float64(memoryUsed) * 100,
	}
}

func measureWorkerPoolPerformance(t *testing.T, parser *UnifiedParser, testFiles []string, name string) ComparisonResult {
	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run test
	start := time.Now()
	results, err := parser.BatchParseFiles(ctx, testFiles)
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate totals
	totalChunks := 0
	totalBytes := int64(0)
	for _, chunks := range results {
		totalChunks += len(chunks)
		for _, chunk := range chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}

	memoryUsed := m2.TotalAlloc - m1.TotalAlloc

	return ComparisonResult{
		Name:             name,
		Duration:         duration,
		ThroughputMBps:   float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:    float64(memoryUsed) / (1024 * 1024),
		AllocationsPerOp: int64(m2.Mallocs - m1.Mallocs),
		ChunksProduced:   totalChunks,
		FilesProcessed:   len(testFiles),
		MemoryEfficiency: float64(totalBytes) / float64(memoryUsed) * 100,
	}
}

func measureCachePerformance(t *testing.T, parser Parser, testFiles []string, name string) ComparisonResult {
	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run test twice to test cache effectiveness
	start := time.Now()
	totalChunks := 0
	totalBytes := int64(0)

	// First run (cache miss)
	for _, filePath := range testFiles {
		chunks, err := parser.ParseFile(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
		totalChunks += len(chunks)
		for _, chunk := range chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}

	// Second run (cache hit)
	for _, filePath := range testFiles {
		chunks, err := parser.ParseFile(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
		totalChunks += len(chunks)
		for _, chunk := range chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}

	duration := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	memoryUsed := m2.TotalAlloc - m1.TotalAlloc

	return ComparisonResult{
		Name:             name,
		Duration:         duration,
		ThroughputMBps:   float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:    float64(memoryUsed) / (1024 * 1024),
		AllocationsPerOp: int64(m2.Mallocs - m1.Mallocs),
		ChunksProduced:   totalChunks,
		FilesProcessed:   len(testFiles) * 2, // Two runs
		MemoryEfficiency: float64(totalBytes) / float64(memoryUsed) * 100,
	}
}

func measureRegularParsingPerformance(t *testing.T, filePath, name string) ComparisonResult {
	parser := NewUnifiedParser(DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run test
	start := time.Now()
	chunks, err := parser.ParseFile(ctx, filePath)
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	totalBytes := int64(0)
	for _, chunk := range chunks {
		totalBytes += int64(len(chunk.Content))
	}

	memoryUsed := m2.TotalAlloc - m1.TotalAlloc

	return ComparisonResult{
		Name:             name,
		Duration:         duration,
		ThroughputMBps:   float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:    float64(memoryUsed) / (1024 * 1024),
		AllocationsPerOp: int64(m2.Mallocs - m1.Mallocs),
		ChunksProduced:   len(chunks),
		FilesProcessed:   1,
		MemoryEfficiency: float64(totalBytes) / float64(memoryUsed) * 100,
	}
}

func measureStreamingParsingPerformance(t *testing.T, filePath, name string) ComparisonResult {
	parser := NewStreamingParser(DefaultStreamingConfig(), DefaultChunkingConfig())

	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run test
	start := time.Now()
	result, err := parser.ParseFileStream(ctx, filePath)
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	totalBytes := int64(0)
	for _, chunk := range result.Chunks {
		totalBytes += int64(len(chunk.Content))
	}

	memoryUsed := m2.TotalAlloc - m1.TotalAlloc

	return ComparisonResult{
		Name:             name,
		Duration:         duration,
		ThroughputMBps:   float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:    float64(memoryUsed) / (1024 * 1024),
		AllocationsPerOp: int64(m2.Mallocs - m1.Mallocs),
		ChunksProduced:   len(result.Chunks),
		FilesProcessed:   1,
		MemoryEfficiency: float64(totalBytes) / float64(memoryUsed) * 100,
	}
}

func generateComparisonReport(t *testing.T, results []ComparisonResult) {
	t.Logf("\n=== Performance Comparison Report ===")

	// Find best and worst performers
	fastest := results[0]
	slowest := results[0]
	mostEfficient := results[0]
	leastEfficient := results[0]

	for _, result := range results[1:] {
		if result.Duration < fastest.Duration {
			fastest = result
		}
		if result.Duration > slowest.Duration {
			slowest = result
		}
		if result.ThroughputMBps > mostEfficient.ThroughputMBps {
			mostEfficient = result
		}
		if result.ThroughputMBps < leastEfficient.ThroughputMBps {
			leastEfficient = result
		}
	}

	t.Logf("Fastest: %s (%v)", fastest.Name, fastest.Duration)
	t.Logf("Slowest: %s (%v)", slowest.Name, slowest.Duration)
	t.Logf("Most efficient: %s (%.2f MB/s)", mostEfficient.Name, mostEfficient.ThroughputMBps)
	t.Logf("Least efficient: %s (%.2f MB/s)", leastEfficient.Name, leastEfficient.ThroughputMBps)

	// Performance ranking
	t.Logf("\nPerformance Ranking (by throughput):")
	for i, result := range results {
		t.Logf("%d. %s: %.2f MB/s", i+1, result.Name, result.ThroughputMBps)
	}
}

func createComparisonTestFiles(t *testing.T, dir string, count int, wordCount int) []string {
	files := make([]string, count)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("comparison_test_%d.txt", i)
		filePath := filepath.Join(dir, filename)

		content := generateComparisonContent(wordCount, i)

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		files[i] = filePath
	}

	return files
}

func createLargeTestFile(t *testing.T, dir, name string, wordCount int) string {
	filename := fmt.Sprintf("large_%s.txt", name)
	filePath := filepath.Join(dir, filename)

	content := generateComparisonContent(wordCount, 0)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return filePath
}

func generateComparisonContent(wordCount int, seed int) string {
	words := []string{
		"comparison", "performance", "benchmark", "testing", "analysis",
		"optimization", "efficiency", "throughput", "latency", "memory",
		"processing", "parsing", "chunking", "streaming", "caching",
		"concurrent", "parallel", "sequential", "algorithm", "data",
	}

	content := fmt.Sprintf("Performance comparison test file %d\n\n", seed)
	paragraphLength := 0

	for i := 0; i < wordCount; i++ {
		word := words[(i+seed)%len(words)]
		content += word
		paragraphLength++

		if i < wordCount-1 {
			content += " "
		}

		// Add paragraph breaks
		if paragraphLength > 45 && (i%53 == 0 || paragraphLength > 65) {
			content += "\n\n"
			paragraphLength = 0
		}
	}

	return content
}
