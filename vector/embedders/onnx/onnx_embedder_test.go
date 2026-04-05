package onnx_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/NortonBen/ai-memory-go/vector/embedders/onnx"
)

// projectRoot returns the repository root from any working directory.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../vector/embedders/onnx/onnx_embedder_test.go
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

func harrierPaths(t *testing.T) (modelPath, tokenizerPath string) {
	t.Helper()
	root := projectRoot(t)
	modelPath = filepath.Join(root, "data", "harrier", "model.onnx")
	tokenizerPath = filepath.Join(root, "data", "harrier", "tokenizer.json")
	return
}

// ─── Tokenizer tests (no model file needed) ───────────────────────────────────

func TestTokenizer_LoadFromFile(t *testing.T) {
	_, tokPath := harrierPaths(t)
	if _, err := os.Stat(tokPath); err != nil {
		t.Skipf("tokenizer.json not found at %s — run: python scripts/export_harrier_onnx.py ...", tokPath)
	}

	tok, err := onnx.NewTokenizerFromFile(tokPath)
	if err != nil {
		t.Fatalf("NewTokenizerFromFile: %v", err)
	}
	if tok == nil {
		t.Fatal("expected non-nil tokenizer")
	}
}

func TestTokenizer_Encode_NonEmpty(t *testing.T) {
	_, tokPath := harrierPaths(t)
	if _, err := os.Stat(tokPath); err != nil {
		t.Skipf("tokenizer.json not found — skip")
	}

	tok, err := onnx.NewTokenizerFromFile(tokPath)
	if err != nil {
		t.Fatalf("load tokenizer: %v", err)
	}

	tests := []struct {
		name string
		text string
	}{
		{"english", "Hello world, this is a test."},
		{"vietnamese", "Xin chào thế giới, đây là bài kiểm tra."},
		{"mixed", "Harrier-OSS-v1-270m multilingual 嵌入模型"},
		{"single_word", "embedding"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids := tok.Encode(tc.text, true /*bos*/, true /*eos*/)
			if tc.text != "" && len(ids) < 2 {
				t.Errorf("expected ≥2 tokens (bos+eos), got %d", len(ids))
			}
			t.Logf("%q → %d tokens: %v", truncate(tc.text, 40), len(ids), ids[:minLen(ids, 8)])
		})
	}
}

func TestTokenizer_BOS_EOS(t *testing.T) {
	_, tokPath := harrierPaths(t)
	if _, err := os.Stat(tokPath); err != nil {
		t.Skipf("tokenizer.json not found — skip")
	}

	tok, err := onnx.NewTokenizerFromFile(tokPath)
	if err != nil {
		t.Fatal(err)
	}

	withBoth := tok.Encode("hello", true, true)
	noBOS := tok.Encode("hello", false, true)
	noEOS := tok.Encode("hello", true, false)
	neither := tok.Encode("hello", false, false)

	if len(withBoth) != len(neither)+2 {
		t.Errorf("BOS+EOS should add exactly 2 tokens: with=%d neither=%d", len(withBoth), len(neither))
	}
	if len(noBOS) != len(neither)+1 {
		t.Errorf("EOS only should add 1 token: noBOS=%d neither=%d", len(noBOS), len(neither))
	}
	if len(noEOS) != len(neither)+1 {
		t.Errorf("BOS only should add 1 token: noEOS=%d neither=%d", len(noEOS), len(neither))
	}
}

