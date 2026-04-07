package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/stretchr/testify/require"
)

func TestSQLConnectionWrappers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	pg := NewPostgreSQLConnection(db)
	require.True(t, pg.IsValid())
	require.NotNil(t, pg.GetDB())
	pg.SetLastUsed(time.Now().Add(-time.Minute))
	require.False(t, pg.LastUsed().IsZero())
	mock.ExpectClose()
	require.NoError(t, pg.Close())
	require.False(t, pg.IsValid())
	require.NoError(t, mock.ExpectationsWereMet())

	db2, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	sqlc := NewSQLiteConnection(db2)
	require.True(t, sqlc.IsValid())
	require.NoError(t, sqlc.Ping(context.Background()))
	require.NotNil(t, sqlc.GetDB())
	require.NoError(t, sqlc.Close())
}

func TestGraphAndVectorConnectionWrappers(t *testing.T) {
	gStore := graph.NewInMemoryGraphStore()
	gc := NewGraphConnection(gStore)
	require.NoError(t, gc.Ping(context.Background()))
	require.NotNil(t, gc.GetStore())
	require.NoError(t, gc.Close())
	require.False(t, gc.IsValid())

	vStore, err := vector.NewVectorStore(&vector.VectorConfig{
		Type:      vector.StoreTypeInMemory,
		Dimension: 3,
		Collection:"x",
	})
	require.NoError(t, err)
	vc := NewVectorConnection(vStore)
	require.NoError(t, vc.Ping(context.Background()))
	require.NotNil(t, vc.GetStore())
	require.NoError(t, vc.Close())
	require.False(t, vc.IsValid())
}

func TestFactories_CreateAndValidateTypeMismatch(t *testing.T) {
	pgFactory := NewPostgreSQLConnectionFactory(&RelationalConfig{})
	require.Error(t, pgFactory.ValidateConnection(context.Background(), &mockConn{valid: true}))

	pgdb, pgmock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	pgmock.ExpectPing()
	pgconn := NewPostgreSQLConnection(pgdb)
	require.NoError(t, pgFactory.ValidateConnection(context.Background(), pgconn))
	require.NoError(t, pgmock.ExpectationsWereMet())
	_ = pgconn.Close()

	sqlFactory := NewSQLiteConnectionFactory(&RelationalConfig{
		Database:       ":memory:",
		MaxConnections: 1,
		MinConnections: 0,
	})
	conn, err := sqlFactory.CreateConnection(context.Background())
	require.NoError(t, err)
	require.NoError(t, sqlFactory.ValidateConnection(context.Background(), conn))
	require.Error(t, sqlFactory.ValidateConnection(context.Background(), &mockConn{valid: true}))
	_ = conn.Close()

	graphFactory := NewGraphConnectionFactory(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
	gconn, err := graphFactory.CreateConnection(context.Background())
	require.NoError(t, err)
	require.NoError(t, graphFactory.ValidateConnection(context.Background(), gconn))
	require.Error(t, graphFactory.ValidateConnection(context.Background(), &mockConn{valid: true}))
	_ = gconn.Close()

	vectorFactory := NewVectorConnectionFactory(&vector.VectorConfig{Type: vector.StoreTypeInMemory, Dimension: 3, Collection:"c"})
	vconn, err := vectorFactory.CreateConnection(context.Background())
	require.NoError(t, err)
	require.NoError(t, vectorFactory.ValidateConnection(context.Background(), vconn))
	require.Error(t, vectorFactory.ValidateConnection(context.Background(), &mockConn{valid: true}))
	_ = vconn.Close()
}

func TestRelationalStoreHealthChecker(t *testing.T) {
	rs := &fakeRelStore{}
	checker := NewRelationalStoreHealthChecker("rel", rs)
	c := checker.Check(context.Background())
	require.Equal(t, HealthStatusHealthy, c.Status)
	require.Equal(t, "rel", checker.Name())
}

func TestMockVectorStore_PassthroughMethods(t *testing.T) {
	m := &mockVectorStore{}
	ctx := context.Background()

	require.NoError(t, m.StoreEmbedding(ctx, "id", []float32{1, 2}, map[string]interface{}{"k": "v"}))
	emb, meta, err := m.GetEmbedding(ctx, "id")
	require.NoError(t, err)
	require.Nil(t, emb)
	require.Nil(t, meta)
	require.NoError(t, m.UpdateEmbedding(ctx, "id", []float32{3}))
	require.NoError(t, m.DeleteEmbedding(ctx, "id"))

	r1, err := m.SimilaritySearch(ctx, []float32{1}, 5, 0.8)
	require.NoError(t, err)
	require.Nil(t, r1)
	r2, err := m.SimilaritySearchWithFilter(ctx, []float32{1}, map[string]interface{}{"a": 1}, 5, 0.8)
	require.NoError(t, err)
	require.Nil(t, r2)

	require.NoError(t, m.StoreBatchEmbeddings(ctx, []*vector.EmbeddingData{{ID: "e1", Embedding: []float32{1}}}))
	require.NoError(t, m.DeleteBatchEmbeddings(ctx, []string{"e1"}))
	require.NoError(t, m.CreateCollection(ctx, "c", 3, &vector.CollectionConfig{}))
	require.NoError(t, m.DeleteCollection(ctx, "c"))

	cols, err := m.ListCollections(ctx)
	require.NoError(t, err)
	require.Nil(t, cols)
	info, err := m.GetCollectionInfo(ctx, "c")
	require.NoError(t, err)
	require.Nil(t, info)
	cnt, err := m.GetEmbeddingCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 0, cnt)
	require.NoError(t, m.Health(ctx))
	require.NoError(t, m.Close())
}

