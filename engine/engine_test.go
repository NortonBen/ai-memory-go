package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"

	_ "github.com/NortonBen/ai-memory-go/graph/adapters/inmemory"
	_ "github.com/NortonBen/ai-memory-go/vector/adapters/inmemory"
)

func TestNewMemoryEngine(t *testing.T) {
	cfg := EngineConfig{MaxWorkers: 2}
	
	engine, ok := NewMemoryEngine(nil, nil, nil, cfg).(*defaultMemoryEngine)
	if !ok || engine == nil {
		t.Fatal("Expected NewMemoryEngine to return an instance of *defaultMemoryEngine")
	}
	
	if engine.workerPool == nil {
		t.Fatal("Expected workerPool to be initialized")
	}
	
	engine.Close()
}

// ----- Mocks for TestRequest -----

type mockExtractor struct {
	intent   schema.RequestIntent
	entities []schema.Node
	edges    []schema.Edge
}

func (m *mockExtractor) ExtractEntities(ctx context.Context, input string) ([]schema.Node, error) {
	return m.entities, nil
}

func (m *mockExtractor) ExtractRelationships(ctx context.Context, input string, entities []schema.Node) ([]schema.Edge, error) {
	return m.edges, nil
}

func (m *mockExtractor) ExtractRequestIntent(ctx context.Context, input string) (*schema.RequestIntent, error) {
	return &m.intent, nil
}

func (m *mockExtractor) ExtractBridgingRelationship(ctx context.Context, q, a string) (*schema.Edge, error) {
	return nil, nil
}

func (m *mockExtractor) CompareEntities(ctx context.Context, e1, e2 schema.Node) (*schema.ConsistencyResult, error) {
	return &schema.ConsistencyResult{Action: schema.ResolutionUpdate, Reason: "mock"}, nil
}

func (m *mockExtractor) AnalyzeQuery(ctx context.Context, text string) (*schema.ThinkQueryAnalysis, error) {
	if strings.Contains(text, "Lucian") {
		return &schema.ThinkQueryAnalysis{
			QueryType:      "factual",
			Subjects:       []string{"Lucian"},
			SearchKeywords: []string{"Lucian"},
			ExpectedAnswer: "mock Lucian",
			Reasoning:      "mock reasoning",
		}, nil
	}
	return &schema.ThinkQueryAnalysis{
		QueryType:      "factual",
		Subjects:       []string{"mock"},
		SearchKeywords: []string{"mock"},
		ExpectedAnswer: "mock response",
		Reasoning:      "mock reasoning",
	}, nil
}

func (m *mockExtractor) ExtractWithSchema(ctx context.Context, text string, schemaStruct interface{}) (interface{}, error) {
	// Return a dummy result that maps to schema.ThinkResult structure
	return map[string]interface{}{
		"answer":    "this is a mock answer",
		"reasoning": "this is mock reasoning",
	}, nil
}

type mockLLMProvider struct {
	extractor.LLMProvider
}

func (m *mockLLMProvider) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	return "mock completion", nil
}

func (m *mockLLMProvider) GetModel() string {
	return "mock-model"
}

func (m *mockExtractor) GetProvider() extractor.LLMProvider {
	return &mockLLMProvider{}
}

func (m *mockExtractor) SetProvider(p extractor.LLMProvider) {}

func (m *mockExtractor) Close() error { return nil }

type mockStorage struct {
	dataPoints map[string]*schema.DataPoint
	messages   map[string][]schema.Message
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		dataPoints: make(map[string]*schema.DataPoint),
		messages:   make(map[string][]schema.Message),
	}
}

func (m *mockStorage) StoreDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	m.dataPoints[dp.ID] = dp
	return nil
}

func (m *mockStorage) QueryDataPoints(ctx context.Context, query *storage.DataPointQuery) ([]*schema.DataPoint, error) {
	var res []*schema.DataPoint
	for _, dp := range m.dataPoints {
		if query.UnscopedSessionOnly {
			if dp.SessionID != "" {
				continue
			}
		} else if query.SessionID != "" {
			if query.IncludeGlobalSession {
				if dp.SessionID != "" && dp.SessionID != query.SessionID {
					continue
				}
			} else if dp.SessionID != query.SessionID {
				continue
			}
		}
		if query.SearchText != "" && dp.Content != query.SearchText {
			continue
		}
		res = append(res, dp)
	}
	return res, nil
}

