package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/vector"
)

func TestPooledStorageManagerIntegration(t *testing.T) {
	// Create a test configuration
	config := &StorageConfig{
		Relational: &RelationalConfig{
			Type:           StorageTypeSQLite,
			Database:       ":memory:",
			MaxConnections: 5,
			MinConnections: 1,
			ConnTimeout:    10 * time.Second,
			IdleTimeout:    5 * time.Minute,
			MaxLifetime:    1 * time.Hour,
			BatchSize:      100,
		},
		Graph: &graph.GraphConfig{
			Type:           graph.StoreTypeInMemory,
			MaxConnections: 3,
			ConnTimeout:    10 * time.Second,
			IdleTimeout:    5 * time.Minute,
			BatchSize:      50,
		},
		Vector: &vector.VectorConfig{
			Type:           vector.StoreTypeInMemory,
			Dimension:      768,
			DistanceMetric: "cosine",
			MaxConnections: 3,
			ConnTimeout:    10 * time.Second,
			IdleTimeout:    5 * time.Minute,
			BatchSize:      50,
		},
		EnableTransactions: true,
		ConnectionTimeout:  30 * time.Second,
		QueryTimeout:       30 * time.Second,
		MaxRetries:         3,
		Environment:        "test",
	}

	// Create pooled storage manager
	manager, err := NewPooledStorageManager(config)
	if err != nil {
		t.Fatalf("Failed to create pooled storage manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Test relational connections
	t.Run("RelationalConnections", func(t *testing.T) {
		conn, err := manager.GetRelationalConnection(ctx)
		if err != nil {
			t.Fatalf("Failed to get relational connection: %v", err)
		}

		// Test connection
		if err := conn.Ping(ctx); err != nil {
			t.Errorf("Relational connection ping failed: %v", err)
		}

		// Return connection
		if err := manager.PutRelationalConnection(conn); err != nil {
			t.Errorf("Failed to return relational connection: %v", err)
		}
	})

	// Test graph connections
	t.Run("GraphConnections", func(t *testing.T) {
		conn, err := manager.GetGraphConnection(ctx)
		if err != nil {
			t.Fatalf("Failed to get graph connection: %v", err)
		}

		// Test connection
		if err := conn.Ping(ctx); err != nil {
			t.Errorf("Graph connection ping failed: %v", err)
		}

		// Return connection
		if err := manager.PutGraphConnection(conn); err != nil {
			t.Errorf("Failed to return graph connection: %v", err)
		}
	})

	// Test vector connections
	t.Run("VectorConnections", func(t *testing.T) {
		conn, err := manager.GetVectorConnection(ctx)
		if err != nil {
			t.Fatalf("Failed to get vector connection: %v", err)
		}

		// Test connection
		if err := conn.Ping(ctx); err != nil {
			t.Errorf("Vector connection ping failed: %v", err)
		}

		// Return connection
		if err := manager.PutVectorConnection(conn); err != nil {
			t.Errorf("Failed to return vector connection: %v", err)
		}
	})

	// Test health monitoring
	t.Run("HealthMonitoring", func(t *testing.T) {
		// Get health report
		report := manager.GetHealthReport(ctx)
		if report == nil {
			t.Fatal("Expected health report")
		}

		// Should have health checks for all pools
		expectedChecks := []string{"relational_pool", "graph_pool", "vector_pool"}
		for _, checkName := range expectedChecks {
			if _, exists := report.Checks[checkName]; !exists {
				t.Errorf("Expected health check for %s", checkName)
			}
		}

		// Overall status should be healthy
		if report.OverallStatus != HealthStatusHealthy {
			t.Errorf("Expected healthy overall status, got %s", report.OverallStatus)
		}

		// Test IsHealthy method
		if !manager.IsHealthy() {
			t.Error("Manager should be healthy")
		}
	})

	// Test concurrent access
	t.Run("ConcurrentAccess", func(t *testing.T) {
		const numGoroutines = 10
		const numOperations = 5

		errChan := make(chan error, numGoroutines*numOperations*3) // 3 connection types
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					// Test relational
					conn, err := manager.GetRelationalConnection(ctx)
					if err != nil {
						errChan <- err
						continue
					}
					time.Sleep(1 * time.Millisecond) // Simulate work
					if err := manager.PutRelationalConnection(conn); err != nil {
						errChan <- err
					}

					// Test graph
					conn, err = manager.GetGraphConnection(ctx)
					if err != nil {
						errChan <- err
						continue
					}
					time.Sleep(1 * time.Millisecond) // Simulate work
					if err := manager.PutGraphConnection(conn); err != nil {
						errChan <- err
					}

					// Test vector
					conn, err = manager.GetVectorConnection(ctx)
					if err != nil {
						errChan <- err
						continue
					}
					time.Sleep(1 * time.Millisecond) // Simulate work
					if err := manager.PutVectorConnection(conn); err != nil {
						errChan <- err
					}
				}
			}()
		}

		// Ensure all operations complete before manager is closed by defer.
		wg.Wait()

		// Check for errors
		close(errChan)
		for err := range errChan {
			t.Errorf("Concurrent access error: %v", err)
		}
	})
}

func TestPooledStorageManagerConfiguration(t *testing.T) {
	// Test with file-based configuration
	tempDir := t.TempDir()
	factory := NewConfigFactory()
	config := factory.CreateFileBasedConfig(tempDir)

	manager, err := NewPooledStorageManager(config)
	if err != nil {
		t.Fatalf("Failed to create manager with file-based config: %v", err)
	}
	defer manager.Close()

	// Test health
	report := manager.GetHealthReport(context.Background())
	if report.OverallStatus != HealthStatusHealthy {
		t.Errorf("File-based manager should be healthy, report: %+v", report)
	}
}

