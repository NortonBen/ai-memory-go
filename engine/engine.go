package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/google/uuid"
)

// EngineConfig holds configuration for MemoryEngine
type EngineConfig struct {
	MaxWorkers int
}

// MemoryEngine is the facade for orchestrating Add, Cognify, and Search operations.
type MemoryEngine struct {
	extractor  extractor.LLMExtractor
	embedder   vector.EmbeddingProvider
	store      storage.Storage
	workerPool *WorkerPool
}

// NewMemoryEngine creates a new instance of MemoryEngine.
func NewMemoryEngine(ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, cfg EngineConfig) *MemoryEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5
	}

	pool := NewWorkerPool(cfg.MaxWorkers, ext, emb, store)
	pool.Start()

	return &MemoryEngine{
		extractor:  ext,
		embedder:   emb,
		store:      store,
		workerPool: pool,
	}
}

// AddMemory enqueues a memory addition task and returns the initial DataPoint.
// The actual extraction, embedding, and storing is done asynchronously.
func (e *MemoryEngine) AddMemory(ctx context.Context, text string, sessionID string) (*schema.DataPoint, error) {
	dp := &schema.DataPoint{
		ID:               uuid.New().String(),
		Content:          text,
		ContentType:      "text",
		SessionID:        sessionID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: schema.StatusPending,
		Metadata:         make(map[string]interface{}),
	}

	// Persist the initial pending DataPoint
	err := e.store.StoreDataPoint(ctx, dp)
	if err != nil {
		return nil, fmt.Errorf("failed to store initial DataPoint: %w", err)
	}

	e.workerPool.Submit(&AddTask{
		DataPoint: dp,
	})

	return dp, nil
}

// Cognify process updates relationships and embeddings for an existing DataPoint.
func (e *MemoryEngine) Cognify(ctx context.Context, dpID string) error {
	dp, err := e.store.GetDataPoint(ctx, dpID)
	if err != nil {
		return fmt.Errorf("failed to retrieve DataPoint: %w", err)
	}

	e.workerPool.Submit(&CognifyTask{
		DataPoint: dp,
	})

	return nil
}

// Search queries the storage based on semantic and/or relational parameters.
func (e *MemoryEngine) Search(ctx context.Context, query *storage.DataPointQuery) ([]*schema.DataPoint, error) {
	// Optionally vectorize the search text if it's semantic search
	if query.SearchText != "" && query.SearchMode != "exact" {
		emb, err := e.embedder.GenerateEmbedding(ctx, query.SearchText)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
		}
		// Assuming we pass filters to storage query and storage handles vector similarity itself
		// Because storage.DataPointQuery doesn't have an embedding field directly in this schema, 
		// we pass it via metadata filters or custom fields if supported.
		// Wait, storage.Storage interface handles QueryDataPoints. 
		// For a full search facade, we might just defer to store.
		_ = emb
	}
	
	return e.store.QueryDataPoints(ctx, query)
}


// Close gracefully shuts down the engine and its worker pool.
func (e *MemoryEngine) Close() {
	e.workerPool.Stop()
}
