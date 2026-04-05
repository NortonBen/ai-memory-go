// Package onnx - Local embedding provider for Harrier-OSS-v1-270m using ONNX Runtime.
//
// # Model
//
// Microsoft Harrier-OSS-v1-270m is a decoder-only multilingual embedding model
// with embedding dimension 640 and max context 32,768 tokens.
// It uses last-token pooling followed by L2 normalisation.
//
// # Setup
//
// The model must first be exported from SafeTensors to ONNX format:
//
//	python scripts/export_harrier_onnx.py \
//	    --model microsoft/harrier-oss-v1-270m \
//	    --output data/harrier --seq-len 512
//
// ONNX Runtime shared library is required. Install via:
//
//	# macOS
//	brew install onnxruntime
//	# or download from https://github.com/microsoft/onnxruntime/releases
//	# then set env: ORT_LIB_PATH=/path/to/libonnxruntime.dylib
//
// # Configuration (YAML)
//
//	embedder:
//	  provider: onnx
//	  model: microsoft/harrier-oss-v1-270m
//	  dimensions: 640
//	  custom:
//	    model_path: /path/to/model.onnx
//	    tokenizer_path: /path/to/tokenizer.json
//	    max_seq_len: 512
//	    model_precision: auto   # auto | fp32 | int8 (see onnxmodel)
//	    query_task: "Retrieve semantically similar text"
//	    use_query_instruction: true
package onnx

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/NortonBen/ai-memory-go/onnxmodel"
	"github.com/NortonBen/ai-memory-go/vector"
)

const (
	// harrierDim is the native embedding dimension for harrier-oss-v1-270m.
	harrierDim = 640
	// defaultMaxSeqLen is the default truncation length.
	defaultMaxSeqLen = 512
	// modelName is the canonical model identifier.
	modelName = "microsoft/harrier-oss-v1-270m"
	// HarrierModelName is the exported canonical model identifier.
	HarrierModelName = modelName
)

// ortInitOnce ensures the ONNX Runtime environment is only initialised once.
var ortInitOnce sync.Once

func init() {
	vector.RegisterEmbeddingProvider(EmbeddingProviderONNX, func(cfg map[string]interface{}) (vector.EmbeddingProvider, error) {
		modelPath, _ := cfg["model_path"].(string)
		tokenizerPath, _ := cfg["tokenizer_path"].(string)
		if modelPath == "" {
			return nil, fmt.Errorf("onnx harrier: model_path is required")
		}
		if tokenizerPath == "" {
			return nil, fmt.Errorf("onnx harrier: tokenizer_path is required")
		}

		maxSeqLen := defaultMaxSeqLen
		if v, ok := cfg["max_seq_len"].(int); ok && v > 0 {
			maxSeqLen = v
		}

		queryTask, _ := cfg["query_task"].(string)
		useQueryInst := true
		if v, ok := cfg["use_query_instruction"].(bool); ok {
			useQueryInst = v
		}

		modelPrecision, _ := cfg["model_precision"].(string)
		if modelPrecision != "" {
			return NewHarrierEmbedderWithPrecision(modelPath, tokenizerPath, maxSeqLen, queryTask, useQueryInst, modelPrecision)
		}
		return NewHarrierEmbedder(modelPath, tokenizerPath, maxSeqLen, queryTask, useQueryInst)
	})
}

// EmbeddingProviderONNX is the provider type key used in configuration and factory registration.
const EmbeddingProviderONNX vector.EmbeddingProviderType = "onnx"

// HarrierEmbedder generates embeddings with the Harrier-OSS-v1-270m ONNX model.
type HarrierEmbedder struct {
	modelPath     string
	tokenizerPath string
	maxSeqLen     int
	queryTask     string
	useQueryInst  bool
	execProvider  string // "cpu", "coreml", "cuda", "auto"

	tokenizer      *Tokenizer
	session        *ort.AdvancedSession
	activeProvider string // actual provider used (set after loadSession)
	modelPrecision string // fp32, int8 (after ResolveModel)

	mu sync.Mutex
}

// NewHarrierEmbedder loads the ONNX model and tokenizer from disk.
// It initialises the ONNX Runtime environment on first call.
//
// Weight layout (FP32 / INT8) is selected by onnxmodel.ResolveModel — see package onnxmodel
// (ORT_EMBED_PRECISION, ORT_MODEL_PRECISION, or YAML model_precision). "auto" prefers smaller on-disk models.
//
// Execution provider: ORT_EMBED_PROVIDER, ORT_PROVIDER, default cpu — see ort_provider.go.
//
// Set ORT_LIB_PATH to override the ONNX Runtime shared library path.
func NewHarrierEmbedder(modelPath, tokenizerPath string, maxSeqLen int, queryTask string, useQueryInst bool) (*HarrierEmbedder, error) {
	return newHarrierEmbedder(modelPath, tokenizerPath, maxSeqLen, queryTask, useQueryInst, "")
}

