// Package deberta provides a knowledge-graph extractor that uses a DeBERTa-v3
// NER ONNX model for entity extraction and co-occurrence for relationship extraction.
// It implements extractor.LLMExtractor so it can replace an LLM in the Cognify pipeline,
// making the whole stack fully offline (Harrier embeddings + DeBERTa NER).
//
// # Architecture
//
//	text ──► SentencePiece Unigram tokenizer (Go)
//	      ──► ONNX Runtime (DeBERTa-v3 NER)    → logits [1, seq, numLabels]
//	      ──► IOB2 decoder                      → []Entity{text, label, start, end}
//	      ──► schema.Node (Entity/Concept/...)
//	      ──► co-occurrence edges               → schema.Edge (RELATED_TO / SIMILAR_TO)
//
// # GPU / Execution providers
//
// Control execution provider via ORT_NER_PROVIDER / ORT_PROVIDER or Config.ExecProvider.
// Model precision (separate from EP): ORT_NER_PRECISION / ORT_MODEL_PRECISION / Config.ModelPrecision
// (auto | fp32 | int8). See package onnxmodel for directory layout.
//
// # Setup
//
//  1. Export model:
//     python scripts/export_deberta_onnx.py \
//     --model Gladiator/microsoft-deberta-v3-large_ner_conll2003 \
//     --output data/deberta-ner
//
//  2. Install ONNX Runtime (macOS):
//     brew install onnxruntime
//
// # Go usage
//
//	deb, err := deberta.NewExtractor(deberta.Config{
//	    ModelPath:     "data/deberta-ner/model.onnx",
//	    TokenizerPath: "data/deberta-ner/tokenizer.json",
//	    LabelsPath:    "data/deberta-ner/labels.json",
//	    MaxSeqLen:     512,
//	    ExecProvider:  "coreml",  // or "" to read ORT_PROVIDER env
//	})
//	eng := engine.NewMemoryEngineWithStores(deb, harrierEmb, ...)
package deberta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/onnxmodel"
	"github.com/NortonBen/ai-memory-go/schema"
)

// ─── Configuration ────────────────────────────────────────────────────────────

// Config holds all parameters for the DeBERTa NER extractor.
type Config struct {
	// ModelPath is the path to the exported model.onnx file.
	ModelPath string

	// TokenizerPath is the path to the HuggingFace tokenizer.json (SentencePiece Unigram).
	TokenizerPath string

	// LabelsPath is the path to labels.json ({"0":"O","1":"B-PER",...}).
	LabelsPath string

	// MaxSeqLen is the maximum sequence length fed to the model (default: 512).
	MaxSeqLen int

	// ExecProvider selects the ORT execution provider.
	// Valid values: "cpu", "coreml", "cuda", "auto" (default: read ORT_PROVIDER env, then "auto").
	ExecProvider string

	// OrtLibPath overrides the ONNX Runtime shared library path.
	// If empty, standard paths are searched or ORT_LIB_PATH env is used.
	OrtLibPath string

	// ModelPrecision selects ONNX weights: auto, fp32, int8 (empty = ORT_NER_PRECISION / ORT_MODEL_PRECISION).
	ModelPrecision string
}

// ─── IOB entity span ──────────────────────────────────────────────────────────

type entitySpan struct {
	surface string // surface text from original input
	label   string // NER class: "PER", "ORG", "LOC", "MISC"
}

// ─── Extractor ────────────────────────────────────────────────────────────────

// Extractor implements extractor.LLMExtractor using DeBERTa-v3 NER.
// It holds a persistent ONNX session and pre-allocated tensors for low-latency
// inference (especially important on GPU where session creation is expensive).
type Extractor struct {
	cfg            Config
	tokenizer      *Tokenizer
	labels         []string // label index → IOB string (e.g. "B-PER")
	activeProvider string   // actual ORT provider used
	modelPrecision string   // fp32, int8 after ResolveModel

	// Persistent session and tensors (guarded by mu)
	session      *ort.AdvancedSession
	inputIDs     *ort.Tensor[int64]
	attnMask     *ort.Tensor[int64]
	typeIDs      *ort.Tensor[int64] // nil if model doesn't need token_type_ids
	logitsTensor *ort.Tensor[float32]
	mu           sync.Mutex
}

