package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

// maxVectorChunkSuffixProbes covers nested chunk embedding ids "{id}-chunk-{n}".
const maxVectorChunkSuffixProbes = 512

func deleteVectorsForDataPoint(ctx context.Context, vs vector.VectorStore, dpID string) error {
	if vs == nil || dpID == "" {
		return nil
	}
	var errs []error
	if err := vs.DeleteEmbedding(ctx, dpID); err != nil {
		errs = append(errs, fmt.Errorf("delete embedding %s: %w", dpID, err))
	}
	for i := 0; i < maxVectorChunkSuffixProbes; i++ {
		chunkID := fmt.Sprintf("%s-chunk-%d", dpID, i)
		if err := vs.DeleteEmbedding(ctx, chunkID); err != nil {
			errs = append(errs, fmt.Errorf("delete embedding %s: %w", chunkID, err))
		}
	}
	return errors.Join(errs...)
}

func deleteGraphNodesForSource(ctx context.Context, gs graph.GraphStore, sourceID string) error {
	if gs == nil || sourceID == "" {
		return nil
	}
	nodes, err := gs.FindNodesByProperty(ctx, "source_id", sourceID)
	if err != nil {
		return fmt.Errorf("find graph nodes by source_id=%s: %w", sourceID, err)
	}
	var errs []error
	for _, n := range nodes {
		if n != nil {
			if err := gs.DeleteNode(ctx, n.ID); err != nil {
				errs = append(errs, fmt.Errorf("delete graph node %s: %w", n.ID, err))
			}
		}
	}
	return errors.Join(errs...)
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
		if err := deleteGraphNodesForSource(ctx, e.graphStore, dp.ID); err != nil {
			return err
		}
		if err := deleteVectorsForDataPoint(ctx, e.vectorStore, dp.ID); err != nil {
			return err
		}
	}
	if e.graphStore != nil {
		if unscoped {
			if err := e.graphStore.DeleteGraphBySessionID(ctx, ""); err != nil {
				return fmt.Errorf("delete unscoped graph session: %w", err)
			}
		} else {
			if err := e.graphStore.DeleteGraphBySessionID(ctx, named); err != nil {
				return fmt.Errorf("delete graph session %s: %w", named, err)
			}
		}
	}
	if !unscoped {
		if err := e.store.DeleteSessionMessages(ctx, named); err != nil {
			return fmt.Errorf("delete session messages %s: %w", named, err)
		}
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
