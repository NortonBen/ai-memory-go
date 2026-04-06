// Ví dụ: engine (Add / CognifyPending / Search) + suy luận 100% ONNX Runtime — không LLM server.
//
// Hai mô hình ONNX được nạp rồi inject vào engine:
//   • Harrier-OSS-v1-270m — vector/embedders/onnx (embedding 640 chiều)
//   • DeBERTa-v3 — extractor/deberta (NER, AnalyzeQuery, quan hệ cục bộ)
//
// engine chỉ điều phối: CognifyTask tách chunk (formats.TextParser), embed + trích xuất qua
// các provider ONNX; MemifyTask đẩy graph. ORT_EMBED_PRECISION / ORT_MODEL_PRECISION / ORT_*_PROVIDER
// vẫn áp dụng như mọi example ONNX khác.
//
// Giới hạn demo: TRUY_HEAD_RUNES>0 chỉ nạp đầu file.
//
//	go run ./examples/truy_pipeline/
//	TRUY_HEAD_RUNES=60000 go run ./examples/truy_pipeline/
//	ORT_PROVIDER=coreml go run ./examples/truy_pipeline/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
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

const nerPreviewRunes = 500 // minh họa DeBERTa ONNX trước khi chạy engine (cùng extractor)

// onnxModelPaths — artifact Harrier + DeBERTa cho ONNX Runtime.
type onnxModelPaths struct {
	Root            string
	HarrierONNX     string
	HarrierTokenizer string
	DebertaONNX     string
	DebertaTokenizer string
	DebertaLabels   string
}

// onnxOfflineStack gói providers ONNX + SQLite stores + engine.MemoryEngine.
type onnxOfflineStack struct {
	Emb        *onnx.HarrierEmbedder
	Deberta    *deberta.Extractor
	AutoEmb    *vector.AutoEmbedder
	GraphStore graph.GraphStore
	VecStore   vector.VectorStore
	RelStore   storage.Storage
	Engine     engine.MemoryEngine
}

func (s *onnxOfflineStack) Close() {
	if s.Engine != nil {
		_ = s.Engine.Close()
	}
	if s.GraphStore != nil {
		_ = s.GraphStore.Close()
	}
	if s.VecStore != nil {
		_ = s.VecStore.Close()
	}
	if s.RelStore != nil {
		_ = s.RelStore.Close()
	}
}

func resolveONNXModelPaths(repoRoot string) onnxModelPaths {
	harrierONNX := filepath.Join(repoRoot, "data", "harrier", "model.onnx")
	harrierTok := filepath.Join(repoRoot, "data", "harrier", "tokenizer.json")
	debertaDir := filepath.Join(repoRoot, "data", "deberta-ner-base")
	if _, err := os.Stat(filepath.Join(debertaDir, "model.onnx")); err != nil {
		debertaDir = filepath.Join(repoRoot, "data", "deberta-ner-q")
	}
	if _, err := os.Stat(filepath.Join(debertaDir, "model.onnx")); err != nil {
		debertaDir = filepath.Join(repoRoot, "data", "deberta-ner")
	}
	return onnxModelPaths{
		Root:             repoRoot,
		HarrierONNX:      harrierONNX,
		HarrierTokenizer: harrierTok,
		DebertaONNX:      filepath.Join(debertaDir, "model.onnx"),
		DebertaTokenizer: filepath.Join(debertaDir, "tokenizer.json"),
		DebertaLabels:    filepath.Join(debertaDir, "labels.json"),
	}
}

func (p onnxModelPaths) verify() {
	for _, x := range []struct{ path, hint string }{
		{p.HarrierONNX, "python scripts/export_harrier_onnx.py"},
		{p.HarrierTokenizer, "python scripts/export_harrier_onnx.py"},
		{p.DebertaONNX, "python scripts/export_deberta_onnx.py"},
		{p.DebertaTokenizer, "python scripts/export_deberta_onnx.py"},
		{p.DebertaLabels, "python scripts/export_deberta_onnx.py"},
	} {
		mustExist(x.path, x.hint)
	}
}

