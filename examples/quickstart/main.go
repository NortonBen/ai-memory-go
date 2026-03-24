// Example: Minimal usage of ai-memory-brain with SQLite (zero infra required)
// Run: go run ./examples/quickstart/
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

	// ─── 1. Choose Embedder ────────────────────────────────────────────────────
	//
	// Option A: Ollama (local, free, no API key needed)
	//   $ ollama pull nomic-embed-text
	// ollamaEmb := vector.NewOllamaEmbeddingProvider("", "nomic-embed-text", 768)

	// Option B: OpenAI
	// openaiEmb := vector.NewOpenAIEmbeddingProvider(os.Getenv("OPENAI_API_KEY"), "")
	//
	// Option C: OpenRouter (pay-per-use gateway, supports 30+ providers)
	// openrouterEmb := vector.NewOpenRouterEmbeddingProvider(vector.OpenRouterConfig{
	//     APIKey:  os.Getenv("OPENROUTER_API_KEY"),
	//     Model:   "openai/text-embedding-3-small",
	//     SiteURL: "https://myapp.example.com",
	//     AppName: "ai-memory-brain demo",
	// })
	//
	// Option D: LM Studio (local, free, OpenAI-compatible API)
	lmstudioEmb := vector.NewLMStudioEmbeddingProvider("http://localhost:1234/v1", "text-embedding-nomic-embed-text-v1.5")

	// AutoEmbedder: primary = lmstudio
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)
	// embedder.AddProvider("ollama", ollamaEmb)
	// embedder.AddProvider("openai", openaiEmb)
	// embedder.AddFallback("openai")

	_ = os.MkdirAll("./data", 0o750)

	// ─── 2. Storage: SQLite graph + SQLite vector (2 separate files) ──────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	// SQLite relational store for DataPoints
	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/memory_rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	// ─── 3. LLM Extractor ─────────────────────────────────────────────────────
	//
	// Option A: Ollama (local, free)
	// provider := extractor.NewOllamaProvider("http://localhost:11434", "deepseek-r1:8b")
	// llmExt := extractor.NewBasicExtractor(provider, nil)
	//
	// Option B: OpenAI
	// openaiProvider := extractor.NewOpenAIProvider(os.Getenv("OPENAI_API_KEY"), "gpt-4o-mini")
	// llmExt := extractor.NewBasicExtractor(openaiProvider, nil)
	//
	// Option C: DeepSeek (JSON schema mode — best entity extraction quality)
	// dsProvider := extractor.NewDeepSeekProvider(os.Getenv("DEEPSEEK_API_KEY"), "")
	// llmExt := extractor.NewBasicExtractor(dsProvider, nil)
	//
	// Option D: LM Studio (local, free, OpenAI-compatible API)
	lmstudioProvider, _ := extractor.NewLMStudioProvider("http://localhost:1234/v1", "qwen/qwen3-4b-2507")
	llmExt := extractor.NewBasicExtractor(lmstudioProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 4})
	defer eng.Close()

	// ─── 5. ADD: Ingest knowledge ─────────────────────────────────────────────
	fmt.Println("=== ADD ===")
	texts := []string{
		"Present Perfect uses have/has + past participle. Example: I have studied English for 3 years.",
		"Common mistake: Using Present Perfect with specific past time. Wrong: I have seen him yesterday.",
		"User struggles with distinguishing Present Perfect from Simple Past.",
	}

	sessionID := "session-demo-001"
	dps := make([]*schema.DataPoint, 0, len(texts))
	for _, text := range texts {
		dp, err := eng.Add(ctx, text, engine.WithSessionID(sessionID))
		must(err, "AddMemory")
		dps = append(dps, dp)
		fmt.Printf("  Added: %s [%s]\n", dp.ID[:8], dp.ProcessingStatus)
	}

	// Give worker pool time to process
	time.Sleep(500 * time.Millisecond)

	// ─── 6. COGIFY: Process knowledge → graph + embeddings ────────────────────
	fmt.Println("\n=== COGNIFY ===")
	for _, dp := range dps {
		if _, err := eng.Cognify(ctx, dp); err != nil {
			log.Printf("  Warning: cognify %s: %v", dp.ID[:8], err)
		}
	}

	// Build graph nodes + edges manually to demonstrate graph API
	now := time.Now()
	conceptNode := &schema.Node{
		ID:   "concept-present-perfect",
		Type: schema.NodeTypeConcept,
		Properties: map[string]interface{}{
			"name":    "Present Perfect",
			"domain":  "grammar",
			"formula": "have/has + past participle",
		},
		Weight:    1.0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	userNode := &schema.Node{
		ID:   "user-001",
		Type: schema.NodeTypeUserPreference,
		Properties: map[string]interface{}{
			"name": "Language Learner",
		},
		Weight:    1.0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	must(graphStore.StoreBatch(ctx,
		[]*schema.Node{conceptNode, userNode},
		[]*schema.Edge{{
			ID:         "edge-001",
			From:       "user-001",
			To:         "concept-present-perfect",
			Type:       schema.EdgeTypeStrugglesWIth,
			Weight:     0.9,
			Properties: map[string]interface{}{"since": now.Format(time.RFC3339)},
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
	), "storeBatch")

	// Store embeddings in vector store
	for i, text := range texts {
		emb, err := embedder.GenerateEmbedding(ctx, text)
		if err != nil {
			log.Printf("  Warning: embed[%d]: %v", i, err)
			continue
		}
		vecStore.StoreEmbedding(ctx, dps[i].ID, emb, map[string]interface{}{
			"session_id": sessionID,
			"index":      i,
		})
	}
	fmt.Println("  Graph + vector populated.")

	// ─── 7. SEARCH: Semantic + graph hybrid ───────────────────────────────────
	fmt.Println("\n=== SEARCH ===")
	queryText := "How do I know when to use Present Perfect?"

	// 7a. Semantic search via vector store
	queryEmb, err := embedder.GenerateEmbedding(ctx, queryText)
	must(err, "query embed")

	results, err := vecStore.SimilaritySearch(ctx, queryEmb, 5, 0.4)
	must(err, "vector search")
	fmt.Printf("  Vector hits: %d\n", len(results))
	for _, r := range results {
		fmt.Printf("    [%.3f] id=%s\n", r.Score, r.ID)
	}

	// 7b. Graph traversal: find what "user-001" struggles with → 2 hops
	neighbors, err := graphStore.TraverseGraph(ctx, "user-001", 2, nil)
	must(err, "graph traverse")
	fmt.Printf("  Graph neighbors: %d\n", len(neighbors))
	for _, n := range neighbors {
		fmt.Printf("    [%s] %s — %v\n", n.Type, n.ID, n.Properties["name"])
	}

	// 7c. Relational search
	dpResults, err := eng.Search(ctx, &schema.SearchQuery{
		SessionID: sessionID,
		Text:      queryText,
		Limit:     5,
	})
	must(err, "relational search")
	fmt.Printf("  Relational hits: %d\n", len(dpResults.Results))
	if dpResults.Answer != "" {
		fmt.Printf("\n  🤖 MemoryEngine LLM Answer:\n  %s\n", dpResults.Answer)
	}

	fmt.Println("\n✅ Done.")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
