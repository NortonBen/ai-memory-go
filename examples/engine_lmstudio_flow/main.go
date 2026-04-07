// Example: verify engine flow Add -> Cognify -> graph extraction with LM Studio.
// Run: go run ./examples/engine_lmstudio_flow/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/extractor/registry"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/storage"
	_ "github.com/NortonBen/ai-memory-go/storage/adapters/sqlite"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	ctx := context.Background()

	baseDir := filepath.Join(".", "data", "engine_lmstudio_flow")
	_ = os.MkdirAll(baseDir, 0o750)

	lmStudioURL := getenv("LMSTUDIO_URL", "http://localhost:1234/v1")
	llmModel := getenv("LMSTUDIO_MODEL", "qwen/qwen3-4b-2507")
	embModel := getenv("LMSTUDIO_EMBEDDING_MODEL", "text-embedding-nomic-embed-text-v1.5")

	llmFactory := registry.NewProviderFactory()
	llmProvider, err := llmFactory.CreateProvider(&extractor.ProviderConfig{
		Type:     extractor.ProviderLMStudio,
		Endpoint: lmStudioURL,
		Model:    llmModel,
	})
	must(err, "llm provider")
	defer llmProvider.Close()

	ext := extractor.NewBasicExtractor(llmProvider, &extractor.ExtractionConfig{
		UseJSONSchema: true,
		StrictMode:    true,
	})

	embFactory := registry.NewEmbeddingProviderFactory()
	embedder, err := embFactory.CreateProvider(&extractor.EmbeddingProviderConfig{
		Type:       extractor.EmbeddingProviderLMStudio,
		Endpoint:   lmStudioURL,
		Model:      embModel,
		Dimensions: 768,
	})
	must(err, "embedding provider")
	defer embedder.Close()

	graphStore, err := graph.NewSQLiteGraphStore(filepath.Join(baseDir, "graph.db"))
	must(err, "graph store")
	defer graphStore.Close()

	vectorStore, err := vector.NewSQLiteVectorStore(filepath.Join(baseDir, "vectors.db"), 768)
	must(err, "vector store")
	defer vectorStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    filepath.Join(baseDir, "rel.db"),
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	eng := engine.NewMemoryEngineWithStores(ext, embedder, relStore, graphStore, vectorStore, engine.EngineConfig{MaxWorkers: 4})
	defer eng.Close()

	input := "Alice works at OpenAI and is leading project Apollo. Task API-23 depends on task API-10."
	dp, err := eng.Add(ctx, input, engine.WithSessionID("default"))
	must(err, "engine.Add")

	_, err = eng.Cognify(ctx, dp, engine.WithWaitCognify(true))
	must(err, "engine.Cognify(wait=true)")

	// Read the processed chunk datapoint from relational store.
	all, err := relStore.QueryDataPoints(ctx, &storage.DataPointQuery{SessionID: "default", Limit: 100})
	must(err, "query datapoints")

	var maxNodes, maxEdges int
	for _, item := range all {
		if len(item.Nodes) > maxNodes {
			maxNodes = len(item.Nodes)
		}
		if len(item.Edges) > maxEdges {
			maxEdges = len(item.Edges)
		}
	}

	graphNodes, _ := graphStore.GetNodeCount(ctx)
	graphEdges, _ := graphStore.GetEdgeCount(ctx)

	fmt.Printf("DataPoints: %d | max_nodes_in_dp=%d | max_edges_in_dp=%d\n", len(all), maxNodes, maxEdges)
	fmt.Printf("GraphStore: nodes=%d edges=%d\n", graphNodes, graphEdges)

	if maxNodes == 0 || maxEdges == 0 || graphNodes == 0 || graphEdges == 0 {
		log.Fatalf("engine flow failed: expected extracted graph, got datapoint nodes=%d edges=%d graph nodes=%d edges=%d",
			maxNodes, maxEdges, graphNodes, graphEdges)
	}
	fmt.Println("OK: engine flow extracted graph successfully.")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("%s failed: %v", label, err)
	}
}
