package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// Mock health checker for testing
type mockHealthChecker struct {
	name       string
	status     HealthStatus
	err        error
	duration   time.Duration
	checkCount int
	mu         sync.Mutex
}

func newMockHealthChecker(name string) *mockHealthChecker {
	return &mockHealthChecker{
		name:     name,
		status:   HealthStatusHealthy,
		duration: 10 * time.Millisecond,
	}
}

func (m *mockHealthChecker) Name() string {
	return m.name
}

func (m *mockHealthChecker) Check(ctx context.Context) HealthCheck {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkCount++

	// Simulate check duration
	if m.duration > 0 {
		time.Sleep(m.duration)
	}

	check := HealthCheck{
		Name:      m.name,
		Status:    m.status,
		Duration:  m.duration,
		Timestamp: time.Now(),
	}

	if m.err != nil {
		check.Error = m.err.Error()
		check.Status = HealthStatusUnhealthy
	}

	return check
}

func (m *mockHealthChecker) SetStatus(status HealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
}

func (m *mockHealthChecker) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *mockHealthChecker) SetDuration(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.duration = duration
}

func (m *mockHealthChecker) GetCheckCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checkCount
}

func TestDefaultHealthMonitorConfig(t *testing.T) {
	config := DefaultHealthMonitorConfig()

	if config.CheckInterval != 30*time.Second {
		t.Errorf("Expected CheckInterval 30s, got %v", config.CheckInterval)
	}

	if config.CheckTimeout != 10*time.Second {
		t.Errorf("Expected CheckTimeout 10s, got %v", config.CheckTimeout)
	}

	if config.FailureThreshold != 3 {
		t.Errorf("Expected FailureThreshold 3, got %d", config.FailureThreshold)
	}

	if !config.EnableAutoCheck {
		t.Error("Expected EnableAutoCheck to be true")
	}

	if !config.EnableRetry {
		t.Error("Expected EnableRetry to be true")
	}
}

func TestNewHealthMonitor(t *testing.T) {
	// Test with nil config (should use defaults)
	monitor := NewHealthMonitor(nil)
	if monitor == nil {
		t.Fatal("Expected non-nil monitor")
	}

	if monitor.config.CheckInterval != 30*time.Second {
		t.Error("Should use default config when nil provided")
	}

	// Test with custom config
	config := &HealthMonitorConfig{
		CheckInterval: 1 * time.Minute,
		CheckTimeout:  5 * time.Second,
	}

	monitor = NewHealthMonitor(config)
	if monitor.config.CheckInterval != 1*time.Minute {
		t.Error("Should use provided config")
	}
}

func TestHealthMonitorAddRemoveChecker(t *testing.T) {
	monitor := NewHealthMonitor(nil)

	checker1 := newMockHealthChecker("test1")
	checker2 := newMockHealthChecker("test2")

	// Add checkers
	monitor.AddChecker(checker1)
	monitor.AddChecker(checker2)

	// Check they were added
	ctx := context.Background()
	report := monitor.CheckAll(ctx)

	if len(report.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(report.Checks))
	}

	if _, exists := report.Checks["test1"]; !exists {
		t.Error("test1 checker not found")
	}

	if _, exists := report.Checks["test2"]; !exists {
		t.Error("test2 checker not found")
	}

	// Remove one checker
	monitor.RemoveChecker("test1")

	report = monitor.CheckAll(ctx)
	if len(report.Checks) != 1 {
		t.Errorf("Expected 1 check after removal, got %d", len(report.Checks))
	}

	if _, exists := report.Checks["test1"]; exists {
		t.Error("test1 checker should have been removed")
	}
}

