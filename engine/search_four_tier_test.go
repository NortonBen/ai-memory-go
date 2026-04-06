package engine

import (
	"context"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/vector"
)

func TestVectorResultSourceIDPrefersMetadataSourceID(t *testing.T) {
	vr := &vector.SimilarityResult{
		ID: "root-chunk-000-chunk-1",
		Metadata: map[string]interface{}{
			"source_id": "root-chunk-000",
		},
	}
	if got := vectorResultSourceID(vr); got != "root-chunk-000" {
		t.Fatalf("got %q, want root-chunk-000", got)
	}
	if got := vectorResultSourceID(&vector.SimilarityResult{ID: "only-id", Metadata: nil}); got != "only-id" {
		t.Fatalf("fallback vr.ID: got %q", got)
	}
}

func TestUseFourTierSearchOverride(t *testing.T) {
	enabled := true
	disabled := false
	e := &defaultMemoryEngine{
		fourTier: schema.FourTierEngineConfig{Enabled: false},
	}
	if e.useFourTierSearch(&schema.SearchQuery{FourTier: &schema.FourTierSearchOptions{Enabled: &enabled}}) != true {
		t.Fatal("expected query override to enable four-tier")
	}
	if e.useFourTierSearch(&schema.SearchQuery{FourTier: &schema.FourTierSearchOptions{Enabled: &disabled}}) != false {
		t.Fatal("expected query override to disable four-tier")
	}
	e.fourTier.Enabled = true
	if e.useFourTierSearch(&schema.SearchQuery{}) != true {
		t.Fatal("expected engine config to enable four-tier")
	}
}

func TestFourTierSearchPopulatesStats(t *testing.T) {
	ctx := context.Background()
	store := newMockStorage()
	_ = store.StoreDataPoint(ctx, &schema.DataPoint{
		ID:               "core-1",
		Content:          "quy tắc alpha",
		ContentType:      "text",
		SessionID:        "s1",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: schema.StatusCognified,
		Metadata: map[string]interface{}{
			"memory_tier": schema.MemoryTierCore,
			"is_input":    true,
		},
	})
	_ = store.StoreDataPoint(ctx, &schema.DataPoint{
		ID:               "gen-1",
		Content:          "ghi chú chung beta",
		ContentType:      "text",
		SessionID:        "s1",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: schema.StatusCognified,
		Metadata:         map[string]interface{}{"is_input": true},
	})

	g, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
	v, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory, Dimension: 3})

	emb := []float32{1, 0, 0}
	_ = v.StoreEmbedding(ctx, "gen-1", emb, map[string]interface{}{
		"source_id":   "gen-1",
		"memory_tier": schema.MemoryTierGeneral,
	})
	_ = v.StoreEmbedding(ctx, "stor-1", emb, map[string]interface{}{
		"source_id":   "stor-1",
		"memory_tier": schema.MemoryTierStorage,
	})
	_ = store.StoreDataPoint(ctx, &schema.DataPoint{
		ID:               "stor-1",
		Content:          "kho lưu trữ gamma",
		ContentType:      "text",
		SessionID:        "s1",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: schema.StatusCognified,
		Metadata: map[string]interface{}{
			"memory_tier": schema.MemoryTierStorage,
			"is_input":    true,
		},
	})

	eng := NewMemoryEngineWithStores(&mockExtractor{}, &mockEmbedder{}, store, g, v, EngineConfig{
		MaxWorkers: 2,
		FourTier:   schema.FourTierEngineConfig{Enabled: true},
	}).(*defaultMemoryEngine)
	defer eng.Close()

	q := &schema.SearchQuery{
		Text:                "alpha beta gamma",
		SessionID:           "s1",
		Mode:                schema.ModeHybridSearch,
		Limit:               10,
		SimilarityThreshold: 0.01,
		FourTier: &schema.FourTierSearchOptions{
			IncludeStorageTier: true,
		},
	}
	res, err := eng.retrieveContextFourTier(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if res.FourTierStats == nil {
		t.Fatal("expected FourTierStats")
	}
	if res.FourTierStats.CoreHitCount < 1 {
		t.Errorf("core hits: got %d", res.FourTierStats.CoreHitCount)
	}
	if res.FourTierStats.StorageHitCount < 1 || !res.FourTierStats.StorageSearched {
		t.Errorf("storage: hits=%d searched=%v", res.FourTierStats.StorageHitCount, res.FourTierStats.StorageSearched)
	}
}
