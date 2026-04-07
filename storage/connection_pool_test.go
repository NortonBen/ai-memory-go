package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

type mockConn struct {
	valid    bool
	lastUsed time.Time
	pingErr  error
	closed   bool
}

func (m *mockConn) Ping(ctx context.Context) error { return m.pingErr }
func (m *mockConn) Close() error                   { m.closed = true; return nil }
func (m *mockConn) IsValid() bool                  { return m.valid && !m.closed }
func (m *mockConn) LastUsed() time.Time            { return m.lastUsed }
func (m *mockConn) SetLastUsed(t time.Time)        { m.lastUsed = t }

type mockFactory struct {
	createErr error
}

func (m *mockFactory) CreateConnection(ctx context.Context) (schema.Connection, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &mockConn{valid: true, lastUsed: time.Now()}, nil
}
func (m *mockFactory) ValidateConnection(ctx context.Context, conn schema.Connection) error {
	return nil
}

func TestNewGenericConnectionPool_ConfigValidation(t *testing.T) {
	_, err := NewGenericConnectionPool(&mockFactory{}, &PoolConfig{MaxConnections: 0})
	require.Error(t, err)
	_, err = NewGenericConnectionPool(&mockFactory{}, &PoolConfig{MaxConnections: 1, MinIdleConnections: -1})
	require.Error(t, err)
	_, err = NewGenericConnectionPool(&mockFactory{}, &PoolConfig{MaxConnections: 1, MinIdleConnections: 2})
	require.Error(t, err)
}

func TestGenericConnectionPool_GetPutHealthClose(t *testing.T) {
	p, err := NewGenericConnectionPool(&mockFactory{}, &PoolConfig{
		MaxConnections:        2,
		MinIdleConnections:    1,
		MaxConnectionLifetime: time.Minute,
		ConnectionTimeout:     200 * time.Millisecond,
		CleanupInterval:       time.Hour,
		HealthCheckInterval:   time.Hour,
		ValidateOnGet:         true,
		ValidateOnPut:         true,
	})
	require.NoError(t, err)
	defer p.Close()

	conn, err := p.Get(context.Background())
	require.NoError(t, err)
	require.NoError(t, p.Put(conn))
	require.NoError(t, p.Health(context.Background()))

	stats := p.Stats()
	require.GreaterOrEqual(t, stats.TotalConnections, int32(1))

	require.NoError(t, p.Close())
	_, err = p.Get(context.Background())
	require.Error(t, err)
}

func TestGenericConnectionPool_TimeoutWhenExhausted(t *testing.T) {
	p, err := NewGenericConnectionPool(&mockFactory{}, &PoolConfig{
		MaxConnections:      1,
		MinIdleConnections:  1,
		ConnectionTimeout:   50 * time.Millisecond,
		CleanupInterval:     time.Hour,
		HealthCheckInterval: time.Hour,
	})
	require.NoError(t, err)
	defer p.Close()

	conn, err := p.Get(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = p.Get(ctx)
	require.Error(t, err)

	require.NoError(t, p.Put(conn))
}

func TestGenericConnectionPool_FactoryCreateErrorAndNilPut(t *testing.T) {
	p, err := NewGenericConnectionPool(&mockFactory{createErr: errors.New("boom")}, &PoolConfig{
		MaxConnections:      1,
		MinIdleConnections:  0,
		CleanupInterval:     time.Hour,
		HealthCheckInterval: time.Hour,
	})
	require.NoError(t, err)
	defer p.Close()

	_, err = p.Get(context.Background())
	require.Error(t, err)
	require.Error(t, p.Put(nil))
}

