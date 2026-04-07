package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/require"
)

func TestOllamaGenerateCompletionAndListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response":"hello"}`))
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"m1"},{"name":"m2"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "m")
	out, err := p.GenerateCompletion(context.Background(), "hi")
	require.NoError(t, err)
	require.Equal(t, "hello", out)

	models, err := p.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.NoError(t, p.Health(context.Background()))
}

func TestOllamaExtraMethods(t *testing.T) {
	p := NewOllamaProvider("http://localhost:11434", "m")
	require.Equal(t, extractor.ProviderOllama, p.GetProviderType())
	require.Equal(t, 4096, p.GetMaxTokens())
	require.NoError(t, p.SetModel("x"))
	p.SetTimeout(1 * time.Second)
	require.Equal(t, 1*time.Second, p.timeout)

	_, err := p.ExtractEntities(context.Background(), "x")
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

