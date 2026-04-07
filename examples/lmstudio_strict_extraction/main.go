// Example: Strict ontology extraction with LM Studio
// Run: go run ./examples/lmstudio_strict_extraction/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/extractor/registry"
)

func main() {
	ctx := context.Background()

	lmStudioURL := getenv("LMSTUDIO_URL", "http://localhost:1234/v1")
	modelName := getenv("LMSTUDIO_MODEL", "qwen/qwen3-4b-2507")

	factory := registry.NewProviderFactory()
	provider, err := factory.CreateProvider(&extractor.ProviderConfig{
		Type:     extractor.ProviderLMStudio,
		Endpoint: lmStudioURL,
		Model:    modelName,
	})
	if err != nil {
		log.Fatalf("init LM Studio provider failed: %v", err)
	}
	defer provider.Close()

	ext := extractor.NewBasicExtractor(provider, &extractor.ExtractionConfig{
		UseJSONSchema: true,
		StrictMode:    true, // reject unknown NodeType/EdgeType to catch prompt drift
	})

	text := "Alice works at OpenAI and is leading project Apollo. Task API-23 depends on task API-10."
	entities, err := ext.ExtractEntities(ctx, text)
	if err != nil {
		log.Fatalf("extract entities failed: %v", err)
	}
	rels, err := ext.ExtractRelationships(ctx, text, entities)
	if err != nil {
		log.Fatalf("extract relationships failed: %v", err)
	}

	fmt.Println("=== Entities ===")
	for _, n := range entities {
		fmt.Printf("- %s (%s) confidence=%v\n", nameOf(n.Properties), n.Type, n.Properties["confidence"])
	}

	fmt.Println("\n=== Relationships ===")
	for _, e := range rels {
		fmt.Printf("- %s -> %s (%s) confidence=%v weight=%.2f\n",
			e.From, e.To, e.Type, e.Properties["confidence"], e.Weight)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func nameOf(props map[string]interface{}) string {
	if props == nil {
		return ""
	}
	if n, ok := props["name"].(string); ok {
		return n
	}
	return ""
}
