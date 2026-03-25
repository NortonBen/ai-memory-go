package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

// reproProvider implements the full LLMProvider interface for testing
type reproProvider struct {
	extractor.LLMProvider
}

func (m *reproProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	if strings.Contains(prompt, "CONTEXT") || strings.Contains(prompt, "Knowledge") || strings.Contains(prompt, "Thinking") {
		return `{"answer": "Con chó nhà bạn tên là Vàng đã mất nhiều năm, còn hàng xóm có con chó Đen mới 1 tuổi.", "reasoning": "Dựa vào thông tin đã lưu trữ."}`, nil
	}
	return `{"answer": "I have memorized this information.", "reasoning": "Standard recording."}`, nil
}

func (m *reproProvider) GenerateCompletionWithOptions(ctx context.Context, prompt string, options *extractor.CompletionOptions) (string, error) {
	return m.GenerateCompletion(ctx, prompt)
}

func (m *reproProvider) GenerateStructuredOutput(ctx context.Context, prompt string, schemaStruct interface{}) (interface{}, error) {
	v := reflect.ValueOf(schemaStruct)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	setField := func(name string, value interface{}) {
		f := v.FieldByName(name)
		if f.IsValid() && f.CanSet() {
			val := reflect.ValueOf(value)
			if val.Type().AssignableTo(f.Type()) {
				f.Set(val)
			}
		}
	}

	if strings.Contains(prompt, "nhà tôi có con chó tên là Vàng") {
		setField("NeedsVectorStorage", true)
		setField("Relationships", []schema.RelationshipInfo{{From: "Ben", To: "Con chó Vàng", Type: "OWNS"}})
	} else if strings.Contains(prompt, "nhưng nó đã mất nhiều năm") {
		setField("NeedsVectorStorage", true)
		setField("Relationships", []schema.RelationshipInfo{{From: "Con chó Vàng", To: "DIED", Type: "HAS_STATUS"}})
	} else if strings.Contains(prompt, "Hàng xóm cạnh nhà tôi có một con chó tên là Đen") {
		setField("NeedsVectorStorage", true)
		setField("Relationships", []schema.RelationshipInfo{{From: "Hàng xóm", To: "Con chó Đen", Type: "OWNS"}})
	} else if strings.Contains(prompt, "nó mới 1 tuổi") {
		setField("NeedsVectorStorage", true)
		setField("Relationships", []schema.RelationshipInfo{{From: "Con chó Đen", To: "1", Type: "HAS_AGE"}})
	} else if strings.Contains(prompt, "thế nào") || strings.Contains(prompt, "tên gì") {
		setField("IsQuery", true)
	}
	return schemaStruct, nil
}

func (m *reproProvider) GenerateStructuredOutputWithOptions(ctx context.Context, prompt string, schemaStruct interface{}, options *extractor.CompletionOptions) (interface{}, error) {
	return m.GenerateStructuredOutput(ctx, prompt, schemaStruct)
}

func (m *reproProvider) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	if strings.Contains(text, "chó") || strings.Contains(text, "Vàng") {
		return []schema.Node{
			{
				ID:   "node_dog_vàng",
				Type: schema.NodeTypeEntity,
				Properties: map[string]interface{}{
					"name": "Con chó Vàng",
				},
			},
		}, nil
	}
	return nil, nil
}

func (m *reproProvider) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return nil, nil
}

func (m *reproProvider) ExtractRequestIntent(ctx context.Context, text string) (*schema.RequestIntent, error) {
	intent := &schema.RequestIntent{}
	_, err := m.GenerateStructuredOutput(ctx, text, intent)
	return intent, err
}

func (m *reproProvider) GetModel() string { return "mock-model" }
func (m *reproProvider) SetModel(model string) error { return nil }
func (m *reproProvider) GetProviderType() extractor.ProviderType { return extractor.ProviderOpenAI }
func (m *reproProvider) GetCapabilities() extractor.ProviderCapabilities {
	return extractor.ProviderCapabilities{SupportsJSONMode: true, SupportsJSONSchema: true}
}
func (m *reproProvider) GetTokenCount(text string) (int, error) { return len(text) / 4, nil }
func (m *reproProvider) GetMaxTokens() int { return 4096 }
func (m *reproProvider) Health(ctx context.Context) error { return nil }
func (m *reproProvider) GetUsage(ctx context.Context) (*extractor.UsageStats, error) { return nil, nil }
func (m *reproProvider) GetRateLimit(ctx context.Context) (*extractor.RateLimitStatus, error) { return nil, nil }
func (m *reproProvider) Configure(config *extractor.ProviderConfig) error { return nil }
func (m *reproProvider) GetConfiguration() *extractor.ProviderConfig { return nil }
func (m *reproProvider) Close() error { return nil }

type reproExtractor struct {
	extractor.LLMExtractor
	prov *reproProvider
}

func (e *reproExtractor) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	return e.prov.ExtractEntities(ctx, text)
}

