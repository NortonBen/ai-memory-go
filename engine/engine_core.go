package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/google/uuid"
)

// defaultMemoryEngine is the implementation for orchestrating Add, Cognify, and Search operations.
type defaultMemoryEngine struct {
	extractor   extractor.LLMExtractor
	embedder    vector.EmbeddingProvider
	store       storage.Storage
	graphStore  graph.GraphStore
	vectorStore vector.VectorStore
	workerPool  *WorkerPool
}

// NewMemoryEngine creates a new instance of MemoryEngine using only the relational store (fallback).
func NewMemoryEngine(ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, cfg EngineConfig) MemoryEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5
	}

	pool := NewWorkerPool(cfg.MaxWorkers, ext, emb, store, nil, nil)
	pool.Start()

	return &defaultMemoryEngine{
		extractor:  ext,
		embedder:   emb,
		store:      store,
		workerPool: pool,
	}
}

// NewMemoryEngineWithStores creates a new instance of MemoryEngine including graph and vector stores for advanced features.
func NewMemoryEngineWithStores(ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore, cfg EngineConfig) MemoryEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5
	}

	pool := NewWorkerPool(cfg.MaxWorkers, ext, emb, store, graphStore, vectorStore)
	pool.Start()

	engine := &defaultMemoryEngine{
		extractor:   ext,
		embedder:    emb,
		store:       store,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		workerPool:  pool,
	}

	// Start background history analysis if enabled
	if cfg.EnableBackgroundAnalysis {
		interval := cfg.AnalysisInterval
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				// In a real implementation, we would iterate over active sessions.
				// For now, we'll focus on the 'default' session or use a global tracker.
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				_ = engine.AnalyzeHistory(ctx, "default")
				cancel()
			}
		}()
	}

	return engine
}

// Add persists the initial DataPoint and optionally starts asynchronous processing.
func (e *defaultMemoryEngine) Add(ctx context.Context, content string, opts ...AddOption) (*schema.DataPoint, error) {
	options := &AddOptions{
		Metadata: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(options)
	}

	sessionID := options.SessionID
	if sessionID == "" {
		// Fallback check if someone used sessionID or session_id in metadata instead
		if sid, ok := options.Metadata["session_id"].(string); ok && sid != "" {
			sessionID = sid
			delete(options.Metadata, "session_id")
		} else if sid, ok := options.Metadata["sessionID"].(string); ok && sid != "" {
			sessionID = sid
			delete(options.Metadata, "sessionID")
		} else {
			sessionID = "default"
		}
	}

	// Deduplication: Check if this exact content already exists for this session
	searchQuery := &storage.DataPointQuery{
		SearchText: content,
		SearchMode: "exact",
		SessionID:  sessionID,
		Limit:      1,
	}
	if existingMatches, err := e.store.QueryDataPoints(ctx, searchQuery); err == nil && len(existingMatches) > 0 {
		return existingMatches[0], nil
	}

	dp := &schema.DataPoint{
		ID:               uuid.New().String(),
		Content:          content,
		ContentType:      "text",
		SessionID:        sessionID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: schema.StatusPending,
		Metadata:         options.Metadata,
	}

	// Persist the initial pending DataPoint
	err := e.store.StoreDataPoint(ctx, dp)
	if err != nil {
		return nil, fmt.Errorf("failed to store initial DataPoint: %w", err)
	}

	return dp, nil
}

// Cognify process updates relationships and embeddings for an existing DataPoint synchronously.
func (e *defaultMemoryEngine) Cognify(ctx context.Context, dataPoint *schema.DataPoint, opts ...CognifyOption) (*schema.DataPoint, error) {
	options := &CognifyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	threshold := float32(0.0)
	if th, ok := dataPoint.Metadata["consistency_threshold"].(float64); ok {
		threshold = float32(th)
	} else if th, ok := dataPoint.Metadata["consistency_threshold"].(float32); ok {
		threshold = th
	}

	task := &CognifyTask{
		DataPoint:            dataPoint,
		ConsistencyThreshold: threshold,
	}

	if options.WaitUntilComplete {
		// Run synchronously and wait for the extraction & embedding to finish
		err := task.Execute(ctx, e.workerPool.extractor, e.workerPool.embedder, e.workerPool.store, e.workerPool.graphStore, e.workerPool.vectorStore, e.workerPool)
		return dataPoint, err
	}

	// Run asynchronously
	e.workerPool.Submit(task)
	return dataPoint, nil
}

// CognifyPending sweeps the relational store for items that have ProcessingStatus == StatusPending and processes them synchronously.
func (e *defaultMemoryEngine) CognifyPending(ctx context.Context, sessionID string) error {
	q := &storage.DataPointQuery{
		SessionID: sessionID,
		Limit:     1000,
	}
	dps, err := e.store.QueryDataPoints(ctx, q)
	if err != nil {
		return fmt.Errorf("failed to query data points: %w", err)
	}

	for _, dp := range dps {
		if dp.ProcessingStatus == schema.StatusPending {
			fmt.Printf("Cognifying pending data point: %s (Content: %.30s...)\n", dp.ID, dp.Content)
			_, err := e.Cognify(ctx, dp, WithWaitCognify(true))
			if err != nil {
				fmt.Printf("Failed to cognify data point %s: %v\n", dp.ID, err)
			}
		}
	}
	return nil
}

// Memify finalizes the memory integration (e.g. promoting concepts).
func (e *defaultMemoryEngine) Memify(ctx context.Context, dataPoint *schema.DataPoint, opts ...MemifyOption) error {
	options := &MemifyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	threshold := float32(0.0)
	if th, ok := dataPoint.Metadata["consistency_threshold"].(float64); ok {
		threshold = float32(th)
	} else if th, ok := dataPoint.Metadata["consistency_threshold"].(float32); ok {
		threshold = th
	}

	task := &MemifyTask{
		DataPoint:            dataPoint,
		ConsistencyThreshold: threshold,
	}

	if options.WaitUntilComplete {
		return task.Execute(ctx, e.workerPool.extractor, e.workerPool.embedder, e.workerPool.store, e.workerPool.graphStore, e.workerPool.vectorStore, e.workerPool)
	}

	e.workerPool.Submit(task)
	return nil
}

// Health checks the status of the memory engine.
func (e *defaultMemoryEngine) Health(ctx context.Context) error {
	return nil
}

// Close gracefully shuts down the engine and its worker pool.
func (e *defaultMemoryEngine) Close() error {
	e.workerPool.Stop()
	return nil
}

// cleanJSONResponse helper to strip markdown code blocks from LLM output.
func (e *defaultMemoryEngine) cleanJSONResponse(resp string) string {
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return strings.TrimSpace(resp)
}
// getHistoryBuffer returns the last N messages formatted for context injection.
func (e *defaultMemoryEngine) getHistoryBuffer(ctx context.Context, sessionID string, limit int) string {
	if e.store == nil || sessionID == "" {
		return ""
	}
	messages, err := e.store.GetSessionMessages(ctx, sessionID)
	if err != nil || len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	start := 0
	if len(messages) > limit {
		start = len(messages) - limit
	}
	for _, m := range messages[start:] {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}
	return sb.String()
}
