package lmstudio

import (
	"testing"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/require"
)

func TestCleanJSONResponse(t *testing.T) {
	in := "```json\n{\"a\":1}\n```"
	require.Equal(t, "{\"a\":1}", cleanJSONResponse(in))
	require.Equal(t, "{\"a\":1}", cleanJSONResponse(" {\"a\":1} "))
}

func TestNewLMStudioProviderDefaultsAndType(t *testing.T) {
	p, err := NewLMStudioProvider("", "")
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, extractor.ProviderLMStudio, p.GetProviderType())
}

