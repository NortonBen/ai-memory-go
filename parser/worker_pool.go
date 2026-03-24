// Package parser - Worker pool for parallel file processing
package parser

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// WorkerPoolConfig configures the worker pool behavior
type WorkerPoolConfig struct {
	// NumWorkers is the number of concurrent workers
	NumWorkers int `json:"num_workers"`

	// QueueSize is the size of the task queue buffer
	QueueSize int `json:"queue_size"`

	// Timeout is the maximum time to wait for a task to complete
	Timeout time.Duration `json:"timeout"`

	// RetryAttempts is the number of retry attempts for failed tasks
	RetryAttempts int `json:"retry_attempts"`

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration `json:"retry_delay"`
}

// DefaultWorkerPoolConfig returns a sensible default configuration
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		NumWorkers:    runtime.NumCPU(),
		QueueSize:     100,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
	}
}

// ProcessingTask represents a file processing task
type ProcessingTask struct {
	FilePath  string
	TaskID    string
	Priority  int
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

// ProcessingResult represents the result of a processing task
type ProcessingResult struct {
	TaskID      string
	FilePath    string
	Chunks      []Chunk
	Error       error
	Duration    time.Duration
	Attempts    int
	CompletedAt time.Time
}

// WorkerPoolMetrics tracks performance metrics
type WorkerPoolMetrics struct {
	mu                    sync.RWMutex
	TasksSubmitted        int64
	TasksCompleted        int64
	TasksFailed           int64
	TasksRetried          int64
	TotalProcessingTime   time.Duration
	AverageProcessingTime time.Duration
	ActiveWorkers         int
	QueueLength           int
}

// GetMetrics returns a copy of the current metrics
func (m *WorkerPoolMetrics) GetMetrics() WorkerPoolMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m
}

// WorkerPool manages parallel file processing
type WorkerPool struct {
	config  *WorkerPoolConfig
	parser  Parser
	metrics *WorkerPoolMetrics

	// Channels for task management
	taskQueue   chan *ProcessingTask
	resultQueue chan *ProcessingResult

	// Worker management
	workers     []*Worker
	workerGroup sync.WaitGroup

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
	mu      sync.RWMutex
}

// Worker represents a single worker in the pool
type Worker struct {
	id     int
	pool   *WorkerPool
	parser Parser
	active bool
	mu     sync.RWMutex
}

// NewWorkerPool creates a new worker pool for parallel file processing
func NewWorkerPool(parser Parser, config *WorkerPoolConfig) *WorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		config:      config,
		parser:      parser,
		metrics:     &WorkerPoolMetrics{},
		taskQueue:   make(chan *ProcessingTask, config.QueueSize),
		resultQueue: make(chan *ProcessingResult, config.QueueSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start initializes and starts the worker pool
func (wp *WorkerPool) Start() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.started {
		return fmt.Errorf("worker pool already started")
	}

	// Create and start workers
	wp.workers = make([]*Worker, wp.config.NumWorkers)
	for i := 0; i < wp.config.NumWorkers; i++ {
		worker := &Worker{
			id:     i,
			pool:   wp,
			parser: wp.parser,
		}
		wp.workers[i] = worker

		wp.workerGroup.Add(1)
		go worker.run()
	}

	wp.started = true
	return nil
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.started {
		return fmt.Errorf("worker pool not started")
	}

	// Cancel context to signal workers to stop
	wp.cancel()

	// Close task queue to prevent new tasks
	close(wp.taskQueue)

	// Wait for all workers to finish
	wp.workerGroup.Wait()

	// Close result queue
	close(wp.resultQueue)

	wp.started = false
	return nil
}

