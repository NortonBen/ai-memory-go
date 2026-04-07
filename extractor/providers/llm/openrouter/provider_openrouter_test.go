package openrouter

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/require"
)

type rt struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (r *rt) RoundTrip(req *http.Request) (*http.Response, error) { return r.fn(req) }

func TestNewOpenRouterProvider_ValidationAndType(t *testing.T) {
	_, err := NewOpenRouterProvider("", "", "", "")
	require.Error(t, err)

	p, err := NewOpenRouterProvider("k", "", "", "")
	require.NoError(t, err)
	require.Equal(t, extractor.ProviderOpenRouter, p.GetProviderType())
}

func TestOpenRouterTransportSetsHeaders(t *testing.T) {
	tr := &openRouterTransport{
		base: &rt{fn: func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://x", req.Header.Get("X-Origin-Site"))
			require.Equal(t, "app", req.Header.Get("X-Title"))
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}},
		siteURL: "https://x",
		appName: "app",
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	_, err := tr.RoundTrip(req)
	require.NoError(t, err)
}

