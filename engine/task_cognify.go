package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/parser/formats"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
	"golang.org/x/sync/errgroup"
)

// CognifyTask is responsible for processing a new memory DataPoint.
// It generates embeddings, extracts entities and relationships, and saves them as JSON drafts.
type CognifyTask struct {
	DataPoint            *schema.DataPoint
	ConsistencyThreshold float32
}

const (
	// maxChunkDispatchPerSplit controls how many child chunk tasks are dispatched
	// immediately after a parent split. Remaining chunks stay pending and will be
	// picked up by the next Cognify sweep, preventing burst overload.
	maxChunkDispatchPerSplit = 64
)

// Execute performs the extraction and embedding logic for CognifyTask.
func (t *CognifyTask) Execute(ctx context.Context, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore, wp *WorkerPool) error {
	if t.DataPoint.Metadata == nil {
		t.DataPoint.Metadata = make(map[string]interface{})
	}

	// Parent/input DataPoint: split once into child chunk DataPoints and enqueue as pending.
	// Parent is not directly extracted; children are.
	if !isChunkDataPoint(t.DataPoint) {
		if getBoolMetadata(t.DataPoint.Metadata, "chunks_created") {
			// Parent already split, just keep status synchronized.
			if err := syncParentStatusFromChildren(ctx, store, t.DataPoint); err != nil {
				log.Printf("failed to sync parent status for %s: %v", t.DataPoint.ID, err)
			}
			return nil
		}

		config := schema.DefaultChunkingConfig()
		config.Strategy = schema.StrategySentence
		config.MaxSize = 1000
		config.MinSize = 500
		config.Overlap = 100
		tp := formats.NewTextParser(config)

		chunks, err := tp.ParseText(ctx, t.DataPoint.Content)
		if err != nil {
			return t.fail(ctx, store, fmt.Errorf("chunking failed: %w", err))
		}

		log.Printf("Task %s: split into %d child chunk DataPoints", t.DataPoint.ID, len(chunks))
		var createdChildren []*schema.DataPoint
		for i, chunk := range chunks {
			childID := fmt.Sprintf("%s-chunk-%03d", t.DataPoint.ID, i)

			childMeta := copyMetadata(t.DataPoint.Metadata)
			childMeta["is_chunk"] = true
			childMeta["is_input"] = false
			childMeta["parent_id"] = t.DataPoint.ID
			childMeta["chunk_index"] = i
			childMeta["total_chunks"] = len(chunks)

			child := &schema.DataPoint{
				ID:               childID,
				Content:          chunk.Content,
				ContentType:      t.DataPoint.ContentType,
				Metadata:         childMeta,
				SessionID:        t.DataPoint.SessionID,
				UserID:           t.DataPoint.UserID,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
				ProcessingStatus: schema.StatusPending,
			}
			if err := store.StoreDataPoint(ctx, child); err != nil {
				return t.fail(ctx, store, fmt.Errorf("failed to create child chunk datapoint %s: %w", childID, err))
			}
			createdChildren = append(createdChildren, child)
		}

		t.DataPoint.Metadata["chunks_created"] = true
		t.DataPoint.Metadata["total_chunks"] = len(chunks)
		t.DataPoint.Metadata["processed_chunks"] = 0
		t.DataPoint.ProcessingStatus = schema.StatusProcessing
		t.DataPoint.UpdatedAt = time.Now()
		if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
			return t.fail(ctx, store, fmt.Errorf("failed to update parent datapoint after chunk split: %w", err))
		}

		// If worker pool is available, enqueue child chunks best-effort.
		// IMPORTANT: do not block here when queue is full, otherwise large files
		// can deadlock the parent split phase (e.g. >1000 child chunks).
		if wp != nil {
			dispatchCap := maxChunkDispatchPerSplit
			if dispatchCap > len(createdChildren) {
				dispatchCap = len(createdChildren)
			}
			enqueued := 0
			for i, child := range createdChildren {
				if i >= dispatchCap {
					break
				}
				if trySubmitTask(wp, &CognifyTask{
					DataPoint:            child,
					ConsistencyThreshold: t.ConsistencyThreshold,
				}) {
					enqueued++
				}
			}
			if enqueued < dispatchCap {
				log.Printf("Task %s: worker queue saturated, enqueued %d/%d dispatch-cap children; remaining stay pending for sweep",
					t.DataPoint.ID, enqueued, dispatchCap)
			}
			if dispatchCap < len(createdChildren) {
				log.Printf("Task %s: dispatch cap=%d, deferred %d/%d children to pending sweep",
					t.DataPoint.ID, dispatchCap, len(createdChildren)-dispatchCap, len(createdChildren))
			}
		}
		return nil
	}

	t.DataPoint.ProcessingStatus = schema.StatusProcessing
	if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
		log.Printf("Failed to update status to processing: %v", err)
	}

	// 1. Chunking the content if it's large
	config := schema.DefaultChunkingConfig()
	config.Strategy = schema.StrategySentence
	// Keep chunks compact for downstream extraction quality.
	// Target range: ~100-500 characters per chunk.
	config.MaxSize = 500
	config.MinSize = 100
	config.Overlap = 50
	tp := formats.NewTextParser(config)

	chunks, err := tp.ParseText(ctx, t.DataPoint.Content)
	if err != nil {
		return t.fail(ctx, store, fmt.Errorf("chunking failed: %w", err))
	}

	type chunkResult struct {
		embedding []float32
		nodes     []schema.Node
		edges     []schema.Edge
	}

	// Pre-allocate result slots so each goroutine writes to its own index (no mutex needed).
	results := make([]*chunkResult, len(chunks))

	// 2. Process each chunk with max ChunkConcurrency concurrent goroutines (default 4).
	chunkConcurrency := 4
	if wp != nil && wp.ChunkConcurrency > 0 {
		chunkConcurrency = wp.ChunkConcurrency
	}

	log.Printf("Task %s: Processing %d chunks (max %d concurrent)", t.DataPoint.ID, len(chunks), chunkConcurrency)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(chunkConcurrency)

	for i, chunk := range chunks {
		i, chunk := i, chunk
		g.Go(func() error {
			log.Printf("Task %s: chunk[%d] len=%d preview=%q", t.DataPoint.ID, i, len([]rune(chunk.Content)), chunkPreview(chunk.Content, 120))

			// 2a. Generate embedding for chunk
			embedding, embErr := emb.GenerateEmbedding(gctx, chunk.Content)
			if embErr != nil {
				log.Printf("Warning: chunk %d embedding failed for %s: %v", i, t.DataPoint.ID, embErr)
				return nil
			}

			// 2b. Store Chunk Embedding into VectorStore if provided
			if vectorStore != nil {
				chunkID := fmt.Sprintf("%s-chunk-%d", t.DataPoint.ID, i)
				if storeErr := vectorStore.StoreEmbedding(gctx, chunkID, embedding, map[string]interface{}{
					"content_type": t.DataPoint.ContentType,
					"session_id":   t.DataPoint.SessionID,
					"user_id":      t.DataPoint.UserID,
					"source_id":    t.DataPoint.ID,
					"chunk_index":  i,
					"is_chunk":     true,
				}); storeErr != nil {
					log.Printf("failed to store chunk embedding %d in vectorStore: %v", i, storeErr)
				}
			}

			// 3. Extract Entities via LLM per chunk
			nodes, entErr := ext.ExtractEntities(gctx, chunk.Content)
			if entErr != nil {
				log.Printf("Warning: entity extraction failed for chunk %d: %v", i, entErr)
				return nil
			}

			// 4. Extract Relationships via LLM per chunk
			edges, relErr := ext.ExtractRelationships(gctx, chunk.Content, nodes)
			if relErr != nil {
				log.Printf("Warning: relationship extraction returned error for chunk %d: %v", i, relErr)
			}

			results[i] = &chunkResult{embedding: embedding, nodes: nodes, edges: edges}
			return nil
		})
	}

	// Wait for all goroutines to finish (errors are logged inside, never propagated).
	_ = g.Wait()

	// Consolidate results in original chunk order.
	var allNodes []*schema.Node
	var allEdges []schema.Edge
	var chunkEmbeddings [][]float32

	for i, r := range results {
		if r == nil {
			continue
		}
		chunkEmbeddings = append(chunkEmbeddings, r.embedding)
		for j := range r.nodes {
			allNodes = append(allNodes, &r.nodes[j])
		}
		allEdges = append(allEdges, r.edges...)
		_ = i
	}

	// 5. Consolidate results
	if len(chunkEmbeddings) > 0 {
		// Calculate AVERAGE embedding
		dim := len(chunkEmbeddings[0])
		avgEmb := make([]float32, dim)
		for _, vec := range chunkEmbeddings {
			for j := 0; j < dim; j++ {
				avgEmb[j] += vec[j]
			}
		}
		for j := 0; j < dim; j++ {
			avgEmb[j] /= float32(len(chunkEmbeddings))
		}
		t.DataPoint.Embedding = avgEmb

		// Store Document (Average) Embedding if not already stored (for root DP)
		if vectorStore != nil && len(chunks) > 0 {
			err = vectorStore.StoreEmbedding(ctx, t.DataPoint.ID, avgEmb, map[string]interface{}{
				"content_type": t.DataPoint.ContentType,
				"session_id":   t.DataPoint.SessionID,
				"user_id":      t.DataPoint.UserID,
				"is_document":  true,
			})
		}
	}

	t.DataPoint.Nodes = allNodes
	t.DataPoint.Edges = allEdges

	// 6. Update Status
	t.DataPoint.ProcessingStatus = schema.StatusCognified
	t.DataPoint.UpdatedAt = time.Now()

	// 7. Save updated DataPoint with drafts
	if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
		return t.fail(ctx, store, fmt.Errorf("failed to update DataPoint: %w", err))
	}

	// 8. Chain to MemifyTask in background
	if wp != nil {
		wp.Submit(&MemifyTask{
			DataPoint:            t.DataPoint,
			ConsistencyThreshold: t.ConsistencyThreshold,
		})
	}

	return nil
}