var ortOnce sync.Once

// NewExtractor loads the tokenizer, label map, and creates a persistent ONNX session.
func NewExtractor(cfg Config) (*Extractor, error) {
	if cfg.MaxSeqLen <= 0 {
		cfg.MaxSeqLen = 512
	}

	prec, err := onnxmodel.EffectivePrecision(cfg.ModelPrecision, "ner")
	if err != nil {
		return nil, fmt.Errorf("deberta: model precision: %w", err)
	}
	resolvedModel, usedPrec, err := onnxmodel.ResolveModel(cfg.ModelPath, prec)
	if err != nil {
		return nil, fmt.Errorf("deberta: resolve model: %w", err)
	}
	cfg.ModelPath = resolvedModel

	cfg.TokenizerPath, err = onnxmodel.ResolveSiblingFile(cfg.TokenizerPath, resolvedModel)
	if err != nil {
		return nil, fmt.Errorf("deberta: resolve tokenizer: %w", err)
	}
	cfg.LabelsPath, err = onnxmodel.ResolveSiblingFile(cfg.LabelsPath, resolvedModel)
	if err != nil {
		return nil, fmt.Errorf("deberta: resolve labels: %w", err)
	}

	tok, err := NewTokenizerFromFile(cfg.TokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("deberta: load tokenizer: %w", err)
	}

	labels, err := loadLabels(cfg.LabelsPath)
	if err != nil {
		return nil, fmt.Errorf("deberta: load labels: %w", err)
	}

	if _, err := os.Stat(cfg.ModelPath); err != nil {
		return nil, fmt.Errorf("deberta: model not found at %q: %w", cfg.ModelPath, err)
	}

	// Resolve execution provider
	cfg.ExecProvider = resolveProvider(cfg.ExecProvider)

	// Init ORT environment once per process.
	var initErr error
	ortOnce.Do(func() {
		libPath := cfg.OrtLibPath
		if libPath == "" {
			libPath = os.Getenv("ORT_LIB_PATH")
		}
		if libPath == "" {
			switch runtime.GOOS {
			case "darwin":
				for _, p := range []string{
					"/opt/homebrew/lib/libonnxruntime.dylib",
					"/usr/local/lib/libonnxruntime.dylib",
				} {
					if _, e := os.Stat(p); e == nil {
						libPath = p
						break
					}
				}
			case "linux":
				for _, p := range []string{
					"/usr/lib/libonnxruntime.so",
					"/usr/local/lib/libonnxruntime.so",
				} {
					if _, e := os.Stat(p); e == nil {
						libPath = p
						break
					}
				}
			}
		}
		if libPath != "" {
			ort.SetSharedLibraryPath(libPath)
		}
		err := ort.InitializeEnvironment()
		// Ignore "already initialized" — another ONNX model (e.g. Harrier) may have done it first.
		if err != nil && !strings.Contains(err.Error(), "already been initialized") {
			initErr = err
		}
	})
	if initErr != nil {
		return nil, fmt.Errorf("deberta: init ORT: %w", initErr)
	}

	e := &Extractor{
		cfg:            cfg,
		tokenizer:      tok,
		labels:         labels,
		modelPrecision: string(usedPrec),
	}

	if err := e.loadSession(); err != nil {
		return nil, err
	}

	return e, nil
}

