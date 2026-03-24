// Package parser - Worker pool tests
package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPoolConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultWorkerPoolConfig()
		assert.Equal(t, runtime.NumCPU(), config.NumWorkers)
		assert.Equal(t, 100, config.QueueSize)
		assert.Equal(t, 30*time.Second, config.Timeout)
		assert.Equal(t, 3, config.RetryAttempts)
		assert.Equal(t, 1*time.Second, config.RetryDelay)
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := &WorkerPoolConfig{
			NumWorkers:    4,
			QueueSize:     50,
			Timeout:       10 * time.Second,
			RetryAttempts: 2,
			RetryDelay:    500 * time.Millisecond,
		}

		assert.Equal(t, 4, config.NumWorkers)
		assert.Equal(t, 50, config.QueueSize)
		assert.Equal(t, 10*time.Second, config.Timeout)
		assert.Equal(t, 2, config.RetryAttempts)
		assert.Equal(t, 500*time.Millisecond, config.RetryDelay)
	})
}

func TestWorkerPoolLifecycle(t *testing.T) {
	parser := NewUnifiedParser(DefaultChunkingConfig())
	config := &WorkerPoolConfig{
		NumWorkers:    2,
		QueueSize:     10,
		Timeout:       5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    100 * time.Millisecond,
	}

	pool := NewWorkerPool(parser, config)

	t.Run("InitialState", func(t *testing.T) {
		assert.False(t, pool.IsHealthy())
		metrics := pool.GetMetrics()
		assert.Equal(t, int64(0), metrics.TasksSubmitted)
		assert.Equal(t, int64(0), metrics.TasksCompleted)
	})

	t.Run("StartPool", func(t *testing.T) {
		err := pool.Start()
		require.NoError(t, err)

		// Give workers time to start
		time.Sleep(100 * time.Millisecond)
		assert.True(t, pool.IsHealthy())

		// Starting again should fail
		err = pool.Start()
		assert.Error(t, err)
	})

	t.Run("StopPool", func(t *testing.T) {
		err := pool.Stop()
		require.NoError(t, err)
		assert.False(t, pool.IsHealthy())

		// Stopping again should fail
		err = pool.Stop()
		assert.Error(t, err)
	})
}

