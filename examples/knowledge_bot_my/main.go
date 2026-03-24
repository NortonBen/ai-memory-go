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
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	ctx := context.Background()
	// os.RemoveAll("./data/news")
	_ = os.MkdirAll("./data/bot_my", 0o750)

	// ─── 1. LM Studio Embedder ────────────────────────────────────────────────
	lmstudioEmb := vector.NewLMStudioEmbeddingProvider("http://localhost:1234/v1", "text-embedding-nomic-embed-text-v1.5")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)

	// ─── 2. SQLite Stores (Graph, Vector, Relational) ─────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/bot_my/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/bot_my/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/bot_my/memory_rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	// ─── 3. LM Studio Extractor ───────────────────────────────────────────────
	lmstudioProvider, err := extractor.NewLMStudioProvider("http://localhost:1234/v1", "qwen/qwen3-4b-2507")
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
		"Bạn là bot và tên Denji, khi trả lời hãy đổi từ nhân xưng là Tôi",
		"Người dùng tên là Benji, khi trả lời hãy đổi từ nhân xưng là Bạn",
	}

	for _, text := range corpus {
		dp, err := eng.Add(ctx, text, engine.WithWaitAdd(true), engine.WithConsistencyThreshold(0.5))
		must(err, "Add")
		fmt.Printf("Added to memory: %s...\n", text[:10])
		if _, err := eng.Cognify(ctx, dp, engine.WithWaitCognify(true)); err != nil {
			log.Printf("Warning: cognify failed for %s: %v", dp.ID, err)
		}
		if err := eng.Memify(ctx, dp, engine.WithWaitMemify(true)); err != nil {
			log.Printf("Warning: memify failed for %s: %v", dp.ID, err)
		}
	}
	// ─── 6. Graph Traversal ───────────────────────────────────────────────────
	fmt.Println("\n=== GRAPH TRAVERSAL RESULTS ===")

	question := "Mày tên là gì"

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

	fmt.Println("\n✅ Knowledge Graph Builder Example Complete.")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
