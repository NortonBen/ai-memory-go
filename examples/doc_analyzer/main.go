// Ví dụ: Phân tích tài liệu tiếng Việt → Thêm vào bộ nhớ → Knowledge Graph
//
// Ví dụ này minh họa cách xử lý một bài báo dài tiếng Việt:
//  1. Trích xuất trực tiếp thực thể (NER) bằng DeBERTa — trước khi lưu vào bộ nhớ
//  2. Add tài liệu vào engine → CognifyPending → chờ graph hoàn chỉnh
//  3. Hiển thị knowledge graph: thực thể, cạnh và vùng lân cận
//  4. Semantic search để kiểm tra những gì hệ thống đã học
//  5. Báo cáo sử dụng RAM tại mỗi bước chính
//
// Không cần LMStudio, Ollama hay API key. Tất cả chạy local 100% offline.
//
// Chạy:
//
//	go run ./examples/doc_analyzer/
//	ORT_PROVIDER=coreml go run ./examples/doc_analyzer/   # Apple Silicon GPU
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	goruntime "runtime"
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

// ─── Tài liệu mẫu (bài báo tiếng Việt về AI) ─────────────────────────────────
//
// Chứa nhiều thực thể: người (PER), tổ chức (ORG), địa điểm (LOC), khái niệm (MISC)
// để DeBERTa-v3 có thể nhận diện và đưa vào knowledge graph.
var document = struct {
	title    string
	sections []struct{ heading, text string }
}{
	title: "Trí tuệ nhân tạo đang thay đổi Việt Nam và thế giới",
	sections: []struct{ heading, text string }{
		{
			"Làn sóng đầu tư AI toàn cầu",
			"Các tập đoàn công nghệ lớn như Microsoft, Google và NVIDIA đang đầu tư hàng chục tỷ USD vào nghiên cứu và hạ tầng AI. Satya Nadella, CEO của Microsoft, tuyên bố công ty sẽ rót 80 tỷ USD vào trung tâm dữ liệu AI trong năm 2025. Sam Altman từ OpenAI đang đàm phán khoản đầu tư 500 tỷ USD cho dự án Stargate tại Mỹ. Jensen Huang, CEO của NVIDIA, khẳng định chip GPU Blackwell thế hệ mới sẽ tăng tốc quá trình huấn luyện lên 30 lần.",
		},
		{
			"Việt Nam gia nhập cuộc đua AI",
			"Tại Hà Nội, VinAI Research đã công bố các mô hình ngôn ngữ lớn dành riêng cho tiếng Việt. Tiến sĩ Hùng Bùi, Giám đốc nghiên cứu tại VinAI, cho biết nhóm đã huấn luyện trên hơn 100 tỷ token tiếng Việt. FPT Software tại TP.HCM triển khai giải pháp AI cho nhiều doanh nghiệp lớn trong khu vực. Bộ Khoa học và Công nghệ Việt Nam đang xây dựng Chiến lược Quốc gia về AI đến năm 2030, với mục tiêu đưa Việt Nam trở thành trung tâm AI của Đông Nam Á.",
		},
		{
			"Mô hình và công cụ mã nguồn mở",
			"Microsoft Research tại Redmond đã phát hành Harrier-OSS-v1-270m, mô hình embedding đa ngôn ngữ 640 chiều hỗ trợ tiếng Việt với hiệu suất vượt trội. Google DeepMind tại London công bố Gemma3, mô hình transformer decoder-only mã nguồn mở làm nền tảng cho Harrier. HuggingFace tại New York duy trì thư viện Transformers và bộ công cụ Optimum để xuất mô hình sang ONNX Runtime.",
		},
		{
			"Hạ tầng suy luận cục bộ",
			"ONNX Runtime từ Microsoft cho phép chạy mô hình AI trực tiếp trên thiết bị — máy Mac Apple Silicon, GPU NVIDIA, hay CPU phổ thông — mà không cần kết nối mạng. DeBERTa-v3, cũng từ Microsoft Research, là mô hình NER mạnh mẽ có thể nhận diện người, tổ chức và địa điểm trong văn bản. RAG (Retrieval-Augmented Generation) kết hợp truy xuất vector với sinh ngôn ngữ để tạo ra hệ thống hỏi đáp chính xác hơn.",
		},
	},
}