// loadSession creates the persistent ONNX session with pre-allocated tensors.
func (e *Extractor) loadSession() error {
	seqLen := e.cfg.MaxSeqLen
	numLabels := len(e.labels)
	if numLabels == 0 {
		numLabels = 9
	}

	shape := ort.NewShape(1, int64(seqLen))
	outShape := ort.NewShape(1, int64(seqLen), int64(numLabels))

	inputIDs, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		return fmt.Errorf("deberta: create input_ids tensor: %w", err)
	}

	attnMask, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		inputIDs.Destroy()
		return fmt.Errorf("deberta: create attention_mask tensor: %w", err)
	}

	logitsTensor, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		inputIDs.Destroy()
		attnMask.Destroy()
		return fmt.Errorf("deberta: create logits tensor: %w", err)
	}

	// Build session options with execution provider
	opts, err := ort.NewSessionOptions()
	if err != nil {
		inputIDs.Destroy()
		attnMask.Destroy()
		logitsTensor.Destroy()
		return fmt.Errorf("deberta: session options: %w", err)
	}
	defer opts.Destroy()

	used, provErr := applyExecutionProvider(opts, e.cfg.ExecProvider)
	if provErr != nil {
		inputIDs.Destroy()
		attnMask.Destroy()
		logitsTensor.Destroy()
		return fmt.Errorf("deberta: execution provider: %w", provErr)
	}
	e.activeProvider = used

	// Try creating session with input_ids + attention_mask first.
	session, err := ort.NewAdvancedSession(
		e.cfg.ModelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"logits"},
		[]ort.ArbitraryTensor{inputIDs, attnMask},
		[]ort.ArbitraryTensor{logitsTensor},
		opts,
	)
	if err != nil {
		// Some models also require token_type_ids (all zeros).
		typeIDs, terr := ort.NewEmptyTensor[int64](shape)
		if terr != nil {
			inputIDs.Destroy()
			attnMask.Destroy()
			logitsTensor.Destroy()
			return fmt.Errorf("deberta: create token_type_ids tensor: %w", terr)
		}

		session2, err2 := ort.NewAdvancedSession(
			e.cfg.ModelPath,
			[]string{"input_ids", "attention_mask", "token_type_ids"},
			[]string{"logits"},
			[]ort.ArbitraryTensor{inputIDs, attnMask, typeIDs},
			[]ort.ArbitraryTensor{logitsTensor},
			opts,
		)
		if err2 != nil {
			inputIDs.Destroy()
			attnMask.Destroy()
			logitsTensor.Destroy()
			typeIDs.Destroy()
			return fmt.Errorf("deberta: create session (with/without token_type_ids): %w / %w", err, err2)
		}
		e.typeIDs = typeIDs
		session = session2
	}

	e.session = session
	e.inputIDs = inputIDs
	e.attnMask = attnMask
	e.logitsTensor = logitsTensor
	return nil
}

// Close releases the ONNX session and tensors.
func (e *Extractor) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.session != nil {
		e.session.Destroy()
		e.session = nil
	}
	if e.inputIDs != nil {
		e.inputIDs.Destroy()
	}
	if e.attnMask != nil {
		e.attnMask.Destroy()
	}
	if e.typeIDs != nil {
		e.typeIDs.Destroy()
	}
	if e.logitsTensor != nil {
		e.logitsTensor.Destroy()
	}
}

// GetExecutionProvider returns the active ORT execution provider (e.g. "coreml", "cuda", "cpu").
func (e *Extractor) GetExecutionProvider() string { return e.activeProvider }

// GetModelPrecision returns the resolved ONNX weight layout (fp32, int8).
func (e *Extractor) GetModelPrecision() string { return e.modelPrecision }

// ─── extractor.LLMExtractor interface ─────────────────────────────────────────

