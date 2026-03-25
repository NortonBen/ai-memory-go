// Example: Demonstrates the engine.Request multi-intent router with persistent chat history.
// Run: go run ./examples/chat_history_agent/
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	ctx := context.Background()
	_ = os.MkdirAll("./data/chat_history", 0o750)

	// 1. Embedder (ensure LM Studio is running locally or swap this out)
	lmstudioEmb := vector.NewLMStudioEmbeddingProvider("http://localhost:1234/v1", "text-embedding-nomic-embed-text-v1.5")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)

	// 2. Storage
	graphStore, err := graph.NewSQLiteGraphStore("./data/chat_history/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/chat_history/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/chat_history/memory_rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	// 3. LLM Extractor
	// You can swap to OpenAIProvider if you have an API key!
	lmstudioProvider, _ := extractor.NewLMStudioProvider("http://localhost:1234/v1", "qwen/qwen3-4b-2507")
	llmExt := extractor.NewBasicExtractor(lmstudioProvider, nil)

	// 4. Memory Engine
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 4})
	defer eng.Close()

	// A fixed session ID ensures continuity when you stop and restart the program.
	sessionID := "agent-session-001"
	fmt.Printf("=== AI Memory Brain: Chat History Agent ===\n")
	fmt.Printf("Session ID: %s (History will persist across restarts)\n", sessionID)
	fmt.Println("Type your message, or '/exit' to quit.")
	fmt.Println("-------------------------------------------------")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nUser > ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/exit" || input == "/quit" {
			fmt.Println("Goodbye!")
			break
		}

		result, err := eng.Request(ctx, sessionID, input,
			engine.WithEnableThinking(true),
			engine.WithIncludeReasoning(true),
			engine.WithLearnRelationships(true),
		)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Show the intent detected by the LLM (for demonstration)
		if result.Intent != nil {
			fmt.Printf("\n[Intent] Query: %v | Delete: %v", result.Intent.IsQuery, result.Intent.IsDelete)
			if len(result.Intent.DeleteTargets) > 0 {
				fmt.Printf(" | Targets: %v", result.Intent.DeleteTargets)
			}
			fmt.Println()
		}

		if result.Answer != "" {
			fmt.Printf("Agent > %s\n", result.Answer)
		} else {
			fmt.Printf("Agent > %s\n", "No answer generated.")
		}
	}
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
