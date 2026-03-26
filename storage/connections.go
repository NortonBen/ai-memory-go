// Package storage provides specific connection implementations for different storage backends
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	_ "github.com/NortonBen/ai-memory-go/graph/adapters/inmemory"
	_ "github.com/NortonBen/ai-memory-go/graph/adapters/sqlite"
	_ "github.com/NortonBen/ai-memory-go/vector/adapters/inmemory"
	_ "github.com/NortonBen/ai-memory-go/vector/adapters/sqlite"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/vector"
)

// mockVectorStore is a simple mock implementation for testing
type mockVectorStore struct{}

func (m *mockVectorStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	return nil
}

func (m *mockVectorStore) GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error) {
	return nil, nil, nil
}

func (m *mockVectorStore) UpdateEmbedding(ctx context.Context, id string, embedding []float32) error {
	return nil
}

func (m *mockVectorStore) DeleteEmbedding(ctx context.Context, id string) error {
	return nil
}

func (m *mockVectorStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	return nil, nil
}

func (m *mockVectorStore) SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	return nil, nil
}

func (m *mockVectorStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*vector.EmbeddingData) error {
	return nil
}

func (m *mockVectorStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockVectorStore) CreateCollection(ctx context.Context, name string, dimension int, config *vector.CollectionConfig) error {
	return nil
}

func (m *mockVectorStore) DeleteCollection(ctx context.Context, name string) error {
	return nil
}

func (m *mockVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockVectorStore) GetCollectionInfo(ctx context.Context, name string) (*vector.CollectionInfo, error) {
	return nil, nil
}

func (m *mockVectorStore) GetEmbeddingCount(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockVectorStore) Health(ctx context.Context) error {
	return nil
}

func (m *mockVectorStore) Close() error {
	return nil
}

// PostgreSQLConnection implements schema.Connection for PostgreSQL
type PostgreSQLConnection struct {
	db       *sql.DB
	lastUsed time.Time
	mu       sync.RWMutex
	valid    bool
}

// NewPostgreSQLConnection creates a new PostgreSQL connection
func NewPostgreSQLConnection(db *sql.DB) *PostgreSQLConnection {
	return &PostgreSQLConnection{
		db:       db,
		lastUsed: time.Now(),
		valid:    true,
	}
}

// Ping tests the PostgreSQL connection
func (c *PostgreSQLConnection) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Close closes the PostgreSQL connection
func (c *PostgreSQLConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.valid = false
	return c.db.Close()
}

// IsValid checks if the PostgreSQL connection is still valid
func (c *PostgreSQLConnection) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.valid
}

// LastUsed returns when the connection was last used
func (c *PostgreSQLConnection) LastUsed() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastUsed
}

// SetLastUsed updates the last used time
func (c *PostgreSQLConnection) SetLastUsed(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastUsed = t
}

// GetDB returns the underlying database connection
func (c *PostgreSQLConnection) GetDB() *sql.DB {
	return c.db
}

// SQLiteConnection implements schema.Connection for SQLite
type SQLiteConnection struct {
	db       *sql.DB
	lastUsed time.Time
	mu       sync.RWMutex
	valid    bool
}

// NewSQLiteConnection creates a new SQLite connection
func NewSQLiteConnection(db *sql.DB) *SQLiteConnection {
	return &SQLiteConnection{
		db:       db,
		lastUsed: time.Now(),
		valid:    true,
	}
}

// Ping tests the SQLite connection
func (c *SQLiteConnection) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Close closes the SQLite connection
func (c *SQLiteConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.valid = false
	return c.db.Close()
}

// IsValid checks if the SQLite connection is still valid
func (c *SQLiteConnection) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.valid
}

// LastUsed returns when the connection was last used
func (c *SQLiteConnection) LastUsed() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastUsed
}

// SetLastUsed updates the last used time
func (c *SQLiteConnection) SetLastUsed(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastUsed = t
}

// GetDB returns the underlying database connection
func (c *SQLiteConnection) GetDB() *sql.DB {
	return c.db
}

// GraphConnection wraps graph store connections
type GraphConnection struct {
	store    graph.GraphStore
	lastUsed time.Time
	mu       sync.RWMutex
	valid    bool
}

// NewGraphConnection creates a new graph connection
func NewGraphConnection(store graph.GraphStore) *GraphConnection {
	return &GraphConnection{
		store:    store,
		lastUsed: time.Now(),
		valid:    true,
	}
}

// Ping tests the graph connection
func (c *GraphConnection) Ping(ctx context.Context) error {
	return c.store.Health(ctx)
}

