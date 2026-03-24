// Package storage provides connection pooling for storage backends
package storage

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionPool defines the interface for connection pooling
type ConnectionPool interface {
	// Get a connection from the pool
	Get(ctx context.Context) (Connection, error)

	// Put a connection back to the pool
	Put(conn Connection) error

	// Close all connections in the pool
	Close() error

	// Stats returns pool statistics
	Stats() *PoolStats

	// Health check for the pool
	Health(ctx context.Context) error
}

// Connection represents a generic connection interface
type Connection interface {
	// Ping tests the connection
	Ping(ctx context.Context) error

	// Close closes the connection
	Close() error

	// IsValid checks if connection is still valid
	IsValid() bool

	// LastUsed returns when connection was last used
	LastUsed() time.Time

	// SetLastUsed updates the last used time
	SetLastUsed(t time.Time)
}

// PoolStats provides statistics about connection pool usage
type PoolStats struct {
	// Current number of connections in pool
	TotalConnections int32 `json:"total_connections"`

	// Number of active (in-use) connections
	ActiveConnections int32 `json:"active_connections"`

	// Number of idle connections
	IdleConnections int32 `json:"idle_connections"`

	// Number of connections created
	ConnectionsCreated int64 `json:"connections_created"`

	// Number of connections closed
	ConnectionsClosed int64 `json:"connections_closed"`

	// Number of failed connection attempts
	ConnectionsFailed int64 `json:"connections_failed"`

	// Average connection lifetime
	AvgConnectionLifetime time.Duration `json:"avg_connection_lifetime"`

	// Pool creation time
	CreatedAt time.Time `json:"created_at"`

	// Last statistics update
	LastUpdated time.Time `json:"last_updated"`
}

// PoolConfig defines configuration for connection pools
type PoolConfig struct {
	// Maximum number of connections in pool
	MaxConnections int `json:"max_connections"`

	// Minimum number of idle connections to maintain
	MinIdleConnections int `json:"min_idle_connections"`

	// Maximum lifetime of a connection
	MaxConnectionLifetime time.Duration `json:"max_connection_lifetime"`

	// Maximum time a connection can be idle
	MaxIdleTime time.Duration `json:"max_idle_time"`

	// Timeout for getting a connection from pool
	ConnectionTimeout time.Duration `json:"connection_timeout"`

	// Interval for cleaning up idle connections
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// Health check interval
	HealthCheckInterval time.Duration `json:"health_check_interval"`

	// Enable connection validation on get
	ValidateOnGet bool `json:"validate_on_get"`

	// Enable connection validation on put
	ValidateOnPut bool `json:"validate_on_put"`
}

// DefaultPoolConfig returns sensible defaults for connection pooling
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConnections:        20,
		MinIdleConnections:    2,
		MaxConnectionLifetime: 1 * time.Hour,
		MaxIdleTime:           10 * time.Minute,
		ConnectionTimeout:     30 * time.Second,
		CleanupInterval:       1 * time.Minute,
		HealthCheckInterval:   30 * time.Second,
		ValidateOnGet:         true,
		ValidateOnPut:         false,
	}
}

// ConnectionFactory creates new connections
type ConnectionFactory interface {
	CreateConnection(ctx context.Context) (Connection, error)
	ValidateConnection(ctx context.Context, conn Connection) error
}

// GenericConnectionPool implements ConnectionPool interface
type GenericConnectionPool struct {
	factory ConnectionFactory
	config  *PoolConfig

	// Pool state
	connections chan Connection
	active      map[Connection]time.Time
	stats       *PoolStats

	// Synchronization
	mu       sync.RWMutex
	closed   int32
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewGenericConnectionPool creates a new connection pool
func NewGenericConnectionPool(factory ConnectionFactory, config *PoolConfig) (*GenericConnectionPool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	if config.MaxConnections <= 0 {
		return nil, fmt.Errorf("max connections must be positive")
	}

	if config.MinIdleConnections < 0 {
		return nil, fmt.Errorf("min idle connections cannot be negative")
	}

	if config.MinIdleConnections > config.MaxConnections {
		return nil, fmt.Errorf("min idle connections cannot exceed max connections")
	}

	pool := &GenericConnectionPool{
		factory:     factory,
		config:      config,
		connections: make(chan Connection, config.MaxConnections),
		active:      make(map[Connection]time.Time),
		stats: &PoolStats{
			CreatedAt:   time.Now(),
			LastUpdated: time.Now(),
		},
		stopChan: make(chan struct{}),
	}

	// Start background maintenance
	pool.wg.Add(1)
	go pool.maintenanceLoop()

	// Pre-populate with minimum idle connections
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < config.MinIdleConnections; i++ {
		conn, err := pool.createConnection(ctx)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create initial connection: %w", err)
		}

		select {
		case pool.connections <- conn:
			atomic.AddInt32(&pool.stats.IdleConnections, 1)
		default:
			conn.Close()
		}
	}

	return pool, nil
}