func (m *mockStorage) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	if m.dataPoints == nil {
		return nil, nil
	}
	return m.dataPoints[id], nil
}
func (m *mockStorage) UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error { return nil }
func (m *mockStorage) DeleteDataPoint(ctx context.Context, id string) error { return nil }
func (m *mockStorage) DeleteDataPointsBySession(ctx context.Context, sessionID string) error {
	for id, dp := range m.dataPoints {
		if dp != nil && dp.SessionID == sessionID {
			delete(m.dataPoints, id)
		}
	}
	return nil
}
func (m *mockStorage) DeleteDataPointsUnscoped(ctx context.Context) error {
	for id, dp := range m.dataPoints {
		if dp != nil && dp.SessionID == "" {
			delete(m.dataPoints, id)
		}
	}
	return nil
}
func (m *mockStorage) StoreSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (m *mockStorage) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) { return nil, nil }
func (m *mockStorage) UpdateSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (m *mockStorage) DeleteSession(ctx context.Context, sessionID string) error { return nil }
func (m *mockStorage) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) { return nil, nil }
func (m *mockStorage) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error { return nil }
func (m *mockStorage) DeleteBatch(ctx context.Context, ids []string) error { return nil }
func (m *mockStorage) AddMessageToSession(ctx context.Context, sessionID string, message schema.Message) error {
	m.messages[sessionID] = append(m.messages[sessionID], message)
	return nil
}
func (m *mockStorage) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	return m.messages[sessionID], nil
}
func (m *mockStorage) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	delete(m.messages, sessionID)
	return nil
}

func (m *mockStorage) Health(ctx context.Context) error { return nil }
func (m *mockStorage) Close() error { return nil }

type mockEmbedder struct{}
func (m *mockEmbedder) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0, 0.0, 0.0}, nil
}
func (m *mockEmbedder) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}
func (m *mockEmbedder) Name() string { return "mock" }
func (m *mockEmbedder) GetDimensions() int { return 3 }
func (m *mockEmbedder) GetModel() string { return "mock-model" }
func (m *mockEmbedder) Health(ctx context.Context) error { return nil }

func TestAddWithLabelsRuleDefaultsCoreTier(t *testing.T) {
	ctx := context.Background()
	store := newMockStorage()
	eng := NewMemoryEngine(nil, nil, store, EngineConfig{MaxWorkers: 1})
	defer eng.Close()

	dp, err := eng.Add(ctx, "Không tiết lộ mật khẩu", WithSessionID("s1"), WithLabels(schema.LabelRule))
	if err != nil {
		t.Fatal(err)
	}
	if schema.MemoryTierFromDataPoint(dp) != schema.MemoryTierCore {
		t.Fatalf("rule label should default tier core, got %s", schema.MemoryTierFromDataPoint(dp))
	}
	labels := schema.LabelsFromMetadata(dp.Metadata)
	if len(labels) != 1 || labels[0] != schema.LabelRule {
		t.Fatalf("labels: %v", labels)
	}
}

func TestAddWithMemoryTier(t *testing.T) {
	ctx := context.Background()
	store := newMockStorage()
	eng := NewMemoryEngine(nil, nil, store, EngineConfig{MaxWorkers: 1})
	defer eng.Close()

	dp, err := eng.Add(ctx, "nội dung cốt lõi", WithSessionID("s1"), WithMemoryTier(schema.MemoryTierCore))
	if err != nil {
		t.Fatal(err)
	}
	if got := schema.MemoryTierFromDataPoint(dp); got != schema.MemoryTierCore {
		t.Fatalf("memory_tier: got %q want %q", got, schema.MemoryTierCore)
	}

	// Cùng nội dung, tier khác → không dedupe, thêm bản ghi mới.
	dp2, err := eng.Add(ctx, "nội dung cốt lõi", WithSessionID("s1"), WithMemoryTier(schema.MemoryTierStorage))
	if err != nil {
		t.Fatal(err)
	}
	if dp2.ID == dp.ID {
		t.Fatal("expected second DataPoint for different tier")
	}
	if schema.MemoryTierFromDataPoint(dp2) != schema.MemoryTierStorage {
		t.Fatalf("second tier: got %q", schema.MemoryTierFromDataPoint(dp2))
	}
}

