// Package storage provides health monitoring for storage backends
package storage

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthCheck represents a single health check
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

// HealthReport aggregates multiple health checks
type HealthReport struct {
	OverallStatus HealthStatus           `json:"overall_status"`
	Checks        map[string]HealthCheck `json:"checks"`
	Timestamp     time.Time              `json:"timestamp"`
	Duration      time.Duration          `json:"duration"`
	Summary       string                 `json:"summary"`
}

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	// Check performs a health check
	Check(ctx context.Context) HealthCheck

	// Name returns the name of this health checker
	Name() string
}

// HealthMonitor manages health checks for storage backends
type HealthMonitor struct {
	checkers map[string]HealthChecker
	config   *HealthMonitorConfig

	// State management
	mu         sync.RWMutex
	lastReport *HealthReport
	isRunning  int32
	stopChan   chan struct{}
	wg         sync.WaitGroup

	// Callbacks
	onStatusChange func(name string, oldStatus, newStatus HealthStatus)
	onFailure      func(name string, check HealthCheck)
}

// HealthMonitorConfig defines configuration for health monitoring
type HealthMonitorConfig struct {
	// Interval between health checks
	CheckInterval time.Duration `json:"check_interval"`

	// Timeout for individual health checks
	CheckTimeout time.Duration `json:"check_timeout"`

	// Number of consecutive failures before marking as unhealthy
	FailureThreshold int `json:"failure_threshold"`

	// Number of consecutive successes before marking as healthy
	RecoveryThreshold int `json:"recovery_threshold"`

	// Enable automatic health checks
	EnableAutoCheck bool `json:"enable_auto_check"`

	// Enable detailed logging
	EnableLogging bool `json:"enable_logging"`

	// Retry failed checks with exponential backoff
	EnableRetry bool `json:"enable_retry"`

	// Maximum retry attempts
	MaxRetries int `json:"max_retries"`

	// Initial retry delay
	RetryDelay time.Duration `json:"retry_delay"`
}

// DefaultHealthMonitorConfig returns sensible defaults
func DefaultHealthMonitorConfig() *HealthMonitorConfig {
	return &HealthMonitorConfig{
		CheckInterval:     30 * time.Second,
		CheckTimeout:      10 * time.Second,
		FailureThreshold:  3,
		RecoveryThreshold: 2,
		EnableAutoCheck:   true,
		EnableLogging:     true,
		EnableRetry:       true,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
	}
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(config *HealthMonitorConfig) *HealthMonitor {
	if config == nil {
		config = DefaultHealthMonitorConfig()
	}

	return &HealthMonitor{
		checkers: make(map[string]HealthChecker),
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// AddChecker adds a health checker
func (hm *HealthMonitor) AddChecker(checker HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.checkers[checker.Name()] = checker
}

// RemoveChecker removes a health checker
func (hm *HealthMonitor) RemoveChecker(name string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	delete(hm.checkers, name)
}

// Start begins automatic health checking
func (hm *HealthMonitor) Start() error {
	if !atomic.CompareAndSwapInt32(&hm.isRunning, 0, 1) {
		return fmt.Errorf("health monitor is already running")
	}

	if !hm.config.EnableAutoCheck {
		return nil
	}

	hm.mu.Lock()
	hm.stopChan = make(chan struct{})
	hm.mu.Unlock()

	hm.wg.Add(1)
	go hm.monitorLoop()

	return nil
}

// Stop stops automatic health checking
func (hm *HealthMonitor) Stop() error {
	if !atomic.CompareAndSwapInt32(&hm.isRunning, 1, 0) {
		return nil // Already stopped
	}

	close(hm.stopChan)
	hm.wg.Wait()

	return nil
}

// CheckAll performs health checks on all registered checkers
func (hm *HealthMonitor) CheckAll(ctx context.Context) *HealthReport {
	start := time.Now()

	hm.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range hm.checkers {
		checkers[name] = checker
	}
	hm.mu.RUnlock()

	checks := make(map[string]HealthCheck)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Run all health checks concurrently
	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, hm.config.CheckTimeout)
			defer cancel()

			check := hm.performCheck(checkCtx, checker)

			mu.Lock()
			checks[name] = check
			mu.Unlock()
		}(name, checker)
	}

	wg.Wait()

	// Determine overall status
	overallStatus := hm.calculateOverallStatus(checks)

	report := &HealthReport{
		OverallStatus: overallStatus,
		Checks:        checks,
		Timestamp:     time.Now(),
		Duration:      time.Since(start),
		Summary:       hm.generateSummary(checks, overallStatus),
	}

	// Update last report
	hm.mu.Lock()
	hm.lastReport = report
	hm.mu.Unlock()

	return report
}

// CheckOne performs a health check on a specific checker
func (hm *HealthMonitor) CheckOne(ctx context.Context, name string) (*HealthCheck, error) {
	hm.mu.RLock()
	checker, exists := hm.checkers[name]
	hm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("health checker not found: %s", name)
	}

	checkCtx, cancel := context.WithTimeout(ctx, hm.config.CheckTimeout)
	defer cancel()

	check := hm.performCheck(checkCtx, checker)
	return &check, nil
}

