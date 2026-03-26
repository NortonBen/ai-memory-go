// Package parser - Automated performance monitoring and profiling integration
package benchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/parser/core"
	"github.com/NortonBen/ai-memory-go/parser/stream"
)

// PerformanceMetrics holds comprehensive performance data
type PerformanceMetrics struct {
	Timestamp         time.Time         `json:"timestamp"`
	TestName          string            `json:"test_name"`
	Duration          time.Duration     `json:"duration"`
	ThroughputMBps    float64           `json:"throughput_mbps"`
	MemoryUsageMB     float64           `json:"memory_usage_mb"`
	AllocationsPerOp  int64             `json:"allocations_per_op"`
	GCPauses          []time.Duration   `json:"gc_pauses"`
	CPUUsagePercent   float64           `json:"cpu_usage_percent"`
	WorkerPoolMetrics schema.WorkerPoolMetrics `json:"worker_pool_metrics"`
	CacheMetrics      *schema.CacheMetrics     `json:"cache_metrics,omitempty"`
	StreamingMetrics  *StreamingMetrics       `json:"streaming_metrics,omitempty"`
}

// StreamingMetrics holds streaming-specific performance data
type StreamingMetrics struct {
	BufferSize       int           `json:"buffer_size"`
	ChunksProcessed  int           `json:"chunks_processed"`
	BytesProcessed   int64         `json:"bytes_processed"`
	ProcessingTime   time.Duration `json:"processing_time"`
	MemoryEfficiency float64       `json:"memory_efficiency"`
	PeakMemoryUsage  int64         `json:"peak_memory_usage"`
}

// TestPerformanceMonitoring runs comprehensive performance monitoring
func TestPerformanceMonitoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance monitoring in short mode")
	}

	// Create output directory for performance data
	outputDir := filepath.Join("testdata", "performance_monitoring")
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()

	// Define monitoring test cases
	testCases := []struct {
		name     string
		setup    func(string) []string
		testFunc func(*testing.T, []string) PerformanceMetrics
	}{
		{
			"UnifiedParser_SmallFiles",
			func(dir string) []string {
				return createMonitoringTestFiles(t, dir, 20, 100)
			},
			testUnifiedParserPerformance,
		},
		{
			"UnifiedParser_LargeFiles",
			func(dir string) []string {
				return createMonitoringTestFiles(t, dir, 5, 5000)
			},
			testUnifiedParserPerformance,
		},
		{
			"CachedParser_Performance",
			func(dir string) []string {
				return createMonitoringTestFiles(t, dir, 10, 1000)
			},
			testCachedParserPerformance,
		},
		{
			"StreamingParser_Performance",
			func(dir string) []string {
				return createMonitoringTestFiles(t, dir, 3, 10000)
			},
			testStreamingParserPerformance,
		},
		{
			"WorkerPool_Scaling",
			func(dir string) []string {
				return createMonitoringTestFiles(t, dir, 50, 500)
			},
			testWorkerPoolScalingPerformance,
		},
	}

	allMetrics := make([]PerformanceMetrics, 0)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testFiles := tc.setup(tempDir)

			// Run performance test with profiling
			metrics := runPerformanceTestWithProfiling(t, tc.name, testFiles, tc.testFunc)
			allMetrics = append(allMetrics, metrics)

			// Save individual test metrics
			saveMetricsToFile(t, outputDir, tc.name, metrics)

			// Log key metrics
			t.Logf("Performance Metrics for %s:", tc.name)
			t.Logf("  Duration: %v", metrics.Duration)
			t.Logf("  Throughput: %.2f MB/s", metrics.ThroughputMBps)
			t.Logf("  Memory Usage: %.2f MB", metrics.MemoryUsageMB)
			t.Logf("  Allocations/op: %d", metrics.AllocationsPerOp)
			t.Logf("  CPU Usage: %.1f%%", metrics.CPUUsagePercent)
		})
	}

	// Save comprehensive report
	saveComprehensiveReport(t, outputDir, allMetrics)
}

// TestContinuousPerformanceMonitoring simulates continuous monitoring
func TestContinuousPerformanceMonitoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping continuous performance monitoring in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createMonitoringTestFiles(t, tempDir, 10, 1000)

	// Run monitoring for multiple iterations
	iterations := 10
	metrics := make([]PerformanceMetrics, iterations)

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("Iteration_%d", i+1), func(t *testing.T) {
			metrics[i] = testUnifiedParserPerformance(t, testFiles)
			metrics[i].TestName = fmt.Sprintf("ContinuousMonitoring_Iteration_%d", i+1)

			// Check for performance degradation
			if i > 0 {
				checkPerformanceDegradation(t, metrics[i-1], metrics[i])
			}
		})
	}

	// Analyze trends
	analyzePerfomanceTrends(t, metrics)
}

