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
	fourTier    schema.FourTierEngineConfig
}

// NewMemoryEngine creates a new instance of MemoryEngine using only the relational store (fallback).
func NewMemoryEngine(ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, cfg EngineConfig) MemoryEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5
	}
	if cfg.ChunkConcurrency <= 0 {
		cfg.ChunkConcurrency = 4
	}

	pool := NewWorkerPool(cfg.MaxWorkers, ext, emb, store, nil, nil)
	pool.ChunkConcurrency = cfg.ChunkConcurrency
	pool.Start()

	return &defaultMemoryEngine{
		extractor:  ext,
		embedder:   emb,
		store:      store,
		workerPool: pool,
		fourTier:   cfg.FourTier,
	}
}

// NewMemoryEngineWithStores creates a new instance of MemoryEngine including graph and vector stores for advanced features.
func NewMemoryEngineWithStores(ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore, cfg EngineConfig) MemoryEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5
	}
	if cfg.ChunkConcurrency <= 0 {
		cfg.ChunkConcurrency = 4
	}

	pool := NewWorkerPool(cfg.MaxWorkers, ext, emb, store, graphStore, vectorStore)
	pool.ChunkConcurrency = cfg.ChunkConcurrency
	pool.Start()

	engine := &defaultMemoryEngine{
		extractor:   ext,
		embedder:    emb,
		store:       store,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		workerPool:  pool,
		fourTier:    cfg.FourTier,
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
	if options.GlobalSession {
		sessionID = ""
	} else if sessionID == "" {
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

	metaLabels := schema.NormalizeLabels(options.Labels)
	if options.Metadata != nil {
		metaLabels = append(metaLabels, schema.LabelsFromMetadata(options.Metadata)...)
		metaLabels = schema.NormalizeLabels(metaLabels)
	}

	candidateTier := schema.MemoryTierGeneral
	explicitTier := false
	if strings.TrimSpace(options.MemoryTier) != "" {
		candidateTier = schema.NormalizeMemoryTier(options.MemoryTier)
		explicitTier = true
	} else if options.Metadata != nil {
		if v, ok := options.Metadata["memory_tier"].(string); ok && strings.TrimSpace(v) != "" {
			candidateTier = schema.NormalizeMemoryTier(v)
			explicitTier = true
		}
	}
	if !explicitTier {
		if t := schema.DefaultMemoryTierFromLabels(metaLabels); t != "" {
			candidateTier = t
		}
	}

	// Deduplication: Check if this exact content already exists for this session (cùng memory_tier + cùng tập nhãn).
	searchQuery := &storage.DataPointQuery{
		SearchText: content,
		SearchMode: "exact",
		Limit:      1,
	}
	if sessionID == "" {
		searchQuery.UnscopedSessionOnly = true
	} else {
		searchQuery.SessionID = sessionID
	}
	if existingMatches, err := e.store.QueryDataPoints(ctx, searchQuery); err == nil && len(existingMatches) > 0 {
		// Only dedupe against INPUT records.
		// Do not dedupe against chunk/processed datapoints.
		for _, existing := range existingMatches {
			if existing != nil && existing.Metadata != nil {
				if isInput, ok := existing.Metadata["is_input"].(bool); ok && isInput {
					if schema.MemoryTierFromDataPoint(existing) != candidateTier {
						continue
					}
					if !schema.LabelSetsEqual(metaLabels, schema.LabelsFromMetadata(existing.Metadata)) {
						continue
					}
					return existing, nil
				}
			}
		}
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
	// Mark the original user input as an input record.
	if dp.Metadata == nil {
		dp.Metadata = make(map[string]interface{})
	}
	dp.Metadata["is_input"] = true
	dp.Metadata["is_chunk"] = false
	dp.Metadata["memory_tier"] = candidateTier
	if len(metaLabels) > 0 {
		dp.Metadata[schema.MetadataKeyMemoryLabels] = schema.LabelsToMetadataSlice(metaLabels)
		dp.Metadata[schema.MetadataKeyPrimaryLabel] = metaLabels[0]
		dp.Metadata[schema.MetadataKeyLabelsJoined] = schema.JoinLabelsForVector(metaLabels)
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
		WaitUntilComplete:    options.WaitUntilComplete,
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

// CognifyPending keeps sweeping the relational store and processes all pending/processing items
// until no more work remains for the session.
func (e *defaultMemoryEngine) CognifyPending(ctx context.Context, sessionID string) error {
	for {
		q := &storage.DataPointQuery{
			SessionID: sessionID,
			Limit:     1000,
		}
		dps, err := e.store.QueryDataPoints(ctx, q)
		if err != nil {
			return fmt.Errorf("failed to query data points: %w", err)
		}

		var workload []*schema.DataPoint
		for _, dp := range dps {
			if dp.ProcessingStatus == schema.StatusPending || dp.ProcessingStatus == schema.StatusProcessing {
				workload = append(workload, dp)
			}
		}

		if len(workload) == 0 {
			break
		}

		progress := false
		for _, dp := range workload {
			// Re-process stale "processing" items by resetting to pending.
			if dp.ProcessingStatus == schema.StatusProcessing {
				dp.ProcessingStatus = schema.StatusPending
				dp.UpdatedAt = time.Now()
				if err := e.store.UpdateDataPoint(ctx, dp); err != nil {
					fmt.Printf("Failed to reset processing data point %s: %v\n", dp.ID, err)
					continue
				}
			}

			fmt.Printf("Cognifying data point: %s (status=%s, content=%.30s...)\n", dp.ID, dp.ProcessingStatus, dp.Content)
			_, err := e.Cognify(ctx, dp, WithWaitCognify(true))
			if err != nil {
				fmt.Printf("Failed to cognify data point %s: %v\n", dp.ID, err)
				continue
			}
			progress = true
		}

		if !progress {
			break
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