func (t *CognifyTask) fail(ctx context.Context, store storage.Storage, err error) error {
	t.DataPoint.ProcessingStatus = schema.StatusFailed
	t.DataPoint.ErrorMessage = err.Error()
	t.DataPoint.UpdatedAt = time.Now()
	_ = store.UpdateDataPoint(ctx, t.DataPoint)
	return err
}

func chunkPreview(content string, maxRunes int) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\n", " "))
	runes := []rune(normalized)
	if len(runes) <= maxRunes {
		return normalized
	}
	return string(runes[:maxRunes]) + "..."
}

func isChunkDataPoint(dp *schema.DataPoint) bool {
	return getBoolMetadata(dp.Metadata, "is_chunk")
}

func getBoolMetadata(meta map[string]interface{}, key string) bool {
	if meta == nil {
		return false
	}
	v, ok := meta[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func copyMetadata(meta map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func syncParentStatusFromChildren(ctx context.Context, store storage.Storage, parent *schema.DataPoint) error {
	q := &storage.DataPointQuery{
		SessionID: parent.SessionID,
		Limit:     100000,
	}
	all, err := store.QueryDataPoints(ctx, q)
	if err != nil {
		return err
	}

	total := 0
	completed := 0
	for _, dp := range all {
		if dp.Metadata == nil {
			continue
		}
		rawParentID, ok := dp.Metadata["parent_id"].(string)
		if !ok || rawParentID != parent.ID {
			continue
		}
		total++
		if dp.ProcessingStatus == schema.StatusCompleted {
			completed++
		}
	}

	parent.Metadata["total_chunks"] = total
	parent.Metadata["processed_chunks"] = completed
	if total > 0 && completed >= total {
		parent.ProcessingStatus = schema.StatusCompleted
	} else {
		parent.ProcessingStatus = schema.StatusProcessing
	}
	parent.UpdatedAt = time.Now()
	return store.UpdateDataPoint(ctx, parent)
}

func trySubmitTask(wp *WorkerPool, task WorkerTask) bool {
	if wp == nil || task == nil {
		return false
	}
	select {
	case wp.taskQueue <- task:
		return true
	default:
		return false
	}
}
