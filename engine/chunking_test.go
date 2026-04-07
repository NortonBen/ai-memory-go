package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"log"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/parser/formats"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/vector"
)

type counterExtractor struct {
	mockExtractor
	entityCalls int
	edgeCalls   int
}

func (m *counterExtractor) ExtractEntities(ctx context.Context, input string) ([]schema.Node, error) {
	m.entityCalls++
	log.Printf("Mock Extraction on: %s", input)
	return []schema.Node{{ID: fmt.Sprintf("node-%d", m.entityCalls), Type: "Test"}}, nil
}

func (m *counterExtractor) ExtractRelationships(ctx context.Context, input string, entities []schema.Node) ([]schema.Edge, error) {
	m.edgeCalls++
	return []schema.Edge{{ID: fmt.Sprintf("edge-%d", m.edgeCalls), From: "A", To: "B", Type: "TEST"}}, nil
}

func TestCognifyTaskSentenceOverlap(t *testing.T) {
	// Sentences lengths: 20, 23, 23 (approx with spaces)
	content := "First sentence here. Second sentence starts. Third sentence follows."

	// Testing TextParser directly is better for boundary logic.
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategySentence,
		MaxSize:  50, // Fits ~2 sentences
		Overlap:  30, // Fits ~1 sentence
		MinSize:  1,
	}
	tp := formats.NewTextParser(config)
	chunks, _ := tp.ParseText(context.Background(), content)

	if len(chunks) != 2 {
		t.Fatalf("Expected 2 chunks, got %d", len(chunks))
	}

	// Chunk 1 should be: "First sentence here. Second sentence starts."
	expected1 := "First sentence here. Second sentence starts."
	if chunks[0].Content != expected1 {
		t.Errorf("Chunk 0 mismatch.\nGot:  %q\nWant: %q", chunks[0].Content, expected1)
	}

	// Chunk 2 should start with "Second sentence starts." (the overlap)
	// and end with "Third sentence follows."
	expected2 := "Second sentence starts. Third sentence follows."
	if chunks[1].Content != expected2 {
		t.Errorf("Chunk 1 mismatch.\nGot:  %q\nWant: %q", chunks[1].Content, expected2)
	}

	// Verify the overlap is indeed a full sentence
	if !strings.HasPrefix(chunks[1].Content, "Second sentence starts.") {
		t.Errorf("Chunk 2 should start with the full overlapping sentence")
	}
}

func TestCognifyTaskChunking(t *testing.T) {
	// Create a large text that must be split into many small chunks
	// using Cognify defaults (~100-500 chars/chunk).
	largeText := strings.Repeat("This is a long sentence for testing chunking. ", 500)
	if len(largeText) < 10000 {
		t.Fatalf("Text too short: %d", len(largeText))
	}

	dp := &schema.DataPoint{
		ID:               "large-dp",
		Content:          largeText,
		ContentType:      "text",
		SessionID:        "session-1",
		ProcessingStatus: schema.StatusPending,
	}

	ext := &counterExtractor{}
	emb := &mockEmbedder{}
	store := newMockStorage()
	graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
	vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})

	task := &CognifyTask{
		DataPoint:            dp,
		ConsistencyThreshold: 0,
	}

	ctx := context.Background()
	err := task.Execute(ctx, ext, emb, store, graphStore, vecStore, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Parent datapoint should be split only (no extraction on parent itself).
	if ext.entityCalls != 0 {
		t.Errorf("Expected 0 entity extraction calls for parent split phase, got %d", ext.entityCalls)
	}
	if ext.edgeCalls != 0 {
		t.Errorf("Expected 0 relationship extraction calls for parent split phase, got %d", ext.edgeCalls)
	}

	if dp.ProcessingStatus != schema.StatusProcessing {
		t.Errorf("Expected parent status processing after split, got %s", dp.ProcessingStatus)
	}
	if created, ok := dp.Metadata["chunks_created"].(bool); !ok || !created {
		t.Errorf("Expected chunks_created=true in parent metadata")
	}

	// Verify child chunk datapoints were created and marked pending.
	childCount := 0
	for _, stored := range store.dataPointsSnapshot() {
		if stored.Metadata == nil {
			continue
		}
		if isChunk, _ := stored.Metadata["is_chunk"].(bool); !isChunk {
			continue
		}
		if parentID, _ := stored.Metadata["parent_id"].(string); parentID != dp.ID {
			continue
		}
		if stored.ProcessingStatus != schema.StatusPending {
			t.Errorf("Expected child %s status pending, got %s", stored.ID, stored.ProcessingStatus)
		}
		childCount++
	}
	if childCount < 10 {
		t.Errorf("Expected many child chunk datapoints, got %d", childCount)
	}
}

func TestCognifyParentSplitInheritsLabelsOnAllChunkChildren(t *testing.T) {
	story := "mục thần ký"
	content := strings.Repeat("Một đoạn truyện ngắn. ", 120)
	if len(content) < 800 {
		t.Fatal("content too short")
	}

	dp := &schema.DataPoint{
		ID:               "root-input",
		Content:          content,
		ContentType:      "text",
		SessionID:        "session-labels",
		ProcessingStatus: schema.StatusPending,
		Metadata: map[string]interface{}{
			"memory_tier": schema.MemoryTierGeneral,
			schema.MetadataKeyMemoryLabels: schema.LabelsToMetadataSlice([]string{story}),
			schema.MetadataKeyPrimaryLabel: story,
			schema.MetadataKeyLabelsJoined: schema.JoinLabelsForVector([]string{story}),
		},
	}

	ext := &counterExtractor{}
	emb := &mockEmbedder{}
	store := newMockStorage()
	graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
	vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})

	task := &CognifyTask{DataPoint: dp, ConsistencyThreshold: 0}
	if err := task.Execute(context.Background(), ext, emb, store, graphStore, vecStore, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, stored := range store.dataPointsSnapshot() {
		if stored.Metadata == nil {
			continue
		}
		if isChunk, _ := stored.Metadata["is_chunk"].(bool); !isChunk {
			continue
		}
		if parentID, _ := stored.Metadata["parent_id"].(string); parentID != dp.ID {
			continue
		}
		if !schema.DataPointHasAnyLabel(stored, []string{story}) {
			t.Errorf("child %s missing label %q: labels=%v primary=%v",
				stored.ID, story,
				schema.LabelsFromMetadata(stored.Metadata),
				stored.Metadata[schema.MetadataKeyPrimaryLabel])
		}
	}
}
