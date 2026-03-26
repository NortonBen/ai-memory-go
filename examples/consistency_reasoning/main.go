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
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	fmt.Println("=== INITIALIZING CONSISTENCY REASONING ===")

	lmStudioURL := os.Getenv("LMSTUDIO_URL")
	if lmStudioURL == "" {
		lmStudioURL = "http://localhost:1234/v1"
	}
	modelName := os.Getenv("LMSTUDIO_MODEL")
	if modelName == "" {
		modelName = "qwen/qwen3-4b-2507"
	}

	embeddingsModel := os.Getenv("LMSTUDIO_EMBEDDING_MODEL")
	if embeddingsModel == "" {
		embeddingsModel = "text-embedding-nomic-embed-text-v1.5"
	}

	llmFactory := registry.NewProviderFactory()
	provider, err := llmFactory.CreateProvider(&extractor.ProviderConfig{
		Type:     extractor.ProviderLMStudio,
		Endpoint: lmStudioURL,
		Model:    modelName,
	})
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}
	ext := extractor.NewBasicExtractor(provider, nil)

	embFactory := registry.NewEmbeddingProviderFactory()
	emb, err := embFactory.CreateProvider(&extractor.EmbeddingProviderConfig{
		Type:     extractor.EmbeddingProviderLMStudio,
		Endpoint: lmStudioURL,
		Model:    embeddingsModel,
	})
	if err != nil {
		log.Fatalf("Failed to create embedding provider: %v", err)
	}

	graphDBPath := "./data/consistency_reasoning/consistency_graph.db"
	vectorDBPath := "./data/consistency_reasoning/consistency_vector.db"
	relDBPath := "./data/consistency_reasoning/consistency_rel.db"

	os.Remove(graphDBPath)
	os.Remove(vectorDBPath)
	os.Remove(relDBPath)

	graphDB, err := graph.NewSQLiteGraphStore(graphDBPath)
	if err != nil {
		log.Fatalf("Graph DB Error: %v", err)
	}

	vecDB, err := vector.NewSQLiteVectorStore(vectorDBPath, 768)
	if err != nil {
		log.Fatalf("Vector DB Error: %v", err)
	}
	defer vecDB.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    relDBPath,
		ConnTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Relational Store Error: %v", err)
	}
	defer relStore.Close()

	// NOTE: We pass ConsistencyThreshold = 0.85
	// (Cosine distance might be converted to score. 0.85 means very similar).
	memEngine := engine.NewMemoryEngineWithStores(ext, emb, relStore, graphDB, vecDB, engine.EngineConfig{})

	ctx := context.Background()

	// 1. Initial State
	fmt.Println("\n--- Adding Initial Fact ---")
	_, err = memEngine.Add(ctx,
		"TechViet's new headquarters is located in Austin, Texas.",
		engine.WithConsistencyThreshold(0.5),
	)
	if err != nil {
		log.Fatalf("Add failed: %v", err)
	}

	fmt.Println("Waiting 10 seconds for initial fact processing...")
	time.Sleep(10 * time.Second)

	// 2. Similar entity, updating property (UPDATE/CONTRADICTS)
	// Extractor might extract "TechViet Head Office" or "TechViet Headquarters"
	fmt.Println("\n--- Adding Similar Fact (Update/Contradiction) ---")
	_, err = memEngine.Add(ctx,
		"TechViet Headquarters moved its main operations to Silicon Valley yesterday.",
		engine.WithConsistencyThreshold(0.5),
	)
	if err != nil {
		log.Fatalf("Add failed: %v", err)
	}

	fmt.Println("Waiting 15 seconds for consistency reasoning processing...")
	time.Sleep(15 * time.Second)

	// query graph to show nodes and edges
	fmt.Println("\n--- FINAL GRAPH STATE ---")
	nodes, _ := graphDB.FindNodesByType(ctx, "Organization") // Or maybe Entity
	if len(nodes) == 0 {
		nodes, _ = graphDB.FindNodesByType(ctx, "Entity")
		nodesOrg, _ := graphDB.FindNodesByType(ctx, "Organization")
		nodes = append(nodes, nodesOrg...)
		nodesComp, _ := graphDB.FindNodesByType(ctx, "Company")
		nodes = append(nodes, nodesComp...)
		nodesFacility, _ := graphDB.FindNodesByType(ctx, "Facility")
		nodes = append(nodes, nodesFacility...)
	}

	for _, n := range nodes {
		fmt.Printf("Node: %s (%s) | Properties: %+v\n", n.ID, n.Type, n.Properties)

		connected, _ := graphDB.FindConnected(ctx, n.ID, nil)
		for _, c := range connected {
			fmt.Printf("  -> Connected: %s (%s)\n", c.ID, c.Type)
		}
	}

	// Let's specifically look for CONTRADICTS edges
	fmt.Println("\n--- CHECKING CONTRADICTS EDGES ---")
	// Note: GraphStore interface might not have FindEdgesByType directly,
	// but FindConnected finds adjacent nodes. We can iterate over nodes.

	fmt.Println("Test Complete")
}
