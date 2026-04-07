package storage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

type checker struct {
	name   string
	status HealthStatus
	errMsg string
}

func (c *checker) Name() string { return c.name }
func (c *checker) Check(ctx context.Context) HealthCheck {
	return HealthCheck{Name: c.name, Status: c.status, Error: c.errMsg}
}

type retryChecker struct {
	name  string
	count int32
}

func (c *retryChecker) Name() string { return c.name }
func (c *retryChecker) Check(ctx context.Context) HealthCheck {
	n := atomic.AddInt32(&c.count, 1)
	if n < 2 {
		return HealthCheck{Name: c.name, Status: HealthStatusUnhealthy, Error: "first fail"}
	}
	return HealthCheck{Name: c.name, Status: HealthStatusHealthy}
}

type poolMock struct {
	healthErr error
	stats     *PoolStats
}

func (p *poolMock) Get(ctx context.Context) (schema.Connection, error) { return nil, nil }
func (p *poolMock) Put(conn schema.Connection) error                    { return nil }
func (p *poolMock) Close() error                                        { return nil }
func (p *poolMock) Stats() *PoolStats {
	if p.stats != nil {
		return p.stats
	}
	return &PoolStats{}
}
func (p *poolMock) Health(ctx context.Context) error { return p.healthErr }

func TestHealthMonitor_CheckAllAndStatus(t *testing.T) {
	hm := NewHealthMonitor(&HealthMonitorConfig{
		CheckInterval:   5 * time.Millisecond,
		CheckTimeout:    50 * time.Millisecond,
		EnableAutoCheck: false,
		EnableRetry:     false,
	})
	hm.AddChecker(&checker{name: "ok", status: HealthStatusHealthy})
	hm.AddChecker(&checker{name: "bad", status: HealthStatusUnhealthy, errMsg: "x"})

	report := hm.CheckAll(context.Background())
	require.Equal(t, HealthStatusUnhealthy, report.OverallStatus)
	require.Len(t, report.Checks, 2)
	require.False(t, hm.IsHealthy())
	require.Equal(t, HealthStatusUnhealthy, hm.GetStatus())
	require.NotNil(t, hm.GetLastReport())
}

func TestHealthMonitor_CheckOneAndRetry(t *testing.T) {
	hm := NewHealthMonitor(&HealthMonitorConfig{
		CheckTimeout:    100 * time.Millisecond,
		EnableAutoCheck: false,
		EnableRetry:     true,
		MaxRetries:      1,
		RetryDelay:      1 * time.Millisecond,
	})
	rc := &retryChecker{name: "retry"}
	hm.AddChecker(rc)

	chk, err := hm.CheckOne(context.Background(), "retry")
	require.NoError(t, err)
	require.Equal(t, HealthStatusHealthy, chk.Status)

	_, err = hm.CheckOne(context.Background(), "missing")
	require.Error(t, err)
}

func TestHealthMonitor_StartStopAndCallbacks(t *testing.T) {
	var failureCalled int32
	hm := NewHealthMonitor(&HealthMonitorConfig{
		CheckInterval:   5 * time.Millisecond,
		CheckTimeout:    20 * time.Millisecond,
		EnableAutoCheck: true,
		EnableRetry:     false,
	})
	hm.AddChecker(&checker{name: "bad", status: HealthStatusUnhealthy, errMsg: "e"})
	hm.SetFailureCallback(func(name string, check HealthCheck) {
		atomic.AddInt32(&failureCalled, 1)
	})

	require.NoError(t, hm.Start())
	require.Error(t, hm.Start())
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, hm.Stop())
	require.GreaterOrEqual(t, atomic.LoadInt32(&failureCalled), int32(1))
}

func TestConnectionPoolHealthChecker(t *testing.T) {
	unhealthy := NewConnectionPoolHealthChecker("p1", &poolMock{healthErr: errors.New("down")})
	c := unhealthy.Check(context.Background())
	require.Equal(t, HealthStatusUnhealthy, c.Status)

	zero := NewConnectionPoolHealthChecker("p2", &poolMock{stats: &PoolStats{TotalConnections: 0}})
	c = zero.Check(context.Background())
	require.Equal(t, HealthStatusUnhealthy, c.Status)

	degraded := NewConnectionPoolHealthChecker("p3", &poolMock{stats: &PoolStats{TotalConnections: 10, ActiveConnections: 10}})
	c = degraded.Check(context.Background())
	require.Equal(t, HealthStatusDegraded, c.Status)

	healthy := NewConnectionPoolHealthChecker("p4", &poolMock{stats: &PoolStats{TotalConnections: 10, ActiveConnections: 2}})
	c = healthy.Check(context.Background())
	require.Equal(t, HealthStatusHealthy, c.Status)
}