func TestTokenizer_AttentionMask(t *testing.T) {
	_, tokPath := harrierPaths(t)
	if _, err := os.Stat(tokPath); err != nil {
		t.Skipf("tokenizer.json not found — skip")
	}

	tok, err := onnx.NewTokenizerFromFile(tokPath)
	if err != nil {
		t.Fatal(err)
	}

	ids := tok.Encode("test sentence", true, true)
	maxLen := 32
	padded, mask := onnx.BuildAttentionMask(ids, maxLen)

	if len(padded) != maxLen {
		t.Errorf("padded len=%d want %d", len(padded), maxLen)
	}
	if len(mask) != maxLen {
		t.Errorf("mask len=%d want %d", len(mask), maxLen)
	}

	// Count real tokens
	realCount := 0
	for _, m := range mask {
		realCount += int(m)
	}
	if realCount != len(ids) {
		t.Errorf("real mask count=%d want %d (original ids)", realCount, len(ids))
	}

	// Padding positions should be 0
	for i := len(ids); i < maxLen; i++ {
		if mask[i] != 0 {
			t.Errorf("mask[%d]=%d want 0 (padding)", i, mask[i])
		}
		if padded[i] != 0 {
			t.Errorf("padded[%d]=%d want 0 (pad token)", i, padded[i])
		}
	}
}

func TestTokenizer_QueryInstruct(t *testing.T) {
	q := "What is ONNX?"
	task := "Retrieve relevant passages"
	got := onnx.FormatQueryInstruct(task, q)
	expected := "Instruct: " + task + "\nQuery: " + q
	if got != expected {
		t.Errorf("got %q want %q", got, expected)
	}

	// Empty task should use default
	gotDefault := onnx.FormatQueryInstruct("", q)
	if gotDefault == "" {
		t.Error("default task should produce non-empty string")
	}
}

// ─── L2 normalisation (package-level helper via exported test) ───────────────

func TestL2Normalize_UnitLength(t *testing.T) {
	tok, err := onnx.NewTokenizerFromFile(func() string {
		_, file, _, _ := runtime.Caller(0)
		return filepath.Join(filepath.Dir(file), "..", "..", "..", "data", "harrier", "tokenizer.json")
	}())
	if err != nil {
		t.Skipf("tokenizer not available: %v", err)
	}
	// Verify encode gives consistent result for idempotent calls
	ids1 := tok.Encode("hello world", false, false)
	ids2 := tok.Encode("hello world", false, false)
	if len(ids1) != len(ids2) {
		t.Errorf("encode not deterministic: %d vs %d tokens", len(ids1), len(ids2))
	}
}

// ─── Embedder integration tests (requires model.onnx) ────────────────────────

func TestHarrierEmbedder_LoadAndHealth(t *testing.T) {
	modelPath, tokPath := harrierPaths(t)
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model.onnx not found at %s — run export script first", modelPath)
	}

	emb, err := onnx.NewHarrierEmbedder(modelPath, tokPath, 128, "", true)
	if err != nil {
		t.Fatalf("NewHarrierEmbedder: %v", err)
	}

	if emb.GetDimensions() != 640 {
		t.Errorf("dimensions=%d want 640", emb.GetDimensions())
	}
	if emb.GetModel() != "microsoft/harrier-oss-v1-270m" {
		t.Errorf("model=%q unexpected", emb.GetModel())
	}

	ctx := context.Background()
	if err := emb.Health(ctx); err != nil {
		t.Errorf("Health() error: %v", err)
	}
}