// Get retrieves a connection from the pool
func (p *GenericConnectionPool) Get(ctx context.Context) (Connection, error) {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, fmt.Errorf("connection pool is closed")
	}

	// Try to get an existing connection first
	select {
	case conn := <-p.connections:
		atomic.AddInt32(&p.stats.IdleConnections, -1)

		// Validate connection if configured
		if p.config.ValidateOnGet {
			if err := p.validateConnection(ctx, conn); err != nil {
				conn.Close()
				atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
				// Try to create a new connection
				return p.createAndTrackConnection(ctx)
			}
		}

		// Track as active
		p.mu.Lock()
		p.active[conn] = time.Now()
		atomic.AddInt32(&p.stats.ActiveConnections, 1)
		p.mu.Unlock()

		conn.SetLastUsed(time.Now())
		return conn, nil

	default:
		// No idle connections available, try to create new one
		return p.createAndTrackConnection(ctx)
	}
}

// Put returns a connection to the pool
func (p *GenericConnectionPool) Put(conn Connection) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		conn.Close()
		return fmt.Errorf("connection pool is closed")
	}

	if conn == nil {
		return fmt.Errorf("connection cannot be nil")
	}

	// Remove from active tracking
	p.mu.Lock()
	delete(p.active, conn)
	atomic.AddInt32(&p.stats.ActiveConnections, -1)
	p.mu.Unlock()

	// Validate connection if configured
	if p.config.ValidateOnPut {
		if err := p.validateConnection(context.Background(), conn); err != nil {
			conn.Close()
			atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
			return nil // Don't return error for invalid connections
		}
	}

	// Check if connection is still valid and not expired
	if !conn.IsValid() || p.isConnectionExpired(conn) {
		conn.Close()
		atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
		return nil
	}

	// Try to put back in pool
	select {
	case p.connections <- conn:
		atomic.AddInt32(&p.stats.IdleConnections, 1)
		return nil
	default:
		// Pool is full, close the connection
		conn.Close()
		atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
		return nil
	}
}

// Close closes all connections and shuts down the pool
func (p *GenericConnectionPool) Close() error {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return nil // Already closed
	}

	// Stop maintenance loop
	close(p.stopChan)
	p.wg.Wait()

	// Close all idle connections
	close(p.connections)
	for conn := range p.connections {
		conn.Close()
		atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
	}

	// Close all active connections
	p.mu.Lock()
	for conn := range p.active {
		conn.Close()
		atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
	}
	p.active = make(map[Connection]time.Time)
	p.mu.Unlock()

	// Reset counters
	atomic.StoreInt32(&p.stats.ActiveConnections, 0)
	atomic.StoreInt32(&p.stats.IdleConnections, 0)

	return nil
}

// Stats returns current pool statistics
func (p *GenericConnectionPool) Stats() *PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := *p.stats
	stats.TotalConnections = atomic.LoadInt32(&p.stats.TotalConnections)
	stats.ActiveConnections = atomic.LoadInt32(&p.stats.ActiveConnections)
	stats.IdleConnections = atomic.LoadInt32(&p.stats.IdleConnections)
	stats.ConnectionsCreated = atomic.LoadInt64(&p.stats.ConnectionsCreated)
	stats.ConnectionsClosed = atomic.LoadInt64(&p.stats.ConnectionsClosed)
	stats.ConnectionsFailed = atomic.LoadInt64(&p.stats.ConnectionsFailed)
	stats.LastUpdated = time.Now()

	return &stats
}

// Health performs a health check on the pool
func (p *GenericConnectionPool) Health(ctx context.Context) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		return fmt.Errorf("connection pool is closed")
	}

	// Try to get a connection to test pool health
	conn, err := p.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection for health check: %w", err)
	}
	defer p.Put(conn)

	// Test the connection
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("connection health check failed: %w", err)
	}

	return nil
}

// Private helper methods

func (p *GenericConnectionPool) createConnection(ctx context.Context) (Connection, error) {
	conn, err := p.factory.CreateConnection(ctx)
	if err != nil {
		atomic.AddInt64(&p.stats.ConnectionsFailed, 1)
		return nil, err
	}

	atomic.AddInt64(&p.stats.ConnectionsCreated, 1)
	atomic.AddInt32(&p.stats.TotalConnections, 1)

	return conn, nil
}

