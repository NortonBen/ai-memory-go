# Connection Pooling and Health Monitoring

This document describes the connection pooling and health monitoring system implemented for AI Memory Integration storage backends.

## Overview

The connection pooling and health monitoring system provides:

- **Connection Pooling**: Efficient management of database connections across all storage types
- **Health Monitoring**: Continuous monitoring of storage backend health with automatic failover capabilities
- **Resource Management**: Automatic cleanup of expired connections and resource optimization
- **Concurrent Access**: Thread-safe operations with proper synchronization
- **Metrics and Analytics**: Detailed statistics and monitoring capabilities

## Architecture

### Connection Pooling Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        Pooled Storage Manager                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                  │
│  │ Relational Pool │ │   Graph Pool    │ │  Vector Pool    │                  │
│  │                 │ │                 │ │                 │                  │
│  │ • PostgreSQL    │ │ • Neo4j         │ │ • Qdrant        │                  │
│  │ • SQLite        │ │ • SurrealDB     │ │ • LanceDB       │                  │
│  │ • Connection    │ │ • Kuzu          │ │ • pgvector      │                  │
│  │   Validation    │ │ • FalkorDB      │ │ • ChromaDB      │                  │
│  │ • Health Checks │ │ • In-Memory     │ │ • Redis         │                  │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│                           Health Monitor                                        │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                  │
│  │ Pool Health     │ │ Storage Health  │ │ System Health   │                  │
│  │ • Utilization   │ │ • Connectivity  │ │ • Overall Status│                  │
│  │ • Performance   │ │ • Response Time │ │ • Alerts        │                  │
│  │ • Failures      │ │ • Error Rates   │ │ • Callbacks     │                  │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Health Monitoring Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Health Monitoring System                              │
├─────────────────────────────────────────────────────────────────────────────────┤
│ Health Monitor                                                                  │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                  │
│  │ Health Checkers │ │ Status Tracking │ │ Alert System    │                  │
│  │ • Pool Checkers │ │ • Status History│ │ • Callbacks     │                  │
│  │ • Store Checkers│ │ • Trend Analysis│ │ • Notifications │                  │
│  │ • Custom        │ │ • Metrics       │ │ • Escalation    │                  │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│ Health Status Flow                                                              │
│  Healthy → Degraded → Unhealthy → Recovery → Healthy                          │
│     ↑         ↓          ↓           ↑         ↑                              │
│     └─────────┴──────────┴───────────┴─────────┘                              │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Connection Pooling

### Core Features

#### 1. Generic Connection Pool
- **Type-agnostic**: Works with any connection type implementing the `Connection` interface
- **Configurable**: Extensive configuration options for pool behavior
- **Thread-safe**: Concurrent access with proper synchronization
- **Resource management**: Automatic cleanup and lifecycle management

#### 2. Connection Lifecycle Management
- **Creation**: On-demand connection creation with factory pattern
- **Validation**: Optional validation on get/put operations
- **Expiration**: Automatic cleanup of expired connections
- **Health checks**: Periodic validation of idle connections

#### 3. Pool Configuration
```go
type PoolConfig struct {
    MaxConnections        int           // Maximum pool size
    MinIdleConnections    int           // Minimum idle connections
    MaxConnectionLifetime time.Duration // Connection expiration
    MaxIdleTime          time.Duration // Idle connection timeout
    ConnectionTimeout     time.Duration // Get connection timeout
    CleanupInterval      time.Duration // Cleanup frequency
    HealthCheckInterval  time.Duration // Health check frequency
    ValidateOnGet        bool          // Validate on checkout
    ValidateOnPut        bool          // Validate on return
}
```

### Usage Examples