func TestAddGlobalSession(t *testing.T) {
	ctx := context.Background()
	store := newMockStorage()
	eng := NewMemoryEngine(nil, nil, store, EngineConfig{MaxWorkers: 1})
	defer eng.Close()

	dp, err := eng.Add(ctx, "shared fact", WithGlobalSession())
	if err != nil {
		t.Fatal(err)
	}
	if dp.SessionID != "" {
		t.Fatalf("want empty session_id for global, got %q", dp.SessionID)
	}
}

func TestDeleteMemoryBySession(t *testing.T) {
	ctx := context.Background()
	store := newMockStorage()
	_ = store.StoreDataPoint(ctx, &schema.DataPoint{ID: "a", Content: "x", SessionID: "s1"})
	_ = store.StoreDataPoint(ctx, &schema.DataPoint{ID: "b", Content: "y", SessionID: "s2"})
	g, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
	v, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory, Dimension: 3})
	eng := NewMemoryEngineWithStores(nil, nil, store, g, v, EngineConfig{MaxWorkers: 1})
	defer eng.Close()
	_ = v.StoreEmbedding(ctx, "a", []float32{1, 0, 0}, map[string]interface{}{"source_id": "a"})

	if err := eng.DeleteMemory(ctx, "", "s1"); err != nil {
		t.Fatal(err)
	}
	if dp, _ := store.GetDataPoint(ctx, "a"); dp != nil {
		t.Fatal("expected datapoint a removed")
	}
	if store.dataPoints["b"] == nil {
		t.Fatal("expected s2 row kept")
	}
}

// ----- Tests -----

func TestRequest(t *testing.T) {
	t.Run("Does not vectorize when NeedsVectorStorage is false", func(t *testing.T) {
		ext := &mockExtractor{
			intent: schema.RequestIntent{NeedsVectorStorage: false, Reasoning: "just chat"},
			entities: []schema.Node{
				{ID: "node1", Type: "Person"},
			},
		}
		
		store := newMockStorage()
		graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
		vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})
		emb := &mockEmbedder{}
		
		engine := NewMemoryEngineWithStores(ext, emb, store, graphStore, vecStore, EngineConfig{MaxWorkers: 1})
		defer engine.Close()

		ctx := context.Background()
		_, err := engine.Request(ctx, "session-1", "Hello there")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Wait briefly for any async operations (should be none for vector)
		time.Sleep(50 * time.Millisecond)

		// Check vector/relational storage
		if len(store.dataPoints) != 0 {
			t.Errorf("expected 0 data points stored, got %d", len(store.dataPoints))
		}

		// Check graph storage - should be populated regardless of NeedsVectorStorage
		count, _ := graphStore.GetNodeCount(ctx)
		if count != 1 {
			t.Errorf("expected 1 node stored, got %d", count)
		}
	})

	t.Run("Vectorizes when NeedsVectorStorage is true", func(t *testing.T) {
		ext := &mockExtractor{
			intent: schema.RequestIntent{NeedsVectorStorage: true, Reasoning: "user stated a fact"},
		}
		
		store := newMockStorage()
		graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
		vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})
		emb := &mockEmbedder{}
		
		engine := NewMemoryEngineWithStores(ext, emb, store, graphStore, vecStore, EngineConfig{MaxWorkers: 1})
		defer engine.Close()

		ctx := context.Background()
		_, err := engine.Request(ctx, "session-2", "My name is Alice.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// wait for Cognify task queue to process
		time.Sleep(100 * time.Millisecond)

		// Check storage: parent input datapoint + at least one chunk datapoint
		if len(store.dataPoints) < 2 {
			t.Errorf("expected at least 2 data points stored (parent + chunk), got %d", len(store.dataPoints))
		}

		// Need to ensure the worker actually processed it. Since InMemory VectorStore is synchronous and 
		// we submit to a worker pool, the sleep should suffice for this basic test.
		count, _ := vecStore.GetEmbeddingCount(ctx)
		if count < 1 {
			t.Errorf("expected at least 1 embedding stored, got %d", count)
		}
	})

	t.Run("Returns ThinkResult when IsQuery is true", func(t *testing.T) {
		ext := &mockExtractor{
			intent: schema.RequestIntent{IsQuery: true, Reasoning: "user asked a question"},
		}
		
		store := newMockStorage()
		graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
		vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})
		emb := &mockEmbedder{}
		
		engine := NewMemoryEngineWithStores(ext, emb, store, graphStore, vecStore, EngineConfig{MaxWorkers: 1})
		defer engine.Close()

		ctx := context.Background()
		res, err := engine.Request(ctx, "session-q", "What is my name?")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("expected ThinkResult, got nil")
		}
	})

	t.Run("Deletes graph nodes when IsDelete is true", func(t *testing.T) {
		ext := &mockExtractor{
			intent: schema.RequestIntent{IsDelete: true, DeleteTargets: []string{"Alice"}},
		}
		
		store := newMockStorage()
		graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
		vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})
		emb := &mockEmbedder{}
		
		engine := NewMemoryEngineWithStores(ext, emb, store, graphStore, vecStore, EngineConfig{MaxWorkers: 1})
		defer engine.Close()

		// Add a node first
		ctx := context.Background()
		node := &schema.Node{ID: "nodeA", Properties: map[string]interface{}{"name": "Alice"}}
		graphStore.StoreNode(ctx, node)

		res, err := engine.Request(ctx, "session-d", "Forget about Alice.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("expected ThinkResult, got nil")
		}
		
		// Verify node is deleted
		nodes, _ := graphStore.FindNodesByEntity(ctx, "Alice", "")
		if len(nodes) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(nodes))
		}
	})

	t.Run("Injects chat history into context", func(t *testing.T) {
		ext := &mockExtractor{
			intent: schema.RequestIntent{IsQuery: true, Reasoning: "testing history"},
		}
		
		store := newMockStorage()
		graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
		vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})
		emb := &mockEmbedder{}
		
		engine := NewMemoryEngineWithStores(ext, emb, store, graphStore, vecStore, EngineConfig{MaxWorkers: 1})
		defer engine.Close()

		ctx := context.Background()
		sessionID := "session-hist"
		
		// 1. Initial request (adds 1 user msg, 1 asst msg)
		engine.Request(ctx, sessionID, "Hello, my name is Bob.")
		
		// 2. Second request
		engine.Request(ctx, sessionID, "What is my name?")
		
		// 3. We should have 4 messages in storage
		msgs, _ := store.GetSessionMessages(ctx, sessionID)
		if len(msgs) != 4 {
			t.Errorf("expected 4 messages in history, got %d", len(msgs))
		}

		if msgs[0].Role != schema.RoleUser || msgs[0].Content != "Hello, my name is Bob." {
			t.Errorf("expected first message to be user Bob greeting")
		}
		if msgs[1].Role != schema.RoleAssistant {
			t.Errorf("expected second message to be assistant")
		}
	})
}