// SubmitTask submits a file processing task to the worker pool
func (wp *WorkerPool) SubmitTask(filePath string, metadata map[string]interface{}) (*ProcessingTask, error) {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if !wp.started {
		return nil, fmt.Errorf("worker pool not started")
	}

	task := &ProcessingTask{
		FilePath:  filePath,
		TaskID:    generateTaskID(filePath),
		Priority:  0, // Default priority
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	select {
	case wp.taskQueue <- task:
		wp.metrics.mu.Lock()
		wp.metrics.TasksSubmitted++
		wp.metrics.QueueLength++
		wp.metrics.mu.Unlock()
		return task, nil
	case <-wp.ctx.Done():
		return nil, fmt.Errorf("worker pool shutting down")
	default:
		return nil, fmt.Errorf("task queue full")
	}
}

// ProcessFiles processes multiple files concurrently and returns results
// ProcessFiles processes multiple files concurrently and returns results
func (wp *WorkerPool) ProcessFiles(ctx context.Context, filePaths []string) (map[string][]Chunk, error) {
	if len(filePaths) == 0 {
		return make(map[string][]Chunk), nil
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Submit all tasks
	tasks := make([]*ProcessingTask, 0, len(filePaths))
	for _, filePath := range filePaths {
		task, err := wp.SubmitTask(filePath, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to submit task for %s: %w", filePath, err)
		}
		tasks = append(tasks, task)
	}

	// Collect results
	results := make(map[string][]Chunk)
	errors := make(map[string]error)
	completed := 0

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, wp.config.Timeout*time.Duration(len(filePaths)))
	defer cancel()

	for completed < len(tasks) {
		select {
		case result, ok := <-wp.resultQueue:
			if !ok {
				return results, fmt.Errorf("result queue closed, completed %d/%d tasks", completed, len(tasks))
			}
			completed++
			if result.Error != nil {
				errors[result.FilePath] = result.Error
			} else {
				results[result.FilePath] = result.Chunks
			}

		case <-timeoutCtx.Done():
			return results, fmt.Errorf("timeout waiting for results, completed %d/%d tasks", completed, len(tasks))

		case <-wp.ctx.Done():
			return results, fmt.Errorf("worker pool shutting down")
		}
	}

	// Return error if any files failed to process
	if len(errors) > 0 {
		var errorMsg string
		for filePath, err := range errors {
			errorMsg += fmt.Sprintf("%s: %v; ", filePath, err)
		}
		return results, fmt.Errorf("failed to process files: %s", errorMsg)
	}

	return results, nil
}

// GetMetrics returns current worker pool metrics
// GetMetrics returns current worker pool metrics
func (wp *WorkerPool) GetMetrics() WorkerPoolMetrics {
	wp.metrics.mu.Lock()
	defer wp.metrics.mu.Unlock()

	// Update queue length
	wp.metrics.QueueLength = len(wp.taskQueue)

	// Count available workers (workers that exist and are running)
	wp.metrics.ActiveWorkers = len(wp.workers)

	return *wp.metrics
}

// IsHealthy checks if the worker pool is healthy
func (wp *WorkerPool) IsHealthy() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if !wp.started {
		return false
	}

	// Check if context is cancelled
	select {
	case <-wp.ctx.Done():
		return false
	default:
	}

	// Check if we have workers
	return len(wp.workers) > 0
}

// run is the main worker loop
func (w *Worker) run() {
	defer w.pool.workerGroup.Done()

	for {
		select {
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				// Task queue closed, worker should exit
				return
			}

			w.processTask(task)

		case <-w.pool.ctx.Done():
			// Pool is shutting down
			return
		}
	}
}

// processTask processes a single task with retry logic
func (w *Worker) processTask(task *ProcessingTask) {
	w.mu.Lock()
	w.active = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.active = false
		w.mu.Unlock()

		// Update queue length metric
		w.pool.metrics.mu.Lock()
		w.pool.metrics.QueueLength--
		w.pool.metrics.mu.Unlock()
	}()

	startTime := time.Now()
	var result *ProcessingResult

	// Retry logic
	for attempt := 1; attempt <= w.pool.config.RetryAttempts; attempt++ {
		// Create timeout context for this attempt
		ctx, cancel := context.WithTimeout(w.pool.ctx, w.pool.config.Timeout)

		chunks, err := w.parser.ParseFile(ctx, task.FilePath)
		cancel()

		if err == nil {
			// Success
			result = &ProcessingResult{
				TaskID:      task.TaskID,
				FilePath:    task.FilePath,
				Chunks:      chunks,
				Error:       nil,
				Duration:    time.Since(startTime),
				Attempts:    attempt,
				CompletedAt: time.Now(),
			}
			break
		}

		// Failed attempt
		if attempt < w.pool.config.RetryAttempts {
			// Wait before retry
			select {
			case <-time.After(w.pool.config.RetryDelay):
			case <-w.pool.ctx.Done():
				// Pool shutting down, don't retry
				result = &ProcessingResult{
					TaskID:      task.TaskID,
					FilePath:    task.FilePath,
					Chunks:      nil,
					Error:       fmt.Errorf("worker pool shutting down: %w", err),
					Duration:    time.Since(startTime),
					Attempts:    attempt,
					CompletedAt: time.Now(),
				}
				break
			}

			w.pool.metrics.mu.Lock()
			w.pool.metrics.TasksRetried++
			w.pool.metrics.mu.Unlock()
		} else {
			// Final attempt failed
			result = &ProcessingResult{
				TaskID:      task.TaskID,
				FilePath:    task.FilePath,
				Chunks:      nil,
				Error:       fmt.Errorf("failed after %d attempts: %w", attempt, err),
				Duration:    time.Since(startTime),
				Attempts:    attempt,
				CompletedAt: time.Now(),
			}
		}
	}

	// Update metrics
	w.pool.metrics.mu.Lock()
	if result.Error == nil {
		w.pool.metrics.TasksCompleted++
	} else {
		w.pool.metrics.TasksFailed++
	}
	w.pool.metrics.TotalProcessingTime += result.Duration
	if w.pool.metrics.TasksCompleted > 0 {
		w.pool.metrics.AverageProcessingTime = w.pool.metrics.TotalProcessingTime / time.Duration(w.pool.metrics.TasksCompleted)
	}
	w.pool.metrics.mu.Unlock()

	// Send result
	select {
	case w.pool.resultQueue <- result:
	case <-w.pool.ctx.Done():
		// Pool shutting down, can't send result
	}
}

// generateTaskID creates a unique task ID
func generateTaskID(filePath string) string {
	return fmt.Sprintf("task_%x_%d", generateContentHash(filePath)[:8], time.Now().UnixNano())
}