#### Basic Pool Usage
```go
// Create connection factory
factory := NewPostgreSQLConnectionFactory(config)

// Create pool with configuration
poolConfig := &PoolConfig{
    MaxConnections:     20,
    MinIdleConnections: 2,
    ConnectionTimeout:  30 * time.Second,
    ValidateOnGet:     true,
}

pool, err := NewGenericConnectionPool(factory, poolConfig)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// Get connection
ctx := context.Background()
conn, err := pool.Get(ctx)
if err != nil {
    log.Fatal(err)
}

// Use connection
err = conn.Ping(ctx)
if err != nil {
    log.Printf("Connection failed: %v", err)
}

// Return connection
pool.Put(conn)
```

#### Pooled Storage Manager
```go
// Create storage configuration
config := &StorageConfig{
    Relational: &RelationalConfig{
        Type:           StorageTypePostgreSQL,
        Host:           "localhost",
        Port:           5432,
        Database:       "ai_memory",
        MaxConnections: 50,
        MinConnections: 5,
    },
    Graph: &GraphConfig{
        Type:           GraphStoreTypeNeo4j,
        MaxConnections: 20,
    },
    Vector: &VectorConfig{
        Type:           VectorStoreTypeQdrant,
        MaxConnections: 20,
    },
}

// Create pooled storage manager
manager, err := NewPooledStorageManager(config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// Use connections
ctx := context.Background()

// Relational connection
relConn, err := manager.GetRelationalConnection(ctx)
if err != nil {
    log.Fatal(err)
}
// Use relational connection...
manager.PutRelationalConnection(relConn)

// Graph connection
graphConn, err := manager.GetGraphConnection(ctx)
if err != nil {
    log.Fatal(err)
}
// Use graph connection...
manager.PutGraphConnection(graphConn)
```

### Pool Statistics and Monitoring

```go
// Get pool statistics
stats := pool.Stats()

fmt.Printf("Total Connections: %d\n", stats.TotalConnections)
fmt.Printf("Active Connections: %d\n", stats.ActiveConnections)
fmt.Printf("Idle Connections: %d\n", stats.IdleConnections)
fmt.Printf("Connections Created: %d\n", stats.ConnectionsCreated)
fmt.Printf("Connections Closed: %d\n", stats.ConnectionsClosed)
fmt.Printf("Connection Failures: %d\n", stats.ConnectionsFailed)
fmt.Printf("Average Lifetime: %v\n", stats.AvgConnectionLifetime)
```

## Health Monitoring

### Core Features

#### 1. Health Status Levels
- **Healthy**: All components functioning normally
- **Degraded**: Some performance issues but still functional
- **Unhealthy**: Critical issues requiring attention
- **Unknown**: Status cannot be determined

#### 2. Health Checkers
- **Connection Pool Checkers**: Monitor pool utilization and performance
- **Storage Checkers**: Monitor backend connectivity and response times
- **Custom Checkers**: User-defined health checks

#### 3. Monitoring Features
- **Automatic Checks**: Periodic health monitoring
- **Manual Checks**: On-demand health verification
- **Status Callbacks**: Notifications on status changes
- **Retry Logic**: Automatic retry with exponential backoff
- **Concurrent Checks**: Parallel execution for performance

### Usage Examples

#### Basic Health Monitoring
```go
// Create health monitor
monitor := NewHealthMonitor(DefaultHealthMonitorConfig())

// Add health checkers
poolChecker := NewConnectionPoolHealthChecker("db_pool", pool)
storageChecker := NewStorageHealthChecker("main_storage", storage)

monitor.AddChecker(poolChecker)
monitor.AddChecker(storageChecker)

// Start automatic monitoring
monitor.Start()
defer monitor.Stop()

// Get health report
ctx := context.Background()
report := monitor.CheckAll(ctx)

fmt.Printf("Overall Status: %s\n", report.OverallStatus)
fmt.Printf("Summary: %s\n", report.Summary)

for name, check := range report.Checks {
    fmt.Printf("%s: %s (%v)\n", name, check.Status, check.Duration)
    if check.Error != "" {
        fmt.Printf("  Error: %s\n", check.Error)
    }
}
```

