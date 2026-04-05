// Ví dụ: Offline "Think" — Suy luận đa bước bằng Search + Knowledge Graph
//
// Mô phỏng pipeline Think hoàn toàn offline (không cần LLM server):
//
//  Bước 1 — Phân tích truy vấn (AnalyzeQuery):
//             DeBERTa AnalyzeQuery → trích từ khóa + gợi ý thực thể
//
//  Bước 2 — Truy xuất ngữ cảnh (HybridSearch):
//             Vector search (Harrier) + Graph traversal (co-occurrence)
//             → xếp hạng theo VectorScore + GraphScore
//
//  Bước 3 — Mở rộng knowledge graph (Graph expansion):
//             Tìm thực thể trong truy vấn → FindNodesByProperty
//             → TraverseGraph depth-2 → mở rộng vùng lân cận
//
//  Bước 4 — Tổng hợp ngữ cảnh (Context synthesis):
//             Gộp tất cả passages + graph neighbors thành "context"
//             (context này sẽ được gửi cho LLM nếu có)
//
//  Bước 5 — Báo cáo: RAM, thời gian, điểm tương đồng mỗi truy vấn
//
// Không cần LMStudio, Ollama hay API key.
//
// Chạy:
//
//	go run ./examples/think_query/
//	ORT_PROVIDER=coreml go run ./examples/think_query/   # Apple Silicon GPU
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

// ─── Kho tri thức tiếng Việt ──────────────────────────────────────────────────
var knowledgeBase = []struct{ text, topic string }{
	{
		"Harrier-OSS-v1-270m là mô hình embedding văn bản đa ngôn ngữ do Microsoft Research tạo ra. Mô hình này sinh vector 640 chiều và hỗ trợ hơn 100 ngôn ngữ bao gồm tiếng Việt, được dùng trong semantic search.",
		"model-embedding",
	},
	{
		"Microsoft công bố mã nguồn mở Harrier theo giấy phép MIT tại Microsoft Build 2025. Satya Nadella, CEO của Microsoft, dẫn đầu thông báo tại Seattle. Mô hình dựa trên kiến trúc Gemma3 của Google DeepMind.",
		"thông-báo",
	},
	{
		"DeBERTa-v3 là mô hình transformer do Microsoft Research phát triển tại Redmond. Mô hình sử dụng disentangled attention và enhanced mask decoder, đạt SOTA trên nhiều benchmark NLP bao gồm CoNLL-2003 NER.",
		"model-ner",
	},
	{
		"ONNX Runtime là engine suy luận mã nguồn mở từ Microsoft. Nó hỗ trợ CoreML trên Apple Silicon, CUDA trên GPU NVIDIA, và chạy hoàn toàn cục bộ không cần internet. Được dùng để chạy DeBERTa và Harrier offline.",
		"hạ-tầng",
	},
	{
		"HuggingFace là công ty AI tại New York duy trì thư viện Transformers. Bộ công cụ Optimum của HuggingFace được dùng để xuất mô hình sang định dạng ONNX, tương thích với ONNX Runtime.",
		"công-ty",
	},
	{
		"Google DeepMind tại London phát triển kiến trúc Gemma3 — transformer decoder-only mã nguồn mở. Harrier của Microsoft được xây dựng dựa trên nền Gemma3 và tinh chỉnh cho embedding đa ngôn ngữ.",
		"nghiên-cứu",
	},
	{
		"RAG (Retrieval-Augmented Generation) là kỹ thuật kết hợp truy xuất dense vector với sinh ngôn ngữ tự nhiên. Hệ thống RAG lấy thông tin từ knowledge base để bổ sung vào prompt cho LLM, giảm hallucination.",
		"kỹ-thuật-ai",
	},
	{
		"VinAI Research tại Hà Nội là đơn vị nghiên cứu AI hàng đầu Việt Nam. Tiến sĩ Hùng Bùi dẫn dắt nhóm đã huấn luyện các mô hình ngôn ngữ lớn trên 100 tỷ token tiếng Việt.",
		"công-ty-vn",
	},
	{
		"FPT Software tại TP.HCM là công ty phần mềm lớn nhất Việt Nam, triển khai giải pháp AI cho doanh nghiệp. FPT hợp tác với Microsoft để tích hợp Azure AI vào các sản phẩm của mình.",
		"công-ty-vn",
	},
	{
		"Dự án ai-memory-brain kết hợp Harrier embedding với DeBERTa NER và SQLite graph store, tạo ra pipeline bộ nhớ 100% offline. Không cần LMStudio, Ollama hay API key. Hỗ trợ GPU qua CoreML và CUDA.",
		"dự-án",
	},
}