// GetLastReport returns the last health report
func (hm *HealthMonitor) GetLastReport() *HealthReport {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if hm.lastReport == nil {
		return nil
	}

	// Return a copy
	report := *hm.lastReport
	return &report
}

// SetStatusChangeCallback sets a callback for status changes
func (hm *HealthMonitor) SetStatusChangeCallback(callback func(name string, oldStatus, newStatus HealthStatus)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.onStatusChange = callback
}

// SetFailureCallback sets a callback for failures
func (hm *HealthMonitor) SetFailureCallback(callback func(name string, check HealthCheck)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.onFailure = callback
}

// IsHealthy returns true if all components are healthy
func (hm *HealthMonitor) IsHealthy() bool {
	report := hm.GetLastReport()
	if report == nil {
		return false
	}

	return report.OverallStatus == HealthStatusHealthy
}

// GetStatus returns the current overall status
func (hm *HealthMonitor) GetStatus() HealthStatus {
	report := hm.GetLastReport()
	if report == nil {
		return HealthStatusUnknown
	}

	return report.OverallStatus
}

// Private methods

func (hm *HealthMonitor) monitorLoop() {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.config.CheckInterval)
	defer ticker.Stop()

	// Perform initial check
	ctx := context.Background()
	hm.CheckAll(ctx)

	for {
		select {
		case <-hm.stopChan:
			return
		case <-ticker.C:
			hm.CheckAll(ctx)
		}
	}
}

func (hm *HealthMonitor) performCheck(ctx context.Context, checker HealthChecker) HealthCheck {
	start := time.Now()

	var check HealthCheck
	var err error

	// Perform the check with retry logic
	if hm.config.EnableRetry {
		check, err = hm.performCheckWithRetry(ctx, checker)
	} else {
		check = checker.Check(ctx)
		if check.Status == HealthStatusUnhealthy && check.Error != "" {
			err = fmt.Errorf("%s", check.Error)
		}
	}

	check.Duration = time.Since(start)
	check.Timestamp = time.Now()

	// Update counters
	atomic.AddInt64(&check.CheckCount, 1)

	if check.Status == HealthStatusHealthy {
		check.LastSuccess = check.Timestamp
	} else {
		check.LastFailure = check.Timestamp
		atomic.AddInt64(&check.FailCount, 1)

		// Call failure callback
		if hm.onFailure != nil {
			go hm.onFailure(checker.Name(), check)
		}
	}

	// Check for status changes
	if hm.lastReport != nil {
		if lastCheck, exists := hm.lastReport.Checks[checker.Name()]; exists {
			if lastCheck.Status != check.Status && hm.onStatusChange != nil {
				go hm.onStatusChange(checker.Name(), lastCheck.Status, check.Status)
			}
		}
	}

	if err != nil && hm.config.EnableLogging {
		// Log error (in a real implementation, use proper logging)
		fmt.Printf("Health check failed for %s: %v\n", checker.Name(), err)
	}

	return check
}

func (hm *HealthMonitor) performCheckWithRetry(ctx context.Context, checker HealthChecker) (HealthCheck, error) {
	var lastCheck HealthCheck
	var lastErr error

	delay := hm.config.RetryDelay

	for attempt := 0; attempt <= hm.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return lastCheck, ctx.Err()
			case <-time.After(delay):
				delay *= 2 // Exponential backoff
			}
		}

		check := checker.Check(ctx)

		if check.Status == HealthStatusHealthy {
			return check, nil
		}

		lastCheck = check
		if check.Error != "" {
			lastErr = fmt.Errorf("%s", check.Error)
		}
	}

	return lastCheck, lastErr
}

func (hm *HealthMonitor) calculateOverallStatus(checks map[string]HealthCheck) HealthStatus {
	if len(checks) == 0 {
		return HealthStatusUnknown
	}

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	for _, check := range checks {
		switch check.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		case HealthStatusUnhealthy:
			unhealthyCount++
		}
	}

	// If any component is unhealthy, overall is unhealthy
	if unhealthyCount > 0 {
		return HealthStatusUnhealthy
	}

	// If any component is degraded, overall is degraded
	if degradedCount > 0 {
		return HealthStatusDegraded
	}

	// If all components are healthy, overall is healthy
	if healthyCount == len(checks) {
		return HealthStatusHealthy
	}

	return HealthStatusUnknown
}

func (hm *HealthMonitor) generateSummary(checks map[string]HealthCheck, overallStatus HealthStatus) string {
	total := len(checks)
	healthy := 0
	degraded := 0
	unhealthy := 0

	for _, check := range checks {
		switch check.Status {
		case HealthStatusHealthy:
			healthy++
		case HealthStatusDegraded:
			degraded++
		case HealthStatusUnhealthy:
			unhealthy++
		}
	}

	switch overallStatus {
	case HealthStatusHealthy:
		return fmt.Sprintf("All %d components are healthy", total)
	case HealthStatusDegraded:
		return fmt.Sprintf("%d healthy, %d degraded, %d unhealthy out of %d components", healthy, degraded, unhealthy, total)
	case HealthStatusUnhealthy:
		return fmt.Sprintf("%d unhealthy components detected out of %d total", unhealthy, total)
	default:
		return fmt.Sprintf("Unknown status for %d components", total)
	}
}

