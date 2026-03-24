package vector

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
)

func TestQdrantParseID(t *testing.T) {
	id1 := "test-id-1"
	id2 := "test-id-2"

	qId1 := parseID(id1)
	qId2 := parseID(id2)

	if qId1.GetUuid() == "" {
		t.Errorf("Expected parseID to return UUID wrapped ID, got: %v", qId1)
	}

	if qId1.GetUuid() == qId2.GetUuid() {
		t.Errorf("Expected different string IDs to map to different UUIDs")
	}

	// Should be deterministic
	qId1_again := parseID(id1)
	if qId1.GetUuid() != qId1_again.GetUuid() {
		t.Errorf("Expected deterministic UUID generation, but got different: %v != %v", qId1.GetUuid(), qId1_again.GetUuid())
	}
}

func TestQdrantBuildFilter(t *testing.T) {
	filters := map[string]interface{}{
		"category": "docs",
	}

	filter := buildFilter(filters)

	if filter == nil {
		t.Fatalf("Expected filter to be built, got nil")
	}

	if len(filter.Must) != 1 {
		t.Fatalf("Expected 1 Must condition, got %d", len(filter.Must))
	}

	cond := filter.Must[0]
	fieldCond := cond.GetField()
	if fieldCond == nil {
		t.Fatalf("Expected field condition")
	}

	if fieldCond.Key != "category" {
		t.Errorf("Expected key 'category', got '%s'", fieldCond.Key)
	}

	matchCond := fieldCond.Match.GetMatchValue()
	switch m := matchCond.(type) {
	case *qdrant.Match_Keyword:
		if m.Keyword != "docs" {
			t.Errorf("Expected keyword 'docs', got '%s'", m.Keyword)
		}
	default:
		t.Errorf("Expected Match_Keyword condition type, got %T", m)
	}
}
