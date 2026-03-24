package vector

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// InMemoryStore is a thread-safe in-memory VectorStore implementation for testing.
type InMemoryStore struct {
	mu         sync.RWMutex
	embeddings map[string]*EmbeddingData
	config     *VectorConfig
	collection string
}

// NewInMemoryStore creates a new in-memory vector store.
func NewInMemoryStore(config *VectorConfig) *InMemoryStore {
	if config == nil {
		config = DefaultVectorConfig()
	}
	return &InMemoryStore{
		embeddings: make(map[string]*EmbeddingData),
		config:     config,
		collection: config.Collection,
	}
}

// StoreEmbedding implements VectorStore.
func (s *InMemoryStore) StoreEmbedding(_ context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if existing, ok := s.embeddings[id]; ok {
		existing.Embedding = embedding
		existing.Metadata = metadata
		existing.UpdatedAt = now
	} else {
		s.embeddings[id] = &EmbeddingData{
			ID:        id,
			Embedding: embedding,
			Metadata:  metadata,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return nil
}

// GetEmbedding implements VectorStore.
func (s *InMemoryStore) GetEmbedding(_ context.Context, id string) ([]float32, map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	emb, ok := s.embeddings[id]
	if !ok {
		return nil, nil, fmt.Errorf("embedding not found: %s", id)
	}
	return emb.Embedding, emb.Metadata, nil
}

// UpdateEmbedding implements VectorStore.
func (s *InMemoryStore) UpdateEmbedding(_ context.Context, id string, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	emb, ok := s.embeddings[id]
	if !ok {
		return fmt.Errorf("embedding not found: %s", id)
	}
	emb.Embedding = embedding
	emb.UpdatedAt = time.Now()
	return nil
}

// DeleteEmbedding implements VectorStore.
func (s *InMemoryStore) DeleteEmbedding(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.embeddings, id)
	return nil
}

// StoreBatchEmbeddings implements VectorStore.
func (s *InMemoryStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*EmbeddingData) error {
	for _, emb := range embeddings {
		if err := s.StoreEmbedding(ctx, emb.ID, emb.Embedding, emb.Metadata); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatchEmbeddings implements VectorStore.
func (s *InMemoryStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if err := s.DeleteEmbedding(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// SimilaritySearch implements VectorStore using cosine similarity.
func (s *InMemoryStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*SimilarityResult, error) {
	return s.SimilaritySearchWithFilter(ctx, queryEmbedding, nil, limit, threshold)
}

// SimilaritySearchWithFilter implements VectorStore using cosine similarity with optional filters.
func (s *InMemoryStore) SimilaritySearchWithFilter(_ context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*SimilarityResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		emb   *EmbeddingData
		score float64
	}

	var candidates []scored
	for _, emb := range s.embeddings {
		if !matchesFilters(emb.Metadata, filters) {
			continue
		}
		score := cosineSimilarity(queryEmbedding, emb.Embedding)
		if score >= threshold {
			candidates = append(candidates, scored{emb: emb, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]*SimilarityResult, len(candidates))
	for i, c := range candidates {
		results[i] = &SimilarityResult{
			ID:        c.emb.ID,
			Score:     c.score,
			Embedding: c.emb.Embedding,
			Metadata:  c.emb.Metadata,
			Distance:  1.0 - c.score,
		}
	}
	return results, nil
}

// CreateCollection implements VectorStore (no-op for in-memory).
func (s *InMemoryStore) CreateCollection(_ context.Context, name string, _ int, _ *CollectionConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.collection = name
	return nil
}

// DeleteCollection implements VectorStore (clears all embeddings).
func (s *InMemoryStore) DeleteCollection(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embeddings = make(map[string]*EmbeddingData)
	return nil
}

// ListCollections implements VectorStore.
func (s *InMemoryStore) ListCollections(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.collection != "" {
		return []string{s.collection}, nil
	}
	return []string{}, nil
}

// GetCollectionInfo implements VectorStore.
func (s *InMemoryStore) GetCollectionInfo(_ context.Context, name string) (*CollectionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &CollectionInfo{
		Name:        name,
		Dimension:   s.config.Dimension,
		VectorCount: int64(len(s.embeddings)),
		Status:      "ready",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// GetEmbeddingCount implements VectorStore.
func (s *InMemoryStore) GetEmbeddingCount(_ context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.embeddings)), nil
}

// Health implements VectorStore.
func (s *InMemoryStore) Health(_ context.Context) error {
	return nil
}

// Close implements VectorStore.
func (s *InMemoryStore) Close() error {
	return nil
}

// cosineSimilarity computes the cosine similarity between two float32 slices.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// matchesFilters returns true if all filter key-value pairs are present in metadata.
func matchesFilters(metadata map[string]interface{}, filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return true
	}
	for k, v := range filters {
		mv, ok := metadata[k]
		if !ok || fmt.Sprintf("%v", mv) != fmt.Sprintf("%v", v) {
			return false
		}
	}
	return true
}
