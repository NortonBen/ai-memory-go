// Example: Building and traversing a Knowledge Graph with ai-memory-brain and OpenRouter
// Run: go run ./examples/knowledge_bot_openrouter/
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
	_ = os.MkdirAll("./data/bot_openrouter", 0o750)

	openRouterApiKey := os.Getenv("OPENROUTER_API_KEY")
	if openRouterApiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is not set")
	}

	fmt.Println("OPENROUTER_API_KEY: ", openRouterApiKey)

	// ─── 1. OpenRouter Embedder ───────────────────────────────────────────────
	// Using OpenRouter for embeddings
	openRouterEmb := vector.NewOpenRouterEmbeddingProvider(vector.OpenRouterConfig{
		APIKey:  openRouterApiKey,
		Model:   "openai/text-embedding-3-small", // Common model available on OpenRouter
		SiteURL: "http://localhost",
		AppName: "KnowledgeBot",
	})
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("openrouter", cache)
	embedder.AddProvider("openrouter", openRouterEmb)

	// ─── 2. SQLite Stores (Graph, Vector, Relational) ─────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/bot_openrouter/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	// Dimensions matches text-embedding-3-small
	vecStore, err := vector.NewSQLiteVectorStore("./data/bot_openrouter/memory_vectors.db", 1536)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/bot_openrouter/memory_rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	// ─── 3. OpenRouter Extractor ──────────────────────────────────────────────
	// Using OpenRouter for extraction with a cheap but capable model
	openRouterProvider, err := extractor.NewOpenRouterProvider(
		openRouterApiKey,
		"openrouter/free",
		"http://localhost",
		"KnowledgeBot")
	must(err, "openrouter provider")
	llmExt := extractor.NewBasicExtractor(openRouterProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	// We'll use 2 workers for extraction
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 2})
	defer eng.Close()

	sessionID := "kg-session-openrouter"

	// ─── 5. Knowledge Corpus ──────────────────────────────────────────────────
	fmt.Println("=== INGESTING KNOWLEDGE CORPUS ===")
	corpus := []string{
		"OpenRouter là một dịch vụ định tuyến cho các lệnh gọi API LLM.",
		"OpenRouter cung cấp quyền truy cập vào nhiều mô hình bao gồm OpenAI, Anthropic và các giải pháp thay thế của Google.",
		"Sử dụng OpenRouter cho phép các nhà phát triển tránh bị khóa nhà cung cấp và dễ dàng chuyển đổi giữa các mô hình.",
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

	question := "OpenRouter là gì?"

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
