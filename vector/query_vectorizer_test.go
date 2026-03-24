// Package vector - Query vectorizer tests
package vector

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingProvider implements EmbeddingProvider for testing
type MockEmbeddingProvider struct {
	embeddings      map[string][]float32
	batchEmbeddings [][]float32
	dimensions      int
	model           string
	shouldFail      bool
	failureError    error
}

func NewMockEmbeddingProvider(dimensions int, model string) *MockEmbeddingProvider {
	return &MockEmbeddingProvider{
		embeddings:      make(map[string][]float32),
		batchEmbeddings: make([][]float32, 0),
		dimensions:      dimensions,
		model:           model,
		shouldFail:      false,
	}
}

func (m *MockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if m.shouldFail {
		if m.failureError != nil {
			return nil, m.failureError
		}
		return nil, errors.New("mock embedding generation failed")
	}

	// Return pre-configured embedding if exists
	if emb, exists := m.embeddings[text]; exists {
		return emb, nil
	}

	// Generate a simple mock embedding based on text length
	embedding := make([]float32, m.dimensions)
	for i := 0; i < m.dimensions; i++ {
		embedding[i] = float32(len(text)) / float32(m.dimensions+i)
	}
	return embedding, nil
}

func (m *MockEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if m.shouldFail {
		if m.failureError != nil {
			return nil, m.failureError
		}
		return nil, errors.New("mock batch embedding generation failed")
	}

	// Return pre-configured batch embeddings if exists
	if len(m.batchEmbeddings) > 0 {
		return m.batchEmbeddings, nil
	}

	// Generate embeddings for each text
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

func (m *MockEmbeddingProvider) GetDimensions() int {
	return m.dimensions
}

func (m *MockEmbeddingProvider) GetModel() string {
	return m.model
}

func (m *MockEmbeddingProvider) Health(ctx context.Context) error {
	if m.shouldFail {
		return errors.New("mock provider unhealthy")
	}
	return nil
}

func (m *MockEmbeddingProvider) SetEmbedding(text string, embedding []float32) {
	m.embeddings[text] = embedding
}

func (m *MockEmbeddingProvider) SetBatchEmbeddings(embeddings [][]float32) {
	m.batchEmbeddings = embeddings
}

func (m *MockEmbeddingProvider) SetShouldFail(shouldFail bool) {
	m.shouldFail = shouldFail
}

func (m *MockEmbeddingProvider) SetFailureError(err error) {
	m.failureError = err
}

// Helper function to create a test AutoEmbedder with mock provider
func createTestAutoEmbedder(dimensions int, model string) *AutoEmbedder {
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("mock", cache)

	mockProvider := NewMockEmbeddingProvider(dimensions, model)
	embedder.AddProvider("mock", mockProvider)

	return embedder
}

// Helper function to create a test AutoEmbedder with failing provider
func createFailingAutoEmbedder() *AutoEmbedder {
	cache := NewInMemoryEmbeddingCache()
	embedder := NewAutoEmbedder("mock", cache)

	mockProvider := NewMockEmbeddingProvider(768, "mock-model")
	mockProvider.SetShouldFail(true)
	embedder.AddProvider("mock", mockProvider)

	return embedder
}

func TestNewQueryVectorizer(t *testing.T) {
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	assert.NotNil(t, vectorizer)
	assert.NotNil(t, vectorizer.embedder)
}

func TestVectorizeQuery_Success(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	queryText := "What is the present perfect tense?"
	vector, err := vectorizer.VectorizeQuery(ctx, queryText)

	require.NoError(t, err)
	assert.NotNil(t, vector)
	assert.Equal(t, 768, len(vector))

	// Verify vector contains non-zero values
	hasNonZero := false
	for _, val := range vector {
		if val != 0 {
			hasNonZero = true
			break
		}
	}
	assert.True(t, hasNonZero, "Vector should contain non-zero values")
}

func TestVectorizeQuery_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	vector, err := vectorizer.VectorizeQuery(ctx, "")

	assert.Error(t, err)
	assert.Nil(t, vector)
	assert.Contains(t, err.Error(), "query text cannot be empty")
}

func TestVectorizeQuery_EmbeddingFailure(t *testing.T) {
	ctx := context.Background()
	embedder := createFailingAutoEmbedder()
	vectorizer := NewQueryVectorizer(embedder)

	queryText := "Test query"
	vector, err := vectorizer.VectorizeQuery(ctx, queryText)

	assert.Error(t, err)
	assert.Nil(t, vector)
	assert.Contains(t, err.Error(), "failed to generate query embedding")
}

func TestVectorizeProcessedQuery_Success(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	queryText := "How do I use present perfect?"
	processedQuery, err := vectorizer.VectorizeProcessedQuery(ctx, queryText)

	require.NoError(t, err)
	assert.NotNil(t, processedQuery)
	assert.Equal(t, queryText, processedQuery.OriginalText)
	assert.Equal(t, 768, len(processedQuery.Vector))
	assert.NotNil(t, processedQuery.Entities)
	assert.NotNil(t, processedQuery.Keywords)
	assert.NotNil(t, processedQuery.Metadata)
}

func TestVectorizeProcessedQuery_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	processedQuery, err := vectorizer.VectorizeProcessedQuery(ctx, "")

	assert.Error(t, err)
	assert.Nil(t, processedQuery)
	assert.Contains(t, err.Error(), "query text cannot be empty")
}

