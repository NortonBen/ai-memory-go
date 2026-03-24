package vector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStore_CRUD(t *testing.T) {
	store := NewInMemoryStore(DefaultVectorConfig())
	ctx := context.Background()

	emb := []float32{0.1, 0.2, 0.3}
	meta := map[string]interface{}{"key": "val"}

	// Store
	require.NoError(t, store.StoreEmbedding(ctx, "id-1", emb, meta))

	// Get
	gotEmb, gotMeta, err := store.GetEmbedding(ctx, "id-1")
	require.NoError(t, err)
	assert.Equal(t, emb, gotEmb)
	assert.Equal(t, meta, gotMeta)

	// Update
	newEmb := []float32{0.9, 0.8, 0.7}
	require.NoError(t, store.UpdateEmbedding(ctx, "id-1", newEmb))
	gotEmb, _, _ = store.GetEmbedding(ctx, "id-1")
	assert.Equal(t, newEmb, gotEmb)

	// Count
	count, err := store.GetEmbeddingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Delete
	require.NoError(t, store.DeleteEmbedding(ctx, "id-1"))
	_, _, err = store.GetEmbedding(ctx, "id-1")
	assert.Error(t, err)
}

func TestInMemoryStore_SimilaritySearch(t *testing.T) {
	store := NewInMemoryStore(DefaultVectorConfig())
	ctx := context.Background()

	require.NoError(t, store.StoreEmbedding(ctx, "a", []float32{1, 0, 0}, map[string]interface{}{"type": "A"}))
	require.NoError(t, store.StoreEmbedding(ctx, "b", []float32{0, 1, 0}, map[string]interface{}{"type": "B"}))
	require.NoError(t, store.StoreEmbedding(ctx, "c", []float32{1, 0.1, 0}, map[string]interface{}{"type": "A"}))

	// Search — query close to [1,0,0]; should return "a" and "c" first
	results, err := store.SimilaritySearch(ctx, []float32{1, 0, 0}, 3, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)
	assert.Equal(t, "a", results[0].ID) // perfect match
}

func TestInMemoryStore_SimilaritySearchWithFilter(t *testing.T) {
	store := NewInMemoryStore(DefaultVectorConfig())
	ctx := context.Background()

	require.NoError(t, store.StoreEmbedding(ctx, "a", []float32{1, 0, 0}, map[string]interface{}{"type": "A"}))
	require.NoError(t, store.StoreEmbedding(ctx, "b", []float32{1, 0, 0}, map[string]interface{}{"type": "B"}))

	results, err := store.SimilaritySearchWithFilter(ctx, []float32{1, 0, 0}, map[string]interface{}{"type": "A"}, 10, 0)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "a", results[0].ID)
}

func TestInMemoryStore_BatchOperations(t *testing.T) {
	store := NewInMemoryStore(DefaultVectorConfig())
	ctx := context.Background()

	embeddings := []*EmbeddingData{
		{ID: "x1", Embedding: []float32{1, 0}, Metadata: map[string]interface{}{"n": 1}},
		{ID: "x2", Embedding: []float32{0, 1}, Metadata: map[string]interface{}{"n": 2}},
	}
	require.NoError(t, store.StoreBatchEmbeddings(ctx, embeddings))

	count, _ := store.GetEmbeddingCount(ctx)
	assert.Equal(t, int64(2), count)

	require.NoError(t, store.DeleteBatchEmbeddings(ctx, []string{"x1", "x2"}))
	count, _ = store.GetEmbeddingCount(ctx)
	assert.Equal(t, int64(0), count)
}

func TestVectorFactory(t *testing.T) {
	factory := NewVectorFactory()
	types := factory.ListSupportedTypes()
	assert.Contains(t, types, StoreTypeQdrant)
	assert.Contains(t, types, StoreTypePgVector)
	assert.Contains(t, types, StoreTypeInMemory)

	// InMemory can be created without external services
	store, err := factory.CreateVectorStore(&VectorConfig{Type: StoreTypeInMemory, Dimension: 128})
	require.NoError(t, err)
	assert.NotNil(t, store)
	assert.NoError(t, store.Health(context.Background()))
}
