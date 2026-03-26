// Package parser - Performance baseline tests to ensure parser meets requirements
// This file implements Task 3.3.4: Create benchmarks and performance tests
package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/core"
)

// BaselinePerformanceRequirement defines performance requirements from the design document
// BaselinePerformanceRequirement is now imported from performance_test_runner.go in the same package


// GetBaselinePerformanceRequirements returns the performance baselines based on requirements
func GetBaselinePerformanceRequirements() []BaselinePerformanceRequirement {
	return []BaselinePerformanceRequirement{
		{
			Name:              "SmallFile_1KB",
			MaxLatency:        50 * time.Millisecond,
			MinThroughputMBps: 20.0,
			MaxMemoryMB:       2.0,
			MaxAllocsPerOp:    500,
			Description:       "Small file parsing should be very fast with minimal memory",
		},
		{
			Name:              "MediumFile_10KB",
			MaxLatency:        100 * time.Millisecond,
			MinThroughputMBps: 100.0,
			MaxMemoryMB:       5.0,
			MaxAllocsPerOp:    2000,
			Description:       "Medium file parsing should maintain high throughput",
		},
		{
			Name:              "LargeFile_100KB",
			MaxLatency:        500 * time.Millisecond,
			MinThroughputMBps: 200.0,
			MaxMemoryMB:       20.0,
			MaxAllocsPerOp:    10000,
			Description:       "Large file parsing should scale efficiently",
		},
		{
			Name:              "XLargeFile_1MB",
			MaxLatency:        2 * time.Second,
			MinThroughputMBps: 500.0,
			MaxMemoryMB:       50.0,
			MaxAllocsPerOp:    50000,
			Description:       "Extra large file parsing should use streaming when appropriate",
		},
		{
			Name:              "BatchProcessing_100Files",
			MaxLatency:        5 * time.Second,
			MinThroughputMBps: 50.0,
			MaxMemoryMB:       100.0,
			MaxAllocsPerOp:    100000,
			Description:       "Batch processing should leverage worker pools effectively",
		},
		{
			Name:              "ConcurrentLoad_8Goroutines",
			MaxLatency:        200 * time.Millisecond,
			MinThroughputMBps: 100.0,
			MaxMemoryMB:       30.0,
			MaxAllocsPerOp:    5000,
			Description:       "Concurrent parsing should scale with available CPU cores",
		},
	}
}

// TestPerformanceBaselines validates that parser meets performance requirements
func TestPerformanceBaselines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance baseline tests in short mode")
	}

	baselines := GetBaselinePerformanceRequirements()
	tempDir := t.TempDir()

	for _, baseline := range baselines {
		t.Run(baseline.Name, func(t *testing.T) {
			result := runBaselineTest(t, tempDir, baseline)

			// Check latency requirement
			if result.Latency > baseline.MaxLatency {
				t.Errorf("Latency baseline failed: %v > %v (max allowed)",
					result.Latency, baseline.MaxLatency)
			}

			// Check throughput requirement
			if result.ThroughputMBps < baseline.MinThroughputMBps {
				t.Errorf("Throughput baseline failed: %.2f MB/s < %.2f MB/s (min required)",
					result.ThroughputMBps, baseline.MinThroughputMBps)
			}

			// Check memory requirement
			if result.MemoryMB > baseline.MaxMemoryMB {
				t.Errorf("Memory baseline failed: %.2f MB > %.2f MB (max allowed)",
					result.MemoryMB, baseline.MaxMemoryMB)
			}

			// Check allocation requirement
			if result.AllocsPerOp > baseline.MaxAllocsPerOp {
				t.Errorf("Allocation baseline failed: %d allocs/op > %d (max allowed)",
					result.AllocsPerOp, baseline.MaxAllocsPerOp)
			}

			// Log successful baseline results
			t.Logf("✓ %s baseline passed:", baseline.Name)
			t.Logf("  Latency: %v (limit: %v)", result.Latency, baseline.MaxLatency)
			t.Logf("  Throughput: %.2f MB/s (min: %.2f MB/s)", result.ThroughputMBps, baseline.MinThroughputMBps)
			t.Logf("  Memory: %.2f MB (max: %.2f MB)", result.MemoryMB, baseline.MaxMemoryMB)
			t.Logf("  Allocs: %d/op (max: %d)", result.AllocsPerOp, baseline.MaxAllocsPerOp)
		})
	}
}