// NewHarrierEmbedderWithPrecision is like NewHarrierEmbedder but forces model_precision (fp32|int8|auto),
// overriding ORT_* env vars for this instance.
func NewHarrierEmbedderWithPrecision(modelPath, tokenizerPath string, maxSeqLen int, queryTask string, useQueryInst bool, modelPrecision string) (*HarrierEmbedder, error) {
	return newHarrierEmbedder(modelPath, tokenizerPath, maxSeqLen, queryTask, useQueryInst, modelPrecision)
}

func newHarrierEmbedder(modelPath, tokenizerPath string, maxSeqLen int, queryTask string, useQueryInst bool, modelPrecision string) (*HarrierEmbedder, error) {
	prec, err := onnxmodel.EffectivePrecision(modelPrecision, "embed")
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: model precision: %w", err)
	}
	resolvedModel, usedPrec, err := onnxmodel.ResolveModel(modelPath, prec)
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: resolve model: %w", err)
	}
	tokPath, err := onnxmodel.ResolveSiblingFile(tokenizerPath, resolvedModel)
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: resolve tokenizer: %w", err)
	}
	if _, err := os.Stat(resolvedModel); err != nil {
		return nil, fmt.Errorf("onnx harrier: model file not found at %q: %w", resolvedModel, err)
	}
	if _, err := os.Stat(tokPath); err != nil {
		return nil, fmt.Errorf("onnx harrier: tokenizer file not found at %q: %w", tokPath, err)
	}

	tok, err := NewTokenizerFromFile(tokPath)
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: load tokenizer: %w", err)
	}

	// Initialise ORT environment once per process.
	var initErr error
	ortInitOnce.Do(func() {
		if libPath := os.Getenv("ORT_LIB_PATH"); libPath != "" {
			ort.SetSharedLibraryPath(libPath)
		} else {
			// Default search paths per OS
			switch runtime.GOOS {
			case "darwin":
				for _, p := range []string{
					"/opt/homebrew/lib/libonnxruntime.dylib",
					"/usr/local/lib/libonnxruntime.dylib",
				} {
					if _, e := os.Stat(p); e == nil {
						ort.SetSharedLibraryPath(p)
						break
					}
				}
			case "linux":
				for _, p := range []string{
					"/usr/lib/libonnxruntime.so",
					"/usr/local/lib/libonnxruntime.so",
				} {
					if _, e := os.Stat(p); e == nil {
						ort.SetSharedLibraryPath(p)
						break
					}
				}
			}
		}
		initErr = ort.InitializeEnvironment()
	})
	if initErr != nil {
		return nil, fmt.Errorf("onnx harrier: init ORT environment: %w", initErr)
	}

	if maxSeqLen <= 0 {
		maxSeqLen = defaultMaxSeqLen
	}

	e := &HarrierEmbedder{
		modelPath:      resolvedModel,
		tokenizerPath:  tokPath,
		maxSeqLen:      maxSeqLen,
		queryTask:      queryTask,
		useQueryInst:   useQueryInst,
		execProvider:   resolveProvider(""),
		tokenizer:      tok,
		modelPrecision: string(usedPrec),
	}

	if err := e.loadSession(); err != nil {
		return nil, err
	}

	return e, nil
}

// loadSession creates the ONNX Runtime inference session.
func (e *HarrierEmbedder) loadSession() error {
	// Input shape: [1, maxSeqLen]
	shape := ort.NewShape(1, int64(e.maxSeqLen))

	inputIDs, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		return fmt.Errorf("onnx harrier: create input_ids tensor: %w", err)
	}
	attnMask, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		inputIDs.Destroy()
		return fmt.Errorf("onnx harrier: create attention_mask tensor: %w", err)
	}

	// Output: last_hidden_state shape [1, maxSeqLen, 640]
	outShape := ort.NewShape(1, int64(e.maxSeqLen), harrierDim)
	hiddenState, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		inputIDs.Destroy()
		attnMask.Destroy()
		return fmt.Errorf("onnx harrier: create output tensor: %w", err)
	}

	opts, err := ort.NewSessionOptions()
	if err != nil {
		inputIDs.Destroy()
		attnMask.Destroy()
		hiddenState.Destroy()
		return fmt.Errorf("onnx harrier: session options: %w", err)
	}
	defer opts.Destroy()

	// Apply execution provider (CoreML / CUDA / CPU)
	used, provErr := applyExecutionProvider(opts, e.execProvider)
	if provErr != nil {
		inputIDs.Destroy()
		attnMask.Destroy()
		hiddenState.Destroy()
		return fmt.Errorf("onnx harrier: execution provider: %w", provErr)
	}
	e.activeProvider = used

	session, err := ort.NewAdvancedSession(
		e.modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"last_hidden_state"},
		[]ort.ArbitraryTensor{inputIDs, attnMask},
		[]ort.ArbitraryTensor{hiddenState},
		opts,
	)
	if err != nil {
		inputIDs.Destroy()
		attnMask.Destroy()
		hiddenState.Destroy()
		return fmt.Errorf("onnx harrier: create session: %w", err)
	}

	e.session = session
	return nil
}

