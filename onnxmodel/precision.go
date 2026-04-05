// Package onnxmodel resolves ONNX file paths by weight precision (FP32, INT8).
//
// ONNX Runtime loads whatever dtypes are stored in the graph; this package only picks
// the correct file on disk. Export separate artifacts per precision (see scripts/ and Makefile).
//
// FP16 is not supported: full-graph conversion via onnxconverter_common produces invalid
// type edges for transformer-style ONNX graphs in ONNX Runtime (Cast / Add dtype mismatches).
// Use FP32 or INT8 dynamic quantization instead.
//
// Environment (optional overrides):
//
//	ORT_MODEL_PRECISION   — global default: auto | fp32 | int8
//	ORT_EMBED_PRECISION   — Harrier / embedding model (overrides global for embed)
//	ORT_NER_PRECISION     — DeBERTa / NER model (overrides global for NER)
//
// Directory layout (stem = canonical name, e.g. "harrier" or "deberta-ner"):
//
//	{parent}/{stem}/model.onnx              — FP32 (default name)
//	{parent}/{stem}-q/model.onnx            — INT8 (dynamic quantize; existing convention)
//	{parent}/{stem}-int8/model.onnx         — INT8 (alias)
//
// Same-directory filenames (optional):
//
//	model_fp32.onnx, model_int8.onnx
//
// "auto" prefers smaller models first: int8 → fp32.
package onnxmodel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultModelFile = "model.onnx"

// Precision names normalised to lowercase.
type Precision string

const (
	PrecisionAuto Precision = "auto"
	PrecisionFP32 Precision = "fp32"
	PrecisionINT8 Precision = "int8"
)

// ParsePrecision normalises a user string. Empty string → Auto.
func ParsePrecision(s string) (Precision, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return PrecisionAuto, nil
	}
	switch s {
	case "auto", "any":
		return PrecisionAuto, nil
	case "fp32", "float32", "f32":
		return PrecisionFP32, nil
	case "fp16", "float16", "f16", "half":
		return "", fmt.Errorf("onnxmodel: fp16 is not supported (ORT rejects converter-produced transformer graphs); use fp32 or int8")
	case "int8", "i8", "q8", "quant8":
		return PrecisionINT8, nil
	default:
		return "", fmt.Errorf("onnxmodel: unknown precision %q (want auto, fp32, int8)", s)
	}
}

func parseEnvPrecision(key, raw string) (Precision, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	pr, err := ParsePrecision(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "onnxmodel: ignoring %s=%q: %v\n", key, raw, err)
		return "", false
	}
	return pr, true
}

// EnvPrecision returns precision from environment for the given scope.
// scope is "embed" or "ner" (any other value uses only ORT_MODEL_PRECISION).
func EnvPrecision(scope string) Precision {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "embed", "embedding":
		if p, ok := parseEnvPrecision("ORT_EMBED_PRECISION", os.Getenv("ORT_EMBED_PRECISION")); ok {
			return p
		}
	case "ner":
		if p, ok := parseEnvPrecision("ORT_NER_PRECISION", os.Getenv("ORT_NER_PRECISION")); ok {
			return p
		}
	}
	if p, ok := parseEnvPrecision("ORT_MODEL_PRECISION", os.Getenv("ORT_MODEL_PRECISION")); ok {
		return p
	}
	return PrecisionAuto
}

// EffectivePrecision chooses explicit config over environment.
func EffectivePrecision(explicit string, scope string) (Precision, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return ParsePrecision(explicit)
	}
	return EnvPrecision(scope), nil
}

// canonicalStem strips known quantized dir suffixes so siblings resolve correctly
// (e.g. "harrier-q" → "harrier", "deberta-ner-fp16" → "deberta-ner").
func canonicalStem(baseName string) string {
	s := baseName
	for _, suf := range []string{"-int8", "-q", "-fp16"} {
		if strings.HasSuffix(s, suf) {
			s = strings.TrimSuffix(s, suf)
		}
	}
	return s
}

