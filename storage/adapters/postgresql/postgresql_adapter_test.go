package postgresql

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresAdapter_NilConfig(t *testing.T) {
	adapter, err := NewPostgresAdapter(nil)
	require.Error(t, err)
	require.Nil(t, adapter)
}

func TestPostgresTableSelectionHelpers(t *testing.T) {
	require.False(t, isInputDataPoint(nil))
	require.Equal(t, "datapoints", tableForDataPoint(nil))

	dpInput := &schema.DataPoint{Metadata: map[string]interface{}{"is_input": true}}
	dpNormal := &schema.DataPoint{Metadata: map[string]interface{}{"is_input": false}}

	require.True(t, isInputDataPoint(dpInput))
	require.False(t, isInputDataPoint(dpNormal))
	require.Equal(t, "input_datapoints", tableForDataPoint(dpInput))
	require.Equal(t, "datapoints", tableForDataPoint(dpNormal))
}

func newMockAdapter(t *testing.T) (*PostgresAdapter, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
		sqlmock.MonitorPingsOption(true),
	)
	require.NoError(t, err)
	adapter := &PostgresAdapter{
		db: db,
		config: &storage.RelationalConfig{
			Type:           storage.StorageTypePostgreSQL,
			MaxConnections: 1,
		},
	}
	cleanup := func() { _ = db.Close() }
	return adapter, mock, cleanup
}

