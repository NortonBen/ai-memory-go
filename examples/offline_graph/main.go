// Example: Pipeline hoàn toàn offline — Harrier-OSS-v1-270m + DeBERTa-v3-large NER
//
// Không cần LMStudio, Ollama, hay bất kỳ LLM server nào.
// Tất cả chạy local via ONNX Runtime:
//
//	Harrier-OSS-v1-270m   → embedding 640-dim (semantic search)
//	DeBERTa-v3-large NER  → entity extraction + co-occurrence edges (knowledge graph)
//
// ┌─────────────────────────────────────────────────────────────────────┐
// │  TEXT → Add → CognifyPending → VectorSearch + GraphTraversal        │
// │                                                                     │
// │  Cognify pipeline (offline):                                        │
// │   chunk → Harrier embed   → SQLite vector store                     │
// │        → DeBERTa NER      → entities (PER/ORG/LOC/MISC) + edges    │
// │        → MemifyTask async → SQLite graph store                      │
// └─────────────────────────────────────────────────────────────────────┘
//
// Cài đặt:
//
//  1. Export Harrier (chạy một lần):
//     python scripts/export_harrier_onnx.py \
//         --model microsoft/harrier-oss-v1-270m \
//         --output data/harrier
//
//  2. Export DeBERTa NER (chạy một lần):
//     python scripts/export_deberta_onnx.py \
//         --model Gladiator/microsoft-deberta-v3-large_ner_conll2003 \
//         --output data/deberta-ner
//
//  3. Cài ONNX Runtime (macOS):
//     brew install onnxruntime
//
// Chạy:
//
//	go run ./examples/offline_graph/
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor/deberta"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	_ "github.com/NortonBen/ai-memory-go/storage/adapters/sqlite"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/NortonBen/ai-memory-go/vector/embedders/onnx"
)

