// Example: Using ai-memory-brain entirely in memory with LM Studio
// This is perfect for short-lived agents or unit tests that require
// no disk footprint.
// Run: go run ./examples/in_memory_lmstudio/
package main

import (
	"context"
	"fmt"
	"log"
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
	fmt.Println("=== INITIALIZING IN-MEMORY BRAIN ===")

	// ─── 1. LM Studio Embedder ────────────────────────────────────────────────
	lmstudioEmb := vector.NewLMStudioEmbeddingProvider("http://localhost:1234/v1", "text-embedding-nomic-embed-text-v1.5")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)

	// ─── 2. In-Memory Stores (ZERO disk footprint) ────────────────────────────
	graphStore := graph.NewInMemoryGraphStore()
	vecStore := vector.NewInMemoryStore(nil) // nil uses default config (768 dims)

	// SQLite supports pure in-memory relational databases using the standard URI
	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    ":memory:",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "in-memory relational store")
	defer relStore.Close()

	// ─── 3. LM Studio Extractor ───────────────────────────────────────────────
	lmstudioProvider, err := extractor.NewLMStudioProvider("http://localhost:1234/v1", "qwen/qwen3-4b-2507")
	must(err, "lmstudio provider")
	llmExt := extractor.NewBasicExtractor(lmstudioProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	// Initialize Engine with all stores injected
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 2})
	defer eng.Close()

	// ─── 5. Perform Extraction & Memory Indexing ──────────────────────────────
	fmt.Println("\n=== ADDING MEMORIES ===")
	sessionID := "mem-test-999"

	texts := []string{
		"The quick brown fox jumps over the lazy dog.",
		"A fox is a smart animal. Dogs are loyal companions.",
	}

	for _, text := range texts {
		dp, err := eng.Add(ctx, text, engine.WithSessionID(sessionID))
		must(err, "add memory")
		fmt.Printf("- Added to pipeline: %s\n", dp.ID[:8])
	}

	// Give background async workers time to extract entities and build vector embeddings
	fmt.Println("Waiting for background AI workers to index memory...")
	time.Sleep(10 * time.Second)

	fmt.Println("\n=== SEARCHING ===")
	// ─── 6. Search Pipeline ───────────────────────────────────────────────────
	query := &schema.SearchQuery{
		Text:      "What do we know about foxes and dogs?",
		SessionID: sessionID,
		Limit:     3,
	}

	resp, err := eng.Search(ctx, query)
	must(err, "search")

	fmt.Printf("\nSearch Results for '%s':\n", query.Text)
	for i, res := range resp.Results {
		fmt.Printf("%d. [Score: %.2f] %s\n", i+1, res.Score, res.DataPoint.Content)
	}

	fmt.Println("\n=== VERIFYING IN-MEMORY STORES ===")
	count, _ := vecStore.GetEmbeddingCount(ctx)
	fmt.Printf("Vectors stored entirely in memory: %v\n", count)

	fmt.Println("Everything will be destroyed when the app exits.")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
