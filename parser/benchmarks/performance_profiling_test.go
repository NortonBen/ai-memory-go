// Package parser - Performance profiling integration tests
// This file implements Task 3.3.4: Create benchmarks and performance tests
package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/core"
)

type Parser = schema.Parser
type UnifiedParser = core.UnifiedParser
type CachedUnifiedParser = core.CachedUnifiedParser

// TestCPUProfiling tests CPU profiling integration for performance analysis
func TestCPUProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CPU profiling test in short mode")
	}

	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "profiles")
	err := os.MkdirAll(profileDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create test files for profiling
	testFiles := createProfilingTestFiles(t, tempDir, 50, 1000)

	// CPU profiling test scenarios
	scenarios := []struct {
		name        string
		setupParser func() Parser
		cleanup     func(Parser)
	}{
		{
			"UnifiedParser_Sequential",
			func() Parser {
				return core.NewUnifiedParser(schema.DefaultChunkingConfig())
			},
			func(p Parser) {
				if up, ok := p.(*UnifiedParser); ok {
					up.Close()
				}
			},
		},
		{
			"UnifiedParser_Parallel",
			func() Parser {
				config := &schema.WorkerPoolConfig{
					NumWorkers:    runtime.NumCPU(),
					QueueSize:     100,
					Timeout:       30 * time.Second,
					RetryAttempts: 1,
					RetryDelay:    100 * time.Millisecond,
				}
				return core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			},
			func(p Parser) {
				if up, ok := p.(*UnifiedParser); ok {
					up.Close()
				}
			},
		},
		{
			"CachedParser_WithCache",
			func() Parser {
				cacheConfig := &schema.CacheConfig{
					MaxSize:         500,
					TTL:             10 * time.Minute,
					Policy:          schema.PolicyLRU,
					CleanupInterval: 2 * time.Minute,
				}
				return core.NewCachedUnifiedParser(schema.DefaultChunkingConfig(), cacheConfig)
			},
			func(p Parser) {
				if cp, ok := p.(*CachedUnifiedParser); ok {
					cp.Close()
				}
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			parser := scenario.setupParser()
			defer scenario.cleanup(parser)

			// Create CPU profile
			cpuProfilePath := filepath.Join(profileDir, fmt.Sprintf("cpu_%s.prof", scenario.name))
			cpuFile, err := os.Create(cpuProfilePath)
			if err != nil {
				t.Fatal(err)
			}
			defer cpuFile.Close()

			// Start CPU profiling
			if err := pprof.StartCPUProfile(cpuFile); err != nil {
				t.Fatal(err)
			}

			// Run parsing operations
			ctx := context.Background()
			start := time.Now()

			if up, ok := parser.(*UnifiedParser); ok {
				// Use batch parsing for UnifiedParser
				_, err = up.BatchParseFiles(ctx, testFiles)
			} else {
				// Sequential parsing for other parsers
				for _, filePath := range testFiles {
					_, err = parser.ParseFile(ctx, filePath)
					if err != nil {
						break
					}
				}
			}

			duration := time.Since(start)
			pprof.StopCPUProfile()

			if err != nil {
				t.Fatal(err)
			}

			t.Logf("CPU profiling completed for %s:", scenario.name)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Files processed: %d", len(testFiles))
			t.Logf("  Profile saved to: %s", cpuProfilePath)
		})
	}
}

