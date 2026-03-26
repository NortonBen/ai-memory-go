// Example: Simulating an Agent Memory Loop with LM Studio and SQLite
// Run: go run ./examples/agent_memory_loop/
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
	_ = os.MkdirAll("./data/agent_loop", 0o750)

	// ─── 1. LM Studio Embedder ────────────────────────────────────────────────
	embFactory := registry.NewEmbeddingProviderFactory()
	lmstudioEmb, err := embFactory.CreateProvider(&extractor.EmbeddingProviderConfig{
		Type:     extractor.EmbeddingProviderLMStudio,
		Endpoint: "http://localhost:1234/v1",
		Model:    "text-embedding-nomic-embed-text-v1.5",
	})
	must(err, "lmstudio embedding provider")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)

	// ─── 2. SQLite Stores (Graph, Vector, Relational) ─────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/agent_loop/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/agent_loop/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/agent_loop/memory_rel.db",
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
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 2})
	defer eng.Close()

	// ─── 5. Simulated Conversation Loop ───────────────────────────────────────
	sessionID := "user-benji-001"
	
	fmt.Println("=== STARTING AGENT MEMORY LOOP ===")
	fmt.Println("The agent will remember facts across the conversation.")
	
	conversation := []string{
		"Hi, my name is Benji. I work on an open source project called ai-memory-go.",
		"I'm currently trying to integrate LM Studio into my project.",
		"What project am I working on?",
		"What am I trying to integrate into it?",
	}

	for i, turn := range conversation {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("User: %s\n", turn)
		
		// Agent checks memory first if it's a question
		// A real agent would decide with an LLM whether to search, but we just search on questions.
		if turn[len(turn)-1] == '?' {
			// DEBUG check vector store
			count, cerr := vecStore.GetEmbeddingCount(ctx)
			fmt.Printf(" [DEBUG] VectorStore embedding count: %d (err: %v)\n", count, cerr)

			searchRes, err := eng.Search(ctx, &schema.SearchQuery{
				Text: turn,
				SessionID:           sessionID,
				Limit:               2,
				SimilarityThreshold: 0.1,
			})
			must(err, "search memory")
			
			// Build prompt with context
			var promptContext string
			if len(searchRes.Results) > 0 {
				promptContext += "Relevant Memory Context:\n"
				for _, res := range searchRes.Results {
					fmt.Printf(" [DEBUG] Hit: %.3f - %s\n", res.Score, res.DataPoint.Content)
					promptContext += fmt.Sprintf("- %s\n", res.DataPoint.Content)
				}
			} else {
				fmt.Println(" [DEBUG] No Memory Context Hits!")
			}
			
			prompt := fmt.Sprintf("%s\nUser query: %s\nProvide a concise answer based ONLY on the context.", promptContext, turn)
			
			fmt.Println("Agent Thinking...")
			answer, err := lmstudioProvider.GenerateCompletionWithOptions(ctx, prompt, nil)
			if err != nil {
				log.Printf("Agent failed to respond: %v", err)
			} else {
				fmt.Printf("Agent: %s\n", answer)
			}
			
		} else {
			// If it's a statement, acknowledge and memorize
			fmt.Println("Agent: Got it. I'll remember that.")
			
			// Add to memory
			dp, err := eng.Add(ctx, turn, engine.WithSessionID(sessionID))
			must(err, "Add")
			
			// Cognify (Extract entities) synchronously for this example to ensure data is ready
			fmt.Println("Agent saving to memory bank (Cognify)...")
			if _, err := eng.Cognify(context.Background(), dp); err != nil {
				log.Printf("Cognify error: %v", err)
			}
			fmt.Println("Agent saved memory.")
			
			// Give background workers time to complete embedding and graph extraction via LM Studio!
			// A real production app would either use webhooks/websockets to notify completion or search synchronously on demand.
			time.Sleep(5 * time.Second)
		}
	}

	fmt.Println("\n✅ Agent Memory Loop Complete. Check data/agent_loop/ for the SQLite files.")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
