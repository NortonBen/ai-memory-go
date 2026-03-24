package storage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Mock connection for testing
type mockConnection struct {
	id       int
	valid    bool
	lastUsed time.Time
	mu       sync.RWMutex
	pingErr  error
	closeErr error
}

func newMockConnection(id int) *mockConnection {
	return &mockConnection{
		id:       id,
		valid:    true,
		lastUsed: time.Now(),
	}
}

func (c *mockConnection) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pingErr
}

func (c *mockConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.valid = false
	return c.closeErr
}

func (c *mockConnection) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.valid
}

func (c *mockConnection) LastUsed() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUsed
}

func (c *mockConnection) SetLastUsed(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUsed = t
}

func (c *mockConnection) SetPingError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pingErr = err
}

func (c *mockConnection) SetCloseError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeErr = err
}

func (c *mockConnection) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.valid = false
}

// Mock connection factory for testing
type mockConnectionFactory struct {
	connectionCounter int32
	createErr         error
	validateErr       error
	mu                sync.Mutex
}

func newMockConnectionFactory() *mockConnectionFactory {
	return &mockConnectionFactory{}
}

func (f *mockConnectionFactory) CreateConnection(ctx context.Context) (Connection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return nil, f.createErr
	}

	id := int(atomic.AddInt32(&f.connectionCounter, 1))
	return newMockConnection(id), nil
}

func (f *mockConnectionFactory) ValidateConnection(ctx context.Context, conn Connection) error {
	return f.validateErr
}

func (f *mockConnectionFactory) SetCreateError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createErr = err
}

func (f *mockConnectionFactory) SetValidateError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.validateErr = err
}

func (f *mockConnectionFactory) GetConnectionCount() int32 {
	return atomic.LoadInt32(&f.connectionCounter)
}

func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()

	if config.MaxConnections != 20 {
		t.Errorf("Expected MaxConnections 20, got %d", config.MaxConnections)
	}

	if config.MinIdleConnections != 2 {
		t.Errorf("Expected MinIdleConnections 2, got %d", config.MinIdleConnections)
	}

	if config.MaxConnectionLifetime != 1*time.Hour {
		t.Errorf("Expected MaxConnectionLifetime 1h, got %v", config.MaxConnectionLifetime)
	}

	if !config.ValidateOnGet {
		t.Error("Expected ValidateOnGet to be true")
	}
}

func TestNewGenericConnectionPool(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 2

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Check that minimum idle connections were created
	stats := pool.Stats()
	if stats.IdleConnections != 2 {
		t.Errorf("Expected 2 idle connections, got %d", stats.IdleConnections)
	}

	if factory.GetConnectionCount() != 2 {
		t.Errorf("Expected 2 connections created, got %d", factory.GetConnectionCount())
	}
}

func TestNewGenericConnectionPoolInvalidConfig(t *testing.T) {
	factory := newMockConnectionFactory()

	// Test with nil config
	pool, err := NewGenericConnectionPool(factory, nil)
	if err != nil {
		t.Fatalf("Should accept nil config and use defaults: %v", err)
	}
	pool.Close()

	// Test with invalid max connections
	config := DefaultPoolConfig()
	config.MaxConnections = 0

	_, err = NewGenericConnectionPool(factory, config)
	if err == nil {
		t.Error("Expected error for zero max connections")
	}

	// Test with invalid min idle connections
	config = DefaultPoolConfig()
	config.MinIdleConnections = -1

	_, err = NewGenericConnectionPool(factory, config)
	if err == nil {
		t.Error("Expected error for negative min idle connections")
	}

	// Test with min > max
	config = DefaultPoolConfig()
	config.MinIdleConnections = 30
	config.MaxConnections = 20

	_, err = NewGenericConnectionPool(factory, config)
	if err == nil {
		t.Error("Expected error for min idle > max connections")
	}
}

func TestConnectionPoolGetPut(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 1
	config.MaxConnections = 5

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Get a connection
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	// Check stats
	stats := pool.Stats()
	if stats.ActiveConnections != 1 {
		t.Errorf("Expected 1 active connection, got %d", stats.ActiveConnections)
	}

	// Put connection back
	err = pool.Put(conn)
	if err != nil {
		t.Fatalf("Failed to put connection: %v", err)
	}

	// Check stats
	stats = pool.Stats()
	if stats.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections, got %d", stats.ActiveConnections)
	}

	if stats.IdleConnections != 1 {
		t.Errorf("Expected 1 idle connection, got %d", stats.IdleConnections)
	}
}