func TestHealthMonitorCheckAll(t *testing.T) {
	monitor := NewHealthMonitor(nil)

	checker1 := newMockHealthChecker("healthy")
	checker1.SetStatus(HealthStatusHealthy)

	checker2 := newMockHealthChecker("degraded")
	checker2.SetStatus(HealthStatusDegraded)

	checker3 := newMockHealthChecker("unhealthy")
	checker3.SetStatus(HealthStatusUnhealthy)

	monitor.AddChecker(checker1)
	monitor.AddChecker(checker2)
	monitor.AddChecker(checker3)

	ctx := context.Background()
	report := monitor.CheckAll(ctx)

	// Check overall status (should be unhealthy due to one unhealthy component)
	if report.OverallStatus != HealthStatusUnhealthy {
		t.Errorf("Expected overall status unhealthy, got %s", report.OverallStatus)
	}

	// Check individual statuses
	if report.Checks["healthy"].Status != HealthStatusHealthy {
		t.Error("healthy checker should be healthy")
	}

	if report.Checks["degraded"].Status != HealthStatusDegraded {
		t.Error("degraded checker should be degraded")
	}

	if report.Checks["unhealthy"].Status != HealthStatusUnhealthy {
		t.Error("unhealthy checker should be unhealthy")
	}

	// Check report metadata
	if report.Timestamp.IsZero() {
		t.Error("Report timestamp should be set")
	}

	if report.Duration <= 0 {
		t.Error("Report duration should be positive")
	}

	if report.Summary == "" {
		t.Error("Report summary should not be empty")
	}
}

func TestHealthMonitorCheckOne(t *testing.T) {
	monitor := NewHealthMonitor(nil)

	checker := newMockHealthChecker("test")
	checker.SetStatus(HealthStatusHealthy)
	monitor.AddChecker(checker)

	ctx := context.Background()

	// Check existing checker
	check, err := monitor.CheckOne(ctx, "test")
	if err != nil {
		t.Fatalf("Failed to check one: %v", err)
	}

	if check.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s", check.Status)
	}

	// Check non-existent checker
	_, err = monitor.CheckOne(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent checker")
	}
}

func TestHealthMonitorOverallStatusCalculation(t *testing.T) {
	monitor := NewHealthMonitor(nil)
	ctx := context.Background()

	// Test all healthy
	checker1 := newMockHealthChecker("test1")
	checker1.SetStatus(HealthStatusHealthy)
	checker2 := newMockHealthChecker("test2")
	checker2.SetStatus(HealthStatusHealthy)

	monitor.AddChecker(checker1)
	monitor.AddChecker(checker2)

	report := monitor.CheckAll(ctx)
	if report.OverallStatus != HealthStatusHealthy {
		t.Errorf("Expected healthy overall status, got %s", report.OverallStatus)
	}

	// Test with degraded component
	checker2.SetStatus(HealthStatusDegraded)
	report = monitor.CheckAll(ctx)
	if report.OverallStatus != HealthStatusDegraded {
		t.Errorf("Expected degraded overall status, got %s", report.OverallStatus)
	}

	// Test with unhealthy component
	checker2.SetStatus(HealthStatusUnhealthy)
	report = monitor.CheckAll(ctx)
	if report.OverallStatus != HealthStatusUnhealthy {
		t.Errorf("Expected unhealthy overall status, got %s", report.OverallStatus)
	}

	// Test with no checkers
	monitor.RemoveChecker("test1")
	monitor.RemoveChecker("test2")
	report = monitor.CheckAll(ctx)
	if report.OverallStatus != HealthStatusUnknown {
		t.Errorf("Expected unknown overall status with no checkers, got %s", report.OverallStatus)
	}
}

func TestHealthMonitorStartStop(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckInterval = 50 * time.Millisecond
	config.EnableAutoCheck = true

	monitor := NewHealthMonitor(config)

	checker := newMockHealthChecker("test")
	monitor.AddChecker(checker)

	// Start monitoring
	err := monitor.Start()
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	// Wait for a few checks
	time.Sleep(150 * time.Millisecond)

	// Stop monitoring
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop monitor: %v", err)
	}

	// Check that some checks were performed
	checkCount := checker.GetCheckCount()
	if checkCount < 2 {
		t.Errorf("Expected at least 2 checks, got %d", checkCount)
	}

	// Test starting already running monitor
	monitor.Start()
	err = monitor.Start()
	if err == nil {
		t.Error("Expected error when starting already running monitor")
	}
	monitor.Stop()
}