// TestMemoryProfiling tests memory profiling integration
func TestMemoryProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profiling test in short mode")
	}

	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "profiles")
	err := os.MkdirAll(profileDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create test files with different sizes for memory analysis
	testCases := []struct {
		name      string
		fileCount int
		wordCount int
	}{
		{"SmallFiles", 20, 100},
		{"MediumFiles", 10, 1000},
		{"LargeFiles", 5, 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testFiles := createProfilingTestFiles(t, tempDir, tc.fileCount, tc.wordCount)
			parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
			defer parser.Close()

			// Create memory profile before
			memProfileBeforePath := filepath.Join(profileDir, fmt.Sprintf("mem_before_%s.prof", tc.name))
			memFileBefore, err := os.Create(memProfileBeforePath)
			if err != nil {
				t.Fatal(err)
			}

			runtime.GC()
			if err := pprof.WriteHeapProfile(memFileBefore); err != nil {
				t.Fatal(err)
			}
			memFileBefore.Close()

			// Measure memory before
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Run parsing operations
			ctx := context.Background()
			start := time.Now()
			results, err := parser.BatchParseFiles(ctx, testFiles)
			duration := time.Since(start)

			if err != nil {
				t.Fatal(err)
			}

			// Measure memory after
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			// Create memory profile after
			memProfileAfterPath := filepath.Join(profileDir, fmt.Sprintf("mem_after_%s.prof", tc.name))
			memFileAfter, err := os.Create(memProfileAfterPath)
			if err != nil {
				t.Fatal(err)
			}

			runtime.GC()
			if err := pprof.WriteHeapProfile(memFileAfter); err != nil {
				t.Fatal(err)
			}
			memFileAfter.Close()

			// Calculate metrics
			totalChunks := 0
			for _, chunks := range results {
				totalChunks += len(chunks)
			}

			memoryUsed := m2.TotalAlloc - m1.TotalAlloc
			memoryPerFile := float64(memoryUsed) / float64(len(testFiles))

			t.Logf("Memory profiling completed for %s:", tc.name)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Files processed: %d", len(testFiles))
			t.Logf("  Total chunks: %d", totalChunks)
			t.Logf("  Memory used: %d bytes", memoryUsed)
			t.Logf("  Memory per file: %.2f KB", memoryPerFile/1024)
			t.Logf("  Profile before: %s", memProfileBeforePath)
			t.Logf("  Profile after: %s", memProfileAfterPath)
		})
	}
}

// TestGoroutineProfiling tests goroutine profiling for concurrency analysis
func TestGoroutineProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping goroutine profiling test in short mode")
	}

	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "profiles")
	err := os.MkdirAll(profileDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	testFiles := createProfilingTestFiles(t, tempDir, 30, 500)

	workerCounts := []int{1, 2, 4, 8, runtime.NumCPU()}

	for _, workers := range workerCounts {
		t.Run(fmt.Sprintf("Workers_%d", workers), func(t *testing.T) {
			config := &schema.WorkerPoolConfig{
				NumWorkers:    workers,
				QueueSize:     len(testFiles) * 2,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			}

			parser := core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			defer parser.Close()

			// Create goroutine profile before
			goroutineProfileBeforePath := filepath.Join(profileDir, fmt.Sprintf("goroutine_before_w%d.prof", workers))
			goroutineFileBefore, err := os.Create(goroutineProfileBeforePath)
			if err != nil {
				t.Fatal(err)
			}

			if err := pprof.Lookup("goroutine").WriteTo(goroutineFileBefore, 0); err != nil {
				t.Fatal(err)
			}
			goroutineFileBefore.Close()

			// Run parsing with worker pool
			ctx := context.Background()
			start := time.Now()
			results, err := parser.BatchParseFiles(ctx, testFiles)
			duration := time.Since(start)

			if err != nil {
				t.Fatal(err)
			}

			// Create goroutine profile after
			goroutineProfileAfterPath := filepath.Join(profileDir, fmt.Sprintf("goroutine_after_w%d.prof", workers))
			goroutineFileAfter, err := os.Create(goroutineProfileAfterPath)
			if err != nil {
				t.Fatal(err)
			}

			if err := pprof.Lookup("goroutine").WriteTo(goroutineFileAfter, 0); err != nil {
				t.Fatal(err)
			}
			goroutineFileAfter.Close()

			// Get worker pool metrics
			metrics := parser.GetWorkerPoolMetrics()

			totalChunks := 0
			for _, chunks := range results {
				totalChunks += len(chunks)
			}

			t.Logf("Goroutine profiling completed for %d workers:", workers)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Files processed: %d", len(testFiles))
			t.Logf("  Total chunks: %d", totalChunks)
			t.Logf("  Tasks completed: %d", metrics.TasksCompleted)
			t.Logf("  Tasks failed: %d", metrics.TasksFailed)
			t.Logf("  Average processing time: %v", metrics.AverageProcessingTime)
			t.Logf("  Profile before: %s", goroutineProfileBeforePath)
			t.Logf("  Profile after: %s", goroutineProfileAfterPath)
		})
	}
}