func (p *GenericConnectionPool) createAndTrackConnection(ctx context.Context) (Connection, error) {
	// Check if we can create more connections
	if atomic.LoadInt32(&p.stats.TotalConnections) >= int32(p.config.MaxConnections) {
		// Wait for a connection to become available
		select {
		case conn := <-p.connections:
			atomic.AddInt32(&p.stats.IdleConnections, -1)

			if p.config.ValidateOnGet {
				if err := p.validateConnection(ctx, conn); err != nil {
					conn.Close()
					atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
					atomic.AddInt32(&p.stats.TotalConnections, -1)
					return p.createAndTrackConnection(ctx) // Retry
				}
			}

			p.mu.Lock()
			p.active[conn] = time.Now()
			atomic.AddInt32(&p.stats.ActiveConnections, 1)
			p.mu.Unlock()

			conn.SetLastUsed(time.Now())
			return conn, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Create new connection
	conn, err := p.createConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Track as active
	p.mu.Lock()
	p.active[conn] = time.Now()
	atomic.AddInt32(&p.stats.ActiveConnections, 1)
	p.mu.Unlock()

	conn.SetLastUsed(time.Now())
	return conn, nil
}

func (p *GenericConnectionPool) validateConnection(ctx context.Context, conn Connection) error {
	if !conn.IsValid() {
		return fmt.Errorf("connection is not valid")
	}

	if p.isConnectionExpired(conn) {
		return fmt.Errorf("connection is expired")
	}

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("connection ping failed: %w", err)
	}

	return nil
}

func (p *GenericConnectionPool) isConnectionExpired(conn Connection) bool {
	if p.config.MaxConnectionLifetime <= 0 {
		return false
	}

	return time.Since(conn.LastUsed()) > p.config.MaxConnectionLifetime
}

func (p *GenericConnectionPool) maintenanceLoop() {
	defer p.wg.Done()

	cleanupTicker := time.NewTicker(p.config.CleanupInterval)
	defer cleanupTicker.Stop()

	healthTicker := time.NewTicker(p.config.HealthCheckInterval)
	defer healthTicker.Stop()

	for {
		select {
		case <-p.stopChan:
			return

		case <-cleanupTicker.C:
			p.cleanupIdleConnections()

		case <-healthTicker.C:
			p.performHealthCheck()
		}
	}
}

func (p *GenericConnectionPool) cleanupIdleConnections() {
	// Clean up expired idle connections
	var connectionsToClose []Connection

	// Collect expired connections
loop:
	for {
		select {
		case conn := <-p.connections:
			if p.isConnectionExpired(conn) || !conn.IsValid() {
				connectionsToClose = append(connectionsToClose, conn)
				atomic.AddInt32(&p.stats.IdleConnections, -1)
			} else {
				// Put valid connection back
				select {
				case p.connections <- conn:
					// Successfully put back
				default:
					// Pool is full, close this connection
					connectionsToClose = append(connectionsToClose, conn)
					atomic.AddInt32(&p.stats.IdleConnections, -1)
				}
				break loop // Stop collecting once we find a valid connection
			}
		default:
			// No more idle connections
			break loop
		}
	}

	// Close expired connections
	for _, conn := range connectionsToClose {
		conn.Close()
		atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
		atomic.AddInt32(&p.stats.TotalConnections, -1)
	}

	// Ensure minimum idle connections
	p.ensureMinimumIdleConnections()
}

func (p *GenericConnectionPool) ensureMinimumIdleConnections() {
	currentIdle := atomic.LoadInt32(&p.stats.IdleConnections)
	needed := int32(p.config.MinIdleConnections) - currentIdle

	if needed <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := int32(0); i < needed; i++ {
		if atomic.LoadInt32(&p.stats.TotalConnections) >= int32(p.config.MaxConnections) {
			break
		}

		conn, err := p.createConnection(ctx)
		if err != nil {
			break // Stop trying if we can't create connections
		}

		select {
		case p.connections <- conn:
			atomic.AddInt32(&p.stats.IdleConnections, 1)
		default:
			conn.Close()
			atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
			atomic.AddInt32(&p.stats.TotalConnections, -1)
			break
		}
	}
}

func (p *GenericConnectionPool) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check a sample of idle connections
	var connectionsToTest []Connection
	var connectionsToReturn []Connection

	// Get up to 3 connections for testing
	for i := 0; i < 3; i++ {
		select {
		case conn := <-p.connections:
			connectionsToTest = append(connectionsToTest, conn)
			atomic.AddInt32(&p.stats.IdleConnections, -1)
		default:
			break
		}
	}

	// Test connections
	for _, conn := range connectionsToTest {
		if err := p.validateConnection(ctx, conn); err != nil {
			conn.Close()
			atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
			atomic.AddInt32(&p.stats.TotalConnections, -1)
		} else {
			connectionsToReturn = append(connectionsToReturn, conn)
		}
	}

	// Return valid connections to pool
	for _, conn := range connectionsToReturn {
		select {
		case p.connections <- conn:
			atomic.AddInt32(&p.stats.IdleConnections, 1)
		default:
			conn.Close()
			atomic.AddInt64(&p.stats.ConnectionsClosed, 1)
			atomic.AddInt32(&p.stats.TotalConnections, -1)
		}
	}
}
