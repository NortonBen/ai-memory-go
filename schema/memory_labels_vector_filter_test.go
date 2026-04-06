package schema

import "testing"

func TestEmbeddingMetadataHasAnyLabel(t *testing.T) {
	meta := map[string]interface{}{
		MetadataKeyPrimaryLabel: "Mục thần ký",
		MetadataKeyLabelsJoined: JoinLabelsForVector([]string{"rule", "mục thần ký"}),
	}
	if !EmbeddingMetadataHasAnyLabel(meta, []string{"mục thần ký"}) {
		t.Fatal("expected match primary / joined")
	}
	if EmbeddingMetadataHasAnyLabel(meta, []string{"khác hẳn"}) {
		t.Fatal("unexpected match")
	}
}

func TestMetadataMatchesVectorSearchFilters(t *testing.T) {
	meta := map[string]interface{}{
		"memory_tier":           MemoryTierGeneral,
		MetadataKeyLabelsJoined: JoinLabelsForVector([]string{"story"}),
	}
	f := map[string]interface{}{
		"memory_tier":            MemoryTierGeneral,
		VectorFilterKeyLabelsAny: []string{"story"},
	}
	if !MetadataMatchesVectorSearchFilters(meta, f) {
		t.Fatal("tier+label should match")
	}
	bad := map[string]interface{}{
		"memory_tier":            MemoryTierGeneral,
		VectorFilterKeyLabelsAny: []string{"policy"},
	}
	if MetadataMatchesVectorSearchFilters(meta, bad) {
		t.Fatal("wrong label should not match")
	}
}