// satisfy compile for accidental interface drift in tests
var _ schema.Connection = (*mockConn)(nil)

type poolStub struct {
	getErr   error
	putErr   error
	closeErr error
	conn     schema.Connection
}

func (p *poolStub) Get(ctx context.Context) (schema.Connection, error) {
	if p.getErr != nil {
		return nil, p.getErr
	}
	if p.conn != nil {
		return p.conn, nil
	}
	return &mockConn{valid: true, lastUsed: time.Now()}, nil
}
func (p *poolStub) Put(conn schema.Connection) error {
	if p.putErr != nil {
		return p.putErr
	}
	return nil
}
func (p *poolStub) Health(ctx context.Context) error { return nil }
func (p *poolStub) Close() error                     { return p.closeErr }
func (p *poolStub) Stats() *PoolStats               { return &PoolStats{} }

func TestPooledStorageManager_NilPools(t *testing.T) {
	m := &PooledStorageManager{
		healthMonitor: NewHealthMonitor(DefaultHealthMonitorConfig()),
	}
	ctx := context.Background()

	_, err := m.GetRelationalConnection(ctx)
	require.Error(t, err)
	require.Error(t, m.PutRelationalConnection(&mockConn{valid: true}))
	_, err = m.GetGraphConnection(ctx)
	require.Error(t, err)
	require.Error(t, m.PutGraphConnection(&mockConn{valid: true}))
	_, err = m.GetVectorConnection(ctx)
	require.Error(t, err)
	require.Error(t, m.PutVectorConnection(&mockConn{valid: true}))
}

func TestPooledStorageManager_CloseAggregatesErrors(t *testing.T) {
	m := &PooledStorageManager{
		relationalPool: &poolStub{closeErr: errors.New("rel close")},
		graphPool:      &poolStub{closeErr: errors.New("graph close")},
		vectorPool:     &poolStub{closeErr: nil},
		healthMonitor:  NewHealthMonitor(DefaultHealthMonitorConfig()),
	}

	err := m.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple errors during close")

	// second close should be idempotent
	require.NoError(t, m.Close())
}