// openONNXOfflineEngine tạo Harrier + DeBERTa ONNX, AutoEmbedder, stores và engine.NewMemoryEngineWithStores.
func openONNXOfflineEngine(dataDir string, p onnxModelPaths, engCfg engine.EngineConfig) (*onnxOfflineStack, error) {
	emb, err := onnx.NewHarrierEmbedder(p.HarrierONNX, p.HarrierTokenizer, 512,
		"Retrieve semantically similar text", true)
	if err != nil {
		return nil, fmt.Errorf("harrier onnx: %w", err)
	}
	ext, err := deberta.NewExtractor(deberta.Config{
		ModelPath:     p.DebertaONNX,
		TokenizerPath: p.DebertaTokenizer,
		LabelsPath:    p.DebertaLabels,
		MaxSeqLen:     256,
	})
	if err != nil {
		return nil, fmt.Errorf("deberta onnx: %w", err)
	}

	autoEmb := vector.NewAutoEmbedder("onnx", vector.NewInMemoryEmbeddingCache())
	autoEmb.AddProvider("onnx", emb)

	graphStore, err := graph.NewSQLiteGraphStore(filepath.Join(dataDir, "graph.db"))
	if err != nil {
		return nil, err
	}
	vecStore, err := vector.NewSQLiteVectorStore(filepath.Join(dataDir, "vectors.db"), 640)
	if err != nil {
		_ = graphStore.Close()
		return nil, err
	}
	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    filepath.Join(dataDir, "rel.db"),
		ConnTimeout: 5 * time.Second,
	})
	if err != nil {
		_ = vecStore.Close()
		_ = graphStore.Close()
		return nil, err
	}

	eng := engine.NewMemoryEngineWithStores(
		ext, autoEmb, relStore, graphStore, vecStore, engCfg,
	)

	return &onnxOfflineStack{
		Emb:        emb,
		Deberta:    ext,
		AutoEmb:    autoEmb,
		GraphStore: graphStore,
		VecStore:   vecStore,
		RelStore:   relStore,
		Engine:     eng,
	}, nil
}