// ExtractEntities runs DeBERTa-v3 NER on text and returns named-entity nodes.
func (e *Extractor) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	spans, err := e.runNER(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("deberta ExtractEntities: %w", err)
	}

	nodes := make([]schema.Node, 0, len(spans))
	for _, sp := range spans {
		node := spanToNode(sp)
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// ExtractRelationships generates co-occurrence edges between extracted entities.
func (e *Extractor) ExtractRelationships(_ context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	if len(entities) < 2 {
		return nil, nil
	}

	sentences := splitSentences(text)
	var edges []schema.Edge

	for _, sent := range sentences {
		var inSent []schema.Node
		sentLower := strings.ToLower(sent)
		for _, n := range entities {
			name, _ := n.Properties["name"].(string)
			if name != "" && strings.Contains(sentLower, strings.ToLower(name)) {
				inSent = append(inSent, n)
			}
		}
		for i := 0; i < len(inSent); i++ {
			for j := i + 1; j < len(inSent); j++ {
				a, b := inSent[i], inSent[j]
				edgeType := schema.EdgeTypeRelatedTo
				if a.Type == b.Type {
					edgeType = schema.EdgeTypeSimilarTo
				}
				edge := schema.NewEdge(a.ID, b.ID, edgeType, 0.8)
				edge.Properties = map[string]interface{}{
					"context":  "co-occurrence",
					"source":   "deberta-ner",
					"sentence": truncate(sent, 120),
				}
				edges = append(edges, *edge)
			}
		}
	}
	return edges, nil
}

func (e *Extractor) ExtractBridgingRelationship(_ context.Context, _, _ string) (*schema.Edge, error) {
	return nil, nil
}

func (e *Extractor) ExtractRequestIntent(_ context.Context, _ string) (*schema.RequestIntent, error) {
	return nil, nil
}

func (e *Extractor) CompareEntities(_ context.Context, existing schema.Node, newEntity schema.Node) (*schema.ConsistencyResult, error) {
	existName, _ := existing.Properties["name"].(string)
	newName, _ := newEntity.Properties["name"].(string)
	existLow := strings.ToLower(strings.TrimSpace(existName))
	newLow := strings.ToLower(strings.TrimSpace(newName))
	switch {
	case existLow == newLow:
		return &schema.ConsistencyResult{Action: schema.ResolutionIgnore, Reason: "same name"}, nil
	case strings.Contains(existLow, newLow) || strings.Contains(newLow, existLow):
		return &schema.ConsistencyResult{Action: schema.ResolutionUpdate, Reason: "name is substring"}, nil
	default:
		return &schema.ConsistencyResult{Action: schema.ResolutionKeepSeparate, Reason: "different entities"}, nil
	}
}

func (e *Extractor) ExtractWithSchema(_ context.Context, _ string, _ interface{}) (interface{}, error) {
	return nil, nil
}

func (e *Extractor) AnalyzeQuery(_ context.Context, text string) (*schema.ThinkQueryAnalysis, error) {
	words := strings.Fields(text)
	keywords := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, ".,;:?!")
		if len([]rune(w)) > 3 {
			keywords = append(keywords, w)
		}
	}
	return &schema.ThinkQueryAnalysis{
		QueryType:      "Factual",
		SearchKeywords: keywords,
	}, nil
}

func (e *Extractor) SetProvider(_ extractor.LLMProvider) {}
func (e *Extractor) GetProvider() extractor.LLMProvider  { return nil }

// ─── Core NER inference ───────────────────────────────────────────────────────

// runNER tokenizes text, fills pre-allocated tensors, runs the persistent session,
// and decodes IOB entity spans.
func (e *Extractor) runNER(_ context.Context, text string) ([]entitySpan, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	seqLen := e.cfg.MaxSeqLen

	// 1. Tokenize
	tokenIDs, wordIDs := e.tokenizer.Encode(text, true, true)
	paddedIDs, paddedWordIDs, mask := e.tokenizer.BuildAttentionMask(tokenIDs, wordIDs, seqLen)

	// 2. Fill pre-allocated tensors (avoids per-call allocation)
	inputData := e.inputIDs.GetData()
	attnData := e.attnMask.GetData()
	for i := 0; i < seqLen; i++ {
		inputData[i] = int64(paddedIDs[i])
		attnData[i] = int64(mask[i])
	}
	if e.typeIDs != nil {
		typeData := e.typeIDs.GetData()
		for i := range typeData {
			typeData[i] = 0
		}
	}

	// 3. Run persistent session
	if err := e.session.Run(); err != nil {
		return nil, fmt.Errorf("deberta: inference: %w", err)
	}

	// 4. Decode IOB spans
	numLabels := len(e.labels)
	if numLabels == 0 {
		numLabels = 9
	}
	rawLogits := e.logitsTensor.GetData()
	tokenLabels := argmaxLabels(rawLogits, seqLen, numLabels, e.labels)

	wordLabelMap := make(map[int]string)
	maxWord := -1
	for i, wid := range paddedWordIDs {
		if wid < 0 || mask[i] == 0 {
			continue
		}
		if _, seen := wordLabelMap[wid]; !seen {
			wordLabelMap[wid] = tokenLabels[i]
		}
		if wid > maxWord {
			maxWord = wid
		}
	}

	wordLabels := make([]string, maxWord+1)
	for i := range wordLabels {
		if l, ok := wordLabelMap[i]; ok {
			wordLabels[i] = l
		} else {
			wordLabels[i] = "O"
		}
	}

	words := splitWords(text)
	return decodeIOB(wordLabels, words), nil
}

