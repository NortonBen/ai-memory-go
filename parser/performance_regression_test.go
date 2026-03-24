// Package parser - Performance regression tests to detect performance degradation
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

// PerformanceBaseline defines expected performance characteristics
type PerformanceBaseline struct {
	MaxLatency        time.Duration
	MinThroughputMBps float64
	MaxMemoryMB       float64
	MaxAllocsPerOp    int64
}

// TestPerformanceRegression runs regression tests against performance baselines
func TestPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression tests in short mode")
	}

	tempDir := t.TempDir()

	// Define performance baselines for different scenarios
	baselines := map[string]PerformanceBaseline{
		"SmallFile": {
			MaxLatency:        50 * time.Millisecond,
			MinThroughputMBps: 10.0,
			MaxMemoryMB:       5.0,
			MaxAllocsPerOp:    1000,
		},
		"MediumFile": {
			MaxLatency:        200 * time.Millisecond,
			MinThroughputMBps: 20.0,
			MaxMemoryMB:       20.0,
			MaxAllocsPerOp:    5000,
		},
		"LargeFile": {
			MaxLatency:        1 * time.Second,
			MinThroughputMBps: 50.0,
			MaxMemoryMB:       100.0,
			MaxAllocsPerOp:    20000,
		},
		"BatchProcessing": {
			MaxLatency:        5 * time.Second,
			MinThroughputMBps: 30.0,
			MaxMemoryMB:       200.0,
			MaxAllocsPerOp:    50000,
		},
	}

	testCases := []struct {
		name     string
		setup    func(string) ([]string, int64)
		baseline string
	}{
		{
			"SmallFile",
			func(dir string) ([]string, int64) {
				return createRegressionTestFiles(t, dir, 1, 100), 100 * 10 // ~1KB
			},
			"SmallFile",
		},
		{
			"MediumFile",
			func(dir string) ([]string, int64) {
				return createRegressionTestFiles(t, dir, 1, 1000), 1000 * 10 // ~10KB
			},
			"MediumFile",
		},
		{
			"LargeFile",
			func(dir string) ([]string, int64) {
				return createRegressionTestFiles(t, dir, 1, 10000), 10000 * 10 // ~100KB
			},
			"LargeFile",
		},
		{
			"BatchProcessing",
			func(dir string) ([]string, int64) {
				return createRegressionTestFiles(t, dir, 20, 500), 20 * 500 * 10 // ~100KB total
			},
			"BatchProcessing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testFiles, expectedBytes := tc.setup(tempDir)
			baseline := baselines[tc.baseline]

			// Run performance test
			result := runPerformanceTest(t, testFiles, expectedBytes)

			// Check against baselines
			if result.Latency > baseline.MaxLatency {
				t.Errorf("Latency regression: %v > %v (baseline)", result.Latency, baseline.MaxLatency)
			}

			if result.ThroughputMBps < baseline.MinThroughputMBps {
				t.Errorf("Throughput regression: %.2f MB/s < %.2f MB/s (baseline)",
					result.ThroughputMBps, baseline.MinThroughputMBps)
			}

			if result.MemoryMB > baseline.MaxMemoryMB {
				t.Errorf("Memory regression: %.2f MB > %.2f MB (baseline)",
					result.MemoryMB, baseline.MaxMemoryMB)
			}

			if result.AllocsPerOp > baseline.MaxAllocsPerOp {
				t.Errorf("Allocation regression: %d allocs/op > %d (baseline)",
					result.AllocsPerOp, baseline.MaxAllocsPerOp)
			}

			// Log performance metrics for tracking
			t.Logf("Performance Metrics for %s:", tc.name)
			t.Logf("  Latency: %v (baseline: %v)", result.Latency, baseline.MaxLatency)
			t.Logf("  Throughput: %.2f MB/s (baseline: %.2f MB/s)", result.ThroughputMBps, baseline.MinThroughputMBps)
			t.Logf("  Memory: %.2f MB (baseline: %.2f MB)", result.MemoryMB, baseline.MaxMemoryMB)
			t.Logf("  Allocs/op: %d (baseline: %d)", result.AllocsPerOp, baseline.MaxAllocsPerOp)
		})
	}
}