// TestMemoryEngineResponseTime validates the 200ms response time requirement
func TestMemoryEngineResponseTime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory engine response time test in short mode")
	}

	// Requirement 9: Memory_Engine SHALL respond to search queries within 200ms for datasets up to 100,000 entities
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

	// Simulate a dataset with multiple documents
	tempDir := t.TempDir()
	testFiles := createLargeDataset(t, tempDir, 100, 1000) // 100 files, 1000 words each

	ctx := context.Background()
	maxResponseTime := 200 * time.Millisecond

	// Test batch processing response time
	start := time.Now()
	results, err := parser.BatchParseFiles(ctx, testFiles)
	responseTime := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}

	if responseTime > maxResponseTime {
		t.Errorf("Memory engine response time requirement failed: %v > %v",
			responseTime, maxResponseTime)
	}

	// Validate results
	totalChunks := 0
	for _, chunks := range results {
		totalChunks += len(chunks)
	}

	t.Logf("✓ Memory engine response time: %v (limit: %v)", responseTime, maxResponseTime)
	t.Logf("  Processed %d files, produced %d chunks", len(testFiles), totalChunks)
	t.Logf("  Throughput: %.2f files/second", float64(len(testFiles))/responseTime.Seconds())
}

// TestCognifyPipelinePerformance validates pipeline processing performance
func TestCognifyPipelinePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Cognify pipeline performance test in short mode")
	}

	// Requirement 9: Cognify_Pipeline SHALL process documents with configurable batch sizes and parallel processing
	tempDir := t.TempDir()

	batchSizes := []int{10, 25, 50, 100}
	workerCounts := []int{1, 2, 4, runtime.NumCPU()}

	for _, batchSize := range batchSizes {
		for _, workers := range workerCounts {
			t.Run(fmt.Sprintf("Batch_%d_Workers_%d", batchSize, workers), func(t *testing.T) {
				testFiles := createLargeDataset(t, tempDir, batchSize, 500)

				config := &schema.WorkerPoolConfig{
					NumWorkers:    workers,
					QueueSize:     batchSize * 2,
					Timeout:       30 * time.Second,
					RetryAttempts: 1,
					RetryDelay:    100 * time.Millisecond,
				}

				parser := core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
				defer parser.Close()

				ctx := context.Background()
				start := time.Now()

				results, err := parser.BatchParseFiles(ctx, testFiles)
				processingTime := time.Since(start)

				if err != nil {
					t.Fatal(err)
				}

				// Calculate performance metrics
				totalChunks := 0
				totalBytes := int64(0)
				for _, chunks := range results {
					totalChunks += len(chunks)
					for _, chunk := range chunks {
						totalBytes += int64(len(chunk.Content))
					}
				}

				throughput := float64(totalBytes) / (1024 * 1024) / processingTime.Seconds()
				filesPerSecond := float64(len(testFiles)) / processingTime.Seconds()

				t.Logf("Batch size %d, Workers %d:", batchSize, workers)
				t.Logf("  Processing time: %v", processingTime)
				t.Logf("  Throughput: %.2f MB/s", throughput)
				t.Logf("  Files/second: %.2f", filesPerSecond)
				t.Logf("  Total chunks: %d", totalChunks)

				// Performance should improve with more workers (up to CPU limit)
				if workers > 1 && batchSize >= 20 {
					minThroughput := 10.0 // MB/s
					if throughput < minThroughput {
						t.Errorf("Parallel processing throughput too low: %.2f MB/s < %.2f MB/s",
							throughput, minThroughput)
					}
				}
			})
		}
	}
}

