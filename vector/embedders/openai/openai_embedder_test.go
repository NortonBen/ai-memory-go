package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateEmbeddingAndHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2],"index":0}],"model":"m"}`))
	}))
	defer srv.Close()

	p := NewOpenAIEmbeddingProvider("k", "")
	p.SetEndpoint(srv.URL)

	emb, err := p.GenerateEmbedding(context.Background(), "hello")
	require.NoError(t, err)
	require.Len(t, emb, 2)
	require.NoError(t, p.Health(context.Background()))
}

func TestGenerateBatchEmbeddings_IndexOrdering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","embedding":[9],"index":1},{"object":"embedding","embedding":[8],"index":0}],"model":"m"}`))
	}))
	defer srv.Close()

	p := NewOpenAIEmbeddingProvider("k", "text-embedding-3-small")
	p.SetEndpoint(srv.URL)

	out, err := p.GenerateBatchEmbeddings(context.Background(), []string{"a", "b"})
	require.NoError(t, err)
	require.Equal(t, float32(8), out[0][0])
	require.Equal(t, float32(9), out[1][0])
}

func TestGenerateBatchEmbeddings_InvalidIndexAndAPIError(t *testing.T) {
	badIndexSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","embedding":[1],"index":2}],"model":"m"}`))
	}))
	defer badIndexSrv.Close()

	p := NewOpenAIEmbeddingProvider("k", "")
	p.SetEndpoint(badIndexSrv.URL)
	_, err := p.GenerateBatchEmbeddings(context.Background(), []string{"a"})
	require.Error(t, err)

	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer errSrv.Close()

	p.SetEndpoint(errSrv.URL)
	_, err = p.GenerateEmbedding(context.Background(), "x")
	require.Error(t, err)
}

func TestModelDimensionSetters(t *testing.T) {
	p := NewOpenAIEmbeddingProvider("k", "text-embedding-3-small")
	require.Equal(t, 1536, p.GetDimensions())
	require.Equal(t, "text-embedding-3-small", p.GetModel())

	p.SetModel("text-embedding-3-large")
	require.Equal(t, 3072, p.GetDimensions())

	p.SetDimension(123)
	require.Equal(t, 123, p.GetDimensions())
	p.SetAPIKey("new-key")
	require.Equal(t, "new-key", p.APIKey)
}

