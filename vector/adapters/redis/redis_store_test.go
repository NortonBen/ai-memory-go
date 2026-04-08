package redis

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/stretchr/testify/require"
)

func TestFloat32BytesRoundTrip(t *testing.T) {
	in := []float32{0, 1.25, -3.5, 42}
	b := float32ToBytes(in)
	out := bytesToFloat32(b)
	require.Equal(t, len(in), len(out))
	for i := range in {
		require.InDelta(t, in[i], out[i], 0.0001)
	}
}

func TestBytesToFloat32_InvalidLength(t *testing.T) {
	require.Nil(t, bytesToFloat32([]byte{1, 2, 3}))
}

func TestNewRedisVectorStore_AndBasicCRUD(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	// force DB from env branch
	t.Setenv("REDIS_DB_VECTOR", "2")

	cfg := &vector.VectorConfig{
		Host:      mr.Host(),
		Port:      mr.Server().Addr().Port,
		Dimension: 4,
	}
	s, err := NewRedisVectorStore(cfg)
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()
	meta := map[string]interface{}{"k": "v"}
	require.NoError(t, s.StoreEmbedding(ctx, "id1", []float32{1, 2, 3, 4}, meta))

	emb, gotMeta, err := s.GetEmbedding(ctx, "id1")
	require.NoError(t, err)
	require.NotNil(t, emb)
	require.Equal(t, "v", gotMeta["k"])

	require.NoError(t, s.UpdateEmbedding(ctx, "id1", []float32{9, 8, 7, 6}))
	require.NoError(t, s.DeleteEmbedding(ctx, "id1"))
	_, _, err = s.GetEmbedding(ctx, "id1")
	require.Error(t, err)
}

func TestUpdateEmbedding_NotFound(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &vector.VectorConfig{Host: mr.Host(), Port: mr.Server().Addr().Port, Dimension: 4}
	s, err := NewRedisVectorStore(cfg)
	require.NoError(t, err)
	defer s.Close()

	err = s.UpdateEmbedding(context.Background(), "missing", []float32{1, 2})
	require.Error(t, err)
}

func TestBatchAndCollectionHelpers_ErrorBranches(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &vector.VectorConfig{Host: mr.Host(), Port: mr.Server().Addr().Port, Dimension: 3}
	s, err := NewRedisVectorStore(cfg)
	require.NoError(t, err)
	defer s.Close()
	ctx := context.Background()

	require.NoError(t, s.StoreBatchEmbeddings(ctx, []*vector.EmbeddingData{
		{ID: "b1", Embedding: []float32{1, 2, 3}, Metadata: map[string]interface{}{"a": 1}},
		{ID: "b2", Embedding: []float32{4, 5, 6}, Metadata: map[string]interface{}{"a": 2}},
	}))
	require.NoError(t, s.DeleteBatchEmbeddings(ctx, []string{"b1", "b2"}))
	require.NoError(t, s.DeleteBatchEmbeddings(ctx, nil))

	// unsupported FT commands in miniredis -> exercise error branches
	require.Error(t, s.CreateCollection(ctx, "c1", 3, &vector.CollectionConfig{DistanceMetric: "l2"}))
	require.Error(t, s.DeleteCollection(ctx, "c1"))
	_, err = s.ListCollections(ctx)
	require.Error(t, err)
	_, err = s.GetCollectionInfo(ctx, "c1")
	require.Error(t, err)
	count, err := s.GetEmbeddingCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 0, count)

	require.NoError(t, s.Health(ctx))
	require.NoError(t, s.Close())
}

func TestSimilaritySearchWithFilter_ErrorOrNoIndexBranch(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	cfg := &vector.VectorConfig{Host: mr.Host(), Port: mr.Server().Addr().Port, Dimension: 3}
	s, err := NewRedisVectorStore(cfg)
	require.NoError(t, err)
	defer s.Close()

	// On miniredis this returns command error (no FT module), which still covers error path.
	_, err = s.SimilaritySearchWithFilter(context.Background(), []float32{1, 2, 3}, map[string]interface{}{"k": "v"}, 3, 0.1)
	require.Error(t, err)
}

func TestNewRedisVectorStore_InvalidEnvDBStillWorks(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	_ = os.Setenv("REDIS_DB_VECTOR", "not-int")
	defer os.Unsetenv("REDIS_DB_VECTOR")

	cfg := &vector.VectorConfig{Host: mr.Host(), Port: mr.Server().Addr().Port, Dimension: 2}
	s, err := NewRedisVectorStore(cfg)
	require.NoError(t, err)
	require.NoError(t, s.Close())
}

