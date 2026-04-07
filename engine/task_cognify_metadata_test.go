package engine

import (
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
)

func TestNormalizeExtractedGraphMetadata(t *testing.T) {
	dp := &schema.DataPoint{
		ID:        "dp-1",
		SessionID: "s-1",
		Metadata: map[string]interface{}{
			"confidence": 0.93,
		},
		Nodes: []*schema.Node{
			{ID: "n1", Properties: map[string]interface{}{"name": "Alice", "confidence": 0.61}},
		},
		Edges: []schema.Edge{
			{ID: "e1", From: "n1", To: "n2", Properties: map[string]interface{}{"confidence": 0.55}},
		},
	}

	normalizeExtractedGraphMetadata(dp)

	if dp.Nodes[0].Properties["source_id"] != "dp-1" {
		t.Fatalf("node source_id not set")
	}
	if dp.Nodes[0].Properties["confidence"] != 0.61 {
		t.Fatalf("node confidence mismatch: %#v", dp.Nodes[0].Properties["confidence"])
	}
	if _, ok := dp.Nodes[0].Properties["timestamp"]; !ok {
		t.Fatalf("node timestamp missing")
	}
	if dp.Nodes[0].SessionID != "s-1" {
		t.Fatalf("node session_id not propagated")
	}

	if dp.Edges[0].Properties["source_id"] != "dp-1" {
		t.Fatalf("edge source_id not set")
	}
	if dp.Edges[0].Properties["confidence"] != 0.55 {
		t.Fatalf("edge confidence mismatch: %#v", dp.Edges[0].Properties["confidence"])
	}
	if _, ok := dp.Edges[0].Properties["timestamp"]; !ok {
		t.Fatalf("edge timestamp missing")
	}
	if dp.Edges[0].SessionID != "s-1" {
		t.Fatalf("edge session_id not propagated")
	}
}