// TestMemoryProfilingIntegration tests memory profiling integration
func TestMemoryProfilingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profiling integration in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createMonitoringTestFiles(t, tempDir, 20, 1000)

	// Create memory profile
	memProfilePath := filepath.Join(tempDir, "mem_profile.prof")
	memFile, err := os.Create(memProfilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer memFile.Close()

	// Start memory profiling
	runtime.GC()
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		t.Fatal(err)
	}

	// Run parser operations
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	for i := 0; i < 5; i++ {
		_, err := parser.BatchParseFiles(ctx, testFiles)
		if err != nil {
			t.Fatal(err)
		}
	}

	runtime.ReadMemStats(&m2)

	// Write final memory profile
	runtime.GC()
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		t.Fatal(err)
	}

	// Analyze memory usage
	memoryGrowth := m2.TotalAlloc - m1.TotalAlloc
	t.Logf("Memory profiling results:")
	t.Logf("  Total allocations: %d bytes", memoryGrowth)
	t.Logf("  Current heap: %d bytes", m2.Alloc)
	t.Logf("  GC cycles: %d", m2.NumGC-m1.NumGC)
	t.Logf("  Profile saved to: %s", memProfilePath)
}

// TestCPUProfilingIntegration tests CPU profiling integration
func TestCPUProfilingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CPU profiling integration in short mode")
	}

	tempDir := t.TempDir()
	testFiles := createMonitoringTestFiles(t, tempDir, 30, 1000)

	// Create CPU profile
	cpuProfilePath := filepath.Join(tempDir, "cpu_profile.prof")
	cpuFile, err := os.Create(cpuProfilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer cpuFile.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		t.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	// Run CPU-intensive parser operations
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

	ctx := context.Background()
	start := time.Now()

	for i := 0; i < 10; i++ {
		_, err := parser.BatchParseFiles(ctx, testFiles)
		if err != nil {
			t.Fatal(err)
		}
	}

	duration := time.Since(start)

	t.Logf("CPU profiling results:")
	t.Logf("  Total duration: %v", duration)
	t.Logf("  Operations: %d", 10*len(testFiles))
	t.Logf("  Profile saved to: %s", cpuProfilePath)
}

// runPerformanceTestWithProfiling runs a performance test with comprehensive profiling
func runPerformanceTestWithProfiling(t *testing.T, testName string, testFiles []string, testFunc func(*testing.T, []string) PerformanceMetrics) PerformanceMetrics {
	// Enable detailed GC stats
	oldGCPercent := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(oldGCPercent)

	// Collect GC stats before
	var gcStatsBefore runtime.MemStats
	runtime.ReadMemStats(&gcStatsBefore)

	// Run the test
	start := time.Now()
	metrics := testFunc(t, testFiles)
	duration := time.Since(start)

	// Collect GC stats after
	var gcStatsAfter runtime.MemStats
	runtime.ReadMemStats(&gcStatsAfter)

	// Calculate GC pauses
	gcPauses := make([]time.Duration, 0)
	if gcStatsAfter.NumGC > gcStatsBefore.NumGC {
		// Get recent GC pause times
		for i := gcStatsBefore.NumGC; i < gcStatsAfter.NumGC && i < 256; i++ {
			pause := time.Duration(gcStatsAfter.PauseNs[i%256])
			gcPauses = append(gcPauses, pause)
		}
	}

	// Update metrics with profiling data
	metrics.Timestamp = time.Now()
	metrics.TestName = testName
	metrics.Duration = duration
	metrics.GCPauses = gcPauses
	metrics.CPUUsagePercent = calculateCPUUsage(duration)

	return metrics
}

// testUnifiedParserPerformance tests unified parser performance
func testUnifiedParserPerformance(t *testing.T, testFiles []string) PerformanceMetrics {
	parser := core.NewUnifiedParser(schema.DefaultChunkingConfig())
	defer parser.Close()

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

	// Calculate total bytes processed
	totalBytes := int64(0)
	for _, chunks := range results {
		for _, chunk := range chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}

	return PerformanceMetrics{
		Duration:          duration,
		ThroughputMBps:    float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:     float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024),
		AllocationsPerOp:  int64(m2.Mallocs - m1.Mallocs),
		WorkerPoolMetrics: parser.GetWorkerPoolMetrics(),
	}
}

