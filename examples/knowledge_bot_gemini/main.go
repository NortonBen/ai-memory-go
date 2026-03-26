// Example: Building and traversing a Knowledge Graph with ai-memory-brain and Gemini
// Run: go run ./examples/knowledge_bot_gemini/
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
	_ "github.com/NortonBen/ai-memory-go/storage/adapters/sqlite"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	ctx := context.Background()
	_ = os.MkdirAll("./data/bot_gemini", 0o750)

	geminiApiKey := os.Getenv("GEMINI_API_KEY")
	if geminiApiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	fmt.Println("GEMINI_API_KEY is set. Initializing providers...")

	// ─── 1. Gemini Embedder ───────────────────────────────────────────────
	// Using Google Gemini for embeddings (text-embedding-004)
	embFactory := registry.NewEmbeddingProviderFactory()
	geminiEmb, err := embFactory.CreateProvider(&extractor.EmbeddingProviderConfig{
		Type:   extractor.EmbeddingProviderGemini,
		APIKey: geminiApiKey,
		Model:  "text-embedding-004",
	})
	must(err, "gemini embedding provider")

	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("gemini", cache)
	embedder.AddProvider("gemini", geminiEmb)

	// ─── 2. SQLite Stores (Graph, Vector, Relational) ─────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/bot_gemini/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	// Dimensions matches text-embedding-004 (768 dimensions default)
	vecStore, err := vector.NewSQLiteVectorStore("./data/bot_gemini/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/bot_gemini/memory_rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	// ─── 3. Gemini Extractor ──────────────────────────────────────────────
	// Using Google Gemini for extraction (gemini-2-flash)
	llmFactory := registry.NewProviderFactory()
	geminiProvider, err := llmFactory.CreateProvider(&extractor.ProviderConfig{
		Type:   extractor.ProviderGemini,
		APIKey: geminiApiKey,
		Model:  "gemini-2-flash",
	})
	must(err, "gemini provider")
	llmExt := extractor.NewBasicExtractor(geminiProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	// We'll use 2 workers for extraction
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 2})
	defer eng.Close()

	sessionID := "kg-session-gemini"

	// ─── 5. Knowledge Corpus ──────────────────────────────────────────────────
	fmt.Println("=== INGESTING KNOWLEDGE CORPUS ===")
	corpus := []string{
		"Google Gemini là một mô hình ngôn ngữ lớn (LLM) đa phương thức tiên tiến được phát triển bởi Google DeepMind.",
		"Mô hình Gemini-2.5-Flash là một phiên bản mô hình nhỏ gọn, tốc độ cao được tối ưu hóa cho nhiều tác vụ xử lý ngôn ngữ và suy luận.",
		"Google DeepMind cũng cung cấp các API thông qua Google AI Studio và Vertex AI cho phép nhà phát triển tích hợp Gemini vào ứng dụng của mình.",
	}

	for _, text := range corpus {
		dp, err := eng.Add(ctx, text, engine.WithSessionID(sessionID), engine.WithWaitAdd(true), engine.WithConsistencyThreshold(0.5))
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

	question := "Gemini là gì và do ai tạo ra?"

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