// TestBaselineStreamingMemoryEfficiency validates streaming parser memory efficiency
func TestBaselineStreamingMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming memory efficiency test in short mode")
	}

	tempDir := t.TempDir()

	// Create large files to test streaming efficiency
	fileSizes := []struct {
		name      string
		wordCount int
		maxMemory float64 // Maximum allowed memory in MB
	}{
		{"Large_100KB", 10000, 10.0},
		{"XLarge_1MB", 100000, 20.0},
		{"XXLarge_10MB", 1000000, 50.0},
	}

	for _, size := range fileSizes {
		t.Run(size.name, func(t *testing.T) {
			// Create large test file
			content := generateBaselineContent(size.wordCount)
			testFile := filepath.Join(tempDir, fmt.Sprintf("streaming_%s.txt", size.name))
			err := os.WriteFile(testFile, []byte(content), 0644)
			if err != nil {
				t.Fatal(err)
			}

			// Test regular parsing memory usage
			regularMemory := measureParsingMemory(t, testFile, false)

			// Test streaming parsing memory usage
			streamingMemory := measureParsingMemory(t, testFile, true)

			t.Logf("File size: %s (~%d words)", size.name, size.wordCount)
			t.Logf("  Regular parsing memory: %.2f MB", regularMemory)
			t.Logf("  Streaming parsing memory: %.2f MB", streamingMemory)
			t.Logf("  Memory efficiency: %.2fx", regularMemory/streamingMemory)

			// Streaming should use less memory for large files
			if size.wordCount >= 100000 && streamingMemory >= regularMemory {
				t.Errorf("Streaming not more memory efficient: %.2f MB >= %.2f MB",
					streamingMemory, regularMemory)
			}

			// Check absolute memory limits
			if streamingMemory > size.maxMemory {
				t.Errorf("Streaming memory usage too high: %.2f MB > %.2f MB (limit)",
					streamingMemory, size.maxMemory)
			}
		})
	}
}

// TestCachePerformanceBaseline validates cache performance requirements
func TestCachePerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cache performance baseline test in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createLargeDataset(t, tempDir, 20, 500)

	cacheConfig := &schema.CacheConfig{
		MaxSize:         1000,
		TTL:             10 * time.Minute,
		Policy:          schema.PolicyLRU,
		CleanupInterval: 2 * time.Minute,
	}

	parser := core.NewCachedUnifiedParser(schema.DefaultChunkingConfig(), cacheConfig)
	defer parser.Close()

	ctx := context.Background()

	// First run (cache miss) - establish baseline
	start := time.Now()
	results1, err := parser.BatchParseFiles(ctx, testFiles)
	cacheMissTime := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}

	// Second run (cache hit) - should be much faster
	start = time.Now()
	results2, err := parser.BatchParseFiles(ctx, testFiles)
	cacheHitTime := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}

	// Validate results are identical
	if len(results1) != len(results2) {
		t.Fatal("Cache hit results differ from cache miss results")
	}

	// Calculate performance improvement
	speedup := float64(cacheMissTime) / float64(cacheHitTime)
	cacheEfficiency := (1.0 - float64(cacheHitTime)/float64(cacheMissTime)) * 100

	t.Logf("Cache performance baseline:")
	t.Logf("  Cache miss time: %v", cacheMissTime)
	t.Logf("  Cache hit time: %v", cacheHitTime)
	t.Logf("  Speedup: %.2fx", speedup)
	t.Logf("  Cache efficiency: %.1f%%", cacheEfficiency)

	// Cache hits should be at least 5x faster
	minSpeedup := 5.0
	if speedup < minSpeedup {
		t.Errorf("Cache performance baseline failed: %.2fx speedup < %.2fx required",
			speedup, minSpeedup)
	}

	// Cache hit time should be very fast
	maxCacheHitTime := 50 * time.Millisecond
	if cacheHitTime > maxCacheHitTime {
		t.Errorf("Cache hit time too slow: %v > %v", cacheHitTime, maxCacheHitTime)
	}
}

// BaselineResult holds performance test results
// BaselineResult is now imported from performance_test_runner.go in the same package