func TestPostgresAdapter_StoreAndGetDataPoint(t *testing.T) {
	a, mock, cleanup := newMockAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	dp := &schema.DataPoint{
		ID:               "id1",
		Content:          "hello",
		ContentType:      "text",
		Metadata:         map[string]interface{}{"k": "v"},
		SessionID:        "s1",
		UserID:           "u1",
		CreatedAt:        now,
		UpdatedAt:        now,
		ProcessingStatus: schema.StatusCompleted,
	}

	mock.ExpectExec("INSERT INTO datapoints").
		WithArgs(dp.ID, dp.Content, dp.ContentType, sqlmock.AnyArg(), dp.SessionID, dp.UserID, sqlmock.AnyArg(), sqlmock.AnyArg(), dp.ProcessingStatus, dp.ErrorMessage, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, a.StoreDataPoint(ctx, dp))

	metaBytes, _ := json.Marshal(dp.Metadata)
	rows := sqlmock.NewRows([]string{"id", "content", "content_type", "metadata", "session_id", "user_id", "created_at", "updated_at", "processing_status", "error_message", "nodes", "edges"}).
		AddRow(dp.ID, dp.Content, dp.ContentType, metaBytes, dp.SessionID, dp.UserID, dp.CreatedAt, dp.UpdatedAt, dp.ProcessingStatus, dp.ErrorMessage, []byte("[]"), []byte("[]"))

	mock.ExpectQuery("SELECT id, content, content_type, metadata, session_id, user_id").
		WithArgs("id1").
		WillReturnRows(rows)

	got, err := a.GetDataPoint(ctx, "id1")
	require.NoError(t, err)
	require.Equal(t, "hello", got.Content)
	require.Equal(t, "v", got.Metadata["k"])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresAdapter_UpdateDeleteAndQueryDataPoints(t *testing.T) {
	a, mock, cleanup := newMockAdapter(t)
	defer cleanup()

	ctx := context.Background()
	dp := &schema.DataPoint{
		ID:          "id1",
		Content:     "c",
		ContentType: "text",
		Metadata:    map[string]interface{}{"k": "v"},
		SessionID:   "s1",
		UserID:      "u1",
	}

	mock.ExpectExec("UPDATE datapoints SET").
		WithArgs(dp.Content, dp.ContentType, sqlmock.AnyArg(), dp.SessionID, dp.UserID, sqlmock.AnyArg(), dp.ProcessingStatus, dp.ErrorMessage, sqlmock.AnyArg(), sqlmock.AnyArg(), dp.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, a.UpdateDataPoint(ctx, dp))

	mock.ExpectExec("DELETE FROM datapoints WHERE id = \\$1").
		WithArgs("id1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM input_datapoints WHERE id = \\$1").
		WithArgs("id1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, a.DeleteDataPoint(ctx, "id1"))

	q := storage.DefaultDataPointQuery()
	q.SessionID = "s1"
	q.Limit = 10
	// Return empty rows for both tables (datapoints + input_datapoints)
	mock.ExpectQuery("FROM datapoints WHERE 1=1 AND session_id = \\$1").
		WithArgs("s1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "content", "content_type", "metadata", "session_id", "user_id", "created_at", "updated_at", "processing_status", "error_message", "nodes", "edges"}))
	mock.ExpectQuery("FROM input_datapoints WHERE 1=1 AND session_id = \\$1").
		WithArgs("s1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "content", "content_type", "metadata", "session_id", "user_id", "created_at", "updated_at", "processing_status", "error_message", "nodes", "edges"}))
	out, err := a.QueryDataPoints(ctx, q)
	require.NoError(t, err)
	require.Len(t, out, 0)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresAdapter_SessionAndMessages(t *testing.T) {
	a, mock, cleanup := newMockAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	ms := &schema.MemorySession{
		ID:         "s1",
		UserID:     "u1",
		Context:    map[string]interface{}{"lang": "vi"},
		CreatedAt:  now,
		LastAccess: now,
		IsActive:   true,
	}

	mock.ExpectExec("INSERT INTO memory_sessions").
		WithArgs(ms.ID, ms.UserID, sqlmock.AnyArg(), ms.CreatedAt, ms.LastAccess, ms.IsActive, ms.ExpiresAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, a.StoreSession(ctx, ms))

	contextBytes, _ := json.Marshal(ms.Context)
	mock.ExpectQuery("FROM memory_sessions WHERE id = \\$1").
		WithArgs("s1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "context", "created_at", "last_access", "is_active", "expires_at"}).
			AddRow(ms.ID, ms.UserID, contextBytes, ms.CreatedAt, ms.LastAccess, ms.IsActive, ms.ExpiresAt))
	_, err := a.GetSession(ctx, "s1")
	require.NoError(t, err)

	mock.ExpectExec("INSERT INTO session_messages").
		WithArgs("m1", "s1", schema.RoleUser, "hi", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, a.AddMessageToSession(ctx, "s1", schema.Message{ID: "m1", Role: schema.RoleUser, Content: "hi", Timestamp: now}))

	mock.ExpectQuery("FROM session_messages").
		WithArgs("s1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "role", "content", "created_at"}).
			AddRow("m1", schema.RoleUser, "hi", now))
	msgs, err := a.GetSessionMessages(ctx, "s1")
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	mock.ExpectExec("DELETE FROM session_messages WHERE session_id = \\$1").
		WithArgs("s1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, a.DeleteSessionMessages(ctx, "s1"))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresAdapter_UpdateDeleteListSessionAndBatchCountHealthClose(t *testing.T) {
	a, mock, cleanup := newMockAdapter(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now()

	ms := &schema.MemorySession{
		ID:         "s1",
		UserID:     "u1",
		Context:    map[string]interface{}{"k": "v"},
		LastAccess: now,
		IsActive:   true,
	}

	mock.ExpectExec("UPDATE memory_sessions SET").
		WithArgs(ms.UserID, sqlmock.AnyArg(), sqlmock.AnyArg(), ms.IsActive, ms.ExpiresAt, ms.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, a.UpdateSession(ctx, ms))

	mock.ExpectExec("UPDATE memory_sessions SET").
		WithArgs(ms.UserID, sqlmock.AnyArg(), sqlmock.AnyArg(), ms.IsActive, ms.ExpiresAt, ms.ID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, a.UpdateSession(ctx, ms))

	mock.ExpectExec("DELETE FROM memory_sessions WHERE id = \\$1").
		WithArgs("s1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, a.DeleteSession(ctx, "s1"))

	mock.ExpectExec("DELETE FROM memory_sessions WHERE id = \\$1").
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, a.DeleteSession(ctx, "missing"))

	contextBytes, _ := json.Marshal(map[string]interface{}{"lang": "vi"})
	mock.ExpectQuery("FROM memory_sessions WHERE user_id = \\$1 ORDER BY last_access DESC").
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "context", "created_at", "last_access", "is_active", "expires_at"}).
			AddRow("s1", "u1", contextBytes, now, now, true, nil))
	sessions, err := a.ListSessions(ctx, "u1")
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	dp1 := &schema.DataPoint{ID: "d1", Content: "a", ContentType: "text", Metadata: map[string]interface{}{}, SessionID: "s1", UserID: "u1", CreatedAt: now, UpdatedAt: now}
	dp2 := &schema.DataPoint{ID: "d2", Content: "b", ContentType: "text", Metadata: map[string]interface{}{}, SessionID: "s1", UserID: "u1", CreatedAt: now, UpdatedAt: now}
	mock.ExpectExec("INSERT INTO datapoints").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO datapoints").WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, a.StoreBatch(ctx, []*schema.DataPoint{dp1, dp2}))

	require.NoError(t, a.DeleteBatch(ctx, nil))
	mock.ExpectExec("DELETE FROM datapoints WHERE id = ANY\\(\\$1\\)").WithArgs("{d1,d2}").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM input_datapoints WHERE id = ANY\\(\\$1\\)").WithArgs("{d1,d2}").WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, a.DeleteBatch(ctx, []string{"d1", "d2"}))

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM datapoints").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM input_datapoints").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	count, err := a.GetDataPointCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 5, count)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM memory_sessions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(7)))
	sc, err := a.GetSessionCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 7, sc)

	mock.ExpectPing()
	require.NoError(t, a.Health(ctx))

	mock.ExpectClose()
	require.NoError(t, a.Close())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresAdapter_ErrorBranches(t *testing.T) {
	a, mock, cleanup := newMockAdapter(t)
	defer cleanup()
	ctx := context.Background()

	badSession := &schema.MemorySession{ID: "x", UserID: "u", Context: map[string]interface{}{"bad": make(chan int)}}
	require.Error(t, a.UpdateSession(ctx, badSession))

	mock.ExpectExec("DELETE FROM datapoints WHERE id = ANY\\(\\$1\\)").
		WithArgs("{x}").
		WillReturnError(errors.New("boom"))
	require.Error(t, a.DeleteBatch(ctx, []string{"x"}))

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM datapoints").
		WillReturnError(errors.New("count fail"))
	_, err := a.GetDataPointCount(ctx)
	require.Error(t, err)
}

