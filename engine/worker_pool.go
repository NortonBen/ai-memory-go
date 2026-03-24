package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

// WorkerTask defines a task that can be executed by the worker pool.
type WorkerTask interface {
	Execute(ctx context.Context, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage) error
}

// AddTask is responsible for processing a new memory DataPoint.
// It extracts entities, generates embeddings, and saves relationships.
type AddTask struct {
	DataPoint *schema.DataPoint
}

// Execute performs the extraction and embedding logic for AddTask.
func (t *AddTask) Execute(ctx context.Context, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage) error {
	t.DataPoint.ProcessingStatus = schema.StatusProcessing
	if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
		log.Printf("Failed to update status to processing: %v", err)
	}

	// 1. Generate Embedding
	embedding, err := emb.GenerateEmbedding(ctx, t.DataPoint.Content)
	if err != nil {
		return t.fail(ctx, store, fmt.Errorf("embedding generation failed: %w", err))
	}
	t.DataPoint.Embedding = embedding

	// 2. Extract Entities
	nodes, err := ext.ExtractEntities(ctx, t.DataPoint.Content)
	if err != nil {
		return t.fail(ctx, store, fmt.Errorf("entity extraction failed: %w", err))
	}

	// 3. Extract Relationships
	edges, err := ext.ExtractRelationships(ctx, t.DataPoint.Content, nodes)
	if err != nil {
		log.Printf("Warning: relationship extraction returned error: %v", err)
	}

	// 4. Map edges to Relationships in DataPoint
	var relationships []schema.Relationship
	for _, edge := range edges {
		relationships = append(relationships, edge.ToRelationship())
	}
	t.DataPoint.Relationships = relationships

	// 5. Update Status
	t.DataPoint.ProcessingStatus = schema.StatusCompleted
	t.DataPoint.UpdatedAt = time.Now()

	// 6. Save updated DataPoint
	if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
		return fmt.Errorf("failed to update DataPoint: %w", err)
	}

	return nil
}

func (t *AddTask) fail(ctx context.Context, store storage.Storage, err error) error {
	t.DataPoint.ProcessingStatus = schema.StatusFailed
	t.DataPoint.ErrorMessage = err.Error()
	t.DataPoint.UpdatedAt = time.Now()
	_ = store.UpdateDataPoint(ctx, t.DataPoint)
	return err
}

// CognifyTask re-processes a DataPoint to deepen extraction.
type CognifyTask struct {
	DataPoint *schema.DataPoint
}

// Execute performs re-extraction and updating for CognifyTask.
func (t *CognifyTask) Execute(ctx context.Context, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage) error {
	addT := &AddTask{DataPoint: t.DataPoint}
	return addT.Execute(ctx, ext, emb, store)
}

// WorkerPool manages a pool of workers to process MemoryEngine tasks concurrently.
type WorkerPool struct {
	maxWorkers int
	taskQueue  chan WorkerTask
	extractor  extractor.LLMExtractor
	embedder   vector.EmbeddingProvider
	store      storage.Storage
	wg         sync.WaitGroup
	quit       chan struct{}
}

// NewWorkerPool initializes the WorkerPool.
func NewWorkerPool(maxWorkers int, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage) *WorkerPool {
	return &WorkerPool{
		maxWorkers: maxWorkers,
		taskQueue:  make(chan WorkerTask, 1000), // Buffered channel for backpressure
		extractor:  ext,
		embedder:   emb,
		store:      store,
		quit:       make(chan struct{}),
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
			if err := task.Execute(ctx, wp.extractor, wp.embedder, wp.store); err != nil {
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