func TestVectorizeBatchQueries_Success(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	queries := []string{
		"What is present perfect?",
		"How to use past simple?",
		"Difference between present and past?",
	}

	embeddings, err := vectorizer.VectorizeBatchQueries(ctx, queries)

	require.NoError(t, err)
	assert.Equal(t, len(queries), len(embeddings))

	for i, embedding := range embeddings {
		assert.Equal(t, 768, len(embedding), "Embedding %d should have correct dimensions", i)
	}
}

func TestVectorizeBatchQueries_EmptyList(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	embeddings, err := vectorizer.VectorizeBatchQueries(ctx, []string{})

	require.NoError(t, err)
	assert.Empty(t, embeddings)
}

func TestVectorizeBatchQueries_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	queries := []string{
		"Valid query",
		"",
		"Another valid query",
	}

	embeddings, err := vectorizer.VectorizeBatchQueries(ctx, queries)

	assert.Error(t, err)
	assert.Nil(t, embeddings)
	assert.Contains(t, err.Error(), "query at index 1 is empty")
}

func TestVectorizeBatchQueries_EmbeddingFailure(t *testing.T) {
	ctx := context.Background()
	embedder := createFailingAutoEmbedder()
	vectorizer := NewQueryVectorizer(embedder)

	queries := []string{
		"Query 1",
		"Query 2",
	}

	embeddings, err := vectorizer.VectorizeBatchQueries(ctx, queries)

	assert.Error(t, err)
	assert.Nil(t, embeddings)
	assert.Contains(t, err.Error(), "failed to generate batch embeddings")
}

func TestGetDimensions(t *testing.T) {
	embedder := createTestAutoEmbedder(1536, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	dimensions := vectorizer.GetDimensions()
	assert.Equal(t, 1536, dimensions)
}

func TestGetModel(t *testing.T) {
	embedder := createTestAutoEmbedder(768, "gpt-3.5-turbo")
	vectorizer := NewQueryVectorizer(embedder)

	model := vectorizer.GetModel()
	assert.Equal(t, "gpt-3.5-turbo", model)
}

func TestHealth_Success(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	err := vectorizer.Health(ctx)
	assert.NoError(t, err)
}

func TestHealth_EmbedderNil(t *testing.T) {
	ctx := context.Background()
	vectorizer := &QueryVectorizer{
		embedder: nil,
	}

	err := vectorizer.Health(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedder is not initialized")
}

func TestHealth_ProviderUnhealthy(t *testing.T) {
	ctx := context.Background()
	embedder := createFailingAutoEmbedder()
	vectorizer := NewQueryVectorizer(embedder)

	err := vectorizer.Health(ctx)
	assert.Error(t, err)
}

// Test with different embedding dimensions
func TestVectorizeQuery_DifferentDimensions(t *testing.T) {
	testCases := []struct {
		name       string
		dimensions int
	}{
		{"OpenAI ada-002", 1536},
		{"OpenAI text-embedding-3-small", 1536},
		{"OpenAI text-embedding-3-large", 3072},
		{"Custom 768", 768},
		{"Custom 512", 512},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			embedder := createTestAutoEmbedder(tc.dimensions, "test-model")
			vectorizer := NewQueryVectorizer(embedder)

			vector, err := vectorizer.VectorizeQuery(ctx, "Test query")

			require.NoError(t, err)
			assert.Equal(t, tc.dimensions, len(vector))
		})
	}
}

// Test caching behavior
func TestVectorizeQuery_Caching(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	queryText := "Cached query test"

	// First call - should generate embedding
	vector1, err1 := vectorizer.VectorizeQuery(ctx, queryText)
	require.NoError(t, err1)

	// Second call - should use cache
	vector2, err2 := vectorizer.VectorizeQuery(ctx, queryText)
	require.NoError(t, err2)

	// Vectors should be identical
	assert.Equal(t, len(vector1), len(vector2))
	for i := range vector1 {
		assert.Equal(t, vector1[i], vector2[i])
	}
}

// Test context cancellation
func TestVectorizeQuery_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	// This should handle context cancellation gracefully
	_, err := vectorizer.VectorizeQuery(ctx, "Test query")

	// The error might be from context cancellation or embedding generation
	// We just verify that it doesn't panic and may return an error
	if err != nil {
		// Expected - context was cancelled or embedding failed
		t.Logf("Got expected error: %v", err)
	}
}

// Test with special characters and unicode
func TestVectorizeQuery_SpecialCharacters(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	testCases := []string{
		"Query with émojis 😀🎉",
		"Vietnamese: Cách dùng thì Hiện tại hoàn thành",
		"Chinese: 如何使用现在完成时",
		"Special chars: @#$%^&*()",
		"Mixed: Hello 世界 🌍",
	}

	for _, query := range testCases {
		t.Run(query, func(t *testing.T) {
			vector, err := vectorizer.VectorizeQuery(ctx, query)
			require.NoError(t, err)
			assert.Equal(t, 768, len(vector))
		})
	}
}

// Test batch processing with large number of queries
func TestVectorizeBatchQueries_LargeBatch(t *testing.T) {
	ctx := context.Background()
	embedder := createTestAutoEmbedder(768, "test-model")
	vectorizer := NewQueryVectorizer(embedder)

	// Generate 100 queries
	queries := make([]string, 100)
	for i := 0; i < 100; i++ {
		queries[i] = fmt.Sprintf("Query number %d", i)
	}

	embeddings, err := vectorizer.VectorizeBatchQueries(ctx, queries)

	require.NoError(t, err)
	assert.Equal(t, 100, len(embeddings))

	for i, embedding := range embeddings {
		assert.Equal(t, 768, len(embedding), "Embedding %d should have correct dimensions", i)
	}
}
