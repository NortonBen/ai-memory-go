package openrouter

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeRT struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (f *fakeRT) RoundTrip(req *http.Request) (*http.Response, error) { return f.fn(req) }

func TestInferDimension(t *testing.T) {
	require.Equal(t, 3072, inferDimension("openai/text-embedding-3-large"))
	require.Equal(t, 1024, inferDimension("cohere/embed-multilingual-v3.0"))
	require.Equal(t, 1536, inferDimension("openai/text-embedding-3-small"))
}

func TestOpenRouterTransportHeaders(t *testing.T) {
	rt := &openRouterTransport{
		base: &fakeRT{fn: func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer k", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))
			require.Equal(t, "https://site", req.Header.Get("X-Origin-Site"))
			require.Equal(t, "app", req.Header.Get("X-Title"))
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}},
		apiKey:  "k",
		siteURL: "https://site",
		appName: "app",
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
}

func TestNewProviderDefaults(t *testing.T) {
	p := NewOpenRouterEmbeddingProvider(OpenRouterConfig{})
	require.NotNil(t, p)
	require.Equal(t, defaultOpenRouterModel, p.GetModel())
	require.Equal(t, 1536, p.GetDimensions())
}

