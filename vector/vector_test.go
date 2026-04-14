package vector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testStore struct{}

func (t *testStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	return nil
}
func (t *testStore) GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error) {
	return nil, nil, nil
}
func (t *testStore) UpdateEmbedding(ctx context.Context, id string, embedding []float32) error { return nil }
func (t *testStore) DeleteEmbedding(ctx context.Context, id string) error                        { return nil }
func (t *testStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*SimilarityResult, error) {
	return nil, nil
}
func (t *testStore) SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*SimilarityResult, error) {
	return nil, nil
}
func (t *testStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*EmbeddingData) error { return nil }
func (t *testStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error               { return nil }
func (t *testStore) CreateCollection(ctx context.Context, name string, dimension int, config *CollectionConfig) error {
	return nil
}
func (t *testStore) DeleteCollection(ctx context.Context, name string) error { return nil }
func (t *testStore) ListCollections(ctx context.Context) ([]string, error)   { return nil, nil }
func (t *testStore) GetCollectionInfo(ctx context.Context, name string) (*CollectionInfo, error) {
	return nil, nil
}
func (t *testStore) GetEmbeddingCount(ctx context.Context) (int64, error) { return 0, nil }
func (t *testStore) Health(ctx context.Context) error                     { return nil }
func (t *testStore) Close() error                                         { return nil }

type testEmbedder struct{}

func (t *testEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{1, 2}, nil
}
func (t *testEmbedder) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	return [][]float32{{1, 2}}, nil
}
func (t *testEmbedder) GetDimensions() int            { return 2 }
func (t *testEmbedder) GetModel() string              { return "m" }
func (t *testEmbedder) Health(ctx context.Context) error { return nil }

func TestDefaultConfigsAndSearchOptions(t *testing.T) {
	vc := DefaultVectorConfig()
	require.Equal(t, StoreTypeInMemory, vc.Type)
	require.Equal(t, 768, vc.Dimension)

	so := DefaultSearchOptions()
	require.Equal(t, 10, so.Limit)
	require.True(t, so.IncludeMetadata)
}

func TestVectorStoreRegistryAndFactories(t *testing.T) {
	_, err := NewVectorStore(nil)
	require.Error(t, err)

	_, err = NewVectorStore(&VectorConfig{Type: VectorStoreType("missing")})
	require.Error(t, err)

	RegisterStore(VectorStoreType("unit_store"), func(config *VectorConfig) (VectorStore, error) {
		return &testStore{}, nil
	})
	s, err := NewVectorStore(&VectorConfig{Type: VectorStoreType("unit_store")})
	require.NoError(t, err)
	require.NotNil(t, s)

	RegisterStore(VectorStoreType("unit_store_err"), func(config *VectorConfig) (VectorStore, error) {
		return nil, errors.New("boom")
	})
	_, err = NewVectorStore(&VectorConfig{Type: VectorStoreType("unit_store_err")})
	require.Error(t, err)
}

func TestEmbeddingRegistryAndLegacyFacades(t *testing.T) {
	_, err := NewEmbeddingProvider(EmbeddingProviderType("missing"), nil)
	require.Error(t, err)

	RegisterEmbeddingProvider(EmbeddingProviderType("unit_emb"), func(config map[string]interface{}) (EmbeddingProvider, error) {
		return &testEmbedder{}, nil
	})
	emb, err := NewEmbeddingProvider(EmbeddingProviderType("unit_emb"), map[string]interface{}{"x": 1})
	require.NoError(t, err)
	require.Equal(t, 2, emb.GetDimensions())

	// legacy wrappers should not panic even when provider is not registered
	require.Nil(t, NewLMStudioEmbeddingProvider("http://x", "m"))
	require.Nil(t, NewOpenAIEmbeddingProvider("k", "m"))
	require.Nil(t, NewOllamaEmbeddingProvider("http://x", "m", 1))
	require.Nil(t, NewOpenRouterEmbeddingProvider(OpenRouterConfig{APIKey: "k"}))

	// register in-memory type for this test process then verify legacy wrapper returns instance.
	RegisterStore(StoreTypeInMemory, func(config *VectorConfig) (VectorStore, error) {
		return &testStore{}, nil
	})
	ims := NewInMemoryStore(128)
	require.NotNil(t, ims)
	ims2 := NewInMemoryStore("not-int")
	require.NotNil(t, ims2)

	// register concrete provider names and validate legacy embedding facades
	RegisterEmbeddingProvider(EmbeddingProviderLMStudio, func(config map[string]interface{}) (EmbeddingProvider, error) {
		return &testEmbedder{}, nil
	})
	RegisterEmbeddingProvider(EmbeddingProviderOpenAI, func(config map[string]interface{}) (EmbeddingProvider, error) {
		return &testEmbedder{}, nil
	})
	RegisterEmbeddingProvider(EmbeddingProviderOllama, func(config map[string]interface{}) (EmbeddingProvider, error) {
		return &testEmbedder{}, nil
	})
	RegisterEmbeddingProvider(EmbeddingProviderOpenRouter, func(config map[string]interface{}) (EmbeddingProvider, error) {
		return &testEmbedder{}, nil
	})
	require.NotNil(t, NewLMStudioEmbeddingProvider("http://x", "m"))
	require.NotNil(t, NewOpenAIEmbeddingProvider("k", "m"))
	require.NotNil(t, NewOllamaEmbeddingProvider("http://x", "m", 1))
	require.NotNil(t, NewOpenRouterEmbeddingProvider(OpenRouterConfig{APIKey: "k"}))
}

func TestLegacyVectorStoreFacades(t *testing.T) {
	RegisterStore(StoreTypeSQLite, func(config *VectorConfig) (VectorStore, error) { return &testStore{}, nil })
	RegisterStore(StoreTypeRedis, func(config *VectorConfig) (VectorStore, error) { return &testStore{}, nil })
	RegisterStore(StoreTypePgVector, func(config *VectorConfig) (VectorStore, error) { return &testStore{}, nil })
	RegisterStore(StoreTypeQdrant, func(config *VectorConfig) (VectorStore, error) { return &testStore{}, nil })

	s1, err := NewSQLiteVectorStore("/tmp/a.db", 32)
	require.NoError(t, err)
	require.NotNil(t, s1)

	s2, err := NewRedisVectorStore("127.0.0.1:6379", "", 16)
	require.NoError(t, err)
	require.NotNil(t, s2)

	s3, err := NewPgVectorStore(&VectorConfig{Type: StoreTypePgVector, Dimension: 8})
	require.NoError(t, err)
	require.NotNil(t, s3)

	s4, err := NewQdrantStore(&VectorConfig{Type: StoreTypeQdrant, Dimension: 8})
	require.NoError(t, err)
	require.NotNil(t, s4)
}

func TestVectorDataStructsInstantiation(t *testing.T) {
	now := time.Now()
	_ = EmbeddingData{ID: "e1", Embedding: []float32{1}, CreatedAt: now, UpdatedAt: now}
	_ = SimilarityResult{ID: "s1", Score: 0.9, Distance: 0.1}
	_ = CollectionConfig{Dimension: 2, DistanceMetric: "cosine"}
	_ = CollectionInfo{Name: "c", Status: "ready", CreatedAt: now, UpdatedAt: now}
	_ = VectorMetrics{TotalVectors: 1, LastUpdated: now}
	_ = VectorIndex{Name: "i", CreatedAt: now}
	_ = QueryResult{Total: 1, QueryTime: time.Millisecond}
}

