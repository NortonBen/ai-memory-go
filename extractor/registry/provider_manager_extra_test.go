package registry

import (
	"context"
	"errors"
	"testing"
	"time"

	ext "github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/require"
)

func TestHealthChecker_EmbeddingPaths(t *testing.T) {
	hc := NewHealthChecker(20*time.Millisecond, 20*time.Millisecond, 2)
	mgr := NewEmbeddingProviderManager()
	require.NoError(t, mgr.AddProvider("m1", ext.NewMockEmbeddingProvider("m1", 8), 1))

	// direct path coverage
	hc.performEmbeddingHealthChecks(context.Background(), mgr)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	hc.StartEmbedding(ctx, mgr)
	time.Sleep(30 * time.Millisecond)
	hc.Stop()
}

func TestFailoverManager_ExecuteEmbeddingWithFailover(t *testing.T) {
	fm := NewFailoverManager()
	fm.SetEnabled(false)

	empty := NewEmbeddingProviderManager()
	err := fm.ExecuteEmbeddingWithFailover(context.Background(), nil, empty, func(provider ext.EmbeddingProvider) error {
		return nil
	})
	require.Error(t, err)

	mgr := NewEmbeddingProviderManager()
	require.NoError(t, mgr.AddProvider("p1", ext.NewMockEmbeddingProvider("m", 4), 1))
	called := 0
	err = fm.ExecuteEmbeddingWithFailover(context.Background(), []string{"p1"}, mgr, func(provider ext.EmbeddingProvider) error {
		called++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, called)

	// enabled path: retryable error then success
	fm.SetEnabled(true)
	fm.SetMaxRetries(1)
	attempt := 0
	err = fm.ExecuteEmbeddingWithFailover(context.Background(), []string{"p1"}, mgr, func(provider ext.EmbeddingProvider) error {
		attempt++
		if attempt == 1 {
			return ext.NewExtractorError("temporary_failure", "retry", 503)
		}
		return nil
	})
	require.NoError(t, err)

	// enabled path with no providers
	err = fm.ExecuteEmbeddingWithFailover(context.Background(), []string{}, mgr, func(provider ext.EmbeddingProvider) error {
		return nil
	})
	require.Error(t, err)
}

func TestDefaultProviderManager_ExtraPaths(t *testing.T) {
	pm := NewProviderManager().(*DefaultProviderManager)
	require.NotNil(t, pm.GetFailoverManager())
	require.NotNil(t, pm.GetAvailabilityTracker())

	require.NoError(t, pm.AddProvider("a", ext.NewMockLLMProvider(ext.ProviderOpenAI, "gpt"), 2))
	require.NoError(t, pm.AddProvider("b", ext.NewMockLLMProvider(ext.ProviderOpenAI, "gpt"), 1))

	pm.SetLoadBalancing(ext.LoadBalanceRoundRobin)
	_, err := pm.GetBestProvider(context.Background())
	require.NoError(t, err)
	pm.SetLoadBalancing(ext.LoadBalanceRandom)
	_, err = pm.GetBestProvider(context.Background())
	require.NoError(t, err)
	pm.SetLoadBalancing(ext.LoadBalanceLeastUsed)
	_, err = pm.GetBestProvider(context.Background())
	require.NoError(t, err)

	pm.RecordProviderSuccess("a", 10*time.Millisecond)
	pm.RecordProviderFailure("a")
	require.NotNil(t, pm.GetProviderMetrics("a"))
	require.NotEmpty(t, pm.GetAllProviderMetrics())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	pm.StartHealthChecks(ctx)
	time.Sleep(20 * time.Millisecond)
	pm.StopHealthChecks()
}

func TestDefaultEmbeddingProviderManager_ExtraPaths(t *testing.T) {
	em := NewEmbeddingProviderManager().(*DefaultEmbeddingProviderManager)
	require.NotNil(t, em.GetFailoverManager())
	require.NotNil(t, em.GetAvailabilityTracker())

	p1 := ext.NewMockEmbeddingProvider("m1", 8)
	p2 := ext.NewMockEmbeddingProvider("m2", 8)
	require.NoError(t, em.AddProvider("a", p1, 2))
	require.NoError(t, em.AddProvider("b", p2, 1))

	// List + health paths
	require.Len(t, em.ListProviders(), 2)
	require.Empty(t, em.HealthCheck(context.Background()))

	// selection strategies
	em.SetLoadBalancing(ext.EmbeddingLoadBalanceRoundRobin)
	_, err := em.GetBestProvider(context.Background())
	require.NoError(t, err)
	em.SetLoadBalancing(ext.EmbeddingLoadBalanceRandom)
	_, err = em.GetBestProvider(context.Background())
	require.NoError(t, err)
	em.SetLoadBalancing(ext.EmbeddingLoadBalanceLeastUsed)
	_, err = em.GetBestProvider(context.Background())
	require.NoError(t, err)
	em.SetLoadBalancing(ext.EmbeddingLoadBalanceCostBased)
	_, err = em.GetBestProvider(context.Background())
	require.NoError(t, err)
	em.SetLoadBalancing(ext.EmbeddingLoadBalanceLatency)
	_, err = em.GetBestProvider(context.Background())
	require.NoError(t, err)

	// failover operation wrapper
	callCount := 0
	err = em.ExecuteWithFailover(context.Background(), func(provider ext.EmbeddingProvider) error {
		callCount++
		if callCount == 1 {
			return errors.New("temporary failure")
		}
		return nil
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, callCount, 2)

	em.RecordProviderSuccess("a", 5*time.Millisecond)
	em.RecordProviderFailure("a")
	require.NotNil(t, em.GetProviderMetrics("a"))
	require.NotEmpty(t, em.GetAllProviderMetrics())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	em.StartHealthChecks(ctx)
	time.Sleep(20 * time.Millisecond)
	em.StopHealthChecks()
}

