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
	// Using your local LM Studio instance for generating embeddings
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
	// Using local LM Studio for knowledge extraction
	lmstudioProvider, err := extractor.NewLMStudioProvider("http://localhost:1234/v1", "qwen/qwen3-4b-2507")
	must(err, "lmstudio provider")
	llmExt := extractor.NewBasicExtractor(lmstudioProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	// 2 workers for extraction
	eng := engine.NewMemoryEngine(llmExt, embedder, relStore, engine.EngineConfig{MaxWorkers: 2})
	defer eng.Close()

	// Override the Engine's generic storage interfaces with our in-memory instances
	// Since engine initialization doesn't inject Graph/Vector instances dynamically
	// beyond the Storage facade without internal wiring. Wait! The engine uses GraphStore and VectorStore.
	// We need to inject them. The Engine requires a unified `storage.Storage` facade internally, 
	// OR we can just use the components directly to demonstrate the unified interface isn't strictly necessary.
	// Actually, wait! The current signature for MemoryEngine doesn't accept vector/graph natively.
	// Quick fix: the SQLite "unified" approach in our previous example uses `storage.NewSQLiteAdapter` which handles BOTH.
	// But `storage.Storage` only covers relational store. Vector and Graph are managed by `engine.MemoryEngine` directly but we don't pass them to NewMemoryEngine?
	
	// Wait, engine.NewMemoryEngine:
	// func NewMemoryEngine(ex extractor.Extractor, emb vector.Embedder, store storage.Storage, config EngineConfig) *MemoryEngine
	// Where does GraphStore and VectorStore get set?!
	// Oh, `MemoryEngine` might not actually use GraphStore directly yet, or it fetches it from globals? No, in quickstart we did:
	// vecStore.StoreEmbedding(...) and graphStore.StoreBatch(...) DIRECTLY. 
	// The `MemoryEngine` just uses Extractors and `AddMemory` to Relational store.
	
	// Ah, that's right! The `Cognify` method of MemoryEngine: if it needs GraphStore and VectorStore, how does it get them?
	// Let's just do it exactly like Quickstart! In Quickstart, Cognify calls the extractor but didn't automatically save to graph/vector unless we did it. 
	// Wait, if it didn't save automatically, we would save them ourselves.
	// Let's do that!

	// ─── 5. Perform Extraction & Memory Indexing ──────────────────────────────
	fmt.Println("Adding volatile memory to the Engine...")
	sessionID := "mem-test-999"

	texts := []string{
		"The quick brown fox jumps over the lazy dog.",
		"A fox is a smart animal. Dogs are loyal companions.",
	}

	for _, text := range texts {
		dp, err := eng.AddMemory(ctx, text, sessionID)
		must(err, "add memory")

		fmt.Printf("- Added: %s\n", text)
		
		// Run Cognify to get Entities
		nodes, err := llmExt.ExtractEntities(ctx, dp.Content)
		if err != nil {
			log.Printf("Entity extraction failed: %v", err)
			continue
		}

		// Run Cognify to get Edges
		edges, err := llmExt.ExtractRelationships(ctx, dp.Content, nodes)
		if err != nil {
			log.Printf("Relationship extraction failed: %v", err)
			continue
		}

		// Store in Graph
		for _, n := range nodes {
			_ = graphStore.StoreNode(ctx, &n)
		}
		var edgePtrs []*schema.Edge
		for i := range edges {
			edgePtrs = append(edgePtrs, &edges[i])
		}
		_ = graphStore.StoreBatch(ctx, nil, edgePtrs)

		// Store in Vector
		emb, err := embedder.GenerateEmbedding(ctx, dp.Content)
		if err == nil {
			_ = vecStore.StoreEmbedding(ctx, dp.ID, emb, map[string]interface{}{"source": "text"})
		}
	}

	fmt.Println("\n=== SUCCESS ===")
	count, _ := vecStore.GetEmbeddingCount(ctx)
	fmt.Printf("Vectors stored entirely in memory: %d\n", count)
	
	fmt.Println("Everything will be destroyed when the app exits.")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