// testCachedParserPerformance tests cached parser performance
func testCachedParserPerformance(t *testing.T, testFiles []string) PerformanceMetrics {
	parser := core.NewCachedUnifiedParser(schema.DefaultChunkingConfig(), schema.DefaultCacheConfig())
	defer parser.Close()

	ctx := context.Background()

	// Measure memory before
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run test (first time - cache miss)
	start := time.Now()
	results, err := parser.BatchParseFiles(ctx, testFiles)
	if err != nil {
		t.Fatal(err)
	}

	// Run again (cache hit)
	_, err = parser.BatchParseFiles(ctx, testFiles)
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate total bytes processed
	totalBytes := int64(0)
	for _, chunks := range results {
		for _, chunk := range chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}

	// Get cache metrics
	cacheMetrics := parser.GetCacheMetrics()

	return PerformanceMetrics{
		Duration:         duration,
		ThroughputMBps:   float64(totalBytes*2) / (1024 * 1024) / duration.Seconds(), // *2 for two runs
		MemoryUsageMB:    float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024),
		AllocationsPerOp: int64(m2.Mallocs - m1.Mallocs),
		CacheMetrics:     cacheMetrics,
	}
}

// testStreamingParserPerformance tests streaming parser performance
func testStreamingParserPerformance(t *testing.T, testFiles []string) PerformanceMetrics {
	parser := stream.NewStreamingParser(schema.DefaultStreamingConfig(), schema.DefaultChunkingConfig())

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
		result, err := parser.ParseFileStream(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
		totalChunks += len(result.Chunks)
		for _, chunk := range result.Chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}
	duration := time.Since(start)

	// Measure memory after
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	memoryUsed := m2.TotalAlloc - m1.TotalAlloc
	memoryEfficiency := float64(totalBytes) / float64(memoryUsed) * 100

	streamingMetrics := &StreamingMetrics{
		BufferSize:       schema.DefaultStreamingConfig().BufferSize,
		ChunksProcessed:  totalChunks,
		BytesProcessed:   totalBytes,
		ProcessingTime:   duration,
		MemoryEfficiency: memoryEfficiency,
		PeakMemoryUsage:  int64(m2.Alloc),
	}

	return PerformanceMetrics{
		Duration:         duration,
		ThroughputMBps:   float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:    float64(memoryUsed) / (1024 * 1024),
		AllocationsPerOp: int64(m2.Mallocs - m1.Mallocs),
		StreamingMetrics: streamingMetrics,
	}
}

// testWorkerPoolScalingPerformance tests worker pool scaling performance
func testWorkerPoolScalingPerformance(t *testing.T, testFiles []string) PerformanceMetrics {
	config := &schema.WorkerPoolConfig{
		NumWorkers:    runtime.NumCPU(),
		QueueSize:     len(testFiles) * 2,
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    100 * time.Millisecond,
	}

	parser := core.NewUnifiedParserWithWorkerPool(schema.DefaultChunkingConfig(), config)
	defer parser.Close()

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

	// Calculate total bytes processed
	totalBytes := int64(0)
	for _, chunks := range results {
		for _, chunk := range chunks {
			totalBytes += int64(len(chunk.Content))
		}
	}

	return PerformanceMetrics{
		Duration:          duration,
		ThroughputMBps:    float64(totalBytes) / (1024 * 1024) / duration.Seconds(),
		MemoryUsageMB:     float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024),
		AllocationsPerOp:  int64(m2.Mallocs - m1.Mallocs),
		WorkerPoolMetrics: parser.GetWorkerPoolMetrics(),
	}
}

// Helper functions

func calculateCPUUsage(duration time.Duration) float64 {
	// Simplified CPU usage calculation
	// In a real implementation, you would use more sophisticated CPU monitoring
	return float64(runtime.NumCPU()) * 50.0 // Assume 50% CPU usage per core
}

func checkPerformanceDegradation(t *testing.T, previous, current PerformanceMetrics) {
	// Check for significant performance degradation (>20%)
	if current.Duration > previous.Duration*12/10 {
		t.Logf("Warning: Duration increased by %.1f%%",
			float64(current.Duration-previous.Duration)/float64(previous.Duration)*100)
	}

	if current.ThroughputMBps < previous.ThroughputMBps*8/10 {
		t.Logf("Warning: Throughput decreased by %.1f%%",
			(previous.ThroughputMBps-current.ThroughputMBps)/previous.ThroughputMBps*100)
	}

	if current.MemoryUsageMB > previous.MemoryUsageMB*12/10 {
		t.Logf("Warning: Memory usage increased by %.1f%%",
			(current.MemoryUsageMB-previous.MemoryUsageMB)/previous.MemoryUsageMB*100)
	}
}

