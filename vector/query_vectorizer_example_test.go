// Package vector - Query vectorizer usage examples
package vector

import (
	"context"
	"fmt"
	"log"
)

// Example_queryVectorizer demonstrates basic query vectorization
func Example_queryVectorizer() {
	// Create an AutoEmbedder with OpenAI provider
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("openai", cache)

	// Add OpenAI provider (requires API key in production)
	openaiProvider := NewOpenAIEmbeddingProvider("your-api-key", "text-embedding-3-small")
	embedder.AddProvider("openai", openaiProvider)

	// Create query vectorizer
	vectorizer := NewQueryVectorizer(embedder)

	// Vectorize a search query
	ctx := context.Background()
	queryText := "What is the present perfect tense?"

	vector, err := vectorizer.VectorizeQuery(ctx, queryText)
	if err != nil {
		log.Fatalf("Failed to vectorize query: %v", err)
	}

	fmt.Printf("Generated vector with %d dimensions\n", len(vector))
	fmt.Printf("First 5 values: %.4f, %.4f, %.4f, %.4f, %.4f\n",
		vector[0], vector[1], vector[2], vector[3], vector[4])
}

// Example_processedQuery demonstrates creating a ProcessedQuery
func Example_processedQuery() {
	// Setup
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("openai", cache)
	openaiProvider := NewOpenAIEmbeddingProvider("your-api-key", "text-embedding-3-small")
	embedder.AddProvider("openai", openaiProvider)

	vectorizer := NewQueryVectorizer(embedder)

	// Create a ProcessedQuery with vector
	ctx := context.Background()
	queryText := "How do I use past simple tense?"

	processedQuery, err := vectorizer.VectorizeProcessedQuery(ctx, queryText)
	if err != nil {
		log.Fatalf("Failed to create processed query: %v", err)
	}

	fmt.Printf("Original text: %s\n", processedQuery.OriginalText)
	fmt.Printf("Vector dimensions: %d\n", len(processedQuery.Vector))
	fmt.Printf("Ready for Step 2: Hybrid Search\n")
}

// Example_batchVectorization demonstrates batch query processing
func Example_batchVectorization() {
	// Setup
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("openai", cache)
	openaiProvider := NewOpenAIEmbeddingProvider("your-api-key", "text-embedding-3-small")
	embedder.AddProvider("openai", openaiProvider)

	vectorizer := NewQueryVectorizer(embedder)

	// Vectorize multiple queries at once
	ctx := context.Background()
	queries := []string{
		"What is present perfect?",
		"How to use past simple?",
		"Difference between present and past?",
	}

	embeddings, err := vectorizer.VectorizeBatchQueries(ctx, queries)
	if err != nil {
		log.Fatalf("Failed to vectorize batch: %v", err)
	}

	fmt.Printf("Vectorized %d queries\n", len(embeddings))
	for i, embedding := range embeddings {
		fmt.Printf("Query %d: %d dimensions\n", i+1, len(embedding))
	}
}

// Example_multiProvider demonstrates provider fallback
func Example_multiProvider() {
	// Create AutoEmbedder with multiple providers
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("openai", cache)

	// Add primary provider (OpenAI)
	openaiProvider := NewOpenAIEmbeddingProvider("your-openai-key", "text-embedding-3-small")
	embedder.AddProvider("openai", openaiProvider)

	// Add fallback provider (Ollama for local processing)
	// ollamaProvider := NewOllamaEmbeddingProvider("http://localhost:11434", "nomic-embed-text")
	// embedder.AddProvider("ollama", ollamaProvider)
	// embedder.AddFallback("ollama")

	vectorizer := NewQueryVectorizer(embedder)

	// If OpenAI fails, it will automatically try Ollama
	ctx := context.Background()
	vector, err := vectorizer.VectorizeQuery(ctx, "Test query")
	if err != nil {
		log.Fatalf("All providers failed: %v", err)
	}

	fmt.Printf("Successfully generated vector with %d dimensions\n", len(vector))
	fmt.Printf("Provider used: %s\n", vectorizer.GetModel())
}

// Example_searchPipelineIntegration demonstrates integration with search pipeline
func Example_searchPipelineIntegration() {
	// This example shows how query vectorization fits into the 4-step search pipeline

	// Setup
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("openai", cache)
	openaiProvider := NewOpenAIEmbeddingProvider("your-api-key", "text-embedding-3-small")
	embedder.AddProvider("openai", openaiProvider)

	vectorizer := NewQueryVectorizer(embedder)
	ctx := context.Background()

	// Step 1: Input Processing & Vectorization (THIS TASK)
	queryText := "Cách dùng thì Hiện tại hoàn thành mà tôi đã học là gì?"
	processedQuery, err := vectorizer.VectorizeProcessedQuery(ctx, queryText)
	if err != nil {
		log.Fatalf("Step 1 failed: %v", err)
	}

	fmt.Println("=== Step 1: Input Processing & Vectorization ===")
	fmt.Printf("Query: %s\n", processedQuery.OriginalText)
	fmt.Printf("Vector: [%.4f, %.4f, ...] (%d dimensions)\n",
		processedQuery.Vector[0], processedQuery.Vector[1], len(processedQuery.Vector))

	// Step 2: Hybrid Search (Next task - uses the vector)
	fmt.Println("\n=== Step 2: Hybrid Search ===")
	fmt.Println("- Vector Search: Use processedQuery.Vector for similarity search")
	fmt.Println("- Entity Search: Extract entities from processedQuery.OriginalText")
	fmt.Println("- Result: Anchor nodes for graph traversal")

	// Step 3: Graph Traversal (Future task)
	fmt.Println("\n=== Step 3: Graph Traversal ===")
	fmt.Println("- 1-hop: Direct neighbors of anchor nodes")
	fmt.Println("- 2-hop: Indirect neighbors for extended context")

	// Step 4: Context Assembly (Future task)
	fmt.Println("\n=== Step 4: Context Assembly ===")
	fmt.Println("- Rerank: Multi-factor scoring")
	fmt.Println("- Build: Rich context for LLM")
}

// Example_caching demonstrates caching behavior
func Example_caching() {
	// Create embedder with caching enabled
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("openai", cache)
	openaiProvider := NewOpenAIEmbeddingProvider("your-api-key", "text-embedding-3-small")
	embedder.AddProvider("openai", openaiProvider)

	vectorizer := NewQueryVectorizer(embedder)
	ctx := context.Background()

	queryText := "What is present perfect?"

	// First call - generates embedding and caches it
	fmt.Println("First call (generates embedding)...")
	vector1, _ := vectorizer.VectorizeQuery(ctx, queryText)

	// Second call - uses cached embedding (faster)
	fmt.Println("Second call (uses cache)...")
	vector2, _ := vectorizer.VectorizeQuery(ctx, queryText)

	// Verify they're identical
	identical := true
	for i := range vector1 {
		if vector1[i] != vector2[i] {
			identical = false
			break
		}
	}

	fmt.Printf("Vectors identical: %v\n", identical)
	fmt.Printf("Cache size: %d embeddings\n", cache.GetSize())
}
