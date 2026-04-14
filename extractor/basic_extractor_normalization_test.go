package extractor

import (
	"math"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
)

func TestNormalizeNodeType_Aliases(t *testing.T) {
	cases := map[string]schema.NodeType{
		"Organization": schema.NodeTypeOrg,
		"organisation": schema.NodeTypeOrg,
		"company":      schema.NodeTypeOrg,
		"Person":       schema.NodeTypePerson,
		"initiative":   schema.NodeTypeProject,
		"todo":         schema.NodeTypeTask,
	}
	for in, expected := range cases {
		got, ok := normalizeNodeType(in)
		if !ok || got != expected {
			t.Fatalf("normalizeNodeType(%q)=%q, want %q", in, got, expected)
		}
	}
	if _, ok := normalizeNodeType("AlienCategory"); ok {
		t.Fatalf("unknown node type should return ok=false")
	}
}

func TestNormalizeEdgeType_Aliases(t *testing.T) {
	cases := map[string]schema.EdgeType{
		"mention":      schema.EdgeTypeMentions,
		"assigned_to":  schema.EdgeTypeWorksOn,
		"leads":        schema.EdgeTypeWorksOn,
		"manages":      schema.EdgeTypeWorksOn,
		"owns":         schema.EdgeTypeWorksOn,
		"requires":     schema.EdgeTypeDependsOn,
		"needs":        schema.EdgeTypeDependsOn,
		"mentioned_in": schema.EdgeTypeDiscussedIn,
		"relates_to":   schema.EdgeTypeRelatedTo,
		"member_of":    schema.EdgeTypeRelatedTo,
		"located_in":   schema.EdgeTypeRelatedTo,
	}
	for in, expected := range cases {
		got, ok := normalizeEdgeType(in)
		if !ok || got != expected {
			t.Fatalf("normalizeEdgeType(%q)=%q, want %q", in, got, expected)
		}
	}
	if _, ok := normalizeEdgeType("MYSTERY_EDGE"); ok {
		t.Fatalf("unknown edge type should return ok=false")
	}
}

func TestNormalizeConfidence(t *testing.T) {
	if v, ok := normalizeConfidence(0.72); !ok || v != 0.72 {
		t.Fatalf("expected valid confidence 0.72, got %v ok=%v", v, ok)
	}
	if v, ok := normalizeConfidence(1.7); !ok || v != 1 {
		t.Fatalf("expected clamped confidence 1.0, got %v ok=%v", v, ok)
	}
	if _, ok := normalizeConfidence(0); ok {
		t.Fatalf("expected zero confidence to be invalid")
	}
	if _, ok := normalizeConfidence(math.NaN()); ok {
		t.Fatalf("expected NaN confidence to be invalid")
	}
	if _, ok := normalizeConfidence(math.Inf(1)); ok {
		t.Fatalf("expected +Inf confidence to be invalid")
	}
}