func TestConnectionPoolMaxConnections(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 0
	config.MaxConnections = 2

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Get maximum connections
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get first connection: %v", err)
	}

	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get second connection: %v", err)
	}

	// Try to get one more (should block or create new if under limit)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = pool.Get(ctx)
	if err == nil {
		t.Error("Expected timeout when exceeding max connections")
	}

	// Put one back and try again
	pool.Put(conn1)

	ctx = context.Background()
	conn3, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection after putting one back: %v", err)
	}

	pool.Put(conn2)
	pool.Put(conn3)
}

func TestConnectionPoolValidation(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 1
	config.ValidateOnGet = true
	config.ValidateOnPut = true

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Get a connection
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	mockConn := conn.(*mockConnection)

	// Invalidate the connection
	mockConn.Invalidate()

	// Put it back - should be rejected due to validation
	err = pool.Put(conn)
	if err != nil {
		t.Fatalf("Put should not return error for invalid connection: %v", err)
	}

	// The invalid connection should have been closed, not returned to pool
	stats := pool.Stats()
	if stats.IdleConnections > 0 {
		t.Error("Invalid connection should not be returned to pool")
	}
}

func TestConnectionPoolHealth(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 1

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Health check should pass
	err = pool.Health(ctx)
	if err != nil {
		t.Errorf("Health check should pass: %v", err)
	}

	// Close the pool and check health
	pool.Close()

	err = pool.Health(ctx)
	if err == nil {
		t.Error("Health check should fail on closed pool")
	}
}

func TestConnectionPoolConcurrency(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 2
	config.MaxConnections = 10

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Test concurrent access
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := pool.Get(ctx)
			if err != nil {
				errors <- err
				return
			}

			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			err = pool.Put(conn)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}

	// Check final stats
	stats := pool.Stats()
	if stats.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections, got %d", stats.ActiveConnections)
	}
}

func TestConnectionPoolCleanup(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 2
	config.MaxConnectionLifetime = 50 * time.Millisecond
	config.CleanupInterval = 25 * time.Millisecond

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Wait for connections to expire and be cleaned up
	time.Sleep(100 * time.Millisecond)

	// New connections should be created to maintain minimum
	stats := pool.Stats()
	if stats.IdleConnections < 2 {
		t.Errorf("Expected at least 2 idle connections after cleanup, got %d", stats.IdleConnections)
	}
}

func TestConnectionPoolStats(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 2

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	stats := pool.Stats()

	if stats.TotalConnections != 2 {
		t.Errorf("Expected 2 total connections, got %d", stats.TotalConnections)
	}

	if stats.IdleConnections != 2 {
		t.Errorf("Expected 2 idle connections, got %d", stats.IdleConnections)
	}

	if stats.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections, got %d", stats.ActiveConnections)
	}

	if stats.ConnectionsCreated != 2 {
		t.Errorf("Expected 2 connections created, got %d", stats.ConnectionsCreated)
	}

	if stats.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	if stats.LastUpdated.IsZero() {
		t.Error("LastUpdated should be set")
	}
}

func TestConnectionPoolFactoryError(t *testing.T) {
	factory := newMockConnectionFactory()
	factory.SetCreateError(errors.New("connection failed"))

	config := DefaultPoolConfig()
	config.MinIdleConnections = 1

	// Should fail to create pool due to factory error
	_, err := NewGenericConnectionPool(factory, config)
	if err == nil {
		t.Error("Expected error when factory fails to create connections")
	}
}

func TestConnectionPoolClosedOperations(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 1

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	// Close the pool
	pool.Close()

	ctx := context.Background()

	// Operations on closed pool should fail
	_, err = pool.Get(ctx)
	if err == nil {
		t.Error("Get should fail on closed pool")
	}

	conn := newMockConnection(1)
	err = pool.Put(conn)
	if err == nil {
		t.Error("Put should fail on closed pool")
	}

	err = pool.Health(ctx)
	if err == nil {
		t.Error("Health should fail on closed pool")
	}
}

func TestConnectionExpiration(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 1
	config.MaxConnectionLifetime = 50 * time.Millisecond

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Get a connection
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	// Wait for it to expire
	time.Sleep(60 * time.Millisecond)

	// Put it back - should be rejected due to expiration
	err = pool.Put(conn)
	if err != nil {
		t.Fatalf("Put should not return error for expired connection: %v", err)
	}

	// The expired connection should have been closed, not returned to pool
	stats := pool.Stats()
	if stats.IdleConnections > 1 {
		t.Error("Expired connection should not be returned to pool")
	}
}