func (e *reproExtractor) ExtractRequestIntent(ctx context.Context, text string) (*schema.RequestIntent, error) {
	return e.prov.ExtractRequestIntent(ctx, text)
}

func (e *reproExtractor) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return nil, nil
}

func (e *reproExtractor) ExtractBridgingRelationship(ctx context.Context, question string, answer string) (*schema.Edge, error) {
	return nil, nil
}

func (e *reproExtractor) CompareEntities(ctx context.Context, existing schema.Node, newEntity schema.Node) (*schema.ConsistencyResult, error) {
	return nil, nil
}

func (e *reproExtractor) ExtractWithSchema(ctx context.Context, text string, schemaStruct interface{}) (interface{}, error) {
	return nil, nil
}

type reproStorage struct {
	messages map[string][]schema.Message
}

func (s *reproStorage) AddMessageToSession(ctx context.Context, sessionID string, msg schema.Message) error {
	s.messages[sessionID] = append(s.messages[sessionID], msg)
	return nil
}

func (s *reproStorage) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	return s.messages[sessionID], nil
}

func (s *reproStorage) StoreDataPoint(ctx context.Context, dp *schema.DataPoint) error { return nil }
func (s *reproStorage) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) { return nil, nil }
func (s *reproStorage) UpdateDataPoint(ctx context.Context, dp *schema.DataPoint) error { return nil }
func (s *reproStorage) DeleteDataPoint(ctx context.Context, id string) error { return nil }
func (s *reproStorage) DeleteDataPointsBySession(ctx context.Context, sessionID string) error { return nil }
func (s *reproStorage) QueryDataPoints(ctx context.Context, query *storage.DataPointQuery) ([]*schema.DataPoint, error) { return nil, nil }
func (s *reproStorage) StoreSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (s *reproStorage) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) { return nil, nil }
func (s *reproStorage) UpdateSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (s *reproStorage) DeleteSession(ctx context.Context, sessionID string) error { return nil }
func (s *reproStorage) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) { return nil, nil }
func (s *reproStorage) StoreBatch(ctx context.Context, dps []*schema.DataPoint) error { return nil }
func (s *reproStorage) DeleteBatch(ctx context.Context, ids []string) error { return nil }
func (s *reproStorage) Health(ctx context.Context) error { return nil }
func (s *reproStorage) Close() error { return nil }

type reproEmbedder struct{}
func (e *reproEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) { return make([]float32, 768), nil }
func (e *reproEmbedder) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = make([]float32, 768)
	}
	return res, nil
}
func (e *reproEmbedder) GetDimensions() int { return 768 }
func (e *reproEmbedder) GetModel() string { return "mock-embedder" }
func (e *reproEmbedder) Health(ctx context.Context) error { return nil }

func main() {
	ctx := context.Background()
	rp := &reproProvider{}
	emb := &reproEmbedder{}
	rs := &reproStorage{messages: make(map[string][]schema.Message)}
	gs := graph.NewInMemoryGraphStore()
	vs := vector.NewInMemoryStore(nil)

	ext := &reproExtractor{
		prov:         rp,
		LLMExtractor: extractor.NewBasicExtractor(rp, nil),
	}
	eng := engine.NewMemoryEngineWithStores(ext, emb, rs, gs, vs, engine.EngineConfig{MaxWorkers: 1})
	sessionID := "repro-session-003"

	fmt.Println("--- PHASE 1: Storing memories ---")
	eng.Request(ctx, sessionID, "nhà tôi có con chó tên là Vàng")
	eng.Request(ctx, sessionID, "nhưng nó đã mất nhiều năm")
	eng.Request(ctx, sessionID, "Hàng xóm cạnh nhà tôi có một con chó tên là Đen")
	eng.Request(ctx, sessionID, "nó mới 1 tuổi")

	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n--- PHASE 2: Querying 'chó nhà tôi thế nào' ---")
	thinkResult, err := eng.Think(ctx, &schema.ThinkQuery{
		SessionID: sessionID,
		Text:      "chó nhà tôi thế nào",
		HopDepth:  2,
		IncludeReasoning: true,
	})
	if err != nil {
		fmt.Printf("Think Error: %v\n", err)
	} else {
		fmt.Printf("Think Answer: %s\n", thinkResult.Answer)
		fmt.Printf("Think Reasoning: %s\n", thinkResult.Reasoning)
		if thinkResult.ContextUsed != nil {
			fmt.Printf("Retrieved Context: %s\n", thinkResult.ContextUsed.ParsedContext)
		}
	}

	fmt.Println("\n--- Knowledge Graph Entities ---")
	nodes, _ := gs.ListNodes(ctx)
	for _, n := range nodes {
		fmt.Printf("Node: %s Props: %v\n", n.ID, n.Properties)
	}
	edges, _ := gs.ListEdges(ctx)
	for _, e := range edges {
		fmt.Printf("Edge: %s --[%s]--> %s\n", e.From, e.Type, e.To)
	}
}
