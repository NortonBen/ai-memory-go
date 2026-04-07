package extractor

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateEntityPrompt_IncludesNewOntology(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "mock"), nil)
	p := be.generateEntityPrompt("Alice works on Project X at OpenAI.")

	for _, expected := range []string{"Person", "Org", "Project", "Task", "Event"} {
		if !strings.Contains(p, expected) {
			t.Fatalf("entity prompt must include %q, got: %s", expected, p)
		}
	}
}

func TestGenerateRelationshipPrompt_IncludesNewEdgeTypes(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "mock"), nil)
	entities, err := be.ExtractEntities(context.Background(), "Alice and OpenAI")
	if err != nil {
		t.Fatalf("ExtractEntities error: %v", err)
	}
	p := be.generateRelationshipPrompt("Alice works on Project X", entities)

	for _, expected := range []string{"MENTIONS", "WORKS_ON", "DEPENDS_ON", "DISCUSSED_IN"} {
		if !strings.Contains(p, expected) {
			t.Fatalf("relationship prompt must include %q, got: %s", expected, p)
		}
	}
}
