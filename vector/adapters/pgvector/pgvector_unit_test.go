package pgvector

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NortonBen/ai-memory-go/vector"
	pgv "github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/require"
)

func TestDeleteBatchEmbeddings_EmptyIDs(t *testing.T) {
	s := &PgVectorStore{}
	err := s.DeleteBatchEmbeddings(context.Background(), nil)
	require.NoError(t, err)
}

func newMockStore(t *testing.T) (*PgVectorStore, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	s := &PgVectorStore{db: db, tableName: "vector_embeddings", dimension: 3}
	return s, mock, func() { _ = db.Close() }
}

func TestStoreEmbedding_MetadataMarshalError(t *testing.T) {
	s, _, cleanup := newMockStore(t)
	defer cleanup()

	err := s.StoreEmbedding(context.Background(), "id", []float32{1, 2, 3}, map[string]interface{}{"bad": make(chan int)})
	require.Error(t, err)
}

func TestGetEmbedding_NotFoundAndBadMetadata(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectQuery("SELECT embedding, metadata FROM vector_embeddings WHERE id = \\$1").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	_, _, err := s.GetEmbedding(context.Background(), "missing")
	require.Error(t, err)

	rows := sqlmock.NewRows([]string{"embedding", "metadata"}).
		AddRow(pgv.NewVector([]float32{1, 2, 3}), []byte("{invalid"))
	mock.ExpectQuery("SELECT embedding, metadata FROM vector_embeddings WHERE id = \\$1").
		WithArgs("id1").
		WillReturnRows(rows)
	_, _, err = s.GetEmbedding(context.Background(), "id1")
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateEmbedding_NotFound(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec("UPDATE vector_embeddings SET embedding = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "id1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := s.UpdateEmbedding(context.Background(), "id1", []float32{1, 2, 3})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSimilaritySearch_ScanAndUnmarshalErrors(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	badRows := sqlmock.NewRows([]string{"id", "embedding", "metadata", "distance"}).
		AddRow("x", pgv.NewVector([]float32{1, 2, 3}), []byte(`{}`), "not-a-float")
	mock.ExpectQuery("FROM vector_embeddings").
		WillReturnRows(badRows)
	_, err := s.SimilaritySearch(context.Background(), []float32{1, 2, 3}, 3, 0.1)
	require.Error(t, err)

	rows := sqlmock.NewRows([]string{"id", "embedding", "metadata", "distance"}).
		AddRow("x", pgv.NewVector([]float32{1, 2, 3}), []byte("{bad"), float64(0.1))
	mock.ExpectQuery("FROM vector_embeddings").
		WillReturnRows(rows)
	_, err = s.SimilaritySearch(context.Background(), []float32{1, 2, 3}, 3, 0.1)
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSimilaritySearchWithFilter_FiltersAndClientSideMatch(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	cols := []string{"id", "embedding", "metadata", "distance"}
	rows := sqlmock.NewRows(cols).
		AddRow("a", pgv.NewVector([]float32{1, 2, 3}), []byte(`{"k":"v","memory_labels":["x"]}`), float64(0.2)).
		AddRow("b", pgv.NewVector([]float32{4, 5, 6}), []byte(`{"k":"nope"}`), float64(0.3))

	mock.ExpectQuery("FROM vector_embeddings").
		WillReturnRows(rows)

	out, err := s.SimilaritySearchWithFilter(context.Background(), []float32{1, 2, 3}, map[string]interface{}{"k": "v"}, 1, 0.1)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0].ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreBatchEmbeddings_PrepareExecAndCommit(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO vector_embeddings").
		ExpectExec().
		WithArgs("e1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := s.StoreBatchEmbeddings(context.Background(), []*vector.EmbeddingData{{ID: "e1", Embedding: []float32{1, 2, 3}, Metadata: map[string]interface{}{"k": "v"}}})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreBatchEmbeddings_ExecErrorRollsBack(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO vector_embeddings").
		ExpectExec().
		WillReturnError(errors.New("boom"))
	mock.ExpectRollback()

	err := s.StoreBatchEmbeddings(context.Background(), []*vector.EmbeddingData{{ID: "e1", Embedding: []float32{1, 2, 3}}})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteBatchEmbeddings_BuildsPlaceholders(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec("DELETE FROM vector_embeddings WHERE id IN \\(\\$1,\\$2\\)").
		WithArgs("a", "b").
		WillReturnResult(sqlmock.NewResult(0, 2))
	require.NoError(t, s.DeleteBatchEmbeddings(context.Background(), []string{"a", "b"}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateCollection_ErrorBranches(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS vector").WillReturnError(errors.New("no ext"))
	require.Error(t, s.CreateCollection(context.Background(), "c", 3, nil))

	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS vector").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS c").WillReturnError(errors.New("no table"))
	require.Error(t, s.CreateCollection(context.Background(), "c", 3, nil))

	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS vector").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS c").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS c_embedding_idx").WillReturnError(errors.New("no idx"))
	require.Error(t, s.CreateCollection(context.Background(), "c", 3, &vector.CollectionConfig{IndexType: "hnsw", DistanceMetric: "dot"}))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCollections_ScanError(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	// Force scan error by returning 2 columns while Scan expects 1.
	rows := sqlmock.NewRows([]string{"table_name", "extra"}).AddRow("t1", "x")
	mock.ExpectQuery("information_schema\\.tables").WillReturnRows(rows)
	_, err := s.ListCollections(context.Background())
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionInfoAndEmbeddingCount_AndHealthClose(t *testing.T) {
	s, mock, cleanup := newMockStore(t)
	defer cleanup()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM c").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	info, err := s.GetCollectionInfo(context.Background(), "c")
	require.NoError(t, err)
	require.Equal(t, int64(2), info.VectorCount)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM vector_embeddings").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(5)))
	cnt, err := s.GetEmbeddingCount(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 5, cnt)

	mock.ExpectPing()
	require.NoError(t, s.Health(context.Background()))

	mock.ExpectClose()
	require.NoError(t, s.Close())

	require.NoError(t, mock.ExpectationsWereMet())
}

