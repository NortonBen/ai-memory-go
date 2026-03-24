// Example: Using ai-memory-brain with SQLite and LM Studio for project documentation
// Run: go run ./examples/sqlite_lmstudio/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	ctx := context.Background()
	fmt.Println("=== INITIALIZING SQLITE PROJECT BRAIN ===")

	_ = os.MkdirAll("./data/project_brain", 0o750)

	// ─── 1. LM Studio Embedder ────────────────────────────────────────────────
	lmstudioEmb := vector.NewLMStudioEmbeddingProvider("http://localhost:1234/v1", "text-embedding-nomic-embed-text-v1.5")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)

	// ─── 2. SQLite Stores ─────────────────────────────────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/project_brain/graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/project_brain/vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/project_brain/rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "relational store")
	defer relStore.Close()

	// ─── 3. LM Studio Extractor ───────────────────────────────────────────────
	lmstudioProvider, err := extractor.NewLMStudioProvider("http://localhost:1234/v1", "qwen/qwen3-4b-2507")
	must(err, "lmstudio provider")
	llmExt := extractor.NewBasicExtractor(lmstudioProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 4})
	defer eng.Close()

	// ─── 5. Add Project Architecture Knowledge ─────────────────────────────────
	sessionID := "architecture-session"

	fmt.Println("\n=== INGESTING PROJECT ARCHITECTURE ===")
	texts := []string{
		"The MemoryEngine provides a unified API to add and search knowledge.",
		"WorkerPool runs async CognifyTasks to perform AI entity extraction in the background.",
		"Data is persisted across RelationalStore, GraphStore, and VectorStore.",
	}

	for _, text := range texts {
		dp, err := eng.Add(ctx, text, engine.WithSessionID(sessionID), engine.WithMetadata(map[string]interface{}{"source": "docs"}))
		must(err, "add memory")
		fmt.Printf("- Stored datapoint: %s\n", dp.ID[:8])
	}

	// Give background async workers time to extract entities and build vector embeddings
	fmt.Println("Waiting for async background workers to process embeddings and graph nodes (10s)...")
	time.Sleep(10 * time.Second)

	// ─── 6. Search Pipeline ───────────────────────────────────────────────────
	fmt.Println("\n=== SEARCHING PROJECT KNOWLEDGE ===")
	query := &schema.SearchQuery{
		Text:      "How does the system extract entities in the background?",
		SessionID: sessionID,
		Limit:     3,
	}

	resp, err := eng.Search(ctx, query)
	must(err, "search")

	fmt.Printf("\nQ: %s\n", query.Text)
	for i, res := range resp.Results {
		fmt.Printf("%d. [Score: %.2f] %s\n", i+1, res.Score, res.DataPoint.Content)
	}

	fmt.Println("\n=== VERIFYING SQLITE STORES ===")
	count, _ := vecStore.GetEmbeddingCount(ctx)
	fmt.Printf("Vectors persisted in SQLite: %v\n", count)

	fmt.Println("Data is saved to ./data/project_brain/")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