// Close closes the graph connection
func (c *GraphConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.valid = false
	return c.store.Close()
}

// IsValid checks if the graph connection is still valid
func (c *GraphConnection) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.valid
}

// LastUsed returns when the connection was last used
func (c *GraphConnection) LastUsed() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastUsed
}

// SetLastUsed updates the last used time
func (c *GraphConnection) SetLastUsed(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastUsed = t
}

// GetStore returns the underlying graph store
func (c *GraphConnection) GetStore() graph.GraphStore {
	return c.store
}

// VectorConnection wraps vector store connections
type VectorConnection struct {
	store    vector.VectorStore
	lastUsed time.Time
	mu       sync.RWMutex
	valid    bool
}

// NewVectorConnection creates a new vector connection
func NewVectorConnection(store vector.VectorStore) *VectorConnection {
	return &VectorConnection{
		store:    store,
		lastUsed: time.Now(),
		valid:    true,
	}
}

// Ping tests the vector connection
func (c *VectorConnection) Ping(ctx context.Context) error {
	return c.store.Health(ctx)
}

// Close closes the vector connection
func (c *VectorConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.valid = false
	return c.store.Close()
}

// IsValid checks if the vector connection is still valid
func (c *VectorConnection) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.valid
}

// LastUsed returns when the connection was last used
func (c *VectorConnection) LastUsed() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastUsed
}

// SetLastUsed updates the last used time
func (c *VectorConnection) SetLastUsed(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastUsed = t
}

// GetStore returns the underlying vector store
func (c *VectorConnection) GetStore() vector.VectorStore {
	return c.store
}

// schema.Connection factory implementations

// PostgreSQLConnectionFactory creates PostgreSQL connections
type PostgreSQLConnectionFactory struct {
	config *RelationalConfig
}

// NewPostgreSQLConnectionFactory creates a new PostgreSQL connection factory
func NewPostgreSQLConnectionFactory(config *RelationalConfig) *PostgreSQLConnectionFactory {
	return &PostgreSQLConnectionFactory{
		config: config,
	}
}

// CreateConnection creates a new PostgreSQL connection
func (f *PostgreSQLConnectionFactory) CreateConnection(ctx context.Context) (schema.Connection, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		f.config.Host, f.config.Port, f.config.Username, f.config.Password, f.config.Database, f.config.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(f.config.MaxConnections)
	db.SetMaxIdleConns(f.config.MinConnections)
	db.SetConnMaxLifetime(f.config.MaxLifetime)
	db.SetConnMaxIdleTime(f.config.IdleTimeout)

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return NewPostgreSQLConnection(db), nil
}

// ValidateConnection validates a PostgreSQL connection
func (f *PostgreSQLConnectionFactory) ValidateConnection(ctx context.Context, conn schema.Connection) error {
	pgConn, ok := conn.(*PostgreSQLConnection)
	if !ok {
		return fmt.Errorf("invalid connection type for PostgreSQL")
	}

	return pgConn.Ping(ctx)
}

// SQLiteConnectionFactory creates SQLite connections
type SQLiteConnectionFactory struct {
	config *RelationalConfig
}

// NewSQLiteConnectionFactory creates a new SQLite connection factory
func NewSQLiteConnectionFactory(config *RelationalConfig) *SQLiteConnectionFactory {
	return &SQLiteConnectionFactory{
		config: config,
	}
}

// CreateConnection creates a new SQLite connection
func (f *SQLiteConnectionFactory) CreateConnection(ctx context.Context) (schema.Connection, error) {
	db, err := sql.Open("sqlite3", f.config.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite connection: %w", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(f.config.MaxConnections)
	db.SetMaxIdleConns(f.config.MinConnections)
	db.SetConnMaxLifetime(f.config.MaxLifetime)
	db.SetConnMaxIdleTime(f.config.IdleTimeout)

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping SQLite: %w", err)
	}

	return NewSQLiteConnection(db), nil
}

// ValidateConnection validates a SQLite connection
func (f *SQLiteConnectionFactory) ValidateConnection(ctx context.Context, conn schema.Connection) error {
	sqliteConn, ok := conn.(*SQLiteConnection)
	if !ok {
		return fmt.Errorf("invalid connection type for SQLite")
	}

	return sqliteConn.Ping(ctx)
}

// GraphConnectionFactory creates graph connections
type GraphConnectionFactory struct {
	config *graph.GraphConfig
}

