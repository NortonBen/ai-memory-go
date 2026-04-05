// Example: Harrier-OSS-v1-270m ONNX — RAG mode (embedding-only) và Graph mode (+ LLM)
//
// Pipeline:
//   Add → CognifyPending → waitMemify → VectorSearch → GraphTraversal → Think
//
// Prerequisites:
//   1. Export Harrier model (run once):
//      python scripts/export_harrier_onnx.py \
//          --model microsoft/harrier-oss-v1-270m \
//          --output data/harrier --seq-len 512
//
//   2. Install ONNX Runtime (macOS):
//      brew install onnxruntime
//
//   3. (Graph mode only) LM Studio / Ollama for entity extraction.
//      Default: LM Studio @ http://localhost:1234/v1
//      Override: LLM_ENDPOINT, LLM_MODEL env vars.
//
// Nếu muốn pipeline 100% offline (không cần LMStudio), dùng:
//   go run ./examples/offline_graph/
//
// Run:
//   DEMO_MODE=rag go run ./examples/harrier_onnx/   ← embedding only, không cần LLM
//   DEMO_MODE=graph go run ./examples/harrier_onnx/ ← full graph, cần LMStudio
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
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/extractor/registry"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	_ "github.com/NortonBen/ai-memory-go/storage/adapters/sqlite"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/NortonBen/ai-memory-go/vector/embedders/onnx"
)

// MODE controls the demo behaviour via env var DEMO_MODE:
//
//	DEMO_MODE=rag   (default) — embedding only, no LLM required.
//	                  Cognify stores vectors; graph stays empty.
//	DEMO_MODE=graph           — embedding + LLM entity/relationship extraction.
//	                  Requires LM Studio (or Ollama) running locally.
//	                  Populates knowledge graph; enables Think() multi-hop.
const (
	modeRAG   = "rag"
	modeGraph = "graph"
)

