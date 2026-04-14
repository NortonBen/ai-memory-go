package onnx

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveProvider_Priority(t *testing.T) {
	t.Setenv("ORT_EMBED_PROVIDER", "")
	t.Setenv("ORT_PROVIDER", "")
	require.Equal(t, "cpu", resolveProvider(""))

	t.Setenv("ORT_PROVIDER", "CUDA")
	require.Equal(t, "cuda", resolveProvider(""))

	t.Setenv("ORT_EMBED_PROVIDER", "CoreML")
	require.Equal(t, "coreml", resolveProvider(""))

	require.Equal(t, "cpu", resolveProvider(" CPU "))
}

func TestParseMerges_BothFormatsAndInvalid(t *testing.T) {
	rawPairs, _ := json.Marshal([][]string{{"a", "b"}, {"c", "d", "e"}, {"x", "y"}})
	parsed, err := parseMerges(rawPairs)
	require.NoError(t, err)
	require.Equal(t, [][2]string{{"a", "b"}, {"x", "y"}}, parsed)

	rawStrings, _ := json.Marshal([]string{"m n", "invalid", "p q"})
	parsed, err = parseMerges(rawStrings)
	require.NoError(t, err)
	require.Equal(t, [][2]string{{"m", "n"}, {"p", "q"}}, parsed)

	_, err = parseMerges(json.RawMessage(`{"bad":true}`))
	require.Error(t, err)
}

func TestTokenizerHelpers_PretokenizeAndBPE(t *testing.T) {
	words := preTokenize("xin chao\nban")
	require.NotEmpty(t, words)
	require.Contains(t, words[1], "▁")

	chars := splitToChars("aé")
	require.Equal(t, []string{"a", "é"}, chars)
	require.Equal(t, "<0xFF>", byteToHex(255))

	tok := &Tokenizer{
		merges: map[bpePair]int{
			{a: "a", b: "b"}: 0,
			{a: "ab", b: "c"}: 1,
		},
		vocab: map[string]int{"abc": 7},
	}
	out := tok.applyBPE([]string{"a", "b", "c"})
	require.Equal(t, []string{"abc"}, out)
	ids := tok.bpeEncode("abc")
	require.Equal(t, []int{7}, ids)
}

func TestBuildAttentionMask_TrimAndPad(t *testing.T) {
	padded, mask := BuildAttentionMask([]int32{1, 2, 3, 4}, 3)
	require.Equal(t, []int32{1, 2, 3}, padded)
	require.Equal(t, []int32{1, 1, 1}, mask)

	padded, mask = BuildAttentionMask([]int32{9}, 4)
	require.Equal(t, []int32{9, 0, 0, 0}, padded)
	require.Equal(t, []int32{1, 0, 0, 0}, mask)
}

func TestL2Normalize_ZeroAndNonZero(t *testing.T) {
	zero := []float32{0, 0, 0}
	require.Equal(t, zero, l2Normalize(zero))

	v := []float32{3, 4}
	n := l2Normalize(v)
	require.InDelta(t, 0.6, n[0], 1e-6)
	require.InDelta(t, 0.8, n[1], 1e-6)
}

func TestHarrierEmbedder_BasicMethodsWithoutSession(t *testing.T) {
	e := &HarrierEmbedder{
		activeProvider: "cpu",
		modelPrecision: "fp32",
	}
	require.Equal(t, 640, e.GetDimensions())
	require.Equal(t, "microsoft/harrier-oss-v1-270m", e.GetModel())
	require.Equal(t, "cpu", e.GetExecutionProvider())
	require.Equal(t, "fp32", e.GetModelPrecision())

	err := e.Health(context.Background())
	require.Error(t, err)

	require.NoError(t, e.Close())
}

func TestGenerateBatchEmbeddings_EmptyInput(t *testing.T) {
	e := &HarrierEmbedder{}
	got, err := e.GenerateBatchEmbeddings(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, got, 0)
}

func TestCfgIntFileExists(t *testing.T) {
	_, err := os.Stat("cfg_int.go")
	require.NoError(t, err)
}