func TestHealthMonitorCallbacks(t *testing.T) {
	monitor := NewHealthMonitor(nil)

	var statusChanges []string
	var failures []string
	var mu sync.Mutex

	// Set callbacks
	monitor.SetStatusChangeCallback(func(name string, oldStatus, newStatus HealthStatus) {
		mu.Lock()
		defer mu.Unlock()
		statusChanges = append(statusChanges, name+":"+string(oldStatus)+"->"+string(newStatus))
	})

	monitor.SetFailureCallback(func(name string, check HealthCheck) {
		mu.Lock()
		defer mu.Unlock()
		failures = append(failures, name)
	})

	checker := newMockHealthChecker("test")
	checker.SetStatus(HealthStatusHealthy)
	monitor.AddChecker(checker)

	ctx := context.Background()

	// Initial check
	monitor.CheckAll(ctx)

	// Change status to unhealthy
	checker.SetStatus(HealthStatusUnhealthy)
	monitor.CheckAll(ctx)

	// Wait for callbacks to be called
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(statusChanges) != 1 {
		t.Errorf("Expected 1 status change, got %d", len(statusChanges))
	}

	if len(failures) != 1 {
		t.Errorf("Expected 1 failure callback, got %d", len(failures))
	}
}

func TestHealthMonitorRetry(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.EnableRetry = true
	config.MaxRetries = 2
	config.RetryDelay = 10 * time.Millisecond

	monitor := NewHealthMonitor(config)

	checker := newMockHealthChecker("test")
	checker.SetError(errors.New("temporary failure"))
	monitor.AddChecker(checker)

	ctx := context.Background()

	start := time.Now()
	report := monitor.CheckAll(ctx)
	duration := time.Since(start)

	// Should have taken time for retries
	expectedMinDuration := time.Duration(config.MaxRetries) * config.RetryDelay
	if duration < expectedMinDuration {
		t.Errorf("Expected at least %v duration for retries, got %v", expectedMinDuration, duration)
	}

	// Should still be unhealthy after retries
	if report.Checks["test"].Status != HealthStatusUnhealthy {
		t.Error("Expected unhealthy status after failed retries")
	}

	// Check count should include retries
	checkCount := checker.GetCheckCount()
	expectedChecks := config.MaxRetries + 1 // Initial attempt + retries
	if checkCount != expectedChecks {
		t.Errorf("Expected %d checks (including retries), got %d", expectedChecks, checkCount)
	}
}

func TestHealthMonitorTimeout(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	config.CheckTimeout = 50 * time.Millisecond

	monitor := NewHealthMonitor(config)

	checker := newMockHealthChecker("test")
	checker.SetDuration(100 * time.Millisecond) // Longer than timeout
	monitor.AddChecker(checker)

	ctx := context.Background()

	start := time.Now()
	report := monitor.CheckAll(ctx)
	duration := time.Since(start)

	// Should have been interrupted by timeout
	if duration > 80*time.Millisecond {
		t.Errorf("Check took too long, expected ~50ms, got %v", duration)
	}

	// Check should still complete (mock doesn't respect context cancellation)
	if len(report.Checks) != 1 {
		t.Error("Expected check to complete despite timeout")
	}
}

func TestHealthMonitorConcurrency(t *testing.T) {
	monitor := NewHealthMonitor(nil)

	// Add multiple checkers
	for i := 0; i < 10; i++ {
		checker := newMockHealthChecker(fmt.Sprintf("test%d", i))
		checker.SetDuration(10 * time.Millisecond)
		monitor.AddChecker(checker)
	}

	ctx := context.Background()

	// Run multiple concurrent health checks
	var wg sync.WaitGroup
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			report := monitor.CheckAll(ctx)
			if len(report.Checks) != 10 {
				errors <- fmt.Errorf("expected 10 checks, got %d", len(report.Checks))
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent check error: %v", err)
	}
}