// ─── Decode helpers ───────────────────────────────────────────────────────────

func argmaxLabels(logits []float32, seqLen, numLabels int, labels []string) []string {
	result := make([]string, seqLen)
	for i := 0; i < seqLen; i++ {
		best := 0
		bestV := logits[i*numLabels]
		for j := 1; j < numLabels; j++ {
			if v := logits[i*numLabels+j]; v > bestV {
				bestV = v
				best = j
			}
		}
		if best < len(labels) {
			result[i] = labels[best]
		} else {
			result[i] = "O"
		}
	}
	return result
}

func decodeIOB(wordLabels, words []string) []entitySpan {
	var spans []entitySpan
	var curLabel string
	var curWords []string

	flush := func() {
		if len(curWords) > 0 && curLabel != "" {
			spans = append(spans, entitySpan{
				surface: strings.Join(curWords, " "),
				label:   curLabel,
			})
		}
		curWords = nil
		curLabel = ""
	}

	for i, lbl := range wordLabels {
		word := ""
		if i < len(words) {
			word = words[i]
		}
		switch {
		case strings.HasPrefix(lbl, "B-"):
			flush()
			curLabel = lbl[2:]
			curWords = []string{word}
		case strings.HasPrefix(lbl, "I-"):
			nerClass := lbl[2:]
			if curLabel == nerClass {
				curWords = append(curWords, word)
			} else {
				flush()
				curLabel = nerClass
				curWords = []string{word}
			}
		default:
			flush()
		}
	}
	flush()
	return spans
}

func spanToNode(sp entitySpan) schema.Node {
	var nodeType schema.NodeType
	switch sp.label {
	case "PER", "ORG", "LOC":
		nodeType = schema.NodeTypeEntity
	default:
		nodeType = schema.NodeTypeConcept
	}
	surface := strings.Trim(sp.surface, ".,;:!?\"'()")
	node := schema.NewNode(nodeType, map[string]interface{}{
		"name":      surface,
		"ner_label": sp.label,
		"source":    "deberta-ner",
	})
	return *node
}

// ─── Text utilities ───────────────────────────────────────────────────────────

func splitSentences(text string) []string {
	var sents []string
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		if r == '.' || r == '!' || r == '?' {
			s := strings.TrimSpace(string(runes[start : i+1]))
			if s != "" {
				sents = append(sents, s)
			}
			start = i + 1
		}
	}
	if tail := strings.TrimSpace(string(runes[start:])); tail != "" {
		sents = append(sents, tail)
	}
	if len(sents) == 0 {
		sents = []string{text}
	}
	return sents
}

func splitWords(text string) []string {
	var words []string
	var buf strings.Builder
	for _, r := range text {
		if unicode.IsSpace(r) {
			if buf.Len() > 0 {
				words = append(words, buf.String())
				buf.Reset()
			}
		} else {
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		words = append(words, buf.String())
	}
	return words
}

func loadLabels(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read labels: %w", err)
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}
	maxIdx := -1
	for k := range raw {
		if idx, err := strconv.Atoi(k); err == nil && idx > maxIdx {
			maxIdx = idx
		}
	}
	if maxIdx < 0 {
		return nil, fmt.Errorf("labels.json: no numeric keys found")
	}
	labels := make([]string, maxIdx+1)
	for k, v := range raw {
		if idx, err := strconv.Atoi(k); err == nil {
			labels[idx] = v
		}
	}
	return labels, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Compile-time check that *Extractor implements extractor.LLMExtractor.
var _ extractor.LLMExtractor = (*Extractor)(nil)
