// Ví dụ: Pipeline hoàn toàn offline — Harrier-OSS-v1-270m + DeBERTa-v3-large NER
//
// Không cần LMStudio, Ollama hay bất kỳ LLM server nào.
// Tất cả chạy local qua ONNX Runtime:
//
//	Harrier-OSS-v1-270m   → embedding 640 chiều (semantic search)
//	DeBERTa-v3-large NER  → trích xuất thực thể + cạnh co-occurrence (knowledge graph)
//
// ┌──────────────────────────────────────────────────────────────────────┐
// │  VĂN BẢN → Add → CognifyPending → VectorSearch + GraphTraversal     │
// │                                                                      │
// │  Pipeline Cognify (offline):                                         │
// │   chunk → Harrier embed   → SQLite vector store                      │
// │        → DeBERTa NER      → thực thể (PER/ORG/LOC/MISC) + cạnh     │
// │        → MemifyTask async → SQLite graph store                       │
// └──────────────────────────────────────────────────────────────────────┘
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
//
// Chọn GPU:
//
//	ORT_PROVIDER=coreml go run ./examples/offline_graph/   # Apple Silicon
//	ORT_PROVIDER=cuda   go run ./examples/offline_graph/   # NVIDIA
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

	// ─── Đường dẫn ────────────────────────────────────────────────────────────
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")

	// onnxmodel.ResolveModel (auto) chọn harrier-q (INT8) nếu có, không thì FP32.
	harrierModel := filepath.Join(root, "data", "harrier", "model.onnx")
	harrierTok := filepath.Join(root, "data", "harrier", "tokenizer.json")

	// Ưu tiên model nhỏ nhất theo thứ tự: base > large-INT8 > large-FP32
	// base:     ~400 MB  (make export-deberta-base)
	// large-q:  ~640 MB  (đã quantize tự động)
	// large:   ~1738 MB  (mặc định khi export lần đầu)
	debertaDir := filepath.Join(root, "data", "deberta-ner-base")
	if _, err := os.Stat(filepath.Join(debertaDir, "model.onnx")); err != nil {
		debertaDir = filepath.Join(root, "data", "deberta-ner-q")
	}
	if _, err := os.Stat(filepath.Join(debertaDir, "model.onnx")); err != nil {
		debertaDir = filepath.Join(root, "data", "deberta-ner")
	}
	debertaModel := filepath.Join(debertaDir, "model.onnx")
	debertaTok := filepath.Join(debertaDir, "tokenizer.json")
	debertaLabels := filepath.Join(debertaDir, "labels.json")

	dataDir := filepath.Join(root, "data", "offline_graph_demo")
	_ = os.RemoveAll(dataDir)
	_ = os.MkdirAll(dataDir, 0o750)

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
	section("[1] Harrier-OSS-v1-270m — bộ mã hoá embedding")
	harrierEmb, err := onnx.NewHarrierEmbedder(harrierModel, harrierTok, 512,
		"Retrieve semantically similar text", true)
	must(err, "harrier embedder")
	fmt.Printf("    Mô hình  : %s\n", harrierEmb.GetModel())
	fmt.Printf("    Chiều    : %d\n", harrierEmb.GetDimensions())
	fmt.Printf("    Provider : %s\n", harrierEmb.GetExecutionProvider())
	fmt.Printf("    Trọng số : %s  (fp32|int8 — ORT_EMBED_PRECISION / auto)\n", harrierEmb.GetModelPrecision())
	if err := harrierEmb.Health(ctx); err != nil {
		log.Fatalf("FATAL Harrier health check: %v", err)
	}
	fmt.Println("    Trạng thái: OK ✓")

	// ─── 2. DeBERTa-v3 NER ────────────────────────────────────────────────────
	section("[2] DeBERTa-v3-large — nhận diện thực thể (NER)")
	debExt, err := deberta.NewExtractor(deberta.Config{
		ModelPath:     debertaModel,
		TokenizerPath: debertaTok,
		LabelsPath:    debertaLabels,
		// MaxSeqLen 256 tiết kiệm RAM: DeBERTa-v3-large với CoreML tốn ~4-6 GB GPU buffer.
		// CPU provider (mặc định) chỉ cần ~200 MB — đủ nhanh cho NER.
		// Dùng ORT_NER_PROVIDER=coreml để bật GPU cho DeBERTa nếu muốn.
		MaxSeqLen: 256,
	})
	must(err, "deberta extractor")
	fmt.Println("    Mô hình   : Gladiator/microsoft-deberta-v3-large_ner_conll2003")
	fmt.Println("    Nhãn      : O B/I-PER B/I-ORG B/I-LOC B/I-MISC")
	fmt.Printf("    Provider  : %s\n", debExt.GetExecutionProvider())
	fmt.Printf("    Trọng số  : %s  (ORT_NER_PRECISION / auto)\n", debExt.GetModelPrecision())
	fmt.Println("    Trạng thái: OK ✓")

	// Thử nghiệm nhanh NER
	smokeText := "Satya Nadella is the CEO of Microsoft, founded by Bill Gates in Seattle."
	smokeNodes, _ := debExt.ExtractEntities(ctx, smokeText)
	fmt.Printf("    Thử NER   : %q\n    →", truncate(smokeText, 55))
	for _, n := range smokeNodes {
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
	fmt.Println("    graph.db ✓   vectors.db ✓   rel.db ✓")

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	section("[4] Memory engine")
	eng := engine.NewMemoryEngineWithStores(
		debExt,   // LLMExtractor → DeBERTa-v3 NER (không cần LLM server!)
		autoEmb,  // EmbeddingProvider → Harrier ONNX
		relStore,
		graphStore,
		vecStore,
		engine.EngineConfig{MaxWorkers: 4, ChunkConcurrency: 2},
	)
	defer eng.Close()
	fmt.Printf("    Embedder  : Harrier-OSS-v1-270m  [%s]\n", harrierEmb.GetExecutionProvider())
	fmt.Printf("    Extractor : DeBERTa-v3-large NER [%s]\n", debExt.GetExecutionProvider())
	fmt.Println("    LLM server: — (không cần)")
	fmt.Println("    ── RAM guide ─────────────────────────────────────────────")
	fmt.Println("    Default (cpu+cpu)          : ~2 GB  — tiết kiệm nhất")
	fmt.Println("    ORT_EMBED_PROVIDER=coreml  : ~4 GB  — Harrier dùng GPU")
	fmt.Println("    ORT_NER_PROVIDER=coreml    : ~6 GB  — DeBERTa dùng GPU")
	fmt.Println("    base model (--size base)   : 4x nhỏ hơn — export lại")
	fmt.Println("    ORT_MODEL_PRECISION=fp32   : ép FP32 (bỏ qua thư mục -q)")
	// ─── 5. ADD corpus tiếng Việt ─────────────────────────────────────────────
	section("[5] ADD — nạp kho tri thức tiếng Việt")
	sessionID := "offline-graph-vi"

	corpus := []struct{ text, topic string }{
		{
			"Harrier-OSS-v1-270m là mô hình embedding văn bản đa ngôn ngữ do Microsoft Research tạo ra. Mô hình sinh vector 640 chiều và hỗ trợ hơn 100 ngôn ngữ bao gồm tiếng Việt.",
			"mô-hình",
		},
		{
			"Microsoft công bố mã nguồn mở Harrier theo giấy phép MIT. Satya Nadella, CEO của Microsoft, thông báo phát hành tại Microsoft Build 2025 ở Seattle.",
			"thông-báo",
		},
		{
			"DeBERTa-v3 là mô hình transformer do Microsoft Research phát triển, sử dụng cơ chế disentangled attention và enhanced mask decoder cho các tác vụ NLP như nhận diện thực thể.",
			"mô-hình",
		},
		{
			"ONNX Runtime là engine suy luận mã nguồn mở từ Microsoft, tăng tốc các mô hình học máy bao gồm DeBERTa và Harrier thông qua provider CoreML và CUDA.",
			"hạ-tầng",
		},
		{
			"HuggingFace là công ty AI duy trì thư viện Transformers và bộ công cụ Optimum dùng để xuất mô hình sang định dạng ONNX.",
			"công-ty",
		},
		{
			"Google DeepMind phát triển kiến trúc Gemma3 mà mô hình Harrier dựa trên đó. Gemma3 là transformer chỉ-giải-mã (decoder-only).",
			"nghiên-cứu",
		},
		{
			"RAG (Retrieval-Augmented Generation) kết hợp bước truy xuất vector dày đặc với bước sinh ngôn ngữ để trả lời câu hỏi thực tế chính xác hơn.",
			"kỹ-thuật",
		},
		{
			"Dự án ai-memory-brain kết hợp Harrier embedding với DeBERTa NER và SQLite graph store để tạo pipeline bộ nhớ hoàn toàn offline, không cần LLM server.",
			"dự-án",
		},
	}

	var dps []*schema.DataPoint
	dpText := make(map[string]string)
	for _, item := range corpus {
		dp, err := eng.Add(ctx, item.text,
			engine.WithSessionID(sessionID),
			engine.WithMetadata(map[string]interface{}{
				"topic":  item.topic,
				"source": "offline-graph-vi",
				"text":   item.text,
			}),
		)
		must(err, "add")
		dps = append(dps, dp)
		dpText[dp.ID] = item.text
		fmt.Printf("    + [%s] (%s) %s\n", dp.ID[:8], item.topic, truncate(item.text, 60))
	}

	// ─── 6. COGNIFY ───────────────────────────────────────────────────────────
	section("[6] COGNIFY — Harrier embed + DeBERTa NER")
	fmt.Println("    Đang xử lý tất cả chunk …")

	t0 := time.Now()
	if err := eng.CognifyPending(ctx, sessionID); err != nil {
		log.Printf("  cảnh báo CognifyPending: %v", err)
	}
	cogDur := time.Since(t0)
	runtime.GC() // giải phóng bộ nhớ tạm sau embed+NER
	fmt.Printf("    CognifyPending: xong trong %s  (%.1f văn bản/giây)\n",
		cogDur.Round(time.Millisecond), float64(len(corpus))/cogDur.Seconds())

	// ─── 7. Chờ Memify (graph writes) ────────────────────────────────────────
	section("[7] Chờ Memify → knowledge graph")
	deadline := time.Now().Add(60 * time.Second)
	var nodeCount, edgeCount int64
	for time.Now().Before(deadline) {
		nodeCount, _ = graphStore.GetNodeCount(ctx)
		if nodeCount > 0 {
			edgeCount, _ = graphStore.GetEdgeCount(ctx)
			fmt.Printf("    ✓ Graph sẵn sàng: %d node, %d cạnh\n", nodeCount, edgeCount)
			break
		}
		fmt.Print(".")
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println()
	if nodeCount == 0 {
		fmt.Println("    ⚠ Không có graph node (NER không tìm thấy thực thể?)")
	}

	// ─── 8. KNOWLEDGE GRAPH ───────────────────────────────────────────────────
	section("[8] KNOWLEDGE GRAPH — thực thể theo loại")
	for _, nodeType := range []schema.NodeType{schema.NodeTypeEntity, schema.NodeTypeConcept} {
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
	section("[9] GRAPH TRAVERSAL — cạnh co-occurrence")
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
		neighbors, err := graphStore.TraverseGraph(ctx, ms.ID, 2, nil)
		if err == nil {
			fmt.Printf("\n  TraverseGraph depth-2 từ %q: %d node có thể đến\n", msName, len(neighbors))
			for _, n := range neighbors {
				nName, _ := n.Properties["name"].(string)
				fmt.Printf("    → [%s] %s\n", n.Type, nName)
			}
		}
	} else {
		fmt.Println("  (không tìm thấy node 'Microsoft' — xem NER output bên trên)")
	}

	// ─── 10. TÌM KIẾM VECTOR ─────────────────────────────────────────────────
	section("[10] TÌM KIẾM VECTOR — semantic similarity (Harrier)")
	queries := []string{
		"Harrier được tạo ra bởi ai?",
		"engine suy luận mã nguồn mở cho học máy",
		"kết hợp truy xuất và sinh ngôn ngữ để hỏi đáp",
		"nhận diện thực thể trong văn bản",
	}
	for _, q := range queries {
		qEmb, err := harrierEmb.GenerateQueryEmbedding(ctx, q)
		must(err, "query embed")
		hits, err := vecStore.SimilaritySearch(ctx, qEmb, 2, 0.3)
		if err != nil {
			log.Printf("  cảnh báo search: %v", err)
			continue
		}
		fmt.Printf("\n  Q: %q\n", q)
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
			fmt.Printf("    [%.3f] %s\n", h.Score, truncate(text, 80))
		}
	}

	// ─── 11. HYBRID SEARCH (vector + graph) ──────────────────────────────────
	section("[11] HYBRID SEARCH — vector + knowledge graph")
	hybridQueries := []string{
		"Mối liên hệ giữa Microsoft và ONNX Runtime?",
		"RAG là gì và ứng dụng như thế nào?",
	}
	for _, q := range hybridQueries {
		results, err := eng.Search(ctx, &schema.SearchQuery{
			Text:                q,
			SessionID:           sessionID,
			Mode:                schema.ModeHybridSearch,
			Limit:               3,
			HopDepth:            1,
			SimilarityThreshold: 0.3,
		})
		if err != nil {
			log.Printf("  hybrid search lỗi: %v", err)
			continue
		}
		fmt.Printf("\n  Q: %q\n", q)
		fmt.Printf("  → %d kết quả (query time: %s)\n", results.Total, results.QueryTime.Round(time.Millisecond))
		for i, r := range results.Results {
			if i >= 3 {
				break
			}
			if r.DataPoint == nil {
				continue
			}
			content := r.DataPoint.Content
			fmt.Printf("    [vec=%.3f graph=%.3f] %s\n",
				r.VectorScore, r.GraphScore, truncate(content, 75))
		}
	}

	// ─── 12. Kiểm tra tương đồng embedding ───────────────────────────────────
	section("[12] Kiểm tra tương đồng embedding")
	pairs := [][2]string{
		{"Harrier là mô hình embedding đa ngôn ngữ.", "Đây là mô hình truy xuất vector dày đặc."},
		{"Harrier là mô hình embedding đa ngôn ngữ.", "Thời tiết hôm nay ở Hà Nội rất đẹp."},
		{"DeBERTa-v3 nhận diện thực thể.", "Mô hình NER phát hiện người và tổ chức trong văn bản."},
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

	// ─── 13. Tóm tắt ──────────────────────────────────────────────────────────
	section("[13] Tóm tắt")
	vecCnt, _ := vecStore.GetEmbeddingCount(ctx)
	nodeCount, _ = graphStore.GetNodeCount(ctx)
	edgeCount, _ = graphStore.GetEdgeCount(ctx)
	allocMiB, sysMiB := memUsage()

	fmt.Printf(`
  Pipeline     : 100%% offline (không cần LLM server)
  ─────────────────────────────────────────────────
  Embedder     : Harrier-OSS-v1-270m  (640 chiều, đa ngôn ngữ)  [%s]
  NER extractor: DeBERTa-v3-large     (CoNLL-2003, PER/ORG/LOC/MISC)  [%s]
  Inference    : ONNX Runtime  (ORT_PROVIDER=coreml|cuda|cpu|auto)
  ─────────────────────────────────────────────────
  DataPoints   : %d
  Vectors      : %d
  Graph nodes  : %d
  Graph edges  : %d
  Cognify time : %s
  ─────────────────────────────────────────────────
  RAM alloc    : %.1f MiB   (Go heap đang dùng)
  RAM sys      : %.1f MiB   (tổng OS cấp)
  ─────────────────────────────────────────────────
  Data dir     : %s

`, harrierEmb.GetExecutionProvider(), debExt.GetExecutionProvider(),
		len(dps), vecCnt, nodeCount, edgeCount,
		cogDur.Round(time.Millisecond),
		allocMiB, sysMiB,
		dataDir,
	)
	fmt.Println("✅  Offline graph demo (tiếng Việt) hoàn tất.")
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func banner() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Pipeline Knowledge Graph Offline — 100% ONNX Local Inference   ║")
	fmt.Println("║  Harrier-OSS-v1-270m (embed) + DeBERTa-v3-large (NER)          ║")
	fmt.Println("║  Không LMStudio · Không Ollama · Không API key                 ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
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

// stripChunkSuffix loại bỏ hậu tố "-chunk-NNN" để lấy root DataPoint ID.
func stripChunkSuffix(id string) string {
	for {
		idx := strings.LastIndex(id, "-chunk-")
		if idx < 0 {
			return id
		}
		id = id[:idx]
	}
}

// memUsage trả về (allocMiB, sysMiB) — heap đang dùng và tổng OS cấp cho Go runtime.
func memUsage() (allocMiB, sysMiB float64) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.Alloc) / 1024 / 1024,
		float64(ms.Sys) / 1024 / 1024
}

func mustExist(path, hint string) {
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("FATAL: file không tìm thấy\n  path: %s\n  fix : %s", path, hint)
	}
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL [%s]: %v", label, err)
	}
}
