package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOllamaDimension(t *testing.T) {
	require.Equal(t, 1024, ollamaDimension("mxbai-embed-large"))
	require.Equal(t, 384, ollamaDimension("all-minilm"))
	require.Equal(t, 768, ollamaDimension("unknown"))
}

func TestNewOllamaEmbeddingProvider_Defaults(t *testing.T) {
	p := NewOllamaEmbeddingProvider("", "", 0)
	require.Equal(t, "nomic-embed-text", p.GetModel())
	require.Equal(t, 768, p.GetDimensions())
}

func TestGenerateEmbeddingAndHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[0.1,0.2,0.3]}`))
	}))
	defer srv.Close()

	p := NewOllamaEmbeddingProvider(srv.URL, "m", 3)
	emb, err := p.GenerateEmbedding(context.Background(), "hello")
	require.NoError(t, err)
	require.Len(t, emb, 3)
	require.NoError(t, p.Health(context.Background()))
}

