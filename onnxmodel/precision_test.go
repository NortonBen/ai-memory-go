package onnxmodel

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveModel_autoPrefersInt8Sibling(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "harrier", "model.onnx"))
	touch(t, filepath.Join(root, "harrier-q", "model.onnx"))

	ref := filepath.Join(root, "harrier", "model.onnx")
	got, used, err := ResolveModel(ref, PrecisionAuto)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "harrier-q", "model.onnx")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if used != PrecisionINT8 {
		t.Fatalf("used precision %q want int8", used)
	}
}

func TestResolveModel_fp32Explicit(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "harrier", "model.onnx"))
	touch(t, filepath.Join(root, "harrier-q", "model.onnx"))

	ref := filepath.Join(root, "harrier", "model.onnx")
	got, used, err := ResolveModel(ref, PrecisionFP32)
	if err != nil {
		t.Fatal(err)
	}
	if used != PrecisionFP32 {
		t.Fatalf("used %q", used)
	}
	if filepath.Base(filepath.Dir(got)) != "harrier" {
		t.Fatalf("should stay on fp32 dir, got %q", got)
	}
}

func TestResolveSiblingFile(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "harrier", "tokenizer.json"))
	touch(t, filepath.Join(root, "harrier-q", "model.onnx"))
	touch(t, filepath.Join(root, "harrier-q", "tokenizer.json"))

	origTok := filepath.Join(root, "harrier", "tokenizer.json")
	model := filepath.Join(root, "harrier-q", "model.onnx")
	got, err := ResolveSiblingFile(origTok, model)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "harrier-q", "tokenizer.json")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCanonicalStem(t *testing.T) {
	if canonicalStem("harrier-q") != "harrier" {
		t.Fatal()
	}
	if canonicalStem("deberta-ner-fp16") != "deberta-ner" {
		t.Fatal()
	}
}

func TestParsePrecision_fp16Rejected(t *testing.T) {
	_, err := ParsePrecision("fp16")
	if err == nil {
		t.Fatal("expected error for fp16")
	}
}

func TestResolveModel_missingFP32FileUsesInt8Sibling(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "harrier"), 0o755); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(root, "harrier-q", "model.onnx"))
	// Reference path points at non-existent FP32 file; parent dir exists.
	ref := filepath.Join(root, "harrier", "model.onnx")
	got, used, err := ResolveModel(ref, PrecisionAuto)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "harrier-q", "model.onnx")
	if got != want || used != PrecisionINT8 {
		t.Fatalf("got %q %q want %q int8", got, used, want)
	}
}
