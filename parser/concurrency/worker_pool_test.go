package concurrency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

type mockParser struct {
	parseFile func(ctx context.Context, filePath string) ([]*schema.Chunk, error)
}

func (m *mockParser) ParseFile(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	if m.parseFile != nil {
		return m.parseFile(ctx, filePath)
	}
	return []*schema.Chunk{{Content: filePath}}, nil
}

func (m *mockParser) ParseText(ctx context.Context, content string) ([]*schema.Chunk, error) {
	return []*schema.Chunk{{Content: content}}, nil
}

func (m *mockParser) ParseMarkdown(ctx context.Context, content string) ([]*schema.Chunk, error) {
	return []*schema.Chunk{{Content: content}}, nil
}

func (m *mockParser) ParsePDF(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
	return []*schema.Chunk{{Content: filePath}}, nil
}

func (m *mockParser) DetectContentType(content string) schema.ChunkType {
	return schema.ChunkTypeText
}

func TestWorkerPool_StartStopAndHealth(t *testing.T) {
	wp := NewWorkerPool(&mockParser{}, &schema.WorkerPoolConfig{
		NumWorkers:    1,
		QueueSize:     2,
		Timeout:       100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    1 * time.Millisecond,
	})

	require.False(t, wp.IsHealthy())
	require.Error(t, wp.Stop())

	require.NoError(t, wp.Start())
	require.True(t, wp.IsHealthy())
	require.Error(t, wp.Start())

	_, err := wp.SubmitTask("a.txt", map[string]interface{}{"x": 1})
	require.NoError(t, err)

	metrics := wp.GetMetrics()
	require.GreaterOrEqual(t, metrics.TasksSubmitted, int64(1))
	require.Equal(t, 1, metrics.ActiveWorkers)

	require.NoError(t, wp.Stop())
	require.False(t, wp.IsHealthy())
}

func TestWorkerPool_ProcessFiles_Success(t *testing.T) {
	wp := NewWorkerPool(&mockParser{}, &schema.WorkerPoolConfig{
		NumWorkers:    2,
		QueueSize:     10,
		Timeout:       100 * time.Millisecond,
		RetryAttempts: 2,
		RetryDelay:    1 * time.Millisecond,
	})
	require.NoError(t, wp.Start())
	t.Cleanup(func() { _ = wp.Stop() })

	result, err := wp.ProcessFiles(context.Background(), []string{"f1", "f2"})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "f1", result["f1"][0].Content)
}

func TestWorkerPool_ProcessFiles_WithFailure(t *testing.T) {
	wp := NewWorkerPool(&mockParser{
		parseFile: func(ctx context.Context, filePath string) ([]*schema.Chunk, error) {
			if filePath == "bad" {
				return nil, errors.New("boom")
			}
			return []*schema.Chunk{{Content: filePath}}, nil
		},
	}, &schema.WorkerPoolConfig{
		NumWorkers:    1,
		QueueSize:     10,
		Timeout:       100 * time.Millisecond,
		RetryAttempts: 2,
		RetryDelay:    1 * time.Millisecond,
	})
	require.NoError(t, wp.Start())
	t.Cleanup(func() { _ = wp.Stop() })

	result, err := wp.ProcessFiles(context.Background(), []string{"ok", "bad"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to process files")
	require.Contains(t, result, "ok")
}

func TestWorkerPool_ProcessFiles_ContextCancelled(t *testing.T) {
	wp := NewWorkerPool(&mockParser{}, &schema.WorkerPoolConfig{
		NumWorkers:    1,
		QueueSize:     2,
		Timeout:       100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    1 * time.Millisecond,
	})
	require.NoError(t, wp.Start())
	t.Cleanup(func() { _ = wp.Stop() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := wp.ProcessFiles(ctx, []string{"x"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "canceled")
}