// TestBlockingProfiling tests blocking profiling for synchronization analysis
func TestBlockingProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping blocking profiling test in short mode")
	}

	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "profiles")
	err := os.MkdirAll(profileDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Enable block profiling
	runtime.SetBlockProfileRate(1)
	defer runtime.SetBlockProfileRate(0)

	testFiles := createProfilingTestFiles(t, tempDir, 20, 1000)

	// Test different concurrency scenarios
	scenarios := []struct {
		name    string
		workers int
		queue   int
	}{
		{"HighContention", 8, 5}, // More workers than queue size
		{"LowContention", 2, 20}, // Fewer workers than queue size
		{"Balanced", 4, 10},      // Balanced workers and queue
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			config := &schema.WorkerPoolConfig{
				NumWorkers:    scenario.workers,
				QueueSize:     scenario.queue,
				Timeout:       30 * time.Second,
				RetryAttempts: 1,
				RetryDelay:    100 * time.Millisecond,
			}

			parser := core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
			defer parser.Close()

			// Run parsing operations
			ctx := context.Background()
			start := time.Now()
			results, err := parser.BatchParseFiles(ctx, testFiles)
			duration := time.Since(start)

			if err != nil {
				t.Fatal(err)
			}

			// Create blocking profile
			blockProfilePath := filepath.Join(profileDir, fmt.Sprintf("block_%s.prof", scenario.name))
			blockFile, err := os.Create(blockProfilePath)
			if err != nil {
				t.Fatal(err)
			}
			defer blockFile.Close()

			if err := pprof.Lookup("block").WriteTo(blockFile, 0); err != nil {
				t.Fatal(err)
			}

			totalChunks := 0
			for _, chunks := range results {
				totalChunks += len(chunks)
			}

			t.Logf("Blocking profiling completed for %s:", scenario.name)
			t.Logf("  Workers: %d, Queue size: %d", scenario.workers, scenario.queue)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Files processed: %d", len(testFiles))
			t.Logf("  Total chunks: %d", totalChunks)
			t.Logf("  Profile saved to: %s", blockProfilePath)
		})
	}
}

// TestMutexProfiling tests mutex profiling for lock contention analysis
func TestMutexProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mutex profiling test in short mode")
	}

	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "profiles")
	err := os.MkdirAll(profileDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Enable mutex profiling
	runtime.SetMutexProfileFraction(1)
	defer runtime.SetMutexProfileFraction(0)

	testFiles := createProfilingTestFiles(t, tempDir, 25, 800)

	// Test cache contention scenarios
	cacheConfigs := []struct {
		name      string
		cacheSize int
		policy    schema.CachePolicy
	}{
		{"SmallCache_LRU", 50, schema.PolicyLRU},
		{"LargeCache_LRU", 500, schema.PolicyLRU},
		{"SmallCache_LFU", 50, schema.PolicyLFU},
	}

	for _, config := range cacheConfigs {
		t.Run(config.name, func(t *testing.T) {
			cacheConfig := &schema.CacheConfig{
				MaxSize:         config.cacheSize,
				TTL:             10 * time.Minute,
				Policy:          config.policy,
				CleanupInterval: 2 * time.Minute,
			}

			parser := core.NewCachedUnifiedParser(schema.DefaultChunkingConfig(), cacheConfig)
			defer parser.Close()

			// Run concurrent parsing to create mutex contention
			ctx := context.Background()
			start := time.Now()

			// First run to populate cache
			_, err := parser.BatchParseFiles(ctx, testFiles)
			if err != nil {
				t.Fatal(err)
			}

			// Second run with cache hits
			_, err = parser.BatchParseFiles(ctx, testFiles)
			if err != nil {
				t.Fatal(err)
			}

			duration := time.Since(start)

			// Create mutex profile
			mutexProfilePath := filepath.Join(profileDir, fmt.Sprintf("mutex_%s.prof", config.name))
			mutexFile, err := os.Create(mutexProfilePath)
			if err != nil {
				t.Fatal(err)
			}
			defer mutexFile.Close()

			if err := pprof.Lookup("mutex").WriteTo(mutexFile, 0); err != nil {
				t.Fatal(err)
			}

			// Get cache metrics
			cacheMetrics := parser.GetCacheMetrics()

			t.Logf("Mutex profiling completed for %s:", config.name)
			t.Logf("  Cache size: %d, Policy: %s", config.cacheSize, config.policy)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Files processed: %d", len(testFiles))
			t.Logf("  Cache hits: %d", cacheMetrics.Hits)
			t.Logf("  Cache misses: %d", cacheMetrics.Misses)
			t.Logf("  Hit rate: %.2f%%", cacheMetrics.HitRate*100)
			t.Logf("  Profile saved to: %s", mutexProfilePath)
		})
	}
}

