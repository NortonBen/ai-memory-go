package extractor

import (
	"context"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
)

func TestGenerateEntityPrompt_CustomTemplate(t *testing.T) {
	cfg := &ExtractionConfig{EntityPrompt: "ONLY: {text}"}
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "mock"), cfg)
	p := be.generateEntityPrompt("hello")
	if p != "ONLY: hello" {
		t.Fatalf("custom entity prompt: got %q", p)
	}
}

func TestGenerateEntityPrompt_IncludesNewOntology(t *testing.T) {
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "mock"), nil)
	p := be.generateEntityPrompt("Alice works on Project X at OpenAI.")

	for _, expected := range []string{"Person", "Org", "Project", "Task", "Event"} {
		if !strings.Contains(p, expected) {
			t.Fatalf("entity prompt must include %q, got: %s", expected, p)
		}
	}
}

func TestGenerateRelationshipPrompt_CustomTemplate(t *testing.T) {
	cfg := &ExtractionConfig{RelationshipPrompt: "names={entities} text={text}"}
	be := NewBasicExtractor(NewMockLLMProvider(ProviderOpenAI, "mock"), cfg)
	a := schema.NewNode(schema.NodeTypePerson, map[string]interface{}{"name": "Ann"})
	b := schema.NewNode(schema.NodeTypeOrg, map[string]interface{}{"name": "Acme"})
	p := be.generateRelationshipPrompt("story", []schema.Node{*a, *b})
	if !strings.Contains(p, "Ann, Acme") || !strings.Contains(p, "story") {
		t.Fatalf("custom relationship prompt: %s", p)
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