func TestWorkerPoolProcessing(t *testing.T) {
	// Create test files
	tempDir := t.TempDir()
	testFiles := createTestFiles(t, tempDir, 5)

	parser := NewUnifiedParser(DefaultChunkingConfig())
	config := &WorkerPoolConfig{
		NumWorkers:    3,
		QueueSize:     10,
		Timeout:       5 * time.Second,
		RetryAttempts: 2,
		RetryDelay:    100 * time.Millisecond,
	}

	pool := NewWorkerPool(parser, config)
	require.NoError(t, pool.Start())
	defer pool.Stop()

	t.Run("ProcessMultipleFiles", func(t *testing.T) {
		ctx := context.Background()
		results, err := pool.ProcessFiles(ctx, testFiles)

		require.NoError(t, err)
		assert.Equal(t, len(testFiles), len(results))

		// Verify each file was processed
		for _, filePath := range testFiles {
			chunks, exists := results[filePath]
			assert.True(t, exists, "File %s not found in results", filePath)
			assert.NotEmpty(t, chunks, "No chunks for file %s", filePath)
		}

		// Check metrics
		metrics := pool.GetMetrics()
		assert.Equal(t, int64(len(testFiles)), metrics.TasksSubmitted)
		assert.Equal(t, int64(len(testFiles)), metrics.TasksCompleted)
		assert.Equal(t, int64(0), metrics.TasksFailed)
	})

	t.Run("ProcessWithTimeout", func(t *testing.T) {
		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := pool.ProcessFiles(ctx, testFiles)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("ProcessEmptyList", func(t *testing.T) {
		ctx := context.Background()
		results, err := pool.ProcessFiles(ctx, []string{})

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestWorkerPoolConcurrency(t *testing.T) {
	// Create many test files
	tempDir := t.TempDir()
	testFiles := createTestFiles(t, tempDir, 20)

	parser := NewUnifiedParser(DefaultChunkingConfig())
	config := &WorkerPoolConfig{
		NumWorkers:    4,
		QueueSize:     25,
		Timeout:       10 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    50 * time.Millisecond,
	}

	pool := NewWorkerPool(parser, config)
	require.NoError(t, pool.Start())
	defer pool.Stop()

	t.Run("ConcurrentProcessing", func(t *testing.T) {
		ctx := context.Background()
		startTime := time.Now()

		results, err := pool.ProcessFiles(ctx, testFiles)

		duration := time.Since(startTime)
		require.NoError(t, err)
		assert.Equal(t, len(testFiles), len(results))

		// With 4 workers, processing should be faster than sequential
		// This is a rough check - actual timing depends on system load
		t.Logf("Processed %d files in %v", len(testFiles), duration)

		// Verify all files were processed correctly
		for _, filePath := range testFiles {
			chunks, exists := results[filePath]
			assert.True(t, exists)
			assert.NotEmpty(t, chunks)
		}
	})

	t.Run("MultipleSimultaneousRequests", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make([]map[string][]Chunk, 3)
		errors := make([]error, 3)

		// Submit 3 concurrent batch requests
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				ctx := context.Background()
				results[index], errors[index] = pool.ProcessFiles(ctx, testFiles[:5])
			}(i)
		}

		wg.Wait()

		// All requests should succeed
		for i := 0; i < 3; i++ {
			require.NoError(t, errors[i], "Request %d failed", i)
			assert.Equal(t, 5, len(results[i]), "Request %d returned wrong number of results", i)
		}
	})
}

func TestWorkerPoolErrorHandling(t *testing.T) {
	parser := NewUnifiedParser(DefaultChunkingConfig())
	config := &WorkerPoolConfig{
		NumWorkers:    2,
		QueueSize:     5,
		Timeout:       1 * time.Second,
		RetryAttempts: 2,
		RetryDelay:    100 * time.Millisecond,
	}

	pool := NewWorkerPool(parser, config)
	require.NoError(t, pool.Start())
	defer pool.Stop()

	t.Run("NonExistentFiles", func(t *testing.T) {
		nonExistentFiles := []string{
			"/path/to/nonexistent1.txt",
			"/path/to/nonexistent2.txt",
		}

		ctx := context.Background()
		results, err := pool.ProcessFiles(ctx, nonExistentFiles)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to process files")

		// Should still return partial results (empty in this case)
		assert.NotNil(t, results)

		// Check that retry attempts were made
		metrics := pool.GetMetrics()
		assert.Greater(t, metrics.TasksRetried, int64(0))
		assert.Greater(t, metrics.TasksFailed, int64(0))
	})

	t.Run("MixedValidAndInvalidFiles", func(t *testing.T) {
		tempDir := t.TempDir()
		validFiles := createTestFiles(t, tempDir, 2)
		invalidFiles := []string{"/nonexistent.txt"}

		allFiles := append(validFiles, invalidFiles...)

		ctx := context.Background()
		results, err := pool.ProcessFiles(ctx, allFiles)

		// Should return error but also partial results
		assert.Error(t, err)
		assert.Equal(t, len(validFiles), len(results))

		// Valid files should be processed successfully
		for _, filePath := range validFiles {
			chunks, exists := results[filePath]
			assert.True(t, exists)
			assert.NotEmpty(t, chunks)
		}
	})
}

func TestWorkerPoolMetrics(t *testing.T) {
	tempDir := t.TempDir()
	testFiles := createTestFiles(t, tempDir, 3)

	parser := NewUnifiedParser(DefaultChunkingConfig())
	config := &WorkerPoolConfig{
		NumWorkers:    2,
		QueueSize:     10,
		Timeout:       5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    100 * time.Millisecond,
	}

	pool := NewWorkerPool(parser, config)
	require.NoError(t, pool.Start())
	defer pool.Stop()

	t.Run("MetricsTracking", func(t *testing.T) {
		// Initial metrics
		initialMetrics := pool.GetMetrics()
		assert.Equal(t, int64(0), initialMetrics.TasksSubmitted)
		assert.Equal(t, int64(0), initialMetrics.TasksCompleted)

		// Process files
		ctx := context.Background()
		results, err := pool.ProcessFiles(ctx, testFiles)
		require.NoError(t, err)
		assert.Equal(t, len(testFiles), len(results))

		// Check updated metrics
		finalMetrics := pool.GetMetrics()
		assert.Equal(t, int64(len(testFiles)), finalMetrics.TasksSubmitted)
		assert.Equal(t, int64(len(testFiles)), finalMetrics.TasksCompleted)
		assert.Equal(t, int64(0), finalMetrics.TasksFailed)
		assert.Greater(t, finalMetrics.TotalProcessingTime, time.Duration(0))
		assert.Greater(t, finalMetrics.AverageProcessingTime, time.Duration(0))
	})
}

func TestUnifiedParserWithWorkerPool(t *testing.T) {
	tempDir := t.TempDir()
	testFiles := createTestFiles(t, tempDir, 5)

	t.Run("DefaultWorkerPool", func(t *testing.T) {
		parser := NewUnifiedParser(DefaultChunkingConfig())
		defer parser.Close()

		ctx := context.Background()
		results, err := parser.BatchParseFiles(ctx, testFiles)

		require.NoError(t, err)
		assert.Equal(t, len(testFiles), len(results))

		// Check that worker pool was used
		assert.True(t, parser.IsWorkerPoolHealthy())

		metrics := parser.GetWorkerPoolMetrics()
		assert.Equal(t, int64(len(testFiles)), metrics.TasksCompleted)
	})

	t.Run("CustomWorkerPool", func(t *testing.T) {
		config := &WorkerPoolConfig{
			NumWorkers:    3,
			QueueSize:     20,
			Timeout:       10 * time.Second,
			RetryAttempts: 2,
			RetryDelay:    200 * time.Millisecond,
		}

		parser := NewUnifiedParserWithWorkerPool(DefaultChunkingConfig(), config)
		defer parser.Close()

		// Start worker pool explicitly
		require.NoError(t, parser.StartWorkerPool())

		ctx := context.Background()
		results, err := parser.ProcessFilesParallel(ctx, testFiles, map[string]interface{}{
			"batch_id": "test_batch_1",
		})

		require.NoError(t, err)
		assert.Equal(t, len(testFiles), len(results))

		metrics := parser.GetWorkerPoolMetrics()
		assert.Equal(t, int64(len(testFiles)), metrics.TasksCompleted)
		assert.Equal(t, 3, metrics.ActiveWorkers)
	})
}

// Helper function to create test files
func createTestFiles(t *testing.T, dir string, count int) []string {
	files := make([]string, count)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("test_file_%d.txt", i)
		filepath := filepath.Join(dir, filename)

		content := fmt.Sprintf("This is test file number %d.\n\nIt contains some sample content for testing the parser.\n\nParagraph %d with more text.", i, i)

		err := os.WriteFile(filepath, []byte(content), 0644)
		require.NoError(t, err)

		files[i] = filepath
	}

	return files
}

func BenchmarkWorkerPool(b *testing.B) {
	tempDir := b.TempDir()
	testFiles := createBenchmarkTestFiles(b, tempDir, 10)

	parser := NewUnifiedParser(DefaultChunkingConfig())
	defer parser.Close()

	b.Run("Sequential", func(b *testing.B) {
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

	b.Run("Parallel", func(b *testing.B) {
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

// Helper function for benchmark
func createBenchmarkTestFiles(b *testing.B, dir string, count int) []string {
	files := make([]string, count)

	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("bench_file_%d.txt", i)
		filepath := filepath.Join(dir, filename)

		content := fmt.Sprintf("Benchmark test file number %d.\n\nThis file contains sample content for performance testing.\n\nMultiple paragraphs to ensure proper chunking behavior.\n\nFile %d content.", i, i)

		err := os.WriteFile(filepath, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}

		files[i] = filepath
	}

	return files
}