func analyzePerfomanceTrends(t *testing.T, metrics []PerformanceMetrics) {
	if len(metrics) < 2 {
		return
	}

	// Calculate average performance
	var avgDuration time.Duration
	var avgThroughput, avgMemory float64

	for _, m := range metrics {
		avgDuration += m.Duration
		avgThroughput += m.ThroughputMBps
		avgMemory += m.MemoryUsageMB
	}

	count := len(metrics)
	avgDuration /= time.Duration(count)
	avgThroughput /= float64(count)
	avgMemory /= float64(count)

	t.Logf("Performance Trends Analysis:")
	t.Logf("  Average Duration: %v", avgDuration)
	t.Logf("  Average Throughput: %.2f MB/s", avgThroughput)
	t.Logf("  Average Memory: %.2f MB", avgMemory)

	// Check for trends
	firstHalf := metrics[:count/2]
	secondHalf := metrics[count/2:]

	var firstAvgDuration, secondAvgDuration time.Duration
	for _, m := range firstHalf {
		firstAvgDuration += m.Duration
	}
	for _, m := range secondHalf {
		secondAvgDuration += m.Duration
	}

	firstAvgDuration /= time.Duration(len(firstHalf))
	secondAvgDuration /= time.Duration(len(secondHalf))

	if secondAvgDuration > firstAvgDuration*11/10 {
		t.Logf("Warning: Performance degradation trend detected")
	}
}

func saveMetricsToFile(t *testing.T, outputDir, testName string, metrics PerformanceMetrics) {
	filename := fmt.Sprintf("%s_%s.json", testName, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(outputDir, filename)

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		t.Logf("Failed to marshal metrics: %v", err)
		return
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		t.Logf("Failed to save metrics to file: %v", err)
		return
	}

	t.Logf("Metrics saved to: %s", filePath)
}

func saveComprehensiveReport(t *testing.T, outputDir string, allMetrics []PerformanceMetrics) {
	report := map[string]interface{}{
		"timestamp":    time.Now(),
		"total_tests":  len(allMetrics),
		"test_results": allMetrics,
		"summary": map[string]interface{}{
			"fastest_test":    findFastestTest(allMetrics),
			"slowest_test":    findSlowestTest(allMetrics),
			"most_efficient":  findMostEfficientTest(allMetrics),
			"least_efficient": findLeastEfficientTest(allMetrics),
		},
	}

	filename := fmt.Sprintf("comprehensive_report_%s.json", time.Now().Format("20060102_150405"))
	filePath := filepath.Join(outputDir, filename)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Logf("Failed to marshal comprehensive report: %v", err)
		return
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		t.Logf("Failed to save comprehensive report: %v", err)
		return
	}

	t.Logf("Comprehensive report saved to: %s", filePath)
}

func findFastestTest(metrics []PerformanceMetrics) string {
	if len(metrics) == 0 {
		return ""
	}

	fastest := metrics[0]
	for _, m := range metrics[1:] {
		if m.Duration < fastest.Duration {
			fastest = m
		}
	}
	return fastest.TestName
}

func findSlowestTest(metrics []PerformanceMetrics) string {
	if len(metrics) == 0 {
		return ""
	}

	slowest := metrics[0]
	for _, m := range metrics[1:] {
		if m.Duration > slowest.Duration {
			slowest = m
		}
	}
	return slowest.TestName
}

func findMostEfficientTest(metrics []PerformanceMetrics) string {
	if len(metrics) == 0 {
		return ""
	}

	mostEfficient := metrics[0]
	for _, m := range metrics[1:] {
		if m.ThroughputMBps > mostEfficient.ThroughputMBps {
			mostEfficient = m
		}
	}
	return mostEfficient.TestName
}

func findLeastEfficientTest(metrics []PerformanceMetrics) string {
	if len(metrics) == 0 {
		return ""
	}

	leastEfficient := metrics[0]
	for _, m := range metrics[1:] {
		if m.ThroughputMBps < leastEfficient.ThroughputMBps {
			leastEfficient = m
		}
	}
	return leastEfficient.TestName
}

func createMonitoringTestFiles(t *testing.T, dir string, count int, wordCount int) []string {
	files := make([]string, count)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("monitoring_test_%d.txt", i)
		filePath := filepath.Join(dir, filename)

		content := generateMonitoringContent(wordCount, i)

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		files[i] = filePath
	}

	return files
}

func generateMonitoringContent(wordCount int, seed int) string {
	words := []string{
		"monitoring", "performance", "metrics", "analysis", "benchmark",
		"testing", "profiling", "optimization", "efficiency", "throughput",
		"latency", "memory", "allocation", "garbage", "collection",
		"concurrency", "parallel", "processing", "streaming", "caching",
	}

	content := fmt.Sprintf("Performance monitoring test file %d\n\n", seed)
	paragraphLength := 0

	for i := 0; i < wordCount; i++ {
		word := words[(i+seed)%len(words)]
		content += word
		paragraphLength++

		if i < wordCount-1 {
			content += " "
		}

		// Add paragraph breaks
		if paragraphLength > 40 && (i%47 == 0 || paragraphLength > 60) {
			content += "\n\n"
			paragraphLength = 0
		}
	}

	return content
}
