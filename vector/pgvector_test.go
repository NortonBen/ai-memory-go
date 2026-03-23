package vector

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgVectorStore_Integration(t *testing.T) {
	host := os.Getenv("TEST_PGVECTOR_HOST")
	if host == "" {
		t.Skip("Skipping PgVector integration test. Set TEST_PGVECTOR_HOST to run.")
	}

	port, _ := strconv.Atoi(os.Getenv("TEST_PGVECTOR_PORT"))
	if port == 0 {
		port = 5432
	}
	user := os.Getenv("TEST_PGVECTOR_USER")
	if user == "" {
		user = "postgres"
	}
	pass := os.Getenv("TEST_PGVECTOR_PASSWORD")
	dbName := os.Getenv("TEST_PGVECTOR_DB")
	if dbName == "" {
		dbName = "postgres"
	}

	config := &VectorConfig{
		Host:           host,
		Port:           port,
		Username:       user,
		Password:       pass,
		Database:       dbName,
		Collection:     "test_pgvector_collection",
		Dimension:      3,
		MaxConnections: 5,
		IdleTimeout:    time.Minute,
	}

	store, err := NewPgVectorStore(config)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// 1. Create collection
	err = store.CreateCollection(ctx, config.Collection, config.Dimension, nil)
	require.NoError(t, err)

	// Clean up after test
	defer func() {
		_ = store.DeleteCollection(ctx, config.Collection)
	}()

	// 2. Health check
	err = store.Health(ctx)
	assert.NoError(t, err)

	// 3. Store Embedding
	emb1 := []float32{1.0, 0.0, 0.0}
	meta1 := map[string]interface{}{"type": "doc1"}
	err = store.StoreEmbedding(ctx, "id1", emb1, meta1)
	assert.NoError(t, err)

	emb2 := []float32{0.0, 1.0, 0.0}
	meta2 := map[string]interface{}{"type": "doc2"}
	err = store.StoreEmbedding(ctx, "id2", emb2, meta2)
	assert.NoError(t, err)

	// 4. Get Embedding
	retrievedEmb, retrievedMeta, err := store.GetEmbedding(ctx, "id1")
	assert.NoError(t, err)
	assert.Equal(t, emb1, retrievedEmb)
	assert.Equal(t, "doc1", retrievedMeta["type"])

	// 5. Similarity Search (L2/Cosine)
	queryEmb := []float32{0.9, 0.1, 0.0} // closer to emb1
	results, err := store.SimilaritySearch(ctx, queryEmb, 2, 0.0)
	assert.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "id1", results[0].ID) // Should be closest

	// 6. Delete Embedding
	err = store.DeleteEmbedding(ctx, "id1")
	assert.NoError(t, err)
	_, _, err = store.GetEmbedding(ctx, "id1")
	assert.Error(t, err)

	// 7. Store Batch
	batch := []*EmbeddingData{
		{ID: "batch1", Embedding: []float32{0.5, 0.5, 0.0}, Metadata: map[string]interface{}{"tag": "a"}},
		{ID: "batch2", Embedding: []float32{0.0, 0.5, 0.5}, Metadata: map[string]interface{}{"tag": "b"}},
	}
	err = store.StoreBatchEmbeddings(ctx, batch)
	assert.NoError(t, err)

	// 8. Delete Batch
	err = store.DeleteBatchEmbeddings(ctx, []string{"batch1", "batch2"})
	assert.NoError(t, err)
}