func TestThinkIncludingAnalysis(t *testing.T) {
	ext := &mockExtractor{}
	store := newMockStorage()
	graphStore, _ := graph.NewStore(&graph.GraphConfig{Type: graph.StoreTypeInMemory})
	vecStore, _ := vector.NewVectorStore(&vector.VectorConfig{Type: vector.StoreTypeInMemory})
	emb := &mockEmbedder{}

	engine := NewMemoryEngineWithStores(ext, emb, store, graphStore, vecStore, EngineConfig{MaxWorkers: 1})
	defer engine.Close()

	ctx := context.Background()

	// 1. Seed a node in the graph
	graphStore.StoreNode(ctx, &schema.Node{
		ID:   "Lucian",
		Type: schema.NodeTypeEntity,
		Properties: map[string]interface{}{
			"name":        "Lucian",
			"description": "A legendary warrior of light.",
		},
	})

	// 2. Query with AnalyzeQuery = true
	query := &schema.ThinkQuery{
		Text:           "Who is Lucian?",
		SessionID:      "test-session",
		AnalyzeQuery:   true,
		EnableThinking: false,
	}

	res, err := engine.Think(ctx, query)
	if err != nil {
		t.Fatalf("Think failed: %v", err)
	}

	if res.Analysis == nil {
		t.Error("Expected Analysis result in ThinkResult, got nil")
	}

	// Verify the context includes information about Lucian (retrieved via analysis subjects/anchors)
	if !strings.Contains(res.ContextUsed.ParsedContext, "Lucian") {
		t.Errorf("Expected context to contain 'Lucian', got: %s", res.ContextUsed.ParsedContext)
	}
}