// NewGraphConnectionFactory creates a new graph connection factory
func NewGraphConnectionFactory(config *graph.GraphConfig) *GraphConnectionFactory {
	return &GraphConnectionFactory{
		config: config,
	}
}

// CreateConnection creates a new graph connection
func (f *GraphConnectionFactory) CreateConnection(ctx context.Context) (schema.Connection, error) {
	store, err := graph.NewStore(f.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph store: %w", err)
	}

	return NewGraphConnection(store), nil
}

// ValidateConnection validates a graph connection
func (f *GraphConnectionFactory) ValidateConnection(ctx context.Context, conn schema.Connection) error {
	graphConn, ok := conn.(*GraphConnection)
	if !ok {
		return fmt.Errorf("invalid connection type for graph store")
	}

	return graphConn.Ping(ctx)
}

// VectorConnectionFactory creates vector connections
type VectorConnectionFactory struct {
	config *vector.VectorConfig
}

// NewVectorConnectionFactory creates a new vector connection factory
func NewVectorConnectionFactory(config *vector.VectorConfig) *VectorConnectionFactory {
	return &VectorConnectionFactory{
		config: config,
	}
}

// CreateConnection creates a new vector connection
func (f *VectorConnectionFactory) CreateConnection(ctx context.Context) (schema.Connection, error) {
	store, err := vector.NewVectorStore(f.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	return NewVectorConnection(store), nil
}

// ValidateConnection validates a vector connection
func (f *VectorConnectionFactory) ValidateConnection(ctx context.Context, conn schema.Connection) error {
	vectorConn, ok := conn.(*VectorConnection)
	if !ok {
		return fmt.Errorf("invalid connection type for vector store")
	}

	return vectorConn.Ping(ctx)
}

// PooledStorageManager manages connection pools for different storage types
type PooledStorageManager struct {
	relationalPool ConnectionPool
	graphPool      ConnectionPool
	vectorPool     ConnectionPool
	healthMonitor  *HealthMonitor
	config         *StorageConfig
	mu             sync.RWMutex
	closed         bool
}

// NewPooledStorageManager creates a new pooled storage manager
func NewPooledStorageManager(config *StorageConfig) (*PooledStorageManager, error) {
	manager := &PooledStorageManager{
		config:        config,
		healthMonitor: NewHealthMonitor(DefaultHealthMonitorConfig()),
	}

	// Create connection pools for each storage type
	if err := manager.initializePools(); err != nil {
		return nil, fmt.Errorf("failed to initialize connection pools: %w", err)
	}

	// Set up health monitoring
	if err := manager.setupHealthMonitoring(); err != nil {
		return nil, fmt.Errorf("failed to setup health monitoring: %w", err)
	}

	return manager, nil
}

// GetRelationalConnection gets a connection from the relational pool
func (m *PooledStorageManager) GetRelationalConnection(ctx context.Context) (schema.Connection, error) {
	if m.relationalPool == nil {
		return nil, fmt.Errorf("relational connection pool not initialized")
	}

	return m.relationalPool.Get(ctx)
}

// PutRelationalConnection returns a connection to the relational pool
func (m *PooledStorageManager) PutRelationalConnection(conn schema.Connection) error {
	if m.relationalPool == nil {
		return fmt.Errorf("relational connection pool not initialized")
	}

	return m.relationalPool.Put(conn)
}

// GetGraphConnection gets a connection from the graph pool
func (m *PooledStorageManager) GetGraphConnection(ctx context.Context) (schema.Connection, error) {
	if m.graphPool == nil {
		return nil, fmt.Errorf("graph connection pool not initialized")
	}

	return m.graphPool.Get(ctx)
}

// PutGraphConnection returns a connection to the graph pool
func (m *PooledStorageManager) PutGraphConnection(conn schema.Connection) error {
	if m.graphPool == nil {
		return fmt.Errorf("graph connection pool not initialized")
	}

	return m.graphPool.Put(conn)
}

// GetVectorConnection gets a connection from the vector pool
func (m *PooledStorageManager) GetVectorConnection(ctx context.Context) (schema.Connection, error) {
	if m.vectorPool == nil {
		return nil, fmt.Errorf("vector connection pool not initialized")
	}

	return m.vectorPool.Get(ctx)
}

// PutVectorConnection returns a connection to the vector pool
func (m *PooledStorageManager) PutVectorConnection(conn schema.Connection) error {
	if m.vectorPool == nil {
		return fmt.Errorf("vector connection pool not initialized")
	}

	return m.vectorPool.Put(conn)
}

// GetHealthReport returns the current health report
func (m *PooledStorageManager) GetHealthReport(ctx context.Context) *HealthReport {
	return m.healthMonitor.CheckAll(ctx)
}

// IsHealthy returns true if all storage backends are healthy
func (m *PooledStorageManager) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return false
	}

	return m.healthMonitor.IsHealthy()
}

