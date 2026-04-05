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
// # Setup
//
//  1. Export model:
//     python scripts/export_deberta_onnx.py \
//     --model Gladiator/microsoft-deberta-v3-large_ner_conll2003 \
//     --output data/deberta-ner
//
//  2. Install ONNX Runtime (macOS): brew install onnxruntime
//
// # Go usage
//
//	deb, err := deberta.NewExtractor(deberta.Config{
//	    ModelPath:     "data/deberta-ner/model.onnx",
//	    TokenizerPath: "data/deberta-ner/tokenizer.json",
//	    LabelsPath:    "data/deberta-ner/labels.json",
//	    MaxSeqLen:     512,
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

	// OrtLibPath overrides the ONNX Runtime shared library path.
	// If empty, standard paths are searched (/opt/homebrew/lib, etc.)
	// or the ORT_LIB_PATH env variable is used.
	OrtLibPath string
}

// ─── IOB entity span ──────────────────────────────────────────────────────────

type entitySpan struct {
	surface string // surface text from original input
	label   string // NER class: "PER", "ORG", "LOC", "MISC"
}

// ─── Extractor ────────────────────────────────────────────────────────────────

// Extractor implements extractor.LLMExtractor using DeBERTa-v3 NER.
// It is goroutine-safe: inference sessions are created per call.
type Extractor struct {
	cfg       Config
	tokenizer *Tokenizer
	labels    []string // label index → IOB string (e.g. "B-PER")
	mu        sync.Mutex
}

var ortOnce sync.Once

// NewExtractor loads the tokenizer, label map, and verifies the ONNX model path.
func NewExtractor(cfg Config) (*Extractor, error) {
	if cfg.MaxSeqLen <= 0 {
		cfg.MaxSeqLen = 512
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

	// Init ORT environment once per process.
	// If Harrier (or another ONNX embedder) already initialised ORT, we just
	// reuse the existing environment — the "already initialized" error is safe to ignore.
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
		// "already initialized" is expected when another ONNX model (e.g. Harrier)
		// has already called InitializeEnvironment in the same process.
		if err != nil && !strings.Contains(err.Error(), "already been initialized") {
			initErr = err
		}
	})
	if initErr != nil {
		return nil, fmt.Errorf("deberta: init ORT: %w", initErr)
	}

	return &Extractor{
		cfg:       cfg,
		tokenizer: tok,
		labels:    labels,
	}, nil
}

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
// Entities that appear in the same sentence get a RELATED_TO (or SIMILAR_TO if same type) edge.
func (e *Extractor) ExtractRelationships(_ context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	if len(entities) < 2 {
		return nil, nil
	}

	sentences := splitSentences(text)
	var edges []schema.Edge

	for _, sent := range sentences {
		// Collect entities whose surface form appears in this sentence
		var inSent []schema.Node
		sentLower := strings.ToLower(sent)
		for _, n := range entities {
			name, _ := n.Properties["name"].(string)
			if name != "" && strings.Contains(sentLower, strings.ToLower(name)) {
				inSent = append(inSent, n)
			}
		}
		// Create edges between every pair in same sentence
		for i := 0; i < len(inSent); i++ {
			for j := i + 1; j < len(inSent); j++ {
				a, b := inSent[i], inSent[j]
				edgeType := schema.EdgeTypeRelatedTo
				if a.Type == b.Type {
					edgeType = schema.EdgeTypeSimilarTo
				}
				edge := schema.NewEdge(a.ID, b.ID, edgeType, 0.8)
				edge.Properties = map[string]interface{}{
					"context": "co-occurrence",
					"source":  "deberta-ner",
					"sentence": truncate(sent, 120),
				}
				edges = append(edges, *edge)
			}
		}
	}
	return edges, nil
}

// ExtractBridgingRelationship is not supported by NER — returns nil.
func (e *Extractor) ExtractBridgingRelationship(_ context.Context, _, _ string) (*schema.Edge, error) {
	return nil, nil
}

// ExtractRequestIntent is not supported by NER — returns nil.
func (e *Extractor) ExtractRequestIntent(_ context.Context, _ string) (*schema.RequestIntent, error) {
	return nil, nil
}

// CompareEntities performs an exact-match comparison on entity names.
// DeBERTa does not have generation capability so we use simple heuristics.
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

// ExtractWithSchema is not supported — returns nil.
func (e *Extractor) ExtractWithSchema(_ context.Context, _ string, _ interface{}) (interface{}, error) {
	return nil, nil
}

// AnalyzeQuery returns a basic keyword-based analysis without LLM.
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

// SetProvider is a no-op (DeBERTa doesn't use an LLM provider).
func (e *Extractor) SetProvider(_ extractor.LLMProvider) {}

// GetProvider returns nil (no LLM provider).
func (e *Extractor) GetProvider() extractor.LLMProvider { return nil }

// ─── Core NER inference ───────────────────────────────────────────────────────