// Specific health checkers for different storage types

// ConnectionPoolHealthChecker checks the health of a connection pool
type ConnectionPoolHealthChecker struct {
	name string
	pool ConnectionPool
}

// NewConnectionPoolHealthChecker creates a health checker for connection pools
func NewConnectionPoolHealthChecker(name string, pool ConnectionPool) *ConnectionPoolHealthChecker {
	return &ConnectionPoolHealthChecker{
		name: name,
		pool: pool,
	}
}

// Name returns the name of this health checker
func (c *ConnectionPoolHealthChecker) Name() string {
	return c.name
}

// Check performs a health check on the connection pool
func (c *ConnectionPoolHealthChecker) Check(ctx context.Context) HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:      c.name,
		Timestamp: start,
	}

	// Check pool health
	if err := c.pool.Health(ctx); err != nil {
		check.Status = HealthStatusUnhealthy
		check.Error = err.Error()
		check.Message = "Connection pool health check failed"
		return check
	}

	// Get pool statistics
	stats := c.pool.Stats()

	// Determine status based on pool statistics
	utilizationRate := float64(stats.ActiveConnections) / float64(stats.TotalConnections)

	if stats.TotalConnections == 0 {
		check.Status = HealthStatusUnhealthy
		check.Message = "No connections in pool"
	} else if utilizationRate > 0.9 {
		check.Status = HealthStatusDegraded
		check.Message = fmt.Sprintf("High connection utilization: %.1f%%", utilizationRate*100)
	} else {
		check.Status = HealthStatusHealthy
		check.Message = fmt.Sprintf("Pool healthy: %d/%d connections active", stats.ActiveConnections, stats.TotalConnections)
	}

	// Add metadata
	check.Metadata = map[string]interface{}{
		"total_connections":   stats.TotalConnections,
		"active_connections":  stats.ActiveConnections,
		"idle_connections":    stats.IdleConnections,
		"connections_created": stats.ConnectionsCreated,
		"connections_closed":  stats.ConnectionsClosed,
		"connections_failed":  stats.ConnectionsFailed,
		"utilization_rate":    utilizationRate,
	}

	check.Duration = time.Since(start)
	return check
}

// StorageHealthChecker checks the health of a storage backend
type StorageHealthChecker struct {
	name    string
	storage Storage
}

// NewStorageHealthChecker creates a health checker for storage backends
func NewStorageHealthChecker(name string, s Storage) *StorageHealthChecker {
	return &StorageHealthChecker{
		name:    name,
		storage: s,
	}
}

// Name returns the name of this health checker
func (s *StorageHealthChecker) Name() string {
	return s.name
}

// Check performs a health check on the storage backend
func (s *StorageHealthChecker) Check(ctx context.Context) HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:      s.name,
		Timestamp: start,
	}

	// Check storage health
	if err := s.storage.Health(ctx); err != nil {
		check.Status = HealthStatusUnhealthy
		check.Error = err.Error()
		check.Message = "Storage health check failed"
	} else {
		check.Status = HealthStatusHealthy
		check.Message = "Storage is healthy"
	}

	check.Duration = time.Since(start)
	return check
}

// RelationalStoreHealthChecker checks the health of a relational store
type RelationalStoreHealthChecker struct {
	name  string
	store RelationalStore
}

// NewRelationalStoreHealthChecker creates a health checker for relational stores
func NewRelationalStoreHealthChecker(name string, store RelationalStore) *RelationalStoreHealthChecker {
	return &RelationalStoreHealthChecker{
		name:  name,
		store: store,
	}
}

// Name returns the name of this health checker
func (r *RelationalStoreHealthChecker) Name() string {
	return r.name
}

// Check performs a health check on the relational store
func (r *RelationalStoreHealthChecker) Check(ctx context.Context) HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:      r.name,
		Timestamp: start,
	}

	// Check store health
	if err := r.store.Health(ctx); err != nil {
		check.Status = HealthStatusUnhealthy
		check.Error = err.Error()
		check.Message = "Relational store health check failed"
		check.Duration = time.Since(start)
		return check
	}

	// Get additional metrics if available
	if dataPointCount, err := r.store.GetDataPointCount(ctx); err == nil {
		if sessionCount, err := r.store.GetSessionCount(ctx); err == nil {
			check.Metadata = map[string]interface{}{
				"datapoint_count": dataPointCount,
				"session_count":   sessionCount,
			}
		}
	}

	check.Status = HealthStatusHealthy
	check.Message = "Relational store is healthy"
	check.Duration = time.Since(start)
	return check
}