func TestHealthMonitorIsHealthy(t *testing.T) {
	monitor := NewHealthMonitor(nil)

	// No report yet
	if monitor.IsHealthy() {
		t.Error("Should not be healthy with no report")
	}

	checker := newMockHealthChecker("test")
	checker.SetStatus(HealthStatusHealthy)
	monitor.AddChecker(checker)

	ctx := context.Background()
	monitor.CheckAll(ctx)

	// Should be healthy now
	if !monitor.IsHealthy() {
		t.Error("Should be healthy with healthy checker")
	}

	// Change to unhealthy
	checker.SetStatus(HealthStatusUnhealthy)
	monitor.CheckAll(ctx)

	if monitor.IsHealthy() {
		t.Error("Should not be healthy with unhealthy checker")
	}
}

func TestHealthMonitorGetStatus(t *testing.T) {
	monitor := NewHealthMonitor(nil)

	// No report yet
	if monitor.GetStatus() != HealthStatusUnknown {
		t.Error("Should return unknown status with no report")
	}

	checker := newMockHealthChecker("test")
	checker.SetStatus(HealthStatusDegraded)
	monitor.AddChecker(checker)

	ctx := context.Background()
	monitor.CheckAll(ctx)

	if monitor.GetStatus() != HealthStatusDegraded {
		t.Error("Should return degraded status")
	}
}

func TestConnectionPoolHealthChecker(t *testing.T) {
	factory := newMockConnectionFactory()
	config := DefaultPoolConfig()
	config.MinIdleConnections = 2
	config.MaxConnections = 5

	pool, err := NewGenericConnectionPool(factory, config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	checker := NewConnectionPoolHealthChecker("test_pool", pool)

	ctx := context.Background()
	check := checker.Check(ctx)

	if check.Name != "test_pool" {
		t.Errorf("Expected name 'test_pool', got %s", check.Name)
	}

	if check.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s", check.Status)
	}

	if check.Metadata == nil {
		t.Error("Expected metadata to be set")
	}

	// Check metadata contains expected fields
	if _, exists := check.Metadata["total_connections"]; !exists {
		t.Error("Expected total_connections in metadata")
	}

	if _, exists := check.Metadata["utilization_rate"]; !exists {
		t.Error("Expected utilization_rate in metadata")
	}
}

func TestStorageHealthChecker(t *testing.T) {
	// Create a mock storage that implements the Storage interface
	mockStorage := &mockStorage{}

	checker := NewStorageHealthChecker("test_storage", mockStorage)

	ctx := context.Background()
	check := checker.Check(ctx)

	if check.Name != "test_storage" {
		t.Errorf("Expected name 'test_storage', got %s", check.Name)
	}

	if check.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s", check.Status)
	}

	// Test with failing storage
	mockStorage.healthErr = errors.New("storage failure")
	check = checker.Check(ctx)

	if check.Status != HealthStatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", check.Status)
	}

	if check.Error != "storage failure" {
		t.Errorf("Expected error message 'storage failure', got %s", check.Error)
	}
}

// Mock storage for testing
type mockStorage struct {
	healthErr error
}

func (m *mockStorage) StoreDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error {
	return nil
}

func (m *mockStorage) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	return nil, nil
}

func (m *mockStorage) UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error {
	return nil
}

func (m *mockStorage) DeleteDataPoint(ctx context.Context, id string) error {
	return nil
}

func (m *mockStorage) DeleteDataPointsBySession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockStorage) QueryDataPoints(ctx context.Context, query *DataPointQuery) ([]*schema.DataPoint, error) {
	return nil, nil
}

func (m *mockStorage) StoreSession(ctx context.Context, session *schema.MemorySession) error {
	return nil
}

func (m *mockStorage) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) {
	return nil, nil
}

func (m *mockStorage) UpdateSession(ctx context.Context, session *schema.MemorySession) error {
	return nil
}

func (m *mockStorage) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockStorage) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) {
	return nil, nil
}

func (m *mockStorage) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error {
	return nil
}

func (m *mockStorage) AddMessageToSession(ctx context.Context, sessionID string, message schema.Message) error {
	return nil
}

func (m *mockStorage) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	return nil, nil
}


func (m *mockStorage) DeleteBatch(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockStorage) Health(ctx context.Context) error {
	return m.healthErr
}

func (m *mockStorage) Close() error {
	return nil
}
