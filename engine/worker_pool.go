package engine

import (
	"context"
	"log"
	"sync"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

// WorkerTask defines a task that can be executed by the worker pool.
type WorkerTask interface {
	Execute(ctx context.Context, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore, wp *WorkerPool) error
}

// WorkerPool manages a pool of workers to process MemoryEngine tasks concurrently.
type WorkerPool struct {
	maxWorkers  int
	taskQueue   chan WorkerTask
	extractor   extractor.LLMExtractor
	embedder    vector.EmbeddingProvider
	store       storage.Storage
	graphStore  graph.GraphStore
	vectorStore vector.VectorStore
	wg          sync.WaitGroup
	quit        chan struct{}
}

// NewWorkerPool initializes the WorkerPool.
func NewWorkerPool(maxWorkers int, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore) *WorkerPool {
	return &WorkerPool{
		maxWorkers:  maxWorkers,
		taskQueue:   make(chan WorkerTask, 1000), // Buffered channel for backpressure
		extractor:   ext,
		embedder:    emb,
		store:       store,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		quit:        make(chan struct{}),
	}
}

// Start launches the worker goroutines.
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.maxWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	ctx := context.Background()

	for {
		select {
		case task := <-wp.taskQueue:
			if err := task.Execute(ctx, wp.extractor, wp.embedder, wp.store, wp.graphStore, wp.vectorStore, wp); err != nil {
				log.Printf("Task execution error: %v", err)
			}
		case <-wp.quit:
			return
		}
	}
}

// Submit enqueues a new task for processing.
func (wp *WorkerPool) Submit(task WorkerTask) {
	wp.taskQueue <- task
}

// Stop sends a shutdown signal to all workers and waits for them to finish.
func (wp *WorkerPool) Stop() {
	close(wp.quit)
	wp.wg.Wait()
}