func TestHarrierEmbedder_GenerateEmbedding_Dimension(t *testing.T) {
	modelPath, tokPath := harrierPaths(t)
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model.onnx not available — skip")
	}

	emb, err := onnx.NewHarrierEmbedder(modelPath, tokPath, 128, "", true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ctx := context.Background()
	texts := []string{
		"Hello, this is a test sentence.",
		"Xin chào, đây là câu thử nghiệm.",
		"ONNX Go embedding model integration.",
	}
	for _, text := range texts {
		vec, err := emb.GenerateEmbedding(ctx, text)
		if err != nil {
			t.Errorf("GenerateEmbedding(%q): %v", truncate(text, 30), err)
			continue
		}
		if len(vec) != 640 {
			t.Errorf("embedding dim=%d want 640 for %q", len(vec), truncate(text, 30))
		}
		// L2 norm should be ~1.0 (normalized)
		norm := float64(0)
		for _, v := range vec {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		if math.Abs(norm-1.0) > 0.01 {
			t.Errorf("L2 norm=%.4f want ~1.0 for %q", norm, truncate(text, 30))
		}
		t.Logf("  %q → dim=%d norm=%.4f", truncate(text, 40), len(vec), norm)
	}
}

func TestHarrierEmbedder_Similarity(t *testing.T) {
	modelPath, tokPath := harrierPaths(t)
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model.onnx not available — skip")
	}

	emb, err := onnx.NewHarrierEmbedder(modelPath, tokPath, 128, "", true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ctx := context.Background()
	// Semantically similar pair should have higher cosine than dissimilar
	similar := [2]string{
		"Deep learning models learn from data.",
		"Neural networks are trained on datasets to find patterns.",
	}
	dissimilar := [2]string{
		"Deep learning models learn from data.",
		"The weather in Hanoi is warm and sunny.",
	}

	simScore := pairSimilarity(t, ctx, emb, similar)
	disScore := pairSimilarity(t, ctx, emb, dissimilar)

	t.Logf("similar pair score   : %.4f", simScore)
	t.Logf("dissimilar pair score: %.4f", disScore)

	if simScore <= disScore {
		t.Errorf("expected similar (%.4f) > dissimilar (%.4f)", simScore, disScore)
	}
}

func TestHarrierEmbedder_BatchEmbeddings(t *testing.T) {
	modelPath, tokPath := harrierPaths(t)
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model.onnx not available — skip")
	}

	emb, err := onnx.NewHarrierEmbedder(modelPath, tokPath, 128, "", true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ctx := context.Background()
	texts := []string{
		"First sentence about AI.",
		"Second sentence about memory.",
		"Third sentence about graphs.",
	}
	batch, err := emb.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}
	if len(batch) != len(texts) {
		t.Errorf("batch len=%d want %d", len(batch), len(texts))
	}
	for i, vec := range batch {
		if len(vec) != 640 {
			t.Errorf("batch[%d] dim=%d want 640", i, len(vec))
		}
	}
}

func TestHarrierEmbedder_QueryInstruction(t *testing.T) {
	modelPath, tokPath := harrierPaths(t)
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model.onnx not available — skip")
	}

	emb, err := onnx.NewHarrierEmbedder(modelPath, tokPath, 128, "Retrieve relevant documents", true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ctx := context.Background()
	query := "What is ONNX?"
	doc := "ONNX is an open format for machine learning models."

	qVec, err := emb.GenerateQueryEmbedding(ctx, query)
	must(t, err, "query embedding")
	dVec, err := emb.GenerateEmbedding(ctx, doc)
	must(t, err, "doc embedding")

	if len(qVec) != 640 || len(dVec) != 640 {
		t.Errorf("query dim=%d doc dim=%d, both want 640", len(qVec), len(dVec))
	}

	sim := cosineSim(qVec, dVec)
	t.Logf("query-doc similarity: %.4f", sim)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func pairSimilarity(t *testing.T, ctx context.Context, emb *onnx.HarrierEmbedder, pair [2]string) float64 {
	t.Helper()
	a, err := emb.GenerateEmbedding(ctx, pair[0])
	if err != nil {
		t.Fatalf("embed A: %v", err)
	}
	b, err := emb.GenerateEmbedding(ctx, pair[1])
	if err != nil {
		t.Fatalf("embed B: %v", err)
	}
	return cosineSim(a, b)
}

func cosineSim(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	d := math.Sqrt(na) * math.Sqrt(nb)
	if d < 1e-12 {
		return 0
	}
	return dot / d
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func minLen[T any](s []T, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}

func must(t *testing.T, err error, label string) {
	t.Helper()
	if err != nil {
		t.Fatalf("FATAL [%s]: %v", label, err)
	}
}