func TestPooledStorageManagerFailover(t *testing.T) {
	// Create configuration with small pool sizes to test limits
	config := &StorageConfig{
		Relational: &RelationalConfig{
			Type:           StorageTypeSQLite,
			Database:       ":memory:",
			MaxConnections: 2, // Small pool
			MinConnections: 1,
			ConnTimeout:    1 * time.Second, // Short timeout
			IdleTimeout:    1 * time.Minute,
			MaxLifetime:    10 * time.Minute,
		},
		Graph: &graph.GraphConfig{
			Type:           graph.StoreTypeInMemory,
			MaxConnections: 2,
			ConnTimeout:    1 * time.Second,
			IdleTimeout:    1 * time.Minute,
		},
		Vector: &vector.VectorConfig{
			Type:           vector.StoreTypeInMemory,
			Dimension:      768,
			MaxConnections: 2,
			ConnTimeout:    1 * time.Second,
			IdleTimeout:    1 * time.Minute,
		},
		Environment: "test",
	}

	manager, err := NewPooledStorageManager(config)
	if err != nil {
		t.Fatalf("Failed to create pooled storage manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Get all available connections
	var relConns []schema.Connection

	// Exhaust relational pool
	for i := 0; i < 2; i++ {
		conn, err := manager.GetRelationalConnection(ctx)
		if err != nil {
			t.Fatalf("Failed to get relational connection %d: %v", i, err)
		}
		relConns = append(relConns, conn)
	}

	// Try to get one more (should timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = manager.GetRelationalConnection(ctx)
	if err == nil {
		t.Error("Expected timeout when pool is exhausted")
	}

	// Return connections
	for _, conn := range relConns {
		manager.PutRelationalConnection(conn)
	}

	// Should be able to get connections again
	ctx = context.Background()
	conn, err := manager.GetRelationalConnection(ctx)
	if err != nil {
		t.Errorf("Should be able to get connection after returning: %v", err)
	}
	manager.PutRelationalConnection(conn)

	// Clean up
	for _, conn := range relConns {
		_ = conn // Already returned
	}
}

func TestPooledStorageManagerHealthMonitoring(t *testing.T) {
	config := DefaultStorageConfig()

	manager, err := NewPooledStorageManager(config)
	if err != nil {
		t.Fatalf("Failed to create pooled storage manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Initial health check
	report := manager.GetHealthReport(ctx)
	if report.OverallStatus != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s", report.OverallStatus)
	}

	// Test health monitoring over time
	// Note: In a real test, we might simulate failures and recovery
	time.Sleep(50 * time.Millisecond)

	report2 := manager.GetHealthReport(ctx)
	if report2.Timestamp.Before(report.Timestamp) {
		t.Error("Second report should have later timestamp")
	}
}

func TestPooledStorageManagerClose(t *testing.T) {
	config := DefaultStorageConfig()

	manager, err := NewPooledStorageManager(config)
	if err != nil {
		t.Fatalf("Failed to create pooled storage manager: %v", err)
	}

	ctx := context.Background()

	// Get some connections
	relConn, err := manager.GetRelationalConnection(ctx)
	if err != nil {
		t.Fatalf("Failed to get relational connection: %v", err)
	}

	graphConn, err := manager.GetGraphConnection(ctx)
	if err != nil {
		t.Fatalf("Failed to get graph connection: %v", err)
	}

	// Close manager
	err = manager.Close()
	if err != nil {
		t.Errorf("Failed to close manager: %v", err)
	}

	// Operations should fail after close
	_, err = manager.GetRelationalConnection(ctx)
	if err == nil {
		t.Error("Expected error when getting connection from closed manager")
	}

	err = manager.PutRelationalConnection(relConn)
	if err == nil {
		t.Error("Expected error when putting connection to closed manager")
	}

	err = manager.PutGraphConnection(graphConn)
	if err == nil {
		t.Error("Expected error when putting graph connection to closed manager")
	}

	// Health check should indicate unhealthy state
	if manager.IsHealthy() {
		t.Error("Closed manager should not be healthy")
	}
}

func BenchmarkPooledStorageManager(b *testing.B) {
	config := DefaultStorageConfig()
	config.Relational.MaxConnections = 20
	config.Graph.MaxConnections = 20
	config.Vector.MaxConnections = 20

	manager, err := NewPooledStorageManager(config)
	if err != nil {
		b.Fatalf("Failed to create pooled storage manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	b.ResetTimer()

	b.Run("RelationalConnections", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn, err := manager.GetRelationalConnection(ctx)
				if err != nil {
					b.Errorf("Failed to get connection: %v", err)
					continue
				}

				// Simulate some work
				conn.Ping(ctx)

				manager.PutRelationalConnection(conn)
			}
		})
	})

	b.Run("GraphConnections", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn, err := manager.GetGraphConnection(ctx)
				if err != nil {
					b.Errorf("Failed to get connection: %v", err)
					continue
				}

				// Simulate some work
				conn.Ping(ctx)

				manager.PutGraphConnection(conn)
			}
		})
	})

	b.Run("VectorConnections", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn, err := manager.GetVectorConnection(ctx)
				if err != nil {
					b.Errorf("Failed to get connection: %v", err)
					continue
				}

				// Simulate some work
				conn.Ping(ctx)

				manager.PutVectorConnection(conn)
			}
		})
	})

	b.Run("HealthChecks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			manager.GetHealthReport(ctx)
		}
	})
}