func main() {
	ctx := context.Background()

	// ─── Paths ────────────────────────────────────────────────────────────────
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")

	harrierModel := filepath.Join(root, "data", "harrier", "model.onnx")
	harrierTok := filepath.Join(root, "data", "harrier", "tokenizer.json")

	debertaModel := filepath.Join(root, "data", "deberta-ner", "model.onnx")
	debertaTok := filepath.Join(root, "data", "deberta-ner", "tokenizer.json")
	debertaLabels := filepath.Join(root, "data", "deberta-ner", "labels.json")

	dataDir := filepath.Join(root, "data", "offline_graph_demo")
	_ = os.RemoveAll(dataDir)
	_ = os.MkdirAll(dataDir, 0o750)

	// Verify model files exist
	for _, p := range []struct{ path, hint string }{
		{harrierModel, "python scripts/export_harrier_onnx.py ..."},
		{harrierTok, "python scripts/export_harrier_onnx.py ..."},
		{debertaModel, "python scripts/export_deberta_onnx.py ..."},
		{debertaTok, "python scripts/export_deberta_onnx.py ..."},
		{debertaLabels, "python scripts/export_deberta_onnx.py ..."},
	} {
		mustExist(p.path, p.hint)
	}

	banner()

	// ─── 1. Harrier ONNX Embedder ─────────────────────────────────────────────
	section("[1] Harrier-OSS-v1-270m embedder")
	harrierEmb, err := onnx.NewHarrierEmbedder(harrierModel, harrierTok, 512,
		"Retrieve semantically similar text", true)
	must(err, "harrier embedder")
	fmt.Printf("    Model : %s\n", harrierEmb.GetModel())
	fmt.Printf("    Dim   : %d\n", harrierEmb.GetDimensions())
	if err := harrierEmb.Health(ctx); err != nil {
		log.Fatalf("FATAL Harrier health check: %v", err)
	}
	fmt.Println("    Status: OK ✓")

	// ─── 2. DeBERTa-v3 NER Extractor ─────────────────────────────────────────
	section("[2] DeBERTa-v3-large NER extractor")
	debExt, err := deberta.NewExtractor(deberta.Config{
		ModelPath:     debertaModel,
		TokenizerPath: debertaTok,
		LabelsPath:    debertaLabels,
		MaxSeqLen:     512,
	})
	must(err, "deberta extractor")
	fmt.Println("    Model : Gladiator/microsoft-deberta-v3-large_ner_conll2003")
	fmt.Println("    Labels: O B/I-PER B/I-ORG B/I-LOC B/I-MISC")
	fmt.Println("    Status: OK ✓")

	// Quick NER smoke test
	sampleText := "Satya Nadella is the CEO of Microsoft, founded by Bill Gates in Seattle."
	sampleNodes, _ := debExt.ExtractEntities(ctx, sampleText)
	fmt.Printf("    Smoke : %q\n    →", truncate(sampleText, 60))
	for _, n := range sampleNodes {
		fmt.Printf(" [%s:%s]", n.Properties["ner_label"], n.Properties["name"])
	}
	fmt.Println()

	// ─── 3. Stores ────────────────────────────────────────────────────────────
	section("[3] SQLite stores")
	autoEmb := vector.NewAutoEmbedder("onnx", vector.NewInMemoryEmbeddingCache())
	autoEmb.AddProvider("onnx", harrierEmb)

	graphStore, err := graph.NewSQLiteGraphStore(filepath.Join(dataDir, "graph.db"))
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore(filepath.Join(dataDir, "vectors.db"), 640)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    filepath.Join(dataDir, "rel.db"),
		ConnTimeout: 5 * time.Second,
	})
	must(err, "relational store")
	defer relStore.Close()
	fmt.Println("    graph.db  ✓   vectors.db  ✓   rel.db  ✓")

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	section("[4] Memory engine")
	eng := engine.NewMemoryEngineWithStores(
		debExt,   // LLMExtractor → DeBERTa-v3 NER (no LLM server!)
		autoEmb,  // EmbeddingProvider → Harrier ONNX
		relStore,
		graphStore,
		vecStore,
		engine.EngineConfig{MaxWorkers: 4, ChunkConcurrency: 2},
	)
	defer eng.Close()
	fmt.Println("    Extractor : DeBERTa-v3-large NER (offline ONNX)")
	fmt.Println("    Embedder  : Harrier-OSS-v1-270m (offline ONNX)")
	fmt.Println("    LLM server: — (none required)")

	// ─── 5. ADD corpus ────────────────────────────────────────────────────────
	section("[5] ADD — ingesting knowledge corpus")
	sessionID := "offline-graph-demo"

	corpus := []struct{ text, topic string }{
		{
			"Harrier-OSS-v1-270m is a multilingual text embedding model created by Microsoft Research. It produces 640-dimensional vectors and supports over 100 languages.",
			"model",
		},
		{
			"Microsoft open-sourced Harrier under the MIT license. Satya Nadella, CEO of Microsoft, announced the release at Microsoft Build 2025 in Seattle.",
			"announcement",
		},
		{
			"DeBERTa-v3 is a transformer model developed by Microsoft Research. It uses disentangled attention and an enhanced mask decoder for NLP tasks.",
			"model",
		},
		{
			"ONNX Runtime is an open-source inference engine from Microsoft that accelerates machine learning models including DeBERTa and Harrier.",
			"infrastructure",
		},
		{
			"HuggingFace is an AI company that maintains the Transformers library and the Optimum toolkit used to export models to ONNX format.",
			"company",
		},
		{
			"Google DeepMind developed the Gemma3 architecture on which the Harrier model is based. Gemma3 is a decoder-only transformer.",
			"research",
		},
		{
			"RAG (Retrieval-Augmented Generation) combines a dense vector retrieval step with a language model generation step for factual question answering.",
			"technique",
		},
		{
			"The ai-memory-brain project combines Harrier embeddings with DeBERTa NER and a SQLite graph store for a fully offline memory pipeline.",
			"project",
		},
	}

	var dps []*schema.DataPoint
	// dpText maps DataPoint ID → original text (for displaying search results).
	dpText := make(map[string]string)
	for _, item := range corpus {
		dp, err := eng.Add(ctx, item.text,
			engine.WithSessionID(sessionID),
			engine.WithMetadata(map[string]interface{}{
				"topic":  item.topic,
				"source": "offline-graph-demo",
				"text":   item.text,
			}),
		)
		must(err, "add")
		dps = append(dps, dp)
		dpText[dp.ID] = item.text
		fmt.Printf("    + [%s] (%s) %s\n", dp.ID[:8], item.topic, truncate(item.text, 62))
	}

	// ─── 6. COGNIFY: embed + NER ──────────────────────────────────────────────
	section("[6] COGNIFY — Harrier embed + DeBERTa NER")
	fmt.Println("    Processing all chunks (sync drain) …")

	t0 := time.Now()
	if err := eng.CognifyPending(ctx, sessionID); err != nil {
		log.Printf("  warn CognifyPending: %v", err)
	}
	cogDur := time.Since(t0)
	fmt.Printf("    CognifyPending: done in %s  (%.1f texts/s)\n",
		cogDur.Round(time.Millisecond), float64(len(corpus))/cogDur.Seconds())

	// ─── 7. Wait for Memify (async graph writes) ──────────────────────────────
	section("[7] Waiting for Memify → knowledge graph")
	deadline := time.Now().Add(60 * time.Second)
	var nodeCount, edgeCount int64
	for time.Now().Before(deadline) {
		nodeCount, _ = graphStore.GetNodeCount(ctx)
		if nodeCount > 0 {
			edgeCount, _ = graphStore.GetEdgeCount(ctx)
			fmt.Printf("    ✓ Graph ready: %d nodes, %d edges\n", nodeCount, edgeCount)
			break
		}
		fmt.Print(".")
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println()
	if nodeCount == 0 {
		fmt.Println("    ⚠ No graph nodes (NER found no entities in corpus?)")
	}

	// ─── 8. KNOWLEDGE GRAPH — list entities ───────────────────────────────────
	section("[8] KNOWLEDGE GRAPH — entities by type")
	for _, nodeType := range []schema.NodeType{
		schema.NodeTypeEntity,
		schema.NodeTypeConcept,
	} {
		ns, err := graphStore.FindNodesByType(ctx, nodeType)
		if err != nil || len(ns) == 0 {
			continue
		}
		fmt.Printf("\n  ● %s (%d)\n", nodeType, len(ns))
		for _, n := range ns {
			name, _ := n.Properties["name"].(string)
			label, _ := n.Properties["ner_label"].(string)
			if label != "" {
				fmt.Printf("    [%s] %s\n", label, name)
			} else {
				fmt.Printf("    %s\n", name)
			}
		}
	}

	// ─── 9. GRAPH TRAVERSAL ───────────────────────────────────────────────────
	section("[9] GRAPH TRAVERSAL — co-occurrence edges")

	// Find "Microsoft" node and traverse its neighbours
	msNodes, err := graphStore.FindNodesByProperty(ctx, "name", "Microsoft")
	if err == nil && len(msNodes) > 0 {
		ms := msNodes[0]
		msName, _ := ms.Properties["name"].(string)
		fmt.Printf("\n  Hub: %q (%s)\n", msName, ms.Type)
		connected, _ := graphStore.FindConnected(ctx, ms.ID, nil)
		for _, c := range connected {
			cName, _ := c.Properties["name"].(string)
			cLabel, _ := c.Properties["ner_label"].(string)
			fmt.Printf("    ↔ [%s] %s\n", cLabel, cName)
		}

		// TraverseGraph depth-2
		neighbors, err := graphStore.TraverseGraph(ctx, ms.ID, 2, nil)
		if err == nil {
			fmt.Printf("\n  TraverseGraph depth-2 from %q: %d reachable nodes\n", msName, len(neighbors))
			for _, n := range neighbors {
				nName, _ := n.Properties["name"].(string)
				fmt.Printf("    → [%s] %s\n", n.Type, nName)
			}
		}
	} else {
		fmt.Println("  (no 'Microsoft' node found — check NER output above)")
	}

	// ─── 10. VECTOR SEARCH ────────────────────────────────────────────────────
	section("[10] VECTOR SEARCH — semantic similarity (Harrier)")
	queries := []string{
		"What is Harrier and who built it?",
		"open-source AI inference runtime",
		"combining retrieval and generation for question answering",
		"NER entity extraction from text",
	}
	for _, q := range queries {
		qEmb, err := harrierEmb.GenerateQueryEmbedding(ctx, q)
		must(err, "query embed")
		hits, err := vecStore.SimilaritySearch(ctx, qEmb, 2, 0.3)
		if err != nil {
			log.Printf("  warn search: %v", err)
			continue
		}
		fmt.Printf("\n  Q: %q\n", q)
		seen := make(map[string]bool)
		for _, h := range hits {
			// Resolve the root DataPoint ID by stripping "-chunk-N" suffixes.
			// Chain: Add() → DP(root) → DP(root-chunk-0) → vector(source_id=root-chunk-0)
			srcID, _ := h.Metadata["source_id"].(string)
			if srcID == "" {
				srcID = h.ID
			}
			rootID := stripChunkSuffix(srcID)
			if seen[rootID] {
				continue
			}
			seen[rootID] = true
			text := dpText[rootID]
			if text == "" {
				text = srcID
			}
			fmt.Printf("    [%.3f] %s\n", h.Score, truncate(text, 80))
		}
	}

	// ─── 11. Embedding similarity spot-check ──────────────────────────────────
	section("[11] Embedding similarity spot-check")
	pairs := [][2]string{
		{"Harrier is a multilingual embedding model.", "This is a dense vector retrieval model."},
		{"Harrier is a multilingual embedding model.", "The weather in Hanoi is sunny today."},
		{"DeBERTa-v3 extracts named entities.", "NER models detect persons and organisations in text."},
	}
	for _, p := range pairs {
		eA, _ := harrierEmb.GenerateEmbedding(ctx, p[0])
		eB, _ := harrierEmb.GenerateEmbedding(ctx, p[1])
		sim := cosineSim(eA, eB)
		indicator := "⬛"
		if sim > 0.7 {
			indicator = "🟢"
		} else if sim > 0.4 {
			indicator = "🟡"
		}
		fmt.Printf("  %s %.4f\n    A: %s\n    B: %s\n\n",
			indicator, sim, truncate(p[0], 55), truncate(p[1], 55))
	}

	// ─── 12. Summary ──────────────────────────────────────────────────────────
	section("[12] Summary")
	vecCnt, _ := vecStore.GetEmbeddingCount(ctx)
	nodeCount, _ = graphStore.GetNodeCount(ctx)
	edgeCount, _ = graphStore.GetEdgeCount(ctx)

	fmt.Printf(`
  Pipeline     : 100%% offline (no LLM server)
  ─────────────────────────────────────────
  Embedder     : Harrier-OSS-v1-270m  (640-dim, multilingual)
  NER extractor: DeBERTa-v3-large     (CoNLL-2003, PER/ORG/LOC/MISC)
  Inference    : ONNX Runtime         (CPU)
  ─────────────────────────────────────────
  DataPoints   : %d
  Vectors      : %d
  Graph nodes  : %d
  Graph edges  : %d
  Cognify time : %s
  ─────────────────────────────────────────
  Data dir     : %s

`, len(dps), vecCnt, nodeCount, edgeCount,
		cogDur.Round(time.Millisecond),
		dataDir,
	)
	fmt.Println("✅  Offline graph demo complete.")
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func banner() {
	fmt.Println()
	fmt.Println("╔═════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Offline Knowledge Graph Pipeline — 100% Local ONNX Inference  ║")
	fmt.Println("║  Harrier-OSS-v1-270m (embed) + DeBERTa-v3-large (NER)         ║")
	fmt.Println("║  No LMStudio · No Ollama · No API keys                         ║")
	fmt.Println("╚═════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func section(title string) {
	fmt.Printf("\n%s\n", title)
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

// stripChunkSuffix removes "-chunk-NNN" suffixes from a DataPoint ID to recover
// the root ID returned by eng.Add(). The chain is:
//
//	eng.Add() → DP(rootID) → child DP(rootID-chunk-0) → vector(source_id=rootID-chunk-0)
func stripChunkSuffix(id string) string {
	for {
		idx := strings.LastIndex(id, "-chunk-")
		if idx < 0 {
			return id
		}
		id = id[:idx]
	}
}

func mustExist(path, hint string) {
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("FATAL: file not found\n  path: %s\n  fix : %s", path, hint)
	}
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL [%s]: %v", label, err)
	}
}
