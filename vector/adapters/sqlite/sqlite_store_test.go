package sqlite

import (
	"context"
	"os"
	"testing"

	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempVecStore(t *testing.T) *SQLiteVectorStore {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "vec-*.db")
	require.NoError(t, err)
	f.Close()
	config := &vector.VectorConfig{
		Database:  f.Name(),
		Dimension: 3,
	}
	s, err := NewSQLiteVectorStore(config)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteVectorStore_CRUD(t *testing.T) {
	s := tempVecStore(t)
	ctx := context.Background()

	emb := []float32{1.0, 0.0, 0.0}
	meta := map[string]interface{}{"type": "test"}

	require.NoError(t, s.StoreEmbedding(ctx, "v1", emb, meta))

	gotEmb, gotMeta, err := s.GetEmbedding(ctx, "v1")
	require.NoError(t, err)
	assert.Equal(t, emb, gotEmb)
	assert.Equal(t, "test", gotMeta["type"])

	require.NoError(t, s.UpdateEmbedding(ctx, "v1", []float32{0.0, 1.0, 0.0}))

	count, _ := s.GetEmbeddingCount(ctx)
	assert.Equal(t, int64(1), count)

	require.NoError(t, s.DeleteEmbedding(ctx, "v1"))
	count, _ = s.GetEmbeddingCount(ctx)
	assert.Equal(t, int64(0), count)
}

func TestSQLiteVectorStore_SimilaritySearch(t *testing.T) {
	s := tempVecStore(t)
	ctx := context.Background()

	require.NoError(t, s.StoreBatchEmbeddings(ctx, []*vector.EmbeddingData{
		{ID: "x1", Embedding: []float32{1, 0, 0}, Metadata: map[string]interface{}{"tag": "first"}},
		{ID: "x2", Embedding: []float32{0, 1, 0}, Metadata: map[string]interface{}{"tag": "second"}},
	}))

	results, err := s.SimilaritySearch(ctx, []float32{1, 0, 0}, 2, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)
	// x1 should be closest to [1,0,0] (distance 0)
	assert.Equal(t, "x1", results[0].ID)
}

func TestSQLiteVectorStore_BatchDeleteAndCount(t *testing.T) {
	s := tempVecStore(t)
	ctx := context.Background()

	require.NoError(t, s.StoreBatchEmbeddings(ctx, []*vector.EmbeddingData{
		{ID: "b1", Embedding: []float32{1, 0, 0}, Metadata: map[string]interface{}{}},
		{ID: "b2", Embedding: []float32{0, 1, 0}, Metadata: map[string]interface{}{}},
	}))

	count, _ := s.GetEmbeddingCount(ctx)
	assert.Equal(t, int64(2), count)

	require.NoError(t, s.DeleteBatchEmbeddings(ctx, []string{"b1", "b2"}))
	count, _ = s.GetEmbeddingCount(ctx)
	assert.Equal(t, int64(0), count)
}