func TestPostgresAdapter_DeleteBySessionAndUnscoped(t *testing.T) {
	a, mock, cleanup := newMockAdapter(t)
	defer cleanup()
	ctx := context.Background()

	mock.ExpectExec("DELETE FROM datapoints WHERE session_id = \\$1").
		WithArgs("s1").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM input_datapoints WHERE session_id = \\$1").
		WithArgs("s1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, a.DeleteDataPointsBySession(ctx, "s1"))

	mock.ExpectExec("DELETE FROM datapoints WHERE session_id IS NULL OR session_id = ''").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM input_datapoints WHERE session_id IS NULL OR session_id = ''").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, a.DeleteDataPointsUnscoped(ctx))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresAdapter_SearchAndQueryBranches(t *testing.T) {
	a, mock, cleanup := newMockAdapter(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now()

	// SearchDataPoints delegates to QueryDataPoints with ILIKE
	rowCols := []string{"id", "content", "content_type", "metadata", "session_id", "user_id", "created_at", "updated_at", "processing_status", "error_message", "nodes", "edges"}
	mock.ExpectQuery("FROM datapoints WHERE 1=1 AND content ILIKE \\$1").
		WithArgs("%hello%").
		WillReturnRows(sqlmock.NewRows(rowCols).
			AddRow("d2", "hello world", "text", []byte(`{"is_input":false}`), "s1", "u1", now, now, schema.StatusCompleted, "", []byte("[]"), []byte("[]")))
	mock.ExpectQuery("FROM input_datapoints WHERE 1=1 AND content ILIKE \\$1").
		WithArgs("%hello%").
		WillReturnRows(sqlmock.NewRows(rowCols))
	got, err := a.SearchDataPoints(ctx, "hello", map[string]interface{}{"x": 1})
	require.NoError(t, err)
	require.Len(t, got, 1)

	// QueryDataPoints nil query branch (default query object)
	mock.ExpectQuery("FROM datapoints WHERE 1=1").
		WillReturnRows(sqlmock.NewRows(rowCols))
	mock.ExpectQuery("FROM input_datapoints WHERE 1=1").
		WillReturnRows(sqlmock.NewRows(rowCols))
	out, err := a.QueryDataPoints(ctx, nil)
	require.NoError(t, err)
	require.Len(t, out, 0)

	// Offset branch: start >= len(results) returns empty
	mock.ExpectQuery("FROM datapoints WHERE 1=1").
		WillReturnRows(sqlmock.NewRows(rowCols).
			AddRow("d1", "one", "text", []byte(`{}`), "s1", "u1", now, now, schema.StatusCompleted, "", []byte("[]"), []byte("[]")))
	mock.ExpectQuery("FROM input_datapoints WHERE 1=1").
		WillReturnRows(sqlmock.NewRows(rowCols))
	q := storage.DefaultDataPointQuery()
	q.Offset = 10
	res, err := a.QueryDataPoints(ctx, q)
	require.NoError(t, err)
	require.Len(t, res, 0)

	// Error branch: second table query fails
	mock.ExpectQuery("FROM datapoints WHERE 1=1").
		WillReturnRows(sqlmock.NewRows(rowCols))
	mock.ExpectQuery("FROM input_datapoints WHERE 1=1").
		WillReturnError(errors.New("boom query"))
	_, err = a.QueryDataPoints(ctx, storage.DefaultDataPointQuery())
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