// GenerateEmbedding implements vector.EmbeddingProvider.
func (e *HarrierEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return e.embed(ctx, text, false)
}

// GenerateQueryEmbedding embeds a query with the instruction prefix.
func (e *HarrierEmbedder) GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	return e.embed(ctx, query, true)
}

// GenerateBatchEmbeddings implements vector.EmbeddingProvider.
func (e *HarrierEmbedder) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		emb, err := e.GenerateEmbedding(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("onnx harrier batch[%d]: %w", i, err)
		}
		out[i] = emb
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
	return out, nil
}

// GetDimensions implements vector.EmbeddingProvider.
func (e *HarrierEmbedder) GetDimensions() int { return harrierDim }

// GetModel implements vector.EmbeddingProvider.
func (e *HarrierEmbedder) GetModel() string { return modelName }

// GetExecutionProvider returns the active ORT execution provider (e.g. "coreml", "cuda", "cpu").
func (e *HarrierEmbedder) GetExecutionProvider() string { return e.activeProvider }

// GetModelPrecision returns the resolved ONNX weight layout: fp32 or int8.
func (e *HarrierEmbedder) GetModelPrecision() string { return e.modelPrecision }

// Health implements vector.EmbeddingProvider.
func (e *HarrierEmbedder) Health(ctx context.Context) error {
	if e.session == nil {
		return fmt.Errorf("onnx harrier: session not loaded")
	}
	if e.tokenizer == nil {
		return fmt.Errorf("onnx harrier: tokenizer not loaded")
	}
	_, err := e.embed(ctx, "health", false)
	return err
}

// Close releases ORT session resources.
func (e *HarrierEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.session != nil {
		e.session.Destroy()
		e.session = nil
	}
	return nil
}

// embed is the core inference pipeline:
//  1. Tokenise → input_ids, attention_mask
//  2. Fill ORT input tensors
//  3. Run forward pass
//  4. Last-token pool  → shape [1, 640]
//  5. L2 normalise     → unit embedding
func (e *HarrierEmbedder) embed(_ context.Context, text string, isQuery bool) ([]float32, error) {
	if isQuery && e.useQueryInst {
		text = FormatQueryInstruct(e.queryTask, text)
	}

	// Tokenise
	ids := e.tokenizer.Encode(text, true /*addBOS*/, true /*addEOS*/)
	paddedIDs, mask := BuildAttentionMask(ids, e.maxSeqLen)
	seqLen := e.maxSeqLen

	// Rebuild tensors per call (session reuse with fixed-shape tensors).
	e.mu.Lock()
	defer e.mu.Unlock()

	shape := ort.NewShape(1, int64(seqLen))

	inputIDsTensor, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attnMaskTensor, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: attention_mask tensor: %w", err)
	}
	defer attnMaskTensor.Destroy()

	outShape := ort.NewShape(1, int64(seqLen), harrierDim)
	hiddenTensor, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: output tensor: %w", err)
	}
	defer hiddenTensor.Destroy()

	// Fill input data
	idsData := inputIDsTensor.GetData()
	maskData := attnMaskTensor.GetData()
	for i := 0; i < seqLen; i++ {
		idsData[i] = int64(paddedIDs[i])
		maskData[i] = int64(mask[i])
	}

	// Run inference
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: session options: %w", err)
	}
	defer opts.Destroy()

	session, err := ort.NewAdvancedSession(
		e.modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"last_hidden_state"},
		[]ort.ArbitraryTensor{inputIDsTensor, attnMaskTensor},
		[]ort.ArbitraryTensor{hiddenTensor},
		opts,
	)
	if err != nil {
		return nil, fmt.Errorf("onnx harrier: create session: %w", err)
	}
	defer session.Destroy()

	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("onnx harrier: inference: %w", err)
	}

	// last_hidden_state: [1, seqLen, 640] — last-token pooling
	// Find last real token index (last position where mask=1)
	lastRealIdx := 0
	for i := 0; i < seqLen; i++ {
		if mask[i] == 1 {
			lastRealIdx = i
		}
	}

	rawOut := hiddenTensor.GetData() // flat [seqLen * 640]
	offset := lastRealIdx * harrierDim
	if offset+harrierDim > len(rawOut) {
		return nil, fmt.Errorf("onnx harrier: output shape mismatch: got %d elements, expected offset %d+%d", len(rawOut), offset, harrierDim)
	}

	pooled := make([]float32, harrierDim)
	copy(pooled, rawOut[offset:offset+harrierDim])

	return l2Normalize(pooled), nil
}

// l2Normalize normalises a vector to unit length in place and returns it.
func l2Normalize(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm < 1e-12 {
		return v
	}
	inv := float32(1.0 / norm)
	for i := range v {
		v[i] *= inv
	}
	return v
}