// TestConcurrencyRegression tests for performance degradation under concurrent load
func TestConcurrencyRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency regression tests in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createRegressionTestFiles(t, tempDir, 10, 500)

	concurrencyLevels := []int{1, 2, 4, 8}
	expectedSpeedup := map[int]float64{
		1: 1.0,
		2: 1.5, // Expect at least 1.5x speedup with 2 workers
		4: 2.5, // Expect at least 2.5x speedup with 4 workers
		8: 3.0, // Expect at least 3.0x speedup with 8 workers (diminishing returns)
	}

	// Measure baseline (sequential) performance
	baselineTime := measureSequentialPerformance(t, testFiles)

	for _, concurrency := range concurrencyLevels {
		if concurrency == 1 {
			continue // Skip baseline
		}

		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			parallelTime := measureParallelPerformance(t, testFiles, concurrency)
			actualSpeedup := float64(baselineTime) / float64(parallelTime)
			expectedMin := expectedSpeedup[concurrency]

			if actualSpeedup < expectedMin {
				t.Errorf("Concurrency regression: %.2fx speedup < %.2fx expected with %d workers",
					actualSpeedup, expectedMin, concurrency)
			}

			t.Logf("Concurrency %d: %.2fx speedup (expected: %.2fx)",
				concurrency, actualSpeedup, expectedMin)
		})
	}
}

// TestMemoryLeakRegression tests for memory leaks during extended operation
func TestMemoryLeakRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak regression tests in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createRegressionTestFiles(t, tempDir, 5, 1000)

	parser := NewUnifiedParser(DefaultChunkingConfig())
	defer parser.Close()

	// Measure initial memory
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	initialMemory := m1.Alloc

	// Run parsing operations multiple times
	iterations := 100
	ctx := context.Background()

	for i := 0; i < iterations; i++ {
		_, err := parser.BatchParseFiles(ctx, testFiles)
		if err != nil {
			t.Fatal(err)
		}

		// Force GC every 10 iterations
		if i%10 == 0 {
			runtime.GC()
		}
	}

	// Measure final memory
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	finalMemory := m2.Alloc

	// Check for memory growth
	memoryGrowth := int64(finalMemory) - int64(initialMemory)
	maxAllowedGrowth := int64(50 * 1024 * 1024) // 50MB

	if memoryGrowth > maxAllowedGrowth {
		t.Errorf("Memory leak detected: %d bytes growth > %d bytes allowed",
			memoryGrowth, maxAllowedGrowth)
	}

	t.Logf("Memory usage after %d iterations:", iterations)
	t.Logf("  Initial: %d bytes", initialMemory)
	t.Logf("  Final: %d bytes", finalMemory)
	t.Logf("  Growth: %d bytes", memoryGrowth)
}

