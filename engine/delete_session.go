package engine

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

// maxVectorChunkSuffixProbes covers nested chunk embedding ids "{id}-chunk-{n}".
const maxVectorChunkSuffixProbes = 512

func deleteVectorsForDataPoint(ctx context.Context, vs vector.VectorStore, dpID string) {
	if vs == nil || dpID == "" {
		return
	}
	_ = vs.DeleteEmbedding(ctx, dpID)
	for i := 0; i < maxVectorChunkSuffixProbes; i++ {
		_ = vs.DeleteEmbedding(ctx, fmt.Sprintf("%s-chunk-%d", dpID, i))
	}
}

func deleteGraphNodesForSource(ctx context.Context, gs graph.GraphStore, sourceID string) {
	if gs == nil || sourceID == "" {
		return
	}
	nodes, err := gs.FindNodesByProperty(ctx, "source_id", sourceID)
	if err != nil {
		return
	}
	for _, n := range nodes {
		if n != nil {
			_ = gs.DeleteNode(ctx, n.ID)
		}
	}
}

func (e *defaultMemoryEngine) deleteAllMemoryForSession(ctx context.Context, unscoped bool, named string) error {
	if e.store == nil {
		return fmt.Errorf("store not configured")
	}
	q := &storage.DataPointQuery{
		Limit:  500000,
		Offset: 0,
	}
	if unscoped {
		q.UnscopedSessionOnly = true
	} else {
		q.SessionID = named
	}
	dps, err := e.store.QueryDataPoints(ctx, q)
	if err != nil {
		return fmt.Errorf("list datapoints for session: %w", err)
	}
	for _, dp := range dps {
		if dp == nil {
			continue
		}
		deleteGraphNodesForSource(ctx, e.graphStore, dp.ID)
		deleteVectorsForDataPoint(ctx, e.vectorStore, dp.ID)
	}
	if e.graphStore != nil {
		if unscoped {
			_ = e.graphStore.DeleteGraphBySessionID(ctx, "")
		} else {
			_ = e.graphStore.DeleteGraphBySessionID(ctx, named)
		}
	}
	if !unscoped {
		_ = e.store.DeleteSessionMessages(ctx, named)
	}
	if unscoped {
		return e.store.DeleteDataPointsUnscoped(ctx)
	}
	return e.store.DeleteDataPointsBySession(ctx, named)
}

func (e *defaultMemoryEngine) deleteMemoryWholeSession(ctx context.Context, sessionSpec string) error {
	unscoped, named, err := sessionid.ForBulkDeleteAll(sessionSpec)
	if err != nil {
		return err
	}
	return e.deleteAllMemoryForSession(ctx, unscoped, named)
}
