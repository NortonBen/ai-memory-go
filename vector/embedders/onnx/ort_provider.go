// Package ort_provider contains helpers for configuring ONNX Runtime execution providers.
// It is copied into each package that creates ORT sessions (onnx embedder, deberta extractor)
// to keep the dependency graph simple.
//
// # Supported providers
//
//	Provider   Platform           Notes
//	─────────  ─────────────────  ──────────────────────────────────────────────────
//	cpu        all                Default CPU (always available)
//	coreml     macOS / iOS        Apple GPU + Neural Engine via CoreML (recommended
//	                               on Apple Silicon; requires ORT built with CoreML EP)
//	cuda       Linux / Windows    NVIDIA GPU via CUDA (requires ORT with CUDA EP)
//	auto       all                Tries CoreML on macOS, CUDA on Linux/Windows;
//	                               silently falls back to CPU
//
// # Configuration
//
// Set the ORT_PROVIDER environment variable, e.g.:
//
//	ORT_PROVIDER=coreml   # Apple Silicon GPU + Neural Engine
//	ORT_PROVIDER=cuda     # NVIDIA GPU
//	ORT_PROVIDER=cpu      # force CPU only
//	ORT_PROVIDER=auto     # auto-detect (default)
//
// Individual configs can also set ExecProvider / execProvider fields directly.
package onnx

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

// resolveProvider returns the effective ORT provider for the Harrier embedding model.
//
// Priority:
//  1. Explicit Config.ExecProvider field
//  2. ORT_EMBED_PROVIDER env  (embedding-specific override)
//  3. ORT_PROVIDER env
//  4. Default: "cpu"
//
// RAM so sánh cho Harrier-OSS-v1-270m (1.0 GB model):
//
//	Provider  Disk → RSS    Ghi chú
//	cpu       1.0 GB  ~1.5 GB  Đủ nhanh cho embedding, tổng RAM thấp nhất
//	coreml    1.0 GB  ~3.5 GB  Nhanh hơn ~3x nhưng CoreML compile buffer tốn ~2-3 GB
//	auto      —       (cpu)    Tự chọn tốt nhất — mặc định chọn cpu để tiết kiệm RAM
//
// Để bật GPU cho embedding (khi có ≥ 8 GB RAM trống):
//
//	ORT_EMBED_PROVIDER=coreml go run ./examples/...
func resolveProvider(explicit string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return strings.ToLower(p)
	}
	// Embedding-specific override takes priority.
	if p := strings.TrimSpace(os.Getenv("ORT_EMBED_PROVIDER")); p != "" {
		return strings.ToLower(p)
	}
	if p := strings.TrimSpace(os.Getenv("ORT_PROVIDER")); p != "" {
		return strings.ToLower(p)
	}
	return "cpu" // mặc định: tiết kiệm RAM, không compile CoreML buffer
}

// applyExecutionProvider appends the requested execution provider to opts.
// It returns the name of the provider that was actually applied, and any fatal error.
// Soft failures (e.g. CoreML not available) are logged and fall back to CPU silently.
func applyExecutionProvider(opts *ort.SessionOptions, provider string) (used string, err error) {
	switch provider {
	case "coreml":
		if runtime.GOOS != "darwin" {
			log.Printf("ort: provider=coreml requested but GOOS=%s; falling back to CPU", runtime.GOOS)
			return "cpu", nil
		}
		// Flag 0 = all devices (GPU + Apple Neural Engine + CPU fallback).
		// Use V1 API (universally supported in ORT ≥ 1.11); V2 requires ORT ≥ 1.20.
		if err := opts.AppendExecutionProviderCoreML(0); err != nil {
			log.Printf("ort: CoreML EP unavailable (%v); falling back to CPU", err)
			return "cpu", nil
		}
		return "coreml", nil

	case "cuda":
		cudaOpts, err := ort.NewCUDAProviderOptions()
		if err != nil {
			log.Printf("ort: CUDA EP unavailable (%v); falling back to CPU", err)
			return "cpu", nil
		}
		defer cudaOpts.Destroy()
		if err := opts.AppendExecutionProviderCUDA(cudaOpts); err != nil {
			log.Printf("ort: CUDA EP append failed (%v); falling back to CPU", err)
			return "cpu", nil
		}
		return "cuda", nil

	case "cpu", "":
		return "cpu", nil

	case "auto":
		switch runtime.GOOS {
		case "darwin":
			if err := opts.AppendExecutionProviderCoreML(0); err == nil {
				return "coreml", nil
			}
		default:
			cudaOpts, err := ort.NewCUDAProviderOptions()
			if err == nil {
				defer cudaOpts.Destroy()
				if err := opts.AppendExecutionProviderCUDA(cudaOpts); err == nil {
					return "cuda", nil
				}
			}
		}
		return "cpu", nil

	default:
		return "", fmt.Errorf("ort: unknown execution provider %q (valid: cpu, coreml, cuda, auto)", provider)
	}
}
