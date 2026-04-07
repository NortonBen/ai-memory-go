package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

type fakeRelStore struct{}

func (f *fakeRelStore) StoreDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error { return nil }
func (f *fakeRelStore) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	return nil, nil
}
func (f *fakeRelStore) UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error { return nil }
func (f *fakeRelStore) DeleteDataPoint(ctx context.Context, id string) error                    { return nil }
func (f *fakeRelStore) DeleteDataPointsBySession(ctx context.Context, sessionID string) error   { return nil }
func (f *fakeRelStore) DeleteDataPointsUnscoped(ctx context.Context) error                      { return nil }
func (f *fakeRelStore) QueryDataPoints(ctx context.Context, query *DataPointQuery) ([]*schema.DataPoint, error) {
	return nil, nil
}
func (f *fakeRelStore) SearchDataPoints(ctx context.Context, searchQuery string, filters map[string]interface{}) ([]*schema.DataPoint, error) {
	return nil, nil
}
func (f *fakeRelStore) StoreSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (f *fakeRelStore) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) {
	return nil, nil
}
func (f *fakeRelStore) UpdateSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (f *fakeRelStore) DeleteSession(ctx context.Context, sessionID string) error               { return nil }
func (f *fakeRelStore) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) {
	return nil, nil
}
func (f *fakeRelStore) AddMessageToSession(ctx context.Context, sessionID string, message schema.Message) error {
	return nil
}
func (f *fakeRelStore) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	return nil, nil
}
func (f *fakeRelStore) DeleteSessionMessages(ctx context.Context, sessionID string) error { return nil }
func (f *fakeRelStore) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error {
	return nil
}
func (f *fakeRelStore) DeleteBatch(ctx context.Context, ids []string) error { return nil }
func (f *fakeRelStore) GetDataPointCount(ctx context.Context) (int64, error) {
	return 0, nil
}
func (f *fakeRelStore) GetSessionCount(ctx context.Context) (int64, error) { return 0, nil }
func (f *fakeRelStore) Health(ctx context.Context) error                    { return nil }
func (f *fakeRelStore) Close() error                                        { return nil }

func TestRelationalRegistry_NewRelationalStore(t *testing.T) {
	key := StorageType("unit_test_rel")
	RegisterRelationalStore(key, func(config *RelationalConfig) (RelationalStore, error) {
		return &fakeRelStore{}, nil
	})
	cfg := &RelationalConfig{Type: key}
	s, err := NewRelationalStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, s)

	types := GetRegisteredRelationalStores()
	require.Contains(t, types, key)
}

func TestNewRelationalStore_Unsupported(t *testing.T) {
	_, err := NewRelationalStore(&RelationalConfig{Type: StorageType("does_not_exist")})
	require.Error(t, err)
}

func TestStorageConfigManager_LoadAndValidatePlaceholders(t *testing.T) {
	m := NewStorageConfigManager()
	require.NotNil(t, m.GetConfig())
	require.NoError(t, m.LoadFromFile("dummy.yml"))
	require.NoError(t, m.LoadFromEnvironment())
	require.NoError(t, m.ValidateConfig())
}

func TestStorageHealthChecker_Basic(t *testing.T) {
	okStore := &fakeStorage{healthErr: nil}
	checker := NewStorageHealthChecker("s", okStore)
	require.Equal(t, "s", checker.Name())
	c := checker.Check(context.Background())
	require.Equal(t, HealthStatusHealthy, c.Status)

	badStore := &fakeStorage{healthErr: errors.New("down")}
	checker = NewStorageHealthChecker("s2", badStore)
	c = checker.Check(context.Background())
	require.Equal(t, HealthStatusUnhealthy, c.Status)
}

func TestDefaultDataPointQuery_AndNewSQLiteAdapterWrapper(t *testing.T) {
	q := DefaultDataPointQuery()
	require.Equal(t, 100, q.Limit)
	require.Equal(t, "created_at", q.SortBy)
	require.Equal(t, "desc", q.SortOrder)
	require.Equal(t, "fulltext", q.SearchMode)
	require.True(t, q.IncludeRelationships)

	key := StorageTypeSQLite
	RegisterRelationalStore(key, func(config *RelationalConfig) (RelationalStore, error) {
		return &fakeRelStore{}, nil
	})
	cfg := &RelationalConfig{}
	s, err := NewSQLiteAdapter(cfg)
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Equal(t, StorageTypeSQLite, cfg.Type)
}

type fakeStorage struct {
	healthErr error
}

func (f *fakeStorage) StoreDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error { return nil }
func (f *fakeStorage) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	return nil, nil
}
func (f *fakeStorage) UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error { return nil }
func (f *fakeStorage) DeleteDataPoint(ctx context.Context, id string) error                    { return nil }
func (f *fakeStorage) DeleteDataPointsBySession(ctx context.Context, sessionID string) error   { return nil }
func (f *fakeStorage) DeleteDataPointsUnscoped(ctx context.Context) error                      { return nil }
func (f *fakeStorage) QueryDataPoints(ctx context.Context, query *DataPointQuery) ([]*schema.DataPoint, error) {
	return nil, nil
}
func (f *fakeStorage) StoreSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (f *fakeStorage) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) {
	return nil, nil
}
func (f *fakeStorage) UpdateSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (f *fakeStorage) DeleteSession(ctx context.Context, sessionID string) error               { return nil }
func (f *fakeStorage) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) {
	return nil, nil
}
func (f *fakeStorage) AddMessageToSession(ctx context.Context, sessionID string, message schema.Message) error {
	return nil
}
func (f *fakeStorage) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	return nil, nil
}
func (f *fakeStorage) DeleteSessionMessages(ctx context.Context, sessionID string) error { return nil }
func (f *fakeStorage) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error {
	return nil
}
func (f *fakeStorage) DeleteBatch(ctx context.Context, ids []string) error { return nil }
func (f *fakeStorage) Health(ctx context.Context) error                     { return f.healthErr }
func (f *fakeStorage) Close() error                                         { return nil }