#### Health Callbacks
```go
// Set status change callback
monitor.SetStatusChangeCallback(func(name string, oldStatus, newStatus HealthStatus) {
    log.Printf("Health status changed for %s: %s -> %s", name, oldStatus, newStatus)
    
    if newStatus == HealthStatusUnhealthy {
        // Trigger alerts, notifications, etc.
        alertSystem.SendAlert(fmt.Sprintf("Component %s is unhealthy", name))
    }
})

// Set failure callback
monitor.SetFailureCallback(func(name string, check HealthCheck) {
    log.Printf("Health check failed for %s: %s", name, check.Error)
    
    // Log detailed failure information
    metrics.RecordHealthCheckFailure(name, check.Error)
})
```

#### Custom Health Checkers
```go
// Implement custom health checker
type CustomHealthChecker struct {
    name string
    // custom fields...
}

func (c *CustomHealthChecker) Name() string {
    return c.name
}

func (c *CustomHealthChecker) Check(ctx context.Context) HealthCheck {
    start := time.Now()
    
    // Perform custom health check logic
    err := c.performCustomCheck(ctx)
    
    check := HealthCheck{
        Name:      c.name,
        Timestamp: time.Now(),
        Duration:  time.Since(start),
    }
    
    if err != nil {
        check.Status = HealthStatusUnhealthy
        check.Error = err.Error()
    } else {
        check.Status = HealthStatusHealthy
        check.Message = "Custom check passed"
    }
    
    return check
}

// Add to monitor
customChecker := &CustomHealthChecker{name: "custom_service"}
monitor.AddChecker(customChecker)
```

### Health Report Structure

```go
type HealthReport struct {
    OverallStatus HealthStatus           `json:"overall_status"`
    Checks        map[string]HealthCheck `json:"checks"`
    Timestamp     time.Time              `json:"timestamp"`
    Duration      time.Duration          `json:"duration"`
    Summary       string                 `json:"summary"`
}

type HealthCheck struct {
    Name        string                 `json:"name"`
    Status      HealthStatus           `json:"status"`
    Message     string                 `json:"message,omitempty"`
    Error       string                 `json:"error,omitempty"`
    Duration    time.Duration          `json:"duration"`
    Timestamp   time.Time              `json:"timestamp"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    CheckCount  int64                  `json:"check_count"`
    FailCount   int64                  `json:"fail_count"`
    LastSuccess time.Time              `json:"last_success,omitempty"`
    LastFailure time.Time              `json:"last_failure,omitempty"`
}
```

## Configuration Integration

### Environment Variables

The connection pooling and health monitoring system integrates with the existing configuration system:

```bash
# Connection pool settings
AI_MEMORY_RELATIONAL_MAX_CONNECTIONS=50
AI_MEMORY_RELATIONAL_MIN_CONNECTIONS=5
AI_MEMORY_RELATIONAL_CONN_TIMEOUT=30s
AI_MEMORY_RELATIONAL_IDLE_TIMEOUT=5m
AI_MEMORY_RELATIONAL_MAX_LIFETIME=1h

# Health monitoring settings
AI_MEMORY_HEALTH_CHECK_INTERVAL=30s
AI_MEMORY_HEALTH_CHECK_TIMEOUT=10s
AI_MEMORY_HEALTH_FAILURE_THRESHOLD=3
AI_MEMORY_HEALTH_RECOVERY_THRESHOLD=2
```

### Configuration Profiles

#### File-Based Profile (Embedded Deployment)
```yaml
relational:
  type: "sqlite"
  database: "./data/memory.db"
  max_connections: 5
  min_connections: 1
  conn_timeout: "10s"
  idle_timeout: "5m"

graph:
  type: "kuzu"
  max_connections: 3
  conn_timeout: "10s"

vector:
  type: "lancedb"
  max_connections: 3
  conn_timeout: "10s"

health_monitoring:
  check_interval: "1m"
  check_timeout: "5s"
  enable_auto_check: true
