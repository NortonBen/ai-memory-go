package lmstudio

import "testing"

import "github.com/stretchr/testify/require"

func TestNewLMStudioEmbeddingProvider_Defaults(t *testing.T) {
	p := NewLMStudioEmbeddingProvider("", "")
	require.NotNil(t, p)
	require.Equal(t, "http://localhost:1234/v1/embeddings", p.Endpoint)
	require.Equal(t, "nomic-embed-text-v1.5", p.GetModel())
}

func TestNewLMStudioEmbeddingProvider_Custom(t *testing.T) {
	p := NewLMStudioEmbeddingProvider("http://x/v1", "m")
	require.Equal(t, "http://x/v1/embeddings", p.Endpoint)
	require.Equal(t, "m", p.GetModel())
}