// runNER tokenizes text, runs ONNX inference, and decodes IOB entity spans.
func (e *Extractor) runNER(_ context.Context, text string) ([]entitySpan, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	seqLen := e.cfg.MaxSeqLen

	// 1. Tokenize
	tokenIDs, wordIDs := e.tokenizer.Encode(text, true /*CLS*/, true /*SEP*/)
	paddedIDs, paddedWordIDs, mask := e.tokenizer.BuildAttentionMask(tokenIDs, wordIDs, seqLen)

	// 2. Build ORT input tensors
	shape := ort.NewShape(1, int64(seqLen))

	inputIDsTensor, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		return nil, fmt.Errorf("deberta: create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attnTensor, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		return nil, fmt.Errorf("deberta: create attention_mask tensor: %w", err)
	}
	defer attnTensor.Destroy()

	for i := 0; i < seqLen; i++ {
		inputIDsTensor.GetData()[i] = int64(paddedIDs[i])
		attnTensor.GetData()[i] = int64(mask[i])
	}

	// 3. Output: logits [1, seqLen, numLabels]
	numLabels := len(e.labels)
	if numLabels == 0 {
		numLabels = 9 // CoNLL-2003 default
	}
	outShape := ort.NewShape(1, int64(seqLen), int64(numLabels))
	logitsTensor, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return nil, fmt.Errorf("deberta: create logits tensor: %w", err)
	}
	defer logitsTensor.Destroy()

	// 4. Run inference (new session per call — avoids thread-safety issues)
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("deberta: session options: %w", err)
	}
	defer opts.Destroy()

	// Try with both input_ids + attention_mask first; some models also need token_type_ids.
	session, err := ort.NewAdvancedSession(
		e.cfg.ModelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"logits"},
		[]ort.ArbitraryTensor{inputIDsTensor, attnTensor},
		[]ort.ArbitraryTensor{logitsTensor},
		opts,
	)
	if err != nil {
		// Retry with token_type_ids (all zeros)
		session, err = e.runWithTokenTypeIDs(paddedIDs, mask, logitsTensor, opts)
		if err != nil {
			return nil, fmt.Errorf("deberta: create session: %w", err)
		}
	}
	defer session.Destroy()

	if err := session.Run(); err != nil {
		return nil, fmt.Errorf("deberta: inference: %w", err)
	}

	// 5. Decode IOB spans
	rawLogits := logitsTensor.GetData() // flat [seqLen * numLabels]
	tokenLabels := argmaxLabels(rawLogits, seqLen, numLabels, e.labels)

	// Aggregate to word-level (first subword token per word)
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

	// Split original text into words (same order as tokenizer)
	words := splitWords(text)

	return decodeIOB(wordLabels, words), nil
}

// runWithTokenTypeIDs creates a session that includes token_type_ids (all zeros).
func (e *Extractor) runWithTokenTypeIDs(paddedIDs, mask []int32, logitsTensor *ort.Tensor[float32], opts *ort.SessionOptions) (*ort.AdvancedSession, error) {
	seqLen := len(paddedIDs)
	shape := ort.NewShape(1, int64(seqLen))

	inputIDsTensor, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		return nil, err
	}
	attnTensor, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		inputIDsTensor.Destroy()
		return nil, err
	}
	typeTensor, err := ort.NewEmptyTensor[int64](shape)
	if err != nil {
		inputIDsTensor.Destroy()
		attnTensor.Destroy()
		return nil, err
	}

	for i := 0; i < seqLen; i++ {
		inputIDsTensor.GetData()[i] = int64(paddedIDs[i])
		attnTensor.GetData()[i] = int64(mask[i])
		typeTensor.GetData()[i] = 0
	}

	return ort.NewAdvancedSession(
		e.cfg.ModelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"logits"},
		[]ort.ArbitraryTensor{inputIDsTensor, attnTensor, typeTensor},
		[]ort.ArbitraryTensor{logitsTensor},
		opts,
	)
}

// ─── Decode helpers ───────────────────────────────────────────────────────────

// argmaxLabels returns the IOB label string for each token position.
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

// decodeIOB converts word-level IOB2 labels and original words into entity spans.
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
				// Broken I- without matching B- → start new entity
				flush()
				curLabel = nerClass
				curWords = []string{word}
			}

		default: // "O" or unknown
			flush()
		}
	}
	flush()
	return spans
}

// spanToNode converts a NER entity span to a schema.Node.
func spanToNode(sp entitySpan) schema.Node {
	var nodeType schema.NodeType
	switch sp.label {
	case "PER":
		nodeType = schema.NodeTypeEntity
	case "ORG":
		nodeType = schema.NodeTypeEntity
	case "LOC":
		nodeType = schema.NodeTypeEntity
	case "MISC":
		nodeType = schema.NodeTypeConcept
	default:
		nodeType = schema.NodeTypeEntity
	}

	// Strip trailing/leading punctuation from surface form
	surface := strings.Trim(sp.surface, ".,;:!?\"'()")

	node := schema.NewNode(nodeType, map[string]interface{}{
		"name":      surface,
		"ner_label": sp.label,
		"source":    "deberta-ner",
	})
	return *node
}

// ─── Text utilities ───────────────────────────────────────────────────────────

// splitSentences splits text into sentences on [.!?] boundaries.
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

// splitWords splits text into whitespace-separated words (for word alignment).
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

// loadLabels reads the labels.json file ({"0":"O","1":"B-PER",...}).
func loadLabels(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read labels: %w", err)
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}
	// Find max index
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