```

#### Cloud Profile (Production Deployment)
```yaml
relational:
  type: "postgresql"
  host: "db.example.com"
  port: 5432
  max_connections: 50
  min_connections: 5
  conn_timeout: "30s"
  idle_timeout: "10m"
  max_lifetime: "1h"

graph:
  type: "neo4j"
  host: "graph.example.com"
  port: 7687
  max_connections: 20
  conn_timeout: "30s"

vector:
  type: "qdrant"
  host: "vector.example.com"
  port: 6333
  max_connections: 20
  conn_timeout: "30s"

health_monitoring:
  check_interval: "30s"
  check_timeout: "10s"
  failure_threshold: 3
  recovery_threshold: 2
  enable_retry: true
  max_retries: 3
```

## Performance Optimization

### Connection Pool Tuning

#### Pool Size Optimization
```go
// Calculate optimal pool size based on workload
func calculateOptimalPoolSize(avgResponseTime time.Duration, requestRate float64) int {
    // Little's Law: L = λ × W
    // Where L = average number of requests in system
    //       λ = arrival rate (requests per second)  
    //       W = average response time
    
    avgRequests := requestRate * avgResponseTime.Seconds()
    
    // Add buffer for peak loads (20-50% overhead)
    return int(avgRequests * 1.3)
}

// Example usage
poolSize := calculateOptimalPoolSize(100*time.Millisecond, 1000) // 1000 req/s
config.MaxConnections = poolSize
```

#### Connection Lifecycle Tuning
```go
// Optimize connection lifecycle based on usage patterns
config := &PoolConfig{
    MaxConnections:        50,
    MinIdleConnections:    5,  // Keep minimum ready
    MaxConnectionLifetime: 1 * time.Hour,  // Prevent stale connections
    MaxIdleTime:          10 * time.Minute, // Clean up unused
    ConnectionTimeout:     30 * time.Second, // Reasonable wait
    CleanupInterval:      1 * time.Minute,  // Regular maintenance
    HealthCheckInterval:  30 * time.Second, // Proactive health
    ValidateOnGet:        true,  // Ensure connection quality
    ValidateOnPut:        false, // Optimize return performance
}
```

### Health Monitoring Optimization

#### Check Frequency Tuning
```go
// Balance monitoring overhead with detection speed
config := &HealthMonitorConfig{
    CheckInterval:     30 * time.Second, // Frequent enough for quick detection
    CheckTimeout:      5 * time.Second,  // Prevent hanging checks
    FailureThreshold:  3,                // Avoid false positives
    RecoveryThreshold: 2,                // Quick recovery detection
    EnableRetry:       true,             // Handle transient failures
    MaxRetries:        2,                // Limit retry overhead
    RetryDelay:        1 * time.Second,  // Exponential backoff start
}
```

## Best Practices

### Connection Pool Best Practices

1. **Size Appropriately**: Use Little's Law to calculate optimal pool sizes
2. **Monitor Utilization**: Track pool statistics and adjust as needed
3. **Validate Connections**: Enable validation to prevent stale connections
4. **Handle Timeouts**: Set reasonable timeouts for connection acquisition
5. **Clean Up Resources**: Ensure proper cleanup on application shutdown

### Health Monitoring Best Practices

1. **Comprehensive Coverage**: Monitor all critical components
2. **Appropriate Thresholds**: Set failure/recovery thresholds based on SLA requirements
3. **Actionable Alerts**: Configure callbacks for automated responses
4. **Performance Impact**: Balance monitoring frequency with system overhead
5. **Historical Tracking**: Maintain health check history for trend analysis

### Error Handling Best Practices

1. **Graceful Degradation**: Handle pool exhaustion gracefully
2. **Circuit Breaker**: Implement circuit breaker pattern for failing backends
3. **Retry Logic**: Use exponential backoff for transient failures
4. **Logging**: Comprehensive logging for debugging and monitoring
5. **Metrics**: Export metrics for external monitoring systems

## Troubleshooting

### Common Issues

#### Connection Pool Exhaustion
```
Error: failed to get connection: context deadline exceeded
```

**Solutions**:
- Increase `MaxConnections` in pool configuration
- Reduce `ConnectionTimeout` to fail faster
- Check for connection leaks (not returning connections)
- Monitor pool utilization and adjust based on usage patterns

#### Health Check Failures
```
Error: health check failed: connection refused
```

**Solutions**:
- Verify backend service is running and accessible
- Check network connectivity and firewall rules
- Increase `CheckTimeout` for slow backends
- Review failure thresholds and adjust if needed

#### Performance Issues
```
Warning: high connection utilization: 95%
```

**Solutions**:
- Increase pool size or optimize query performance
- Implement connection caching strategies
- Use read replicas for read-heavy workloads
- Monitor and optimize slow queries

### Debugging Tools

#### Pool Statistics Monitoring
```go
// Monitor pool health continuously
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := pool.Stats()
        utilization := float64(stats.ActiveConnections) / float64(stats.TotalConnections)
        
        if utilization > 0.8 {
            log.Printf("High pool utilization: %.1f%%", utilization*100)
        }
        
        if stats.ConnectionsFailed > 0 {
            log.Printf("Connection failures detected: %d", stats.ConnectionsFailed)
        }
    }
}()
```

#### Health Report Analysis
```go
// Analyze health trends
func analyzeHealthTrends(reports []*HealthReport) {
    for _, report := range reports {
        for name, check := range report.Checks {
            if check.Status != HealthStatusHealthy {
                log.Printf("Component %s unhealthy: %s", name, check.Error)
            }
            
            if check.Duration > 5*time.Second {
                log.Printf("Slow health check for %s: %v", name, check.Duration)
            }
        }
    }
}
```

## Integration Examples

### Web Service Integration
```go
// HTTP handler with pooled storage
func (h *Handler) CreateDataPoint(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Get connection from pool
    conn, err := h.storageManager.GetRelationalConnection(ctx)
    if err != nil {
        http.Error(w, "Storage unavailable", http.StatusServiceUnavailable)
        return
    }
    defer h.storageManager.PutRelationalConnection(conn)
    
    // Use connection for database operations
    // ... business logic ...
}

