// Package vector - Query vectorization for search pipeline
package vector

import (
	"context"
	"fmt"

	"github.com/NortonBen/ai-memory-go/schema"
)

// QueryVectorizer handles query vectorization for the search pipeline
// This is Step 1 of the 4-step search pipeline (Input Processing)
type QueryVectorizer struct {
	embedder *AutoEmbedder
}

// NewQueryVectorizer creates a new query vectorizer
func NewQueryVectorizer(embedder *AutoEmbedder) *QueryVectorizer {
	return &QueryVectorizer{
		embedder: embedder,
	}
}

// VectorizeQuery converts a search query into a vector embedding
// This is the core functionality for Step 1: Input Processing & Vectorization
func (qv *QueryVectorizer) VectorizeQuery(ctx context.Context, queryText string) ([]float32, error) {
	if queryText == "" {
		return nil, fmt.Errorf("query text cannot be empty")
	}

	// Generate embedding using AutoEmbedder (supports multiple providers)
	embedding, err := qv.embedder.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("generated embedding is empty")
	}

	return embedding, nil
}

// VectorizeProcessedQuery creates a ProcessedQuery with vector embedding
// This combines query vectorization with the ProcessedQuery structure
func (qv *QueryVectorizer) VectorizeProcessedQuery(ctx context.Context, queryText string) (*schema.ProcessedQuery, error) {
	if queryText == "" {
		return nil, fmt.Errorf("query text cannot be empty")
	}

	// Generate query vector
	vector, err := qv.VectorizeQuery(ctx, queryText)
	if err != nil {
		return nil, err
	}

	// Create ProcessedQuery with vector
	processedQuery := &schema.ProcessedQuery{
		OriginalText: queryText,
		Vector:       vector,
		Entities:     make([]*schema.Node, 0),
		Keywords:     make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}

	return processedQuery, nil
}

// VectorizeBatchQueries vectorizes multiple queries in batch
// This is more efficient for processing multiple queries at once
func (qv *QueryVectorizer) VectorizeBatchQueries(ctx context.Context, queries []string) ([][]float32, error) {
	if len(queries) == 0 {
		return [][]float32{}, nil
	}

	// Validate all queries
	for i, query := range queries {
		if query == "" {
			return nil, fmt.Errorf("query at index %d is empty", i)
		}
	}

	// Generate batch embeddings using AutoEmbedder
	embeddings, err := qv.embedder.GenerateBatchEmbeddings(ctx, queries)
	if err != nil {
		return nil, fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	if len(embeddings) != len(queries) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(queries), len(embeddings))
	}

	return embeddings, nil
}

// GetDimensions returns the embedding dimensions from the underlying embedder
func (qv *QueryVectorizer) GetDimensions() int {
	return qv.embedder.GetDimensions()
}

// GetModel returns the model name from the underlying embedder
func (qv *QueryVectorizer) GetModel() string {
	return qv.embedder.GetModel()
}

// Health checks if the query vectorizer is operational
func (qv *QueryVectorizer) Health(ctx context.Context) error {
	if qv.embedder == nil {
		return fmt.Errorf("embedder is not initialized")
	}

	return qv.embedder.Health(ctx)
}