func modelDirFromGiven(given string) (modelDir string, err error) {
	given = filepath.Clean(given)
	fi, err := os.Stat(given)
	if err == nil {
		if fi.IsDir() {
			return given, nil
		}
		return filepath.Dir(given), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	// Missing file path: use parent directory if it exists (e.g. only harrier-q/ was built).
	parent := filepath.Dir(given)
	if st, e := os.Stat(parent); e == nil && st.IsDir() {
		return parent, nil
	}
	return "", fmt.Errorf("onnxmodel: stat %q: %w", given, err)
}

func defaultModelBasename(given string) string {
	given = filepath.Clean(given)
	fi, err := os.Stat(given)
	if err != nil && os.IsNotExist(err) {
		b := filepath.Base(given)
		if b == "" || b == "." {
			return defaultModelFile
		}
		return b
	}
	if err != nil || fi.IsDir() {
		return defaultModelFile
	}
	return filepath.Base(given)
}

// candidatePaths returns existing ONNX paths to try for prec, in order.
func candidatePaths(modelDir, basename string, prec Precision) []string {
	parent := filepath.Dir(modelDir)
	stem := canonicalStem(filepath.Base(modelDir))
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		out = append(out, p)
	}

	switch prec {
	case PrecisionINT8:
		add(filepath.Join(modelDir, "model_int8.onnx"))
		add(filepath.Join(parent, stem+"-int8", defaultModelFile))
		add(filepath.Join(parent, stem+"-q", defaultModelFile))
	case PrecisionFP32:
		add(filepath.Join(modelDir, "model_fp32.onnx"))
		add(filepath.Join(modelDir, basename))
		add(filepath.Join(parent, stem, defaultModelFile))
	case PrecisionAuto:
		// Smaller first (RAM)
		for _, p := range []Precision{PrecisionINT8, PrecisionFP32} {
			for _, c := range candidatePaths(modelDir, basename, p) {
				add(c)
			}
		}
		return dedupeExisting(out)
	}
	return dedupeExisting(out)
}

func dedupeExisting(paths []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// ResolveModel picks an ONNX file from a reference path (file or directory) and precision.
// If prec is Auto, the first existing candidate in int8→fp32 order wins.
// If prec is fixed and no file exists, returns a wrapped error listing tried paths.
func ResolveModel(given string, prec Precision) (resolved string, used Precision, err error) {
	modelDir, err := modelDirFromGiven(given)
	if err != nil {
		return "", "", fmt.Errorf("onnxmodel: stat %q: %w", given, err)
	}
	basename := defaultModelBasename(given)

	if prec == PrecisionAuto {
		cands := candidatePaths(modelDir, basename, PrecisionAuto)
		if len(cands) == 0 {
			return "", "", fmt.Errorf("onnxmodel: no model found under %q (tried int8/fp32 layouts)", modelDir)
		}
		resolved = cands[0]
		used = inferPrecisionFromPath(resolved)
		return resolved, used, nil
	}

	paths := candidatePaths(modelDir, basename, prec)
	if len(paths) > 0 {
		return paths[0], prec, nil
	}

	// Strict fallback for fp32: exact path user passed as file
	if prec == PrecisionFP32 {
		if fi, e := os.Stat(given); e == nil && !fi.IsDir() {
			return given, PrecisionFP32, nil
		}
	}

	return "", "", fmt.Errorf("onnxmodel: no %s model found (dir %q, tried sibling dirs and model_%s.onnx)", prec, modelDir, prec)
}

func inferPrecisionFromPath(resolved string) Precision {
	low := strings.ToLower(resolved)
	dir := strings.ToLower(filepath.Dir(low))
	base := strings.ToLower(filepath.Base(low))
	switch {
	case strings.HasSuffix(dir, "-int8") || strings.HasSuffix(dir, "-q") || strings.Contains(base, "int8"):
		return PrecisionINT8
	default:
		return PrecisionFP32
	}
}

// ResolveSiblingFile returns a file with the same basename next to resolvedModel if it exists,
// otherwise the original path (e.g. tokenizer.json / labels.json beside quantized model.onnx).
func ResolveSiblingFile(original, resolvedModel string) (string, error) {
	if original == "" {
		return "", fmt.Errorf("onnxmodel: path is empty")
	}
	if _, err := os.Stat(original); err != nil {
		return "", fmt.Errorf("onnxmodel: %q: %w", original, err)
	}
	modelDir := filepath.Dir(resolvedModel)
	name := filepath.Base(original)
	candidate := filepath.Join(modelDir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return original, nil
}
