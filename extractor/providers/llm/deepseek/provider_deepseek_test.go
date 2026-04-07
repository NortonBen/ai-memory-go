package deepseek

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekGenerateCompletionAndHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := NewDeepSeekProvider("k", "")
	p.SetEndpoint(srv.URL)
	got, err := p.GenerateCompletion(context.Background(), "hello")
	require.NoError(t, err)
	require.Equal(t, "ok", got)
	require.NoError(t, p.Health(context.Background()))
}

func TestDeepSeekDoRequestErrorsAndSetters(t *testing.T) {
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer errSrv.Close()

	p := NewDeepSeekProvider("k", "m")
	p.SetEndpoint(errSrv.URL)

	_, err := p.doRequest(context.Background(), DeepSeekRequest{Model: "m", Stream: false})
	require.Error(t, err)

	require.NoError(t, p.SetModel("new-model"))
	require.Equal(t, "new-model", p.GetModel())
	p.SetAPIKey("new-key")
	p.SetTimeout(2 * time.Second)
	require.Equal(t, 2*time.Second, p.timeout)
}

func TestDeepSeekExtraMethods(t *testing.T) {
	p := NewDeepSeekProvider("k", "m")
	require.Equal(t, extractor.ProviderDeepSeek, p.GetProviderType())
	require.Equal(t, 8192, p.GetMaxTokens())
	usage, err := p.GetUsage(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage)
	rl, err := p.GetRateLimit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rl)

	_, err = p.ExtractEntities(context.Background(), "x")
	require.Error(t, err)
	_, err = p.ExtractRelationships(context.Background(), "x", nil)
	require.Error(t, err)
	_, err = p.ExtractWithCustomSchema(context.Background(), "x", nil)
	require.Error(t, err)
	_, err = p.GenerateWithContext(context.Background(), nil, nil)
	require.Error(t, err)
	err = p.GenerateStreamingCompletion(context.Background(), "x", nil)
	require.Error(t, err)
	require.NoError(t, p.Close())
}

