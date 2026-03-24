package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NortonBen/ai-memory-go/extractor"
)

func main() {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		fmt.Println("No API KEY")
		return
	}
	
	prov, err := extractor.NewGeminiEmbeddingProvider(key, "text-embedding-004")
	if err != nil {
		fmt.Println("Error initializing:", err)
		return
	}
	
	emb, err := prov.GenerateEmbedding(context.Background(), "Hello world")
	if err != nil {
		fmt.Println("Error generating:", err)
		return
	}
	
	fmt.Printf("Dimensions returned: %d\n", len(emb))
}