// Health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    report := h.storageManager.GetHealthReport(ctx)
    
    status := http.StatusOK
    if report.OverallStatus != HealthStatusHealthy {
        status = http.StatusServiceUnavailable
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(report)
}
```

### Microservice Integration
```go
// Service with health monitoring
type MemoryService struct {
    storageManager *PooledStorageManager
    healthMonitor  *HealthMonitor
}

func (s *MemoryService) Start() error {
    // Start health monitoring
    if err := s.healthMonitor.Start(); err != nil {
        return fmt.Errorf("failed to start health monitoring: %w", err)
    }
    
    // Set up health callbacks
    s.healthMonitor.SetStatusChangeCallback(s.handleStatusChange)
    s.healthMonitor.SetFailureCallback(s.handleFailure)
    
    return nil
}

func (s *MemoryService) handleStatusChange(name string, oldStatus, newStatus HealthStatus) {
    log.Printf("Component %s status changed: %s -> %s", name, oldStatus, newStatus)
    
    // Implement circuit breaker logic
    if newStatus == HealthStatusUnhealthy {
        s.enableCircuitBreaker(name)
    } else if newStatus == HealthStatusHealthy {
        s.disableCircuitBreaker(name)
    }
}
```

## Conclusion

The connection pooling and health monitoring system provides a robust foundation for managing storage backend connections in the AI Memory Integration library. It offers:

- **Scalability**: Efficient resource utilization and connection management
- **Reliability**: Comprehensive health monitoring and automatic failover
- **Performance**: Optimized connection lifecycle and concurrent access
- **Observability**: Detailed metrics and monitoring capabilities
- **Flexibility**: Configurable behavior for different deployment scenarios

This system ensures that the AI Memory Integration library can handle production workloads with high availability and performance requirements while providing the necessary tools for monitoring and troubleshooting.