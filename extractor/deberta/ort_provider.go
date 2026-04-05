// ort_provider.go — ORT execution provider helpers for the deberta package.
// Mirrors vector/embedders/onnx/ort_provider.go (same logic, different package).
package deberta

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

// resolveProvider returns the effective ORT provider for the DeBERTa NER model.
//
// Priority:
//  1. Explicit Config.ExecProvider field
//  2. ORT_NER_PROVIDER env  (NER-specific override)
//  3. ORT_PROVIDER env      (but CoreML/auto are downgraded to "cpu" — see note)
//  4. Default: "cpu"
//
// Why "cpu" by default?
// DeBERTa-v3-large with CoreML compiles a ~4-6 GB GPU buffer, raising
// process RSS to 10+ GB. CPU inference is fast enough for NER
// (typically 100-300 ms per sentence) and uses only ~200 MB.
// Use ORT_NER_PROVIDER=coreml to opt in if you have spare GPU memory.
func resolveProvider(explicit string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return strings.ToLower(p)
	}
	// NER-specific override takes priority over general ORT_PROVIDER.
	if p := strings.TrimSpace(os.Getenv("ORT_NER_PROVIDER")); p != "" {
		return strings.ToLower(p)
	}
	if p := strings.TrimSpace(os.Getenv("ORT_PROVIDER")); p != "" {
		// Downgrade CoreML/auto → cpu for DeBERTa to avoid the 4-6 GB GPU buffer.
		// GPU gives minimal speedup for NER while costing enormous RAM.
		lp := strings.ToLower(p)
		if lp == "coreml" || lp == "auto" {
			return "cpu"
		}
		return lp
	}
	return "cpu"
}

// applyExecutionProvider appends the requested provider to opts.
// Returns the name of the provider actually used.
// Soft failures (CoreML / CUDA unavailable) log a warning and fall back to CPU.
func applyExecutionProvider(opts *ort.SessionOptions, provider string) (used string, err error) {
	switch provider {
	case "coreml":
		if runtime.GOOS != "darwin" {
			log.Printf("deberta ort: provider=coreml requested but GOOS=%s; falling back to CPU", runtime.GOOS)
			return "cpu", nil
		}
		// Flag 0 = all devices (GPU + Apple Neural Engine + CPU fallback).
		if err := opts.AppendExecutionProviderCoreML(0); err != nil {
			log.Printf("deberta ort: CoreML EP unavailable (%v); falling back to CPU", err)
			return "cpu", nil
		}
		return "coreml", nil

	case "cuda":
		cudaOpts, err := ort.NewCUDAProviderOptions()
		if err != nil {
			log.Printf("deberta ort: CUDA EP unavailable (%v); falling back to CPU", err)
			return "cpu", nil
		}
		defer cudaOpts.Destroy()
		if err := opts.AppendExecutionProviderCUDA(cudaOpts); err != nil {
			log.Printf("deberta ort: CUDA EP append failed (%v); falling back to CPU", err)
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
		return "", fmt.Errorf("deberta ort: unknown execution provider %q (valid: cpu, coreml, cuda, auto)", provider)
	}
}
