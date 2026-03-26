// Example: Testing Knowledge Graph Retrieval with noisy and confusing data
// Run: go run ./examples/noisy_data_query/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/extractor/registry"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	_ "github.com/NortonBen/ai-memory-go/storage/adapters/sqlite"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	// Let's use correct imports
	run()
}

func run() {
	ctx := context.Background()
	// os.RemoveAll("./data/noisy_data")
	_ = os.MkdirAll("./data/noisy_data", 0o750)

	// ─── 1. LM Studio Initialization ──────────────────────────────────────────
	embFactory := registry.NewEmbeddingProviderFactory()
	lmstudioEmb, err := embFactory.CreateProvider(&extractor.EmbeddingProviderConfig{
		Type:     extractor.EmbeddingProviderLMStudio,
		Endpoint: "http://localhost:1234/v1",
		Model:    "text-embedding-nomic-embed-text-v1.5",
	})
	must(err, "lmstudio embedding provider")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)

	// ─── 2. SQLite Stores (Graph, Vector, Relational) ─────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/noisy_data/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/noisy_data/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/noisy_data/relational_data.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "relational store")
	defer relStore.Close()

	// ─── 3. LLM Extractor ─────────────────────────────────────────────────────
	// Assuming LM Studio is running on localhost:1234 handling completion tasks
	llmFactory := registry.NewProviderFactory()
	lmProvider, err := llmFactory.CreateProvider(&extractor.ProviderConfig{
		Type:     extractor.ProviderLMStudio,
		Endpoint: "http://localhost:1234/v1",
		Model:    "qwen/qwen3-4b-2507",
	})
	must(err, "lmstudio provider")

	basicExt := extractor.NewBasicExtractor(lmProvider, &extractor.ExtractionConfig{
		UseJSONSchema: true, // Forces exact JSON schema return for LM Studio
		Domain:        "general",
	})

	// ─── 4. The Memory Engine ─────────────────────────────────────────────────
	eng := engine.NewMemoryEngineWithStores(
		basicExt,
		embedder,
		relStore,
		graphStore,
		vecStore,
		engine.EngineConfig{
			MaxWorkers: 2,
		},
	)

	// ─── THE CORPUS: Heavy details & noise ────────────────────────────────────
	corpus := []string{
		"TechViet là một công ty phần mềm lớn ở Việt Nam, được thành lập từ năm 2010.",
		"Công ty XYZ chuyên sản xuất giày dép, Giám đốc điều hành là anh Tuấn.",
		"Johnathan Doe (hay thường gọi là John) là Giám đốc điều hành (CEO) của công ty TechViet.",
		"Một người khác cũng tên John Smith làm thợ sửa điện nước tại Mỹ, anh ta không liên quan đến ngành CNTT.",
		"Johnathan Doe có một cậu con trai nhỏ tên là Peter Doe, học lớp 3.",
		"Cửa hàng bán hoa Hồng Hạnh ở Góc Phố 5 do cô Liễu làm chủ.",
		"Anh Thành Lê là bạn thân nhất của Johnathan Doe. Anh Thành và John thường đi câu cá vào mỗi cuối tuần.",
		"Thành Nguyễn là nhân viên lao công ở toà nhà Landmark 81.",
		"TechViet có một dự án nội bộ tên là 'Phượng Hoàng Lửa' do nhóm của chị Mai dẫn dắt.",
		"Mary là trưởng phòng Marketing của tập đoàn đồ chơi ToysRUs.",
		"Anh Thành Lê có nuôi một con kỳ đà tên là Godzilla.",
	}

	fmt.Println("=== ĐANG NẠP DỮ LIỆU NHIỄU (NOISY CORPUS) ===")
	for _, text := range corpus {
		// Use WithWaitAdd(true) to process synchronously. Ensure relationships are built sequentially.
		dp, err := eng.Add(ctx, text, engine.WithWaitAdd(true))
		if err != nil {
			log.Printf("Failed to process point: %v", err)
			continue
		}
		if _, err := eng.Cognify(ctx, dp, engine.WithWaitCognify(true)); err != nil {
			log.Printf("Warning: cognify failed for %s: %v", dp.ID, err)
		} else {
			fmt.Printf("✅ Đã nhận thức & ghi nhớ: %s...\n", text[:20])
		}
	}
	fmt.Println()

	// ─── 5. QUESTION / THỰC THI SUY LUẬN ──────────────────────────────────────
	// We want the AI to distinguish the real John (Johnathan Doe CEO TechViet) from John Smith (plumber).
	// We also want to find his best friend (Thành Lê), not Thành Nguyễn.
	// We finally ask for the pet of that friend.

	questions := []string{
		"Bạn thân nhất của Giám đốc điều hành TechViet là ai?",
		"Thú cưng của bạn thân giám đốc TechViet tên là gì?",
	}

	for _, q := range questions {
		fmt.Printf("=== HỎI (%s) ===\n", q)

		query := &schema.ThinkQuery{
			Text:               q,
			SessionID:          "noisy-session-001",
			Limit:              3,
			HopDepth:           2,
			EnableThinking:     true,
			MaxThinkingSteps:   3,
			LearnRelationships: true,
			IncludeReasoning:   true,
		}

		// The "Think" method will first vector-search concepts (like "Giám đốc điều hành TechViet", "bạn thân")
		// Then it jumps around the graph (TechViet -> Johnathan Doe -> Thành Lê)
		result, err := eng.Think(ctx, query)
		if err != nil {
			log.Fatalf("Think failed for string %q: %v", q, err)
		}

		fmt.Printf("🤔 AI Lập luận (Reasoning):\n%s\n\n", result.Reasoning)
		fmt.Printf("💡 AI Trả lời (Answer):\n%s\n", result.Answer)

		fmt.Println("\n-- Bộ nhớ tham chiếu (Contexts Used):")
		if result.ContextUsed != nil {
			for i, ctxRes := range result.ContextUsed.Results {
				fmt.Printf(" [%d] Độ tin cậy: %.2f | Nguồn: %s\n", i+1, ctxRes.Score, ctxRes.DataPoint.Content)
			}
		} else {
			fmt.Println(" Không có context rõ ràng.")
		}
		fmt.Println()
	}

	fmt.Println("✅ Hoàn thành bài test lấy dữ liệu từ tập dữ liệu nhiễu.")
}

func must(err error, desc string) {
	if err != nil {
		log.Fatalf("Fatal error in %s: %v", desc, err)
	}
}
