package processing

import (
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

func chunkWithHash(content, hash string) *schema.Chunk {
	return &schema.Chunk{Content: content, Hash: hash}
}

func TestChunkDeduplicator_BasicFlow(t *testing.T) {
	d := NewChunkDeduplicator()
	c1 := chunkWithHash("a", "h1")
	c2 := chunkWithHash("b", "h2")
	c3 := chunkWithHash("a2", "h1")

	require.False(t, d.IsDuplicate(c1))
	d.MarkAsSeen(c1)
	require.True(t, d.IsDuplicate(c1))

	out := d.DeduplicateChunks([]*schema.Chunk{c1, c2, c3})
	require.Len(t, out, 1)
	require.Equal(t, "h2", out[0].Hash)
	require.Equal(t, 2, d.GetSeenCount())

	d.Reset()
	require.Equal(t, 0, d.GetSeenCount())
}

func TestDeduplicateChunksGlobalAndStats(t *testing.T) {
	in := []*schema.Chunk{
		chunkWithHash("a", "h1"),
		chunkWithHash("b", "h2"),
		chunkWithHash("c", "h1"),
	}
	out := DeduplicateChunksGlobal(in)
	require.Len(t, out, 2)

	stats := ComputeDeduplicationStats(in, out)
	require.Equal(t, 3, stats.TotalChunks)
	require.Equal(t, 2, stats.UniqueChunks)
	require.Equal(t, 1, stats.DuplicateChunks)
	require.Greater(t, stats.DeduplicationRate, 0.0)
}

func TestSimilarityHelpers(t *testing.T) {
	h1 := ComputeSimilarityHash("The quick brown fox jumps")
	h2 := ComputeSimilarityHash("The quick brown fox jumps")
	require.Equal(t, h1, h2)

	require.Equal(t, 0, HammingDistance(h1, h2))
	require.Equal(t, -1, HammingDistance("a", "ab"))

	c1 := &schema.Chunk{Content: "Alice likes machine learning"}
	c2 := &schema.Chunk{Content: "Alice likes machine learning"}
	require.True(t, AreSimilar(c1, c2, 0))
}

func TestFuzzyDeduplicator(t *testing.T) {
	fd := NewFuzzyDeduplicator(0)
	require.True(t, fd.AddChunk(&schema.Chunk{Content: "same"}))
	require.False(t, fd.AddChunk(&schema.Chunk{Content: "same"}))
	require.Len(t, fd.GetUniqueChunks(), 1)
	fd.Reset()
	require.Len(t, fd.GetUniqueChunks(), 0)
}