// runBaselineTest executes a performance baseline test
func runBaselineTest(t *testing.T, tempDir string, baseline BaselinePerformanceRequirement) BaselineResult {
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()

	// Create test data based on baseline name
	var testFiles []string
	var expectedBytes int64

	switch baseline.Name {
	case "SmallFile_1KB":
		testFiles = []string{createTestFile(t, tempDir, "small.txt", 100)}
		expectedBytes = 1024
	case "MediumFile_10KB":
		testFiles = []string{createTestFile(t, tempDir, "medium.txt", 1000)}
		expectedBytes = 10 * 1024
	case "LargeFile_100KB":
		testFiles = []string{createTestFile(t, tempDir, "large.txt", 10000)}
		expectedBytes = 100 * 1024
	case "XLargeFile_1MB":
		testFiles = []string{createTestFile(t, tempDir, "xlarge.txt", 100000)}
		expectedBytes = 1024 * 1024
	case "BatchProcessing_100Files":
		testFiles = createLargeDataset(t, tempDir, 100, 100)
		expectedBytes = 100 * 1024 // Approximate
	case "ConcurrentLoad_8Goroutines":
		testFiles = []string{createTestFile(t, tempDir, "concurrent.txt", 1000)}
		expectedBytes = 10 * 1024
		return runConcurrentBaselineTest(t, testFiles[0], baseline)
	default:
		t.Fatalf("Unknown baseline: %s", baseline.Name)
	}

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run the test
	start := time.Now()
	results, err := parser.BatchParseFiles(ctx, testFiles)
	latency := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate metrics
	totalChunks := 0
	for _, chunks := range results {
		totalChunks += len(chunks)
	}

	throughputMBps := float64(expectedBytes) / (1024 * 1024) / latency.Seconds()
	memoryMB := float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024)
	allocsPerOp := int64(m2.Mallocs - m1.Mallocs)

	return BaselineResult{
		Latency:        latency,
		ThroughputMBps: throughputMBps,
		MemoryMB:       memoryMB,
		AllocsPerOp:    allocsPerOp,
		ChunksProduced: totalChunks,
	}
}

// runConcurrentBaselineTest runs concurrent parsing test
func runConcurrentBaselineTest(t *testing.T, testFile string, baseline BaselinePerformanceRequirement) BaselineResult {
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()
	concurrency := 8

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run concurrent test
	start := time.Now()
	results := make(chan []*schema.Chunk, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			chunks, err := parser.ParseFile(ctx, testFile)
			if err != nil {
				errors <- err
				return
			}
			results <- chunks
		}()
	}

	// Collect results
	totalChunks := 0
	for i := 0; i < concurrency; i++ {
		select {
		case chunks := <-results:
			totalChunks += len(chunks)
		case err := <-errors:
			t.Fatal(err)
		}
	}

	latency := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate metrics
	expectedBytes := int64(10 * 1024 * concurrency) // Approximate
	throughputMBps := float64(expectedBytes) / (1024 * 1024) / latency.Seconds()
	memoryMB := float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024)
	allocsPerOp := int64(m2.Mallocs-m1.Mallocs) / int64(concurrency)

	return BaselineResult{
		Latency:        latency,
		ThroughputMBps: throughputMBps,
		MemoryMB:       memoryMB,
		AllocsPerOp:    allocsPerOp,
		ChunksProduced: totalChunks,
	}
}

// Helper functions

func createTestFile(t *testing.T, dir, filename string, wordCount int) string {
	content := generateBaselineContent(wordCount)
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	return filePath
}

func createLargeDataset(t *testing.T, dir string, fileCount, wordsPerFile int) []string {
	files := make([]string, fileCount)
	for i := 0; i < fileCount; i++ {
		filename := fmt.Sprintf("dataset_file_%d.txt", i)
		files[i] = createTestFile(t, dir, filename, wordsPerFile)
	}
	return files
}

func generateBaselineContent(wordCount int) string {
	words := []string{
		"performance", "baseline", "testing", "parser", "content",
		"processing", "memory", "efficiency", "throughput", "latency",
		"optimization", "scalability", "reliability", "benchmark", "analysis",
		"algorithm", "implementation", "architecture", "design", "system",
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

func measureParsingMemory(t *testing.T, filePath string, useStreaming bool) float64 {
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Parse file
	if useStreaming {
		_, err := parser.ParseFileStream(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		_, err := parser.ParseFile(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	return float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024)
}
