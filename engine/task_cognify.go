package engine

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

// CognifyTask is responsible for processing a new memory DataPoint.
// It generates embeddings, extracts entities and relationships, and saves them as JSON drafts.
type CognifyTask struct {
	DataPoint            *schema.DataPoint
	ConsistencyThreshold float32
}

// Execute performs the extraction and embedding logic for CognifyTask.
func (t *CognifyTask) Execute(ctx context.Context, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore, wp *WorkerPool) error {
	t.DataPoint.ProcessingStatus = schema.StatusProcessing
	if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
		log.Printf("Failed to update status to processing: %v", err)
	}

	// 1. Generate text embedding for the whole document
	embedding, err := emb.GenerateEmbedding(ctx, t.DataPoint.Content)
	if err != nil {
		return t.fail(ctx, store, fmt.Errorf("embedding generation failed: %w", err))
	}
	t.DataPoint.Embedding = embedding

	// 1b. Store Document Embedding into VectorStore if provided
	if vectorStore != nil {
		err = vectorStore.StoreEmbedding(ctx, t.DataPoint.ID, embedding, map[string]interface{}{
			"content_type": t.DataPoint.ContentType,
			"session_id":   t.DataPoint.SessionID,
			"user_id":      t.DataPoint.UserID,
		})
		if err != nil {
			log.Printf("failed to store embedding in vectorStore: %v", err)
		}
	}

	// 2. Extract Entities via LLM
	nodes, err := ext.ExtractEntities(ctx, t.DataPoint.Content)
	if err != nil {
		return t.fail(ctx, store, fmt.Errorf("entity extraction failed: %w", err))
	}

	// 3. Extract Relationships via LLM
	edges, err := ext.ExtractRelationships(ctx, t.DataPoint.Content, nodes)
	if err != nil {
		log.Printf("Warning: relationship extraction returned error: %v", err)
	}

	// 4. Save to JSON drafts in DataPoint
	var pointerNodes []*schema.Node
	for i := range nodes {
		pointerNodes = append(pointerNodes, &nodes[i])
	}
	t.DataPoint.Nodes = pointerNodes
	t.DataPoint.Edges = edges

	// 5. Update Status
	t.DataPoint.ProcessingStatus = schema.StatusCognified
	t.DataPoint.UpdatedAt = time.Now()

	// 6. Save updated DataPoint with drafts
	if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
		return t.fail(ctx, store, fmt.Errorf("failed to update DataPoint: %w", err))
	}

	// 7. Chain to MemifyTask in background
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