// TestAllocationProfiling tests allocation profiling for memory optimization
func TestAllocationProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping allocation profiling test in short mode")
	}

	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "profiles")
	err := os.MkdirAll(profileDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Test different chunking strategies for allocation patterns
	strategies := []struct {
		name     string
		strategy schema.ChunkingStrategy
		config   *schema.ChunkingConfig
	}{
		{
			"Paragraph_Small",
			schema.StrategyParagraph,
			&schema.ChunkingConfig{
				Strategy: schema.StrategyParagraph,
				MaxSize:  500,
				MinSize:  50,
				Overlap:  50,
			},
		},
		{
			"FixedSize_Large",
			schema.StrategyFixedSize,
			&schema.ChunkingConfig{
				Strategy: schema.StrategyFixedSize,
				MaxSize:  2000,
				MinSize:  200,
				Overlap:  200,
			},
		},
		{
			"Sentence_Medium",
			schema.StrategySentence,
			&schema.ChunkingConfig{
				Strategy: schema.StrategySentence,
				MaxSize:  1000,
				MinSize:  100,
				Overlap:  100,
			},
		},
	}

	testContent := generateProfilingContent(5000) // ~50KB content

	for _, strategy := range strategies {
		t.Run(strategy.name, func(t *testing.T) {
			parser := core.NewUnifiedParser(strategy.config)
			defer parser.Close()

			ctx := context.Background()

			// Measure allocations before
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Run parsing operations multiple times
			start := time.Now()
			totalChunks := 0

			for i := 0; i < 10; i++ {
				chunks, err := parser.ParseText(ctx, testContent)
				if err != nil {
					t.Fatal(err)
				}
				totalChunks += len(chunks)
			}

			duration := time.Since(start)

			// Measure allocations after
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			// Create allocation profile
			allocProfilePath := filepath.Join(profileDir, fmt.Sprintf("alloc_%s.prof", strategy.name))
			allocFile, err := os.Create(allocProfilePath)
			if err != nil {
				t.Fatal(err)
			}
			defer allocFile.Close()

			if err := pprof.Lookup("allocs").WriteTo(allocFile, 0); err != nil {
				t.Fatal(err)
			}

			// Calculate allocation metrics
			totalAllocs := m2.Mallocs - m1.Mallocs
			totalBytes := m2.TotalAlloc - m1.TotalAlloc
			allocsPerChunk := float64(totalAllocs) / float64(totalChunks)
			bytesPerChunk := float64(totalBytes) / float64(totalChunks)

			t.Logf("Allocation profiling completed for %s:", strategy.name)
			t.Logf("  Strategy: %s", strategy.strategy)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Total chunks: %d", totalChunks)
			t.Logf("  Total allocations: %d", totalAllocs)
			t.Logf("  Total bytes allocated: %d", totalBytes)
			t.Logf("  Allocations per chunk: %.2f", allocsPerChunk)
			t.Logf("  Bytes per chunk: %.2f", bytesPerChunk)
			t.Logf("  Profile saved to: %s", allocProfilePath)
		})
	}
}

// Helper functions

func createProfilingTestFiles(t *testing.T, dir string, count int, wordCount int) []string {
	files := make([]string, count)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("profiling_test_%d.txt", i)
		filePath := filepath.Join(dir, filename)

		content := generateProfilingContent(wordCount)
		fileContent := fmt.Sprintf("Profiling test file %d\n\n%s", i, content)

		err := os.WriteFile(filePath, []byte(fileContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		files[i] = filePath
	}

	return files
}

func generateProfilingContent(wordCount int) string {
	words := []string{
		"profiling", "performance", "analysis", "optimization", "memory",
		"allocation", "goroutine", "mutex", "blocking", "contention",
		"throughput", "latency", "efficiency", "scalability", "concurrency",
		"parsing", "chunking", "processing", "streaming", "caching",
		"benchmark", "testing", "validation", "monitoring", "metrics",
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

		// Add paragraph breaks every 35-55 words
		if paragraphLength > 35 && (i%43 == 0 || paragraphLength > 55) {
			content += "\n\n"
			paragraphLength = 0
		}
	}

	return content
}