// ─── Các truy vấn kiểm tra ────────────────────────────────────────────────────
var testQueries = []struct {
	text    string
	comment string // giải thích kỳ vọng
}{
	{
		"Harrier được tạo ra bởi ai và dựa trên kiến trúc nào?",
		"Kỳ vọng: Microsoft Research + Gemma3 — cần kết hợp 2 nguồn",
	},
	{
		"Làm thế nào để chạy mô hình AI offline không cần internet?",
		"Kỳ vọng: ONNX Runtime + CoreML/CUDA + ai-memory-brain project",
	},
	{
		"Công ty AI nào ở Việt Nam nghiên cứu về ngôn ngữ tiếng Việt?",
		"Kỳ vọng: VinAI, FPT — địa điểm Hà Nội, TP.HCM",
	},
	{
		"RAG là gì và nó giải quyết vấn đề gì trong LLM?",
		"Kỳ vọng: Retrieval-Augmented Generation, giảm hallucination",
	},
}

// ─── Kết quả think một truy vấn ───────────────────────────────────────────────
type offlineThinkResult struct {
	Query          string
	Comment        string
	Keywords       []string
	Passages       []retrievedPassage
	GraphNeighbors []string
	ContextPreview string
	TopScore       float64
	ElapsedMs      int64
	RAMAllocMiB    float64
}

type retrievedPassage struct {
	Text        string
	VectorScore float64
	GraphScore  float64
}

