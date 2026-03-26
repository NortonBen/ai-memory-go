// Example: Building and traversing a Knowledge Graph with ai-memory-brain and LM Studio
// Run: go run ./examples/knowledge_graph_builder/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/extractor/registry"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	ctx := context.Background()
	// os.RemoveAll("./data/kg_builder")
	_ = os.MkdirAll("./data/kg_builder", 0o750)

	// ─── 1. Ollama Embedder (to avoid LMStudio limit with LLM concurrently) ─────────
	embFactory := registry.NewEmbeddingProviderFactory()
	ollamaEmb, err := embFactory.CreateProvider(&extractor.EmbeddingProviderConfig{
		Type:  extractor.EmbeddingProviderOllama,
		Model: "nomic-embed-text:latest",
	})
	must(err, "ollama embedding provider")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("ollama", cache)
	embedder.AddProvider("ollama", ollamaEmb)

	// ─── 2. SQLite Stores (Graph, Vector, Relational) ─────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/kg_builder/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/kg_builder/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/kg_builder/memory_rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	// ─── 3. LM Studio Extractor ───────────────────────────────────────────────
	llmFactory := registry.NewProviderFactory()
	lmstudioProvider, err := llmFactory.CreateProvider(&extractor.ProviderConfig{
		Type:     extractor.ProviderLMStudio,
		Endpoint: "http://localhost:1234/v1",
		Model:    "qwen/qwen3-4b-2507",
	})
	must(err, "lmstudio provider")
	llmExt := extractor.NewBasicExtractor(lmstudioProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	// We'll use 2 workers for extraction
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 2})
	defer eng.Close()

	sessionID := "kg-session-001"

	// ─── 5. Knowledge Corpus ──────────────────────────────────────────────────
	fmt.Println("=== INGESTING KNOWLEDGE CORPUS ===")
	corpus := []string{
		"Alice is a software engineer who specializes in backend systems. She loves writing code in Go.",
		"Bob is a frontend developer who works closely with Alice on the 'Aurora' project.",
		"The 'Aurora' project is a highly scalable e-commerce platform built by TechCorp.",
		"TechCorp is a technology company headquartered in San Francisco. They recently adopted Go for all backend services.",
		"Alice introduced Bob to Go programming, and now Bob is learning it for a new microservice.",
	}

	for _, text := range corpus {
		dp, err := eng.Add(ctx, text, engine.WithWaitAdd(true), engine.WithConsistencyThreshold(0.5))
		must(err, "Add")
		fmt.Printf("Added to memory: %s...\n", text[:40])
		if _, err := eng.Cognify(ctx, dp, engine.WithWaitCognify(true)); err != nil {
			log.Printf("Warning: cognify failed for %s: %v", dp.ID, err)
		}
		// if err := eng.Memify(ctx, dp, engine.WithWaitMemify(true)); err != nil {
		// 	log.Printf("Warning: memify failed for %s: %v", dp.ID, err)
		// }
	}
	// ─── 6. Graph Traversal ───────────────────────────────────────────────────
	fmt.Println("\n=== GRAPH TRAVERSAL RESULTS ===")

	question := "Nhân viên của TechCorp là ai"

	fmt.Println("\n-- Thinking about: '" + question + "'")
	thinkResult, err := eng.Think(ctx, &schema.ThinkQuery{
		Text:      question,
		SessionID: sessionID,
		Limit:     3,
		HopDepth:  2,
	})
	must(err, "think")

	fmt.Printf("\n🤔 AI Reasoning:\n%s\n", thinkResult.Reasoning)
	fmt.Printf("\n💡 AI Answer:\n%s\n", thinkResult.Answer)

	// Let's also demonstrate a direct search
	fmt.Println("\n-- Direct Search for '" + question + "':")
	results, err := eng.Search(ctx, &schema.SearchQuery{
		Text:      question,
		SessionID: sessionID,
		Mode:      schema.ModeContextualRAG,
		Limit:     3,
	})
	must(err, "search")

	fmt.Printf("Found %d relevant memories for 'Alice'\n", len(results.Results))
	for _, res := range results.Results {
		fmt.Printf("- %s\n", res.DataPoint.Content)
	}

	// For demonstration, let's manually traverse using a likely node ID format.
	// The BasicExtractor typically generates keys by lowercasing and replacing spaces with hyphens.
	likelyAliceID := "alice"

	fmt.Printf("\n-- Traversing 2 hops from node '%s':\n", likelyAliceID)
	neighbors, err := graphStore.TraverseGraph(ctx, likelyAliceID, 2, nil)
	if err == nil && len(neighbors) > 0 {
		fmt.Printf("Found %d connected entities:\n", len(neighbors))
		for _, n := range neighbors {
			fmt.Printf("  -> [%s] %s\n", n.Type, n.ID)
		}
	} else {
		fmt.Println("  (No direct connections found using exact ID 'alice', or extraction yielded a different key.)")

		// Fallback: Just let's list some edges in the graph
		fmt.Println("\n-- Listing all recognized edges in the graph instead:")
		// A real app would query the graph store, but SQLiteGraphStore doesn't expose a ListEdges publicly right now.
		// In a production app you would run a SQL query against memory_graph.db directly.
	}

	fmt.Println("\n✅ Knowledge Graph Builder Example Complete.")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
