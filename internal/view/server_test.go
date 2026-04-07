package view

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

func TestPrimitiveHelpers(t *testing.T) {
	require.Equal(t, 3, boundInt(3, 1, 5))
	require.Equal(t, 1, boundInt(-1, 1, 5))
	require.Equal(t, 5, boundInt(9, 1, 5))
	require.Equal(t, 9, max(9, 1))
	require.Equal(t, 1, max(0, 1))

	require.Equal(t, "", errString(nil))
	require.Equal(t, "x", errString(errors.New("x")))
	require.Equal(t, "", nodeErrString(nil))
	require.Equal(t, "y", nodeErrString(errors.New("y")))

	require.True(t, asBool(true))
	require.True(t, asBool("true"))
	require.True(t, asBool(1))
	require.True(t, asBool(int64(1)))
	require.True(t, asBool(float64(1)))
	require.False(t, asBool("no"))
}

func TestMetadataHelpers(t *testing.T) {
	meta := map[string]interface{}{
		"a": "v",
		"i": 2,
		"f": float64(3),
	}
	s, ok := metadataString(meta, "a")
	require.True(t, ok)
	require.Equal(t, "v", s)
	_, ok = metadataString(meta, "missing")
	require.False(t, ok)

	i, ok := metadataInt(meta, "i")
	require.True(t, ok)
	require.Equal(t, 2, i)

	i, ok = metadataInt(meta, "f")
	require.True(t, ok)
	require.Equal(t, 3, i)

	_, ok = metadataInt(meta, "a")
	require.False(t, ok)
}

func TestDataPointKindAndFilter(t *testing.T) {
	input := &schema.DataPoint{
		ID:       "in",
		Metadata: map[string]interface{}{"is_input": true},
	}
	chunk := &schema.DataPoint{
		ID:               "ch",
		ProcessingStatus: schema.StatusCompleted,
		Metadata: map[string]interface{}{
			"is_chunk":    true,
			"chunk_index": 0,
		},
	}
	legacy := &schema.DataPoint{ID: "legacy", Metadata: nil}

	require.True(t, isInputDP(input))
	require.False(t, isChunkDP(input))
	require.True(t, isChunkDP(chunk))
	require.Equal(t, "chunk", dataPointKind(chunk))
	require.True(t, isInputDP(legacy))

	all := []*schema.DataPoint{input, chunk, legacy, nil}
	require.Len(t, filterDataPoints(all, "input", "all"), 2)
	require.Len(t, filterDataPoints(all, "chunk", "all"), 1)
	require.Len(t, filterDataPoints(all, "processed", string(schema.StatusCompleted)), 1)
}

func TestParseAndQueryHelpers(t *testing.T) {
	r := httptest.NewRequest("GET", "/?n=10&b=true&b2=1", nil)
	require.Equal(t, 10, parseInt(r, "n", 1))
	require.Equal(t, 7, parseInt(r, "missing", 7))
	require.True(t, queryBool(r, "b"))
	require.True(t, queryBool(r, "b2"))
	require.False(t, queryBool(r, "none"))
}

func TestPrimaryLabelAndCards(t *testing.T) {
	dp := &schema.DataPoint{
		ID: "1",
		Metadata: map[string]interface{}{
			schema.MetadataKeyPrimaryLabel: "test-label",
			"is_chunk":                     true,
		},
	}
	require.Equal(t, "test-label", primaryLabelFromDP(dp))
	require.Equal(t, "", primaryLabelFromDP(nil))

	cards := toDataPointCards([]*schema.DataPoint{dp})
	require.Len(t, cards, 1)
	require.Equal(t, "chunk", cards[0]["item_type"])
}