func main() {
	ctx := context.Background()

	// ─── Đường dẫn ────────────────────────────────────────────────────────────
	_, filename, _, _ := goruntime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")

	// Ưu tiên Harrier INT8 quantized (~280 MB) thay vì FP32 (~1 GB).
	harrierDir := filepath.Join(root, "data", "harrier-q")
	if _, err := os.Stat(filepath.Join(harrierDir, "model.onnx")); err != nil {
		harrierDir = filepath.Join(root, "data", "harrier")
	}
	harrierModel := filepath.Join(harrierDir, "model.onnx")
	harrierTok := filepath.Join(harrierDir, "tokenizer.json")
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

	dataDir := filepath.Join(root, "data", "think_query_demo")
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
	ramCheckpoint("Khởi động")

	// ─── 1. Tải model ─────────────────────────────────────────────────────────
	section("[1] Tải mô hình ONNX")
	t0 := time.Now()
	harrierEmb, err := onnx.NewHarrierEmbedder(harrierModel, harrierTok, 512,
		"Retrieve semantically similar text", true)
	must(err, "harrier embedder")
	debExt, err := deberta.NewExtractor(deberta.Config{
		ModelPath:     debertaModel,
		TokenizerPath: debertaTok,
		LabelsPath:    debertaLabels,
		// MaxSeqLen 256: tiết kiệm RAM, đủ dài cho NER (câu thường < 100 token).
		// CPU provider mặc định: ~200 MB vs ~4-6 GB với CoreML cho DeBERTa-large.
		MaxSeqLen: 256,
	})
	must(err, "deberta extractor")
	fmt.Printf("    Harrier  [%s] + DeBERTa [%s] — tải trong %s\n",
		harrierEmb.GetExecutionProvider(), debExt.GetExecutionProvider(),
		time.Since(t0).Round(time.Millisecond))
	fmt.Println("    RAM tip  : ORT_NER_PROVIDER=cpu (mặc định) tiết kiệm ~4-6 GB vs CoreML cho DeBERTa")
	ramCheckpoint("Sau tải model")

	// ─── 2. Thiết lập stores & engine ─────────────────────────────────────────
	section("[2] Thiết lập stores & memory engine")
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
	fmt.Println("    Stores ✓   Engine ✓")

	// ─── 3. Nạp kho tri thức ──────────────────────────────────────────────────
	section("[3] Nạp kho tri thức tiếng Việt")
	sessionID := "think-query-vi"
	dpText := make(map[string]string)
	for _, item := range knowledgeBase {
		dp, err := eng.Add(ctx, item.text,
			engine.WithSessionID(sessionID),
			engine.WithMetadata(map[string]interface{}{
				"topic":  item.topic,
				"source": "think-query-demo",
				"text":   item.text,
			}),
		)
		must(err, "add")
		dpText[dp.ID] = item.text
		fmt.Printf("    + (%s) %s\n", item.topic, truncate(item.text, 65))
	}

	t0 = time.Now()
	if err := eng.CognifyPending(ctx, sessionID); err != nil {
		log.Printf("  cảnh báo: %v", err)
	}
	cogDur := time.Since(t0)
	goruntime.GC() // giải phóng bộ nhớ tạm sau embed+NER

	// Chờ graph
	deadline := time.Now().Add(60 * time.Second)
	var nodeCount int64
	for time.Now().Before(deadline) {
		nodeCount, _ = graphStore.GetNodeCount(ctx)
		if nodeCount > 0 {
			break
		}
		fmt.Print(".")
		time.Sleep(500 * time.Millisecond)
	}
	edgeCount, _ := graphStore.GetEdgeCount(ctx)
	vecCnt, _ := vecStore.GetEmbeddingCount(ctx)
	fmt.Printf("\n    Cognify: %s · %d vector · %d node · %d cạnh\n",
		cogDur.Round(time.Millisecond), vecCnt, nodeCount, edgeCount)
	ramCheckpoint("Sau nạp tri thức")

	// ─── 4. OFFLINE THINK — vòng lặp suy luận ────────────────────────────────
	section("[4] OFFLINE THINK — suy luận đa bước cho từng truy vấn")
	fmt.Println()

	var results []offlineThinkResult

	for qi, tq := range testQueries {
		fmt.Printf("╔═══ Truy vấn %d/%d ═══════════════════════════════════════════════\n",
			qi+1, len(testQueries))
		fmt.Printf("║  Q: %q\n", tq.text)
		fmt.Printf("║  💡 %s\n", tq.comment)
		fmt.Println("╚══════════════════════════════════════════════════════════════════")

		tStart := time.Now()

		// Bước 1: Phân tích truy vấn (AnalyzeQuery)
		fmt.Println("\n  Bước 1 — Phân tích truy vấn (DeBERTa AnalyzeQuery)")
		analysis, err := debExt.AnalyzeQuery(ctx, tq.text)
		if err != nil {
			log.Printf("  AnalyzeQuery lỗi: %v", err)
			analysis = &schema.ThinkQueryAnalysis{}
		}
		if len(analysis.SearchKeywords) > 0 {
			fmt.Printf("  Từ khóa: %s\n", strings.Join(analysis.SearchKeywords, " | "))
		}
		fmt.Printf("  Loại    : %s\n", analysis.QueryType)

		// Bước 2: HybridSearch — vector + graph
		fmt.Println("\n  Bước 2 — HybridSearch (Harrier vector + DeBERTa graph)")
		searchResults, err := eng.Search(ctx, &schema.SearchQuery{
			Text:                tq.text,
			SessionID:           sessionID,
			Mode:                schema.ModeHybridSearch,
			Limit:               5,
			HopDepth:            2,
			SimilarityThreshold: 0.25,
			Analysis:            analysis,
		})

		var passages []retrievedPassage
		var topVec float64
		if err != nil {
			fmt.Printf("  Lỗi search: %v\n", err)
		} else {
			fmt.Printf("  Tìm thấy %d kết quả (trong %s)\n",
				searchResults.Total, searchResults.QueryTime.Round(time.Millisecond))
			for i, r := range searchResults.Results {
				if i >= 4 || r.DataPoint == nil {
					break
				}
				content := r.DataPoint.Content
				passages = append(passages, retrievedPassage{
					Text:        content,
					VectorScore: r.VectorScore,
					GraphScore:  r.GraphScore,
				})
				if r.Score > topVec {
					topVec = r.Score
				}

				indicator := scoreIcon(r.Score)
				fmt.Printf("  %s [score=%.3f] %s\n",
					indicator, r.Score, truncate(content, 70))
			}
		}

		// Bước 3: Mở rộng knowledge graph từ thực thể trong truy vấn
		fmt.Println("\n  Bước 3 — Mở rộng knowledge graph")
		queryNodes, _ := debExt.ExtractEntities(ctx, tq.text)
		var graphNeighbors []string
		neighborSet := map[string]bool{}

		for _, qn := range queryNodes {
			name, _ := qn.Properties["name"].(string)
			label, _ := qn.Properties["ner_label"].(string)
			if name == "" {
				continue
			}
			fmt.Printf("  Thực thể trong Q: [%s] %q → tìm trong graph...\n", label, name)
			foundNodes, err := graphStore.FindNodesByProperty(ctx, "name", name)
			if err != nil || len(foundNodes) == 0 {
				fmt.Printf("    (không tìm thấy trong graph)\n")
				continue
			}
			for _, fn := range foundNodes {
				neighbors, _ := graphStore.TraverseGraph(ctx, fn.ID, 2, nil)
				for _, nb := range neighbors {
					nbName, _ := nb.Properties["name"].(string)
					if nbName != "" && !neighborSet[nbName] {
						neighborSet[nbName] = true
						graphNeighbors = append(graphNeighbors, nbName)
					}
				}
			}
		}
		if len(graphNeighbors) > 0 {
			fmt.Printf("  Graph neighbors: %s\n", strings.Join(graphNeighbors, " · "))
		} else if len(queryNodes) == 0 {
			fmt.Println("  (DeBERTa không tìm thấy thực thể trong truy vấn — thử với entity rõ ràng hơn)")
		}

		// Bước 4: Tổng hợp ngữ cảnh
		fmt.Println("\n  Bước 4 — Tổng hợp ngữ cảnh (context sẽ gửi cho LLM nếu có)")
		var ctxParts []string
		for i, p := range passages {
			ctxParts = append(ctxParts, fmt.Sprintf("[%d] %s", i+1, p.Text))
		}
		if len(graphNeighbors) > 0 {
			ctxParts = append(ctxParts, "[graph] "+strings.Join(graphNeighbors, ", "))
		}
		fullCtx := strings.Join(ctxParts, "\n")
		ctxPreview := truncate(fullCtx, 200)
		fmt.Printf("  Context (%d ký tự): %s\n", len(fullCtx), ctxPreview)

		if len(passages) == 0 {
			fmt.Println("\n  ⚠ Không tìm thấy passage — thử giảm SimilarityThreshold")
		} else {
			fmt.Println("\n  ▶ Nếu có LLM → gửi context trên + câu hỏi → nhận câu trả lời")
			fmt.Println("    Offline: user tự đọc context và suy luận từ evidence trên ✓")
		}

		elapsed := time.Since(tStart)
		allocMiB, _ := memUsage()
		fmt.Printf("\n  ⏱ Thời gian: %s  |  📊 RAM: %.1f MiB\n", elapsed.Round(time.Millisecond), allocMiB)
		fmt.Println()

		results = append(results, offlineThinkResult{
			Query:          tq.text,
			Comment:        tq.comment,
			Keywords:       analysis.SearchKeywords,
			Passages:       passages,
			GraphNeighbors: graphNeighbors,
			ContextPreview: ctxPreview,
			TopScore:       topVec,
			ElapsedMs:      elapsed.Milliseconds(),
			RAMAllocMiB:    allocMiB,
		})
	}

	// ─── 5. Tóm tắt toàn bộ ──────────────────────────────────────────────────
	section("[5] Tóm tắt — hiệu suất từng truy vấn")
	finalAlloc, finalSys := memUsage()

	fmt.Printf("\n  %-4s %-47s %8s %9s %8s\n",
		"STT", "Truy vấn", "Elapsed", "Score↑", "RAM(MiB)")
	fmt.Println("  " + strings.Repeat("─", 80))
	for i, r := range results {
		fmt.Printf("  %-4d %-47s %8s %9.3f %8.1f\n",
			i+1, truncate(r.Query, 46),
			time.Duration(r.ElapsedMs)*time.Millisecond,
			r.TopScore, r.RAMAllocMiB)
	}

	fmt.Printf(`
  ──────────────────────────────────────────────────
  Knowledge base : %d văn bản · %d vector
  Knowledge graph: %d node · %d cạnh
  ──────────────────────────────────────────────────
  RAM (Go heap)  : %.1f MiB đang dùng / %.1f MiB OS cấp
  Provider       : embed=%s  ner=%s
  ──────────────────────────────────────────────────
  Ghi chú: Think() đầy đủ cần LLM provider (LMStudio/Ollama).
            Ví dụ này minh hoạ pipeline RETRIEVAL hoàn toàn offline.
            Context được tổng hợp sẵn — chỉ cần thêm LLM để sinh câu trả lời.
  Data dir       : %s

`,
		len(knowledgeBase), vecCnt,
		nodeCount, edgeCount,
		finalAlloc, finalSys,
		harrierEmb.GetExecutionProvider(),
		debExt.GetExecutionProvider(),
		dataDir,
	)
	fmt.Println("✅  Offline Think demo hoàn tất.")
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func banner() {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Offline Think — Suy luận đa bước 100% Local ONNX Inference     ║")
	fmt.Println("║  AnalyzeQuery (DeBERTa) + HybridSearch (Harrier) + GraphExpand  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
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

func scoreIcon(score float64) string {
	switch {
	case score > 0.65:
		return "🟢"
	case score > 0.45:
		return "🟡"
	default:
		return "⬛"
	}
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
		log.Fatalf("FATAL: file không tìm thấy\n  path: %s\n  fix : %s", path, hint)
	}
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL [%s]: %v", label, err)
	}
}