func main() {
	ctx := context.Background()

	_, filename, _, _ := goruntime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	truyPath := filepath.Join(repoRoot, "parser", "formats", "truy.txt")
	paths := resolveONNXModelPaths(repoRoot)

	dataDir := filepath.Join(repoRoot, "data", "truy_pipeline_demo")
	_ = os.RemoveAll(dataDir)
	_ = os.MkdirAll(dataDir, 0o750)

	mustExist(truyPath, "đặt file parser/formats/truy.txt trong repo")
	paths.verify()

	raw, err := os.ReadFile(truyPath)
	must(err, "đọc truy.txt")
	fileRunes := len([]rune(string(raw)))

	headRunes := 0
	headEnv := os.Getenv("TRUY_HEAD_RUNES")
	if headEnv != "" {
		if n, e := strconv.Atoi(headEnv); e == nil && n > 0 {
			headRunes = n
		}
	}
	payload := string(raw)
	if headRunes > 0 {
		r := []rune(payload)
		if len(r) > headRunes {
			payload = string(r[:headRunes])
		}
	}
	payloadRunes := len([]rune(payload))

	banner(len(raw), fileRunes, payloadRunes, headRunes, headEnv)
	ramCheckpoint("Khởi động")

	section("[1] Nạp ONNX (Harrier + DeBERTa) — chuẩn bị inject vào engine")
	t0 := time.Now()
	stack, err := openONNXOfflineEngine(dataDir, paths, engine.EngineConfig{
		MaxWorkers:       4,
		ChunkConcurrency: 2,
	})
	must(err, "onnx + engine")
	defer stack.Close()

	fmt.Printf("    Harrier ONNX : %s  [%s]  precision=%s  (%s)\n",
		stack.Emb.GetModel(), stack.Emb.GetExecutionProvider(), stack.Emb.GetModelPrecision(),
		filepath.Base(paths.HarrierONNX))
	fmt.Printf("    DeBERTa ONNX : microsoft-deberta-v3-large_ner  [%s]  precision=%s  (%s)\n",
		stack.Deberta.GetExecutionProvider(), stack.Deberta.GetModelPrecision(),
		filepath.Base(paths.DebertaONNX))
	fmt.Printf("    Thời gian nạp : %s\n", time.Since(t0).Round(time.Millisecond))
	ramCheckpoint("Sau ONNX load")

	section("[2] engine.MemoryEngine — ONNX là backend suy luận")
	fmt.Println("    NewMemoryEngineWithStores(")
	fmt.Println("      deberta.Extractor,     // LLMExtractor → chỉ ONNX Runtime (DeBERTa)")
	fmt.Println("      vector.AutoEmbedder,   // default \"onnx\" → HarrierEmbedder ONNX")
	fmt.Println("      relStore, graphStore, vecStore,")
	fmt.Println("    )")
	fmt.Println("    LLM HTTP (Ollama/LMStudio): không dùng — Cognify/Search dựa trên ONNX.")
	fmt.Println()
	fmt.Println("    Pipeline chunk: CognifyTask (task_cognify.go) → TextParser; MemifyTask (task_memify.go) → graph.")

	preview := payload
	if len([]rune(preview)) > nerPreviewRunes {
		preview = string([]rune(preview)[:nerPreviewRunes])
	}
	fmt.Printf("\n  NER ONNX minh họa (~%d rune đầu payload):\n", nerPreviewRunes)
	fmt.Printf("    %s\n", truncate(preview, 120))
	nodes, err := stack.Deberta.ExtractEntities(ctx, preview)
	if err != nil {
		log.Printf("  NER preview: %v", err)
	} else if len(nodes) == 0 {
		fmt.Println("    Thực thể: (không)")
	} else {
		for label, names := range groupByLabel(nodes) {
			fmt.Printf("    [%s] %s\n", label, strings.Join(names, " · "))
		}
	}
	fmt.Println()
	ramCheckpoint("Sau NER preview")

	eng := stack.Engine
	relStore := stack.RelStore

	sessionID := "truy-pipeline"
	title := "truy.txt — văn bản mẫu"

	section("[3] engine.Add — một DataPoint, chunk tự động trong Cognify")
	rootDP, err := eng.Add(ctx, payload,
		engine.WithSessionID(sessionID),
		engine.WithMetadata(map[string]interface{}{
			"title":  title,
			"source": "parser/formats/truy.txt",
			"text":   payload,
		}),
	)
	must(err, "add")
	fmt.Printf("    parent id=%s  (%d byte · %d rune)\n", rootDP.ID[:8], len(payload), payloadRunes)

	section("[4] engine.CognifyPending — ONNX embed + NER từng chunk + Memify")
	t0 = time.Now()
	if err := eng.CognifyPending(ctx, sessionID); err != nil {
		log.Printf("  cảnh báo: %v", err)
	}
	cogDur := time.Since(t0)
	goruntime.GC()

	parentReload, _ := relStore.GetDataPoint(ctx, rootDP.ID)
	engineChunks := 0
	if parentReload != nil && parentReload.Metadata != nil {
		engineChunks = metaInt(parentReload.Metadata, "total_chunks")
	}
	if engineChunks > 0 {
		fmt.Printf("    Xong %s — %d chunk con (metadata total_chunks)\n",
			cogDur.Round(time.Millisecond), engineChunks)
		fmt.Printf("    (~%.2f chunk/giây)\n",
			float64(engineChunks)/max64(cogDur.Seconds(), 1e-6))
	} else {
		fmt.Printf("    Xong %s\n", cogDur.Round(time.Millisecond))
	}
	ramCheckpoint("Sau Cognify")

	section("[5] Chờ graph (sau Memify)")
	deadline := time.Now().Add(120 * time.Second)
	var nodeCount, edgeCount int64
	for time.Now().Before(deadline) {
		nodeCount, _ = stack.GraphStore.GetNodeCount(ctx)
		if nodeCount > 0 {
			edgeCount, _ = stack.GraphStore.GetEdgeCount(ctx)
			fmt.Printf("    %d node · %d cạnh\n", nodeCount, edgeCount)
			break
		}
		fmt.Print(".")
		time.Sleep(400 * time.Millisecond)
	}
	fmt.Println()

	section("[6] engine.Search — HybridSearch (vector Harrier ONNX + graph DeBERTa)")
	queries := []struct {
		text    string
		comment string
	}{
		{"Bạch Dĩ hoặc nam chính thức tỉnh chức nghiệp thế nào?", "kỳ vọng: hạch hệ, xanh lục, hệ thống"},
		{"Ma vương Begoo giao dịch gì với nam chính?", "kỳ vọng: trái tim, nguyên tố, quá khứ"},
		{"Hồng Cương Thịnh hay Hồng Cường Thịnh đánh nhau với ai ở trường?", "kỳ vọng: nam chính, trường học"},
		{"Binh đoàn thần thánh và nhiệm vụ thanh trừng là gì?", "kỳ vọng: đồng đội, phản bội"},
	}

	for qi, tq := range queries {
		fmt.Printf("\n  ─── Q%d: %q\n", qi+1, tq.text)
		fmt.Printf("      💡 %s\n", tq.comment)
		analysis, _ := stack.Deberta.AnalyzeQuery(ctx, tq.text)
		sr, err := eng.Search(ctx, &schema.SearchQuery{
			Text:                tq.text,
			SessionID:           sessionID,
			Mode:                schema.ModeHybridSearch,
			Limit:               4,
			HopDepth:            2,
			SimilarityThreshold: 0.22,
			Analysis:            analysis,
		})
		if err != nil {
			fmt.Printf("      Lỗi: %v\n", err)
			continue
		}
		fmt.Printf("      %d kết quả (%s)\n", sr.Total, sr.QueryTime.Round(time.Millisecond))
		for i, r := range sr.Results {
			if i >= 3 || r.DataPoint == nil {
				break
			}
			fmt.Printf("      [%.3f] %s\n", r.Score, truncate(r.DataPoint.Content, 95))
		}
	}
	ramCheckpoint("Sau Search")

	section("[7] Tóm tắt")
	vecCnt, _ := stack.VecStore.GetEmbeddingCount(ctx)
	nodeCount, _ = stack.GraphStore.GetNodeCount(ctx)
	edgeCount, _ = stack.GraphStore.GetEdgeCount(ctx)
	dpCount := countSessionDataPoints(ctx, relStore, sessionID)
	allocMiB, sysMiB := memUsage()

	headNote := "0 = cả file"
	if headEnv != "" {
		headNote = headEnv
	}

	fmt.Printf(`
  ──────────────────────────────────────────────────
  Stack       : engine + ONNX (Harrier embed + DeBERTa NER)
  Nguồn       : parser/formats/truy.txt
  File gốc    : %d byte · %d rune
  Payload     : %d byte · %d rune  (TRUY_HEAD_RUNES=%s)
  Chunk engine: %d (total_chunks)
  ──────────────────────────────────────────────────
  DataPoints  : %d
  Vectors     : %d
  Graph       : %d node · %d cạnh
  Cognify     : %s
  RAM heap    : %.1f MiB / sys %.1f MiB
  ONNX EP     : embed=%s  ner=%s
  Precision   : embed=%s  ner=%s
  Data dir    : %s
  ──────────────────────────────────────────────────
`,
		len(raw), fileRunes,
		len(payload), payloadRunes, headNote,
		engineChunks,
		dpCount, vecCnt, nodeCount, edgeCount,
		cogDur.Round(time.Millisecond),
		allocMiB, sysMiB,
		stack.Emb.GetExecutionProvider(), stack.Deberta.GetExecutionProvider(),
		stack.Emb.GetModelPrecision(), stack.Deberta.GetModelPrecision(),
		dataDir,
	)
	fmt.Println("✅  truy_pipeline hoàn tất.")
}