// Close closes all connection pools and stops health monitoring
func (m *PooledStorageManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	var errors []error

	// Stop health monitoring
	if err := m.healthMonitor.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop health monitor: %w", err))
	}

	// Close connection pools
	if m.relationalPool != nil {
		if err := m.relationalPool.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close relational pool: %w", err))
		}
	}

	if m.graphPool != nil {
		if err := m.graphPool.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close graph pool: %w", err))
		}
	}

	if m.vectorPool != nil {
		if err := m.vectorPool.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close vector pool: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors during close: %v", errors)
	}

	return nil
}

// Private helper methods

func (m *PooledStorageManager) initializePools() error {
	// Initialize relational pool
	if m.config.Relational != nil {
		poolConfig := &PoolConfig{
			MaxConnections:        m.config.Relational.MaxConnections,
			MinIdleConnections:    m.config.Relational.MinConnections,
			MaxConnectionLifetime: m.config.Relational.MaxLifetime,
			MaxIdleTime:           m.config.Relational.IdleTimeout,
			ConnectionTimeout:     m.config.Relational.ConnTimeout,
			CleanupInterval:       1 * time.Minute,
			HealthCheckInterval:   30 * time.Second,
			ValidateOnGet:         true,
		}

		var factory schema.ConnectionFactory
		switch m.config.Relational.Type {
		case StorageTypePostgreSQL:
			factory = NewPostgreSQLConnectionFactory(m.config.Relational)
		case StorageTypeSQLite:
			factory = NewSQLiteConnectionFactory(m.config.Relational)
		default:
			return fmt.Errorf("unsupported relational storage type: %s", m.config.Relational.Type)
		}

		pool, err := NewGenericConnectionPool(factory, poolConfig)
		if err != nil {
			return fmt.Errorf("failed to create relational connection pool: %w", err)
		}

		m.relationalPool = pool
	}

	// Initialize graph pool
	if m.config.Graph != nil {
		poolConfig := &PoolConfig{
			MaxConnections:        m.config.Graph.MaxConnections,
			MinIdleConnections:    2,
			MaxConnectionLifetime: 1 * time.Hour,
			MaxIdleTime:           m.config.Graph.IdleTimeout,
			ConnectionTimeout:     m.config.Graph.ConnTimeout,
			CleanupInterval:       1 * time.Minute,
			HealthCheckInterval:   30 * time.Second,
			ValidateOnGet:         true,
		}

		factory := NewGraphConnectionFactory(m.config.Graph)

		pool, err := NewGenericConnectionPool(factory, poolConfig)
		if err != nil {
			return fmt.Errorf("failed to create graph connection pool: %w", err)
		}

		m.graphPool = pool
	}

	// Initialize vector pool
	if m.config.Vector != nil {
		poolConfig := &PoolConfig{
			MaxConnections:        m.config.Vector.MaxConnections,
			MinIdleConnections:    2,
			MaxConnectionLifetime: 1 * time.Hour,
			MaxIdleTime:           m.config.Vector.IdleTimeout,
			ConnectionTimeout:     m.config.Vector.ConnTimeout,
			CleanupInterval:       1 * time.Minute,
			HealthCheckInterval:   30 * time.Second,
			ValidateOnGet:         true,
		}

		factory := NewVectorConnectionFactory(m.config.Vector)

		pool, err := NewGenericConnectionPool(factory, poolConfig)
		if err != nil {
			return fmt.Errorf("failed to create vector connection pool: %w", err)
		}

		m.vectorPool = pool
	}

	return nil
}

func (m *PooledStorageManager) setupHealthMonitoring() error {
	// Add health checkers for each pool
	if m.relationalPool != nil {
		checker := NewConnectionPoolHealthChecker("relational_pool", m.relationalPool)
		m.healthMonitor.AddChecker(checker)
	}

	if m.graphPool != nil {
		checker := NewConnectionPoolHealthChecker("graph_pool", m.graphPool)
		m.healthMonitor.AddChecker(checker)
	}

	if m.vectorPool != nil {
		checker := NewConnectionPoolHealthChecker("vector_pool", m.vectorPool)
		m.healthMonitor.AddChecker(checker)
	}

	// Start health monitoring
	return m.healthMonitor.Start()
}