// TestCacheRegressionPerformance tests cache performance regression
func TestCacheRegressionPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cache regression tests in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createRegressionTestFiles(t, tempDir, 10, 500)

	// Test cache hit performance
	t.Run("CacheHitPerformance", func(t *testing.T) {
		parser := NewCachedUnifiedParser(DefaultChunkingConfig(), DefaultCacheConfig())
		defer parser.Close()

		ctx := context.Background()

		// Pre-populate cache
		for _, filePath := range testFiles {
			_, err := parser.ParseFile(ctx, filePath)
			if err != nil {
				t.Fatal(err)
			}
		}

		// Measure cache hit performance
		start := time.Now()
		for i := 0; i < 100; i++ {
			for _, filePath := range testFiles {
				_, err := parser.ParseFile(ctx, filePath)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		cacheHitTime := time.Since(start)

		// Cache hits should be very fast
		maxExpectedTime := 100 * time.Millisecond
		if cacheHitTime > maxExpectedTime {
			t.Errorf("Cache hit performance regression: %v > %v", cacheHitTime, maxExpectedTime)
		}

		t.Logf("Cache hit performance: %v for %d operations", cacheHitTime, 100*len(testFiles))
	})

	// Test cache miss performance
	t.Run("CacheMissPerformance", func(t *testing.T) {
		parser := NewCachedUnifiedParser(DefaultChunkingConfig(), DefaultCacheConfig())
		defer parser.Close()

		ctx := context.Background()

		// Measure cache miss performance (first-time parsing)
		start := time.Now()
		for _, filePath := range testFiles {
			_, err := parser.ParseFile(ctx, filePath)
			if err != nil {
				t.Fatal(err)
			}
		}
		cacheMissTime := time.Since(start)

		// Cache misses should not be significantly slower than uncached parsing
		uncachedParser := NewUnifiedParser(DefaultChunkingConfig())
		defer uncachedParser.Close()

		start = time.Now()
		for _, filePath := range testFiles {
			_, err := uncachedParser.ParseFile(ctx, filePath)
			if err != nil {
				t.Fatal(err)
			}
		}
		uncachedTime := time.Since(start)

		// Cache miss should not be more than 20% slower than uncached
		maxAllowedOverhead := float64(uncachedTime) * 1.2
		if float64(cacheMissTime) > maxAllowedOverhead {
			t.Errorf("Cache miss performance regression: %v > %.0f%% of uncached time (%v)",
				cacheMissTime, maxAllowedOverhead/float64(uncachedTime)*100, uncachedTime)
		}

		t.Logf("Cache miss vs uncached: %v vs %v (%.1f%% overhead)",
			cacheMissTime, uncachedTime, float64(cacheMissTime)/float64(uncachedTime)*100-100)
	})
}

// PerformanceResult holds performance test results
type PerformanceResult struct {
	Latency        time.Duration
	ThroughputMBps float64
	MemoryMB       float64
	AllocsPerOp    int64
}

// runPerformanceTest runs a comprehensive performance test
func runPerformanceTest(t *testing.T, testFiles []string, expectedBytes int64) PerformanceResult {
	parser := NewUnifiedParser(DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Measure latency
	start := time.Now()
	_, err := parser.BatchParseFiles(ctx, testFiles)
	if err != nil {
		t.Fatal(err)
	}
	latency := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate metrics
	throughputMBps := float64(expectedBytes) / (1024 * 1024) / latency.Seconds()
	memoryMB := float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024)
	allocsPerOp := int64(m2.Mallocs - m1.Mallocs)

	return PerformanceResult{
		Latency:        latency,
		ThroughputMBps: throughputMBps,
		MemoryMB:       memoryMB,
		AllocsPerOp:    allocsPerOp,
	}
}

// measureSequentialPerformance measures baseline sequential performance
func measureSequentialPerformance(t *testing.T, testFiles []string) time.Duration {
	parser := NewUnifiedParser(DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()
	start := time.Now()

	for _, filePath := range testFiles {
		_, err := parser.ParseFile(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
	}

	return time.Since(start)
}

// measureParallelPerformance measures parallel performance with specified concurrency
func measureParallelPerformance(t *testing.T, testFiles []string, concurrency int) time.Duration {
	config := &WorkerPoolConfig{
		NumWorkers:    concurrency,
		QueueSize:     concurrency * 5,
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    100 * time.Millisecond,
	}

	parser := NewUnifiedParserWithWorkerPool(DefaultChunkingConfig(), config)
	defer parser.Close()

	ctx := context.Background()
	start := time.Now()

	_, err := parser.BatchParseFiles(ctx, testFiles)
	if err != nil {
		t.Fatal(err)
	}

	return time.Since(start)
}

// createRegressionTestFiles creates test files for regression testing
func createRegressionTestFiles(t *testing.T, dir string, count int, wordCount int) []string {
	files := make([]string, count)

	content := generateRegressionContent(wordCount)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("regression_test_%d.txt", i)
		filePath := filepath.Join(dir, filename)

		fileContent := fmt.Sprintf("Regression test file %d\n\n%s", i, content)

		err := os.WriteFile(filePath, []byte(fileContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		files[i] = filePath
	}

	return files
}

// generateRegressionContent generates consistent content for regression testing
func generateRegressionContent(wordCount int) string {
	// Use consistent content for reproducible performance tests
	words := []string{
		"performance", "regression", "testing", "benchmark", "parser",
		"content", "processing", "memory", "allocation", "throughput",
		"latency", "concurrency", "scaling", "optimization", "efficiency",
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

		// Add paragraph breaks every 30-50 words for consistent structure
		if paragraphLength > 30 && (i%37 == 0 || paragraphLength > 50) {
			content += "\n\n"
			paragraphLength = 0
		}
	}

	return content
}