func countSessionDataPoints(ctx context.Context, store storage.Storage, sessionID string) int {
	q := &storage.DataPointQuery{SessionID: sessionID, Limit: 100000}
	all, err := store.QueryDataPoints(ctx, q)
	if err != nil {
		return -1
	}
	return len(all)
}

func metaInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	case float32:
		return int(x)
	default:
		return 0
	}
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func banner(fileBytes, fileRunes, payloadRunes, headRunes int, headEnv string) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  truy_pipeline — engine + ONNX Runtime (Harrier + DeBERTa)       ║")
	fmt.Println("║  truy.txt → Add → CognifyPending → Search                         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n  File: %d KB · %d rune — payload: %d rune\n", fileBytes/1024, fileRunes, payloadRunes)
	if headRunes > 0 {
		fmt.Printf("  TRUY_HEAD_RUNES=%s (chỉ nạp đầu file)\n", headEnv)
	} else {
		fmt.Println("  TRUY_HEAD_RUNES không set → nạp toàn bộ file")
	}
	fmt.Println()
}

func section(title string) { fmt.Printf("\n%s\n", title) }

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func groupByLabel(nodes []schema.Node) map[string][]string {
	m := map[string][]string{}
	seen := map[string]bool{}
	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		label, _ := n.Properties["ner_label"].(string)
		if label == "" {
			label = string(n.Type)
		}
		key := label + "|" + strings.ToLower(name)
		if !seen[key] {
			seen[key] = true
			m[label] = append(m[label], name)
		}
	}
	return m
}

func ramCheckpoint(label string) {
	var ms goruntime.MemStats
	goruntime.ReadMemStats(&ms)
	fmt.Printf("  📊 RAM [%s]: alloc=%.1f MiB  sys=%.1f MiB\n",
		label, float64(ms.Alloc)/1024/1024, float64(ms.Sys)/1024/1024)
}

func memUsage() (allocMiB, sysMiB float64) {
	var ms goruntime.MemStats
	goruntime.ReadMemStats(&ms)
	return float64(ms.Alloc) / 1024 / 1024,
		float64(ms.Sys) / 1024 / 1024
}

func mustExist(path, hint string) {
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("FATAL: không tìm thấy\n  path: %s\n  fix : %s", path, hint)
	}
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL [%s]: %v", label, err)
	}
}