func main() {
	ctx := context.Background()

	// ─── 0. Paths ─────────────────────────────────────────────────────────────
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")

	harrierModel := filepath.Join(projectRoot, "data", "harrier", "model.onnx")
	harrierTokenizer := filepath.Join(projectRoot, "data", "harrier", "tokenizer.json")

	// Fresh data dir for each run (delete + recreate so state is clean).
	dataDir := filepath.Join(projectRoot, "data", "harrier_demo")
	_ = os.RemoveAll(dataDir)
	_ = os.MkdirAll(dataDir, 0o750)

	mustExist(harrierModel, "model.onnx not found — run: python scripts/export_harrier_onnx.py ...")
	mustExist(harrierTokenizer, "tokenizer.json not found — run: python scripts/export_harrier_onnx.py ...")

	// Determine mode: RAG-only (embedding) or Graph (embedding + LLM entity extraction)
	mode := getEnv("DEMO_MODE", modeRAG)
	if mode != modeRAG && mode != modeGraph {
		mode = modeRAG
	}

	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  Harrier-OSS-v1-270m · ONNX Runtime · RAG+Graph Demo ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Printf("  Mode: %s", mode)
	if mode == modeRAG {
		fmt.Println("  (embedding-only, no LLM required — set DEMO_MODE=graph for knowledge graph)")
	} else {
		fmt.Println("  (embedding + LLM entity extraction → knowledge graph + Think)")
	}

	// ─── 1. Harrier ONNX Embedder ─────────────────────────────────────────────
	fmt.Println("\n[1] Loading Harrier ONNX embedder …")
	harrierEmb, err := onnx.NewHarrierEmbedder(harrierModel, harrierTokenizer, 512,
		"Retrieve semantically similar text", true)
	must(err, "load harrier embedder")
	fmt.Printf("    Model : %s\n", harrierEmb.GetModel())
	fmt.Printf("    Dim   : %d\n", harrierEmb.GetDimensions())

	// ─── 2. Tokenizer Smoke Test ──────────────────────────────────────────────
	fmt.Println("\n[2] Tokenizer test …")
	tok, err := onnx.NewTokenizerFromFile(harrierTokenizer)
	must(err, "tokenizer")
	for _, t := range []string{
		"Hello, world!",
		"Harrier-OSS-v1-270m hỗ trợ đa ngôn ngữ.",
		"ONNX Runtime was developed by Microsoft.",
	} {
		ids := tok.Encode(t, true, true)
		fmt.Printf("    %q → %d tokens\n", truncate(t, 45), len(ids))
	}

	// ─── 3. Embedding Similarity Test ─────────────────────────────────────────
	fmt.Println("\n[3] Embedding similarity …")
	pairs := [][2]string{
		{"Harrier is a multilingual embedding model.", "This is a dense vector retrieval model."},
		{"Harrier is a multilingual embedding model.", "The weather in Hanoi is sunny today."},
	}
	for _, p := range pairs {
		eA, _ := harrierEmb.GenerateEmbedding(ctx, p[0])
		eB, _ := harrierEmb.GenerateEmbedding(ctx, p[1])
		fmt.Printf("    A: %q\n    B: %q\n    → sim: %.4f\n\n", truncate(p[0], 45), truncate(p[1], 45), cosineSim(eA, eB))
	}

	// ─── 4. AutoEmbedder + SQLite stores ──────────────────────────────────────
	fmt.Println("[4] Init stores (graph + vector + relational) …")
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

	// ─── 5. LLM Extractor (optional — only needed in graph mode) ─────────────
	//
	// RAG mode:   NullExtractor — Cognify stores embeddings only (no LLM calls).
	// Graph mode: LMStudio/Ollama — Cognify also extracts entities + relationships.
	//
	fmt.Println("[5] Init LLM extractor …")
	var llmExt extractor.LLMExtractor
	if mode == modeRAG {
		llmExt = extractor.NewNullExtractor()
		fmt.Println("    NullExtractor (embedding-only, no LLM needed)")
	} else {
		llmEndpoint := getEnv("LLM_ENDPOINT", "http://localhost:1234/v1")
		llmModel := getEnv("LLM_MODEL", "qwen/qwen3-4b-2507")

		llmFactory := registry.NewProviderFactory()
		llmProvider, err := llmFactory.CreateProvider(&extractor.ProviderConfig{
			Type:     extractor.ProviderLMStudio,
			Endpoint: llmEndpoint,
			Model:    llmModel,
		})
		must(err, "llm provider")
		llmExt = extractor.NewBasicExtractor(llmProvider, nil)
		fmt.Printf("    LMStudio: %s @ %s\n", llmModel, llmEndpoint)
	}

	// ─── 6. Memory Engine ─────────────────────────────────────────────────────
	eng := engine.NewMemoryEngineWithStores(
		llmExt, autoEmb, relStore, graphStore, vecStore,
		engine.EngineConfig{MaxWorkers: 4, ChunkConcurrency: 2},
	)
	defer eng.Close()

	// ─── 7. ADD: Ingest corpus with clear entities and relationships ───────────
	fmt.Println("\n[7] ADD: ingesting knowledge corpus …")
	sessionID := "harrier-graph-demo"

	// Texts are intentionally rich in named entities so the LLM can extract nodes + edges.
	corpus := []string{
		"Harrier-OSS-v1-270m is a multilingual text embedding model created by Microsoft Research with 270M parameters and 640-dimensional output vectors.",
		"Microsoft open-sourced Harrier under the MIT license, making it freely available for commercial and academic use.",
		"Harrier uses the Gemma3 architecture, a decoder-only transformer model originally developed by Google DeepMind.",
		"ONNX Runtime is an open-source inference engine developed by Microsoft that supports models exported in ONNX format.",
		"HuggingFace Transformers library supports exporting Harrier and other models to ONNX via the Optimum toolkit.",
		"RAG (Retrieval-Augmented Generation) combines a dense retrieval step using embeddings with an LLM generation step for factual answers.",
		"Knowledge graphs store entities such as people, organisations, and concepts as nodes, with labelled edges representing relationships.",
		"The ai-memory-brain project integrates Harrier embeddings with a SQLite-backed knowledge graph for local, privacy-preserving memory.",
	}

	var dps []*schema.DataPoint
	dpText := make(map[string]string)
	for _, text := range corpus {
		dp, err := eng.Add(ctx, text,
			engine.WithSessionID(sessionID),
			engine.WithMetadata(map[string]interface{}{
				"source": "harrier-graph-demo",
				"text":   text,
			}),
		)
		must(err, "add")
		dps = append(dps, dp)
		dpText[dp.ID] = text
		fmt.Printf("    + [%s] %s\n", dp.ID[:8], truncate(text, 65))
	}

	// ─── 8. COGNIFY: pipeline  Add → CognifyPending ───────────────────────────
	//
	// Pipeline walkthrough:
	//   CognifyPending loops until no more pending/processing DataPoints remain.
	//   For each "input" DataPoint it splits into chunk children (StatusPending).
	//   For each "chunk" DataPoint it:
	//     1. Generates embeddings via Harrier ONNX
	//     2. Extracts entities + relationships via LLM (sets DataPoint.Nodes / .Edges)
	//     3. Stores chunk embedding in vecStore
	//     4. Sets status → StatusCognified
	//     5. Submits MemifyTask to background worker pool
	//   MemifyTask (runs async): StoreNode + CreateRelationship in graphStore → StatusCompleted
	//
	fmt.Println("\n[8] COGNIFY: CognifyPending — full sync drain of all chunks …")
	cognifyStart := time.Now()
	if err := eng.CognifyPending(ctx, sessionID); err != nil {
		log.Printf("  warn CognifyPending: %v", err)
	}
	fmt.Printf("    CognifyPending done in %s\n", time.Since(cognifyStart).Round(time.Millisecond))

	// ─── 9. Wait for Memify (async background workers) ────────────────────────
	//
	// After CognifyPending all chunks are StatusCognified and MemifyTasks are
	// queued in the worker pool. In graph mode, poll until nodes appear.
	// In RAG mode, MemifyTask runs but writes no nodes (NullExtractor → empty Nodes[]).
	//
	if mode == modeGraph {
		fmt.Println("\n[9] Waiting for Memify workers to write graph nodes …")
		memifyDeadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(memifyDeadline) {
			n, _ := graphStore.GetNodeCount(ctx)
			if n > 0 {
				fmt.Printf("    ✓ Graph populated: %d node(s)\n", n)
				break
			}
			fmt.Print(".")
			time.Sleep(1 * time.Second)
		}
		fmt.Println()
	} else {
		fmt.Println("\n[9] RAG mode: Memify stores no graph nodes (NullExtractor).")
	}

	// ─── 10. VECTOR SEARCH ────────────────────────────────────────────────────
	fmt.Println("[10] VECTOR SEARCH: semantic similarity …")
	queries := []string{
		"What is Harrier and who created it?",
		"How does ONNX Runtime work?",
		"combining retrieval and generation for QA",
	}
	for _, q := range queries {
		qEmb, err := harrierEmb.GenerateQueryEmbedding(ctx, q)
		must(err, "query embed")
		hits, err := vecStore.SimilaritySearch(ctx, qEmb, 2, 0.0)
		if err != nil {
			log.Printf("  warn search: %v", err)
			continue
		}
		fmt.Printf("  Q: %q\n", truncate(q, 55))
		seen := make(map[string]bool)
		for _, h := range hits {
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
			fmt.Printf("    [%.3f] %s\n", h.Score, truncate(text, 75))
		}
	}

	// ─── 11. KNOWLEDGE GRAPH (graph mode only) ────────────────────────────────
	fmt.Println("\n[11] KNOWLEDGE GRAPH …")

	nodeCount, _ := graphStore.GetNodeCount(ctx)
	edgeCount, _ := graphStore.GetEdgeCount(ctx)
	fmt.Printf("  Nodes : %d\n", nodeCount)
	fmt.Printf("  Edges : %d\n", edgeCount)

	if mode == modeRAG {
		fmt.Println("  (RAG mode: graph is empty — entity extraction skipped)")
		fmt.Println("  Tip: run with DEMO_MODE=graph to populate knowledge graph via LLM")
	} else {
		// List nodes per type
		for _, nodeType := range []schema.NodeType{
			schema.NodeTypeConcept,
			schema.NodeTypeEntity,
			schema.NodeTypeDocument,
			schema.NodeTypeUser,
		} {
			ns, err := graphStore.FindNodesByType(ctx, nodeType)
			if err != nil || len(ns) == 0 {
				continue
			}
			fmt.Printf("\n  [%s] — %d node(s)\n", nodeType, len(ns))
			for _, n := range ns {
				name, _ := n.Properties["name"].(string)
				if name == "" {
					name = n.ID
				}
				fmt.Printf("    ● %s (%s)\n", name, shortID(n.ID))
			}
		}

		// Traverse from common anchor nodes that LLM likely extracted
		anchors := []string{"microsoft", "harrier", "harrier-oss-v1-270m", "onnx-runtime", "onnx", "rag"}
		for _, anchor := range anchors {
			neighbors, err := graphStore.TraverseGraph(ctx, anchor, 2, nil)
			if err != nil || len(neighbors) == 0 {
				continue
			}
			fmt.Printf("\n  TraverseGraph from %q (depth 2): %d connected nodes\n", anchor, len(neighbors))
			for _, n := range neighbors {
				name, _ := n.Properties["name"].(string)
				if name == "" {
					name = n.ID
				}
				fmt.Printf("    → [%s] %s\n", n.Type, name)
			}
			break
		}

		// Entity search
		fmt.Println("\n  FindNodesByProperty (name='microsoft') …")
		msNodes, err := graphStore.FindNodesByProperty(ctx, "name", "microsoft")
		if err == nil && len(msNodes) > 0 {
			for _, n := range msNodes {
				fmt.Printf("    ● [%s] %s\n", n.Type, n.ID)
				connected, _ := graphStore.FindConnected(ctx, n.ID, nil)
				for _, c := range connected {
					cName, _ := c.Properties["name"].(string)
					fmt.Printf("      ↔ %s (%s)\n", cName, c.ID)
				}
			}
		} else {
			fmt.Println("    (none — check extracted node names above)")
		}
	}

	// ─── 12. THINK: multi-hop graph reasoning (graph mode only) ───────────────
	fmt.Println("\n[12] THINK: multi-hop graph reasoning …")
	if mode == modeRAG {
		fmt.Println("  (RAG mode: Think uses vector search only — no graph hops)")
		fmt.Println("  Tip: run with DEMO_MODE=graph to enable multi-hop reasoning")
	}
	thinkQuery := "What projects and technologies is Microsoft involved in?"
	thinkResult, err := eng.Think(ctx, &schema.ThinkQuery{
		Text:      thinkQuery,
		SessionID: sessionID,
		Limit:     5,
		HopDepth:  2,
	})
	if err != nil {
		log.Printf("  warn Think: %v", err)
	} else {
		fmt.Printf("  Q: %q\n", thinkQuery)
		if thinkResult.Reasoning != "" {
			fmt.Printf("\n  Reasoning:\n%s\n", indent(thinkResult.Reasoning, "  "))
		}
		if thinkResult.Answer != "" {
			fmt.Printf("\n  Answer:\n%s\n", indent(thinkResult.Answer, "  "))
		}
	}

	// ─── 13. Health check ─────────────────────────────────────────────────────
	fmt.Println("\n[13] Health check …")
	if err := harrierEmb.Health(ctx); err != nil {
		fmt.Printf("  Harrier: FAIL — %v\n", err)
	} else {
		fmt.Println("  Harrier: OK ✓")
	}
	vecCnt, _ := vecStore.GetEmbeddingCount(ctx)
	fmt.Printf("  Vectors : %d\n", vecCnt)
	fmt.Printf("  Nodes   : %d\n", nodeCount)
	fmt.Printf("  Edges   : %d\n", edgeCount)
	fmt.Printf("  DPs     : %d\n", len(dps))

	fmt.Println("\n✅  Harrier-OSS-v1-270m × ONNX Runtime × Graph demo complete.")
	fmt.Printf("    Data: %s\n", dataDir)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

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

func shortID(id string) string {
	if len(id) > 10 {
		return id[:10] + "…"
	}
	return id
}

func indent(s, prefix string) string {
	lines := []rune(s)
	out := []rune(prefix)
	for _, r := range lines {
		out = append(out, r)
		if r == '\n' {
			out = append(out, []rune(prefix)...)
		}
	}
	return string(out)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func stripChunkSuffix(id string) string {
	for {
		idx := strings.LastIndex(id, "-chunk-")
		if idx < 0 {
			return id
		}
		id = id[:idx]
	}
}

func mustExist(path, msg string) {
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("FATAL: %s\n  path: %s", msg, path)
	}
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL [%s]: %v", label, err)
	}
}