func main() {
	ctx := context.Background()

	// ─── Đường dẫn ────────────────────────────────────────────────────────────
	_, filename, _, _ := goruntime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")

	harrierModel := filepath.Join(root, "data", "harrier", "model.onnx")
	harrierTok := filepath.Join(root, "data", "harrier", "tokenizer.json")
	// Ưu tiên model nhỏ nhất: base > large-INT8 > large-FP32
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

	dataDir := filepath.Join(root, "data", "doc_analyzer_demo")
	_ = os.RemoveAll(dataDir)
	_ = os.MkdirAll(dataDir, 0o750)

	for _, p := range []struct{ path, hint string }{
		{harrierModel, "python scripts/export_harrier_onnx.py"},
		{harrierTok, "python scripts/export_harrier_onnx.py"},
		{debertaModel, "python scripts/export_deberta_onnx.py"},
		{debertaTok, "python scripts/export_deberta_onnx.py"},
		{debertaLabels, "python scripts/export_deberta_onnx.py"},
	} {
		mustExist(p.path, p.hint)
	}

	banner()

	// ─── Checkpoint 0: RAM trước khi tải model ────────────────────────────────
	ramCheckpoint("Khởi động (trước khi tải model)")

	// ─── 1. Tải model ─────────────────────────────────────────────────────────
	section("[1] Tải mô hình ONNX (offline)")
	t0 := time.Now()

	harrierEmb, err := onnx.NewHarrierEmbedder(harrierModel, harrierTok, 512,
		"Retrieve semantically similar text", true)
	must(err, "harrier embedder")

	debExt, err := deberta.NewExtractor(deberta.Config{
		ModelPath:     debertaModel,
		TokenizerPath: debertaTok,
		LabelsPath:    debertaLabels,
		// MaxSeqLen 256: DeBERTa-v3-large với CoreML tốn ~4-6 GB GPU buffer.
		// Provider mặc định là CPU (~200 MB) — đủ nhanh cho NER.
		MaxSeqLen: 256,
	})
	must(err, "deberta extractor")

	loadTime := time.Since(t0)
	fmt.Printf("    Harrier   : %s  [%s]\n", harrierEmb.GetModel(), harrierEmb.GetExecutionProvider())
	fmt.Printf("    DeBERTa   : microsoft-deberta-v3-large_ner_conll2003  [%s]\n", debExt.GetExecutionProvider())
	fmt.Printf("    Thời gian : %s\n", loadTime.Round(time.Millisecond))
	fmt.Println("    Gợi ý RAM : ORT_NER_PROVIDER=cpu (mặc định) tiết kiệm ~4-6 GB so với CoreML")
	ramCheckpoint("Sau khi tải model")

	// ─── 2. Phân tích NER trực tiếp (trước khi lưu vào bộ nhớ) ──────────────
	section("[2] Phân tích NER trực tiếp — DeBERTa-v3")
	fmt.Printf("\n  Tài liệu: %q\n", document.title)
	fmt.Println()

	type sectionEntities struct {
		heading  string
		entities []schema.Node
	}
	var allSectionEntities []sectionEntities

	for _, sec := range document.sections {
		nodes, err := debExt.ExtractEntities(ctx, sec.text)
		if err != nil {
			log.Printf("  NER lỗi ở %q: %v", sec.heading, err)
			continue
		}
		allSectionEntities = append(allSectionEntities, sectionEntities{sec.heading, nodes})

		fmt.Printf("  ▶ %s\n", sec.heading)
		fmt.Printf("    Văn bản : %s\n", truncate(sec.text, 90))
		if len(nodes) == 0 {
			fmt.Println("    Thực thể: (không tìm thấy)")
		} else {
			grouped := groupByLabel(nodes)
			for label, names := range grouped {
				fmt.Printf("    [%s] %s\n", label, strings.Join(names, " · "))
			}
		}
		fmt.Println()
	}
	ramCheckpoint("Sau NER trực tiếp")

	// ─── 3. Thiết lập stores & engine ─────────────────────────────────────────
	section("[3] Thiết lập stores & memory engine")
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

	eng := engine.NewMemoryEngineWithStores(
		debExt, autoEmb, relStore, graphStore, vecStore,
		engine.EngineConfig{MaxWorkers: 4, ChunkConcurrency: 2},
	)
	defer eng.Close()
	fmt.Println("    graph.db ✓   vectors.db ✓   rel.db ✓")

	// ─── 4. Add tài liệu vào bộ nhớ ──────────────────────────────────────────
	section("[4] Thêm tài liệu vào bộ nhớ (Add → DataPoints)")
	sessionID := "doc-analyzer-vi"
	dpText := make(map[string]string)
	var allDPs []*schema.DataPoint

	// Thêm từng đoạn như một DataPoint độc lập
	for _, sec := range document.sections {
		dp, err := eng.Add(ctx, sec.text,
			engine.WithSessionID(sessionID),
			engine.WithMetadata(map[string]interface{}{
				"title":   document.title,
				"heading": sec.heading,
				"source":  "doc-analyzer-demo",
				"text":    sec.text,
			}),
		)
		must(err, "add section")
		allDPs = append(allDPs, dp)
		dpText[dp.ID] = sec.text
		fmt.Printf("    + [%s] %s (%d ký tự)\n", dp.ID[:8], truncate(sec.heading, 35), len([]rune(sec.text)))
	}
	fmt.Printf("    Tổng: %d đoạn văn đã thêm\n", len(allDPs))

	// ─── 5. CognifyPending → embed + NER → graph ──────────────────────────────
	section("[5] CognifyPending — embed (Harrier) + NER (DeBERTa)")
	fmt.Println("    Đang xử lý …")

	t0 = time.Now()
	if err := eng.CognifyPending(ctx, sessionID); err != nil {
		log.Printf("  cảnh báo: %v", err)
	}
	cogDur := time.Since(t0)
	goruntime.GC() // giải phóng bộ nhớ tạm sau embed+NER
	fmt.Printf("    Xong trong %s  (%.2f đoạn/giây)\n",
		cogDur.Round(time.Millisecond), float64(len(document.sections))/cogDur.Seconds())
	ramCheckpoint("Sau CognifyPending")

	// ─── 6. Chờ graph ─────────────────────────────────────────────────────────
	section("[6] Chờ Memify → knowledge graph")
	deadline := time.Now().Add(60 * time.Second)
	var nodeCount, edgeCount int64
	for time.Now().Before(deadline) {
		nodeCount, _ = graphStore.GetNodeCount(ctx)
		if nodeCount > 0 {
			edgeCount, _ = graphStore.GetEdgeCount(ctx)
			fmt.Printf("    ✓ %d node, %d cạnh\n", nodeCount, edgeCount)
			break
		}
		fmt.Print(".")
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println()

	// ─── 7. Hiển thị knowledge graph ─────────────────────────────────────────
	section("[7] Knowledge graph — thực thể đã học")
	entityCounts := map[string]int{}
	for _, nodeType := range []schema.NodeType{schema.NodeTypeEntity, schema.NodeTypeConcept} {
		ns, err := graphStore.FindNodesByType(ctx, nodeType)
		if err != nil || len(ns) == 0 {
			continue
		}
		fmt.Printf("\n  ● %s (%d)\n", nodeType, len(ns))
		for _, n := range ns {
			name, _ := n.Properties["name"].(string)
			label, _ := n.Properties["ner_label"].(string)
			entityCounts[label]++
			if label != "" {
				fmt.Printf("    [%s] %s\n", label, name)
			} else {
				fmt.Printf("    %s\n", name)
			}
		}
	}

	// Thống kê phân bố nhãn
	if len(entityCounts) > 0 {
		fmt.Println("\n  Phân bố nhãn NER:")
		for label, cnt := range entityCounts {
			bar := strings.Repeat("█", cnt)
			fmt.Printf("    %-5s : %s (%d)\n", label, bar, cnt)
		}
	}

	// ─── 8. Khám phá graph ────────────────────────────────────────────────────
	section("[8] Khám phá knowledge graph — vùng lân cận")
	hubs := []string{"Microsoft", "VinAI", "NVIDIA", "Harrier"}
	for _, hubName := range hubs {
		nodes, err := graphStore.FindNodesByProperty(ctx, "name", hubName)
		if err != nil || len(nodes) == 0 {
			continue
		}
		hub := nodes[0]
		connected, _ := graphStore.FindConnected(ctx, hub.ID, nil)
		if len(connected) == 0 {
			continue
		}
		fmt.Printf("\n  Hub %q → %d kết nối:\n", hubName, len(connected))
		for _, c := range connected {
			cName, _ := c.Properties["name"].(string)
			cLabel, _ := c.Properties["ner_label"].(string)
			fmt.Printf("    ↔ [%s] %s\n", cLabel, cName)
		}
	}

	// ─── 9. Tìm kiếm để kiểm tra ─────────────────────────────────────────────
	section("[9] Kiểm tra tìm kiếm — hệ thống đã học được gì?")
	testQueries := []string{
		"CEO của Microsoft là ai?",
		"Việt Nam có chiến lược AI không?",
		"Mô hình embedding đa ngôn ngữ từ Microsoft",
		"chạy AI offline không cần internet",
	}
	for _, q := range testQueries {
		qEmb, err := harrierEmb.GenerateQueryEmbedding(ctx, q)
		if err != nil {
			continue
		}
		hits, err := vecStore.SimilaritySearch(ctx, qEmb, 2, 0.3)
		if err != nil || len(hits) == 0 {
			fmt.Printf("\n  Q: %q → (không tìm thấy)\n", q)
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
			fmt.Printf("    [score=%.3f] %s\n", h.Score, truncate(text, 85))
		}
	}

	// ─── 10. Tóm tắt & RAM report ─────────────────────────────────────────────
	section("[10] Tóm tắt & Báo cáo tài nguyên")
	vecCnt, _ := vecStore.GetEmbeddingCount(ctx)
	nodeCount, _ = graphStore.GetNodeCount(ctx)
	edgeCount, _ = graphStore.GetEdgeCount(ctx)
	allocMiB, sysMiB := memUsage()

	fmt.Printf(`
  ──────────────────────────────────────────────────
  Tài liệu     : %q
  Đoạn văn     : %d đoạn
  ──────────────────────────────────────────────────
  DataPoints   : %d
  Vectors      : %d
  Graph nodes  : %d
  Graph edges  : %d
  Cognify time : %s
  ──────────────────────────────────────────────────
  RAM (Go heap): %.1f MiB đang dùng / %.1f MiB OS cấp
  Provider     : embed=%s  ner=%s
  ──────────────────────────────────────────────────
  Data dir     : %s

`,
		document.title,
		len(document.sections),
		len(allDPs),
		vecCnt, nodeCount, edgeCount,
		cogDur.Round(time.Millisecond),
		allocMiB, sysMiB,
		harrierEmb.GetExecutionProvider(),
		debExt.GetExecutionProvider(),
		dataDir,
	)
	fmt.Println("✅  Phân tích tài liệu hoàn tất.")
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func banner() {
	fmt.Println()
	fmt.Println("╔═════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Phân tích Tài liệu Tiếng Việt — 100% Offline ONNX Inference  ║")
	fmt.Println("║  DeBERTa-v3 NER  +  Harrier-OSS Embedding  +  SQLite Graph    ║")
	fmt.Println("╚═════════════════════════════════════════════════════════════════╝")
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

func stripChunkSuffix(id string) string {
	for {
		idx := strings.LastIndex(id, "-chunk-")
		if idx < 0 {
			return id
		}
		id = id[:idx]
	}
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
	allocMiB := float64(ms.Alloc) / 1024 / 1024
	sysMiB := float64(ms.Sys) / 1024 / 1024
	fmt.Printf("  📊 RAM [%s]: alloc=%.1f MiB  sys=%.1f MiB\n", label, allocMiB, sysMiB)
}

func memUsage() (allocMiB, sysMiB float64) {
	var ms goruntime.MemStats
	goruntime.ReadMemStats(&ms)
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
