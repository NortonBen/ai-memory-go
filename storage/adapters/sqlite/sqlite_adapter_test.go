package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteAdapter_NilConfig(t *testing.T) {
	adapter, err := NewSQLiteAdapter(nil)
	require.Error(t, err)
	require.Nil(t, adapter)
}

func TestSQLiteTableSelectionHelpers(t *testing.T) {
	require.False(t, isInputDataPoint(nil))
	require.Equal(t, "datapoints", tableForDataPoint(nil))

	dpInput := &schema.DataPoint{Metadata: map[string]interface{}{"is_input": true}}
	dpNormal := &schema.DataPoint{Metadata: map[string]interface{}{"is_input": false}}

	require.True(t, isInputDataPoint(dpInput))
	require.False(t, isInputDataPoint(dpNormal))
	require.Equal(t, "input_datapoints", tableForDataPoint(dpInput))
	require.Equal(t, "datapoints", tableForDataPoint(dpNormal))
}

func newTestSQLiteAdapter(t *testing.T) *SQLiteAdapter {
	t.Helper()
	cfg := &storage.RelationalConfig{
		Type:           storage.StorageTypeSQLite,
		Database:       ":memory:",
		MaxConnections: 1,
		ConnTimeout:    2 * time.Second,
	}
	a, err := NewSQLiteAdapter(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestSQLiteAdapter_DataPointCRUDAndQuery(t *testing.T) {
	a := newTestSQLiteAdapter(t)
	ctx := context.Background()
	now := time.Now()

	dp1 := &schema.DataPoint{
		ID:               "dp1",
		Content:          "hello world",
		ContentType:      "text",
		Metadata:         map[string]interface{}{"k": "v"},
		SessionID:        "s1",
		UserID:           "u1",
		CreatedAt:        now,
		UpdatedAt:        now,
		ProcessingStatus: schema.StatusCompleted,
	}
	dp2 := &schema.DataPoint{
		ID:               "dp2",
		Content:          "input row",
		ContentType:      "text",
		Metadata:         map[string]interface{}{"is_input": true},
		SessionID:        "s1",
		UserID:           "u1",
		CreatedAt:        now.Add(1 * time.Second),
		UpdatedAt:        now.Add(1 * time.Second),
		ProcessingStatus: schema.StatusPending,
	}

	require.NoError(t, a.StoreDataPoint(ctx, dp1))
	require.NoError(t, a.StoreDataPoint(ctx, dp2))

	got, err := a.GetDataPoint(ctx, "dp1")
	require.NoError(t, err)
	require.Equal(t, "hello world", got.Content)

	dp1.Content = "hello updated"
	require.NoError(t, a.UpdateDataPoint(ctx, dp1))
	got, err = a.GetDataPoint(ctx, "dp1")
	require.NoError(t, err)
	require.Equal(t, "hello updated", got.Content)

	q := storage.DefaultDataPointQuery()
	q.SessionID = "s1"
	q.Limit = 10
	rows, err := a.QueryDataPoints(ctx, q)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	count, err := a.GetDataPointCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)

	require.NoError(t, a.DeleteDataPoint(ctx, "dp2"))
	_, err = a.GetDataPoint(ctx, "dp2")
	require.Error(t, err)
}

func TestSQLiteAdapter_SessionAndMessages(t *testing.T) {
	a := newTestSQLiteAdapter(t)
	ctx := context.Background()
	now := time.Now()

	ms := &schema.MemorySession{
		ID:         "sess1",
		UserID:     "u1",
		Context:    map[string]interface{}{"lang": "vi"},
		CreatedAt:  now,
		LastAccess: now,
		IsActive:   true,
	}
	require.NoError(t, a.StoreSession(ctx, ms))

	got, err := a.GetSession(ctx, "sess1")
	require.NoError(t, err)
	require.Equal(t, "u1", got.UserID)

	ms.IsActive = false
	require.NoError(t, a.UpdateSession(ctx, ms))

	sessions, err := a.ListSessions(ctx, "u1")
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	msg := schema.Message{
		ID:        "m1",
		Role:      schema.RoleUser,
		Content:   "hello",
		Timestamp: now,
	}
	require.NoError(t, a.AddMessageToSession(ctx, "sess1", msg))
	msgs, err := a.GetSessionMessages(ctx, "sess1")
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, "hello", msgs[0].Content)

	require.NoError(t, a.DeleteSessionMessages(ctx, "sess1"))
	msgs, err = a.GetSessionMessages(ctx, "sess1")
	require.NoError(t, err)
	require.Len(t, msgs, 0)

	sc, err := a.GetSessionCount(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, sc)

	require.NoError(t, a.DeleteSession(ctx, "sess1"))
	_, err = a.GetSession(ctx, "sess1")
	require.Error(t, err)
}

