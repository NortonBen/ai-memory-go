package view

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/stretchr/testify/require"
)

// stubRelStore implements storage.RelationalStore for HTTP handler tests.
type stubRelStore struct {
	queryErr     error
	dpCount      int64
	sessionCount int64
	latest       []*schema.DataPoint
	all          []*schema.DataPoint
}

func (s *stubRelStore) QueryDataPoints(ctx context.Context, q *storage.DataPointQuery) ([]*schema.DataPoint, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	if q != nil && q.Limit == 5 && q.SortBy == "updated_at" {
		return s.latest, nil
	}
	if s.all != nil {
		return s.all, nil
	}
	return nil, nil
}

func (s *stubRelStore) GetDataPointCount(ctx context.Context) (int64, error) { return s.dpCount, nil }
func (s *stubRelStore) GetSessionCount(ctx context.Context) (int64, error)     { return s.sessionCount, nil }

func (s *stubRelStore) StoreDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error {
	return nil
}
func (s *stubRelStore) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	return nil, nil
}
func (s *stubRelStore) UpdateDataPoint(ctx context.Context, dataPoint *schema.DataPoint) error { return nil }
func (s *stubRelStore) DeleteDataPoint(ctx context.Context, id string) error                    { return nil }
func (s *stubRelStore) DeleteDataPointsBySession(ctx context.Context, sessionID string) error   { return nil }
func (s *stubRelStore) DeleteDataPointsUnscoped(ctx context.Context) error                      { return nil }
func (s *stubRelStore) SearchDataPoints(ctx context.Context, searchQuery string, filters map[string]interface{}) ([]*schema.DataPoint, error) {
	return nil, nil
}
func (s *stubRelStore) StoreSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (s *stubRelStore) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) {
	return nil, nil
}
func (s *stubRelStore) UpdateSession(ctx context.Context, session *schema.MemorySession) error { return nil }
func (s *stubRelStore) DeleteSession(ctx context.Context, sessionID string) error               { return nil }
func (s *stubRelStore) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) {
	return nil, nil
}
func (s *stubRelStore) AddMessageToSession(ctx context.Context, sessionID string, message schema.Message) error {
	return nil
}
func (s *stubRelStore) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	return nil, nil
}
func (s *stubRelStore) DeleteSessionMessages(ctx context.Context, sessionID string) error { return nil }
func (s *stubRelStore) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error {
	return nil
}
func (s *stubRelStore) DeleteBatch(ctx context.Context, ids []string) error { return nil }
func (s *stubRelStore) Health(ctx context.Context) error                    { return nil }
func (s *stubRelStore) Close() error                                        { return nil }

type stubVecStore struct {
	embByID map[string][]float32
	metaByID map[string]map[string]interface{}
	errByID map[string]error
}

func (v *stubVecStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	return nil
}
func (v *stubVecStore) GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error) {
	if v.errByID != nil {
		if err, ok := v.errByID[id]; ok && err != nil {
			return nil, nil, err
		}
	}
	if v.embByID == nil {
		return nil, nil, io.EOF
	}
	emb, ok := v.embByID[id]
	if !ok {
		return nil, nil, io.EOF
	}
	meta := map[string]interface{}{}
	if v.metaByID != nil {
		if m, ok := v.metaByID[id]; ok && m != nil {
			meta = m
		}
	}
	return emb, meta, nil
}
func (v *stubVecStore) UpdateEmbedding(ctx context.Context, id string, embedding []float32) error { return nil }
func (v *stubVecStore) DeleteEmbedding(ctx context.Context, id string) error                     { return nil }
func (v *stubVecStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	return nil, nil
}
func (v *stubVecStore) SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	return nil, nil
}
func (v *stubVecStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*vector.EmbeddingData) error { return nil }
func (v *stubVecStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error                      { return nil }
func (v *stubVecStore) CreateCollection(ctx context.Context, name string, dimension int, config *vector.CollectionConfig) error {
	return nil
}
func (v *stubVecStore) DeleteCollection(ctx context.Context, name string) error { return nil }
func (v *stubVecStore) ListCollections(ctx context.Context) ([]string, error)   { return nil, nil }
func (v *stubVecStore) GetCollectionInfo(ctx context.Context, name string) (*vector.CollectionInfo, error) {
	return nil, nil
}
func (v *stubVecStore) GetEmbeddingCount(ctx context.Context) (int64, error) { return 0, nil }
func (v *stubVecStore) Health(ctx context.Context) error                     { return nil }
func (v *stubVecStore) Close() error                                          { return nil }

// stubEngine implements engine.MemoryEngine for HTTP handler tests.
type stubEngine struct {
	healthErr    error
	searchErr    error
	searchResult *schema.SearchResults
	thinkErr     error
	thinkResult  *schema.ThinkResult
	addErr       error
	addDP        *schema.DataPoint
	delErr       error
}

func (e *stubEngine) Health(ctx context.Context) error { return e.healthErr }
func (e *stubEngine) Close() error                     { return nil }

func (e *stubEngine) Add(ctx context.Context, content string, opts ...engine.AddOption) (*schema.DataPoint, error) {
	if e.addErr != nil {
		return nil, e.addErr
	}
	if e.addDP != nil {
		return e.addDP, nil
	}
	now := time.Now()
	return &schema.DataPoint{ID: "stub-id", Content: content, CreatedAt: now, UpdatedAt: now}, nil
}

func (e *stubEngine) Cognify(ctx context.Context, dataPoint *schema.DataPoint, opts ...engine.CognifyOption) (*schema.DataPoint, error) {
	return dataPoint, nil
}
func (e *stubEngine) CognifyPending(ctx context.Context, sessionID string) error { return nil }
func (e *stubEngine) Memify(ctx context.Context, dataPoint *schema.DataPoint, opts ...engine.MemifyOption) error {
	return nil
}

func (e *stubEngine) Search(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error) {
	if e.searchErr != nil {
		return nil, e.searchErr
	}
	if e.searchResult != nil {
		return e.searchResult, nil
	}
	return &schema.SearchResults{Results: nil, Total: 0}, nil
}

func (e *stubEngine) Think(ctx context.Context, query *schema.ThinkQuery) (*schema.ThinkResult, error) {
	if e.thinkErr != nil {
		return nil, e.thinkErr
	}
	if e.thinkResult != nil {
		return e.thinkResult, nil
	}
	return &schema.ThinkResult{Answer: "ok"}, nil
}

func (e *stubEngine) AnalyzeHistory(ctx context.Context, sessionID string) error { return nil }

func (e *stubEngine) Request(ctx context.Context, sessionID string, content string, opts ...engine.RequestOption) (*schema.ThinkResult, error) {
	return &schema.ThinkResult{}, nil
}

func (e *stubEngine) DeleteMemory(ctx context.Context, id string, sessionID string) error {
	return e.delErr
}

func TestNewServer_RequiresEngineAndStore(t *testing.T) {
	_, err := NewServer(Dependencies{Engine: nil, RelStore: &stubRelStore{}})
	require.Error(t, err)
	_, err = NewServer(Dependencies{Engine: &stubEngine{}, RelStore: nil})
	require.Error(t, err)
}

func TestNewServer_Defaults(t *testing.T) {
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{}})
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Equal(t, "AI Memory Viewer", s.deps.AppName)
	require.Equal(t, "/api", s.deps.AppPrefix)
}

func TestHandleHealth_OKAndDegraded(t *testing.T) {
	engOK := &stubEngine{}
	s, err := NewServer(Dependencies{Engine: engOK, RelStore: &stubRelStore{}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])

	engBad := &stubEngine{healthErr: io.EOF}
	s2, err := NewServer(Dependencies{Engine: engBad, RelStore: &stubRelStore{}})
	require.NoError(t, err)
	ts2 := httptest.NewServer(s2.Handler())
	defer ts2.Close()
	resp2, err := http.Get(ts2.URL + "/api/health")
	require.NoError(t, err)
	defer resp2.Body.Close()
	var body2 map[string]interface{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&body2))
	require.Equal(t, "degraded", body2["status"])
}

func TestHandleOverview(t *testing.T) {
	now := time.Now()
	rel := &stubRelStore{
		dpCount:      10,
		sessionCount: 2,
		latest: []*schema.DataPoint{
			{ID: "l1", Content: "x", UpdatedAt: now, ProcessingStatus: schema.StatusCompleted},
		},
		all: []*schema.DataPoint{
			{ID: "a1", ProcessingStatus: schema.StatusPending, Metadata: map[string]interface{}{"is_input": true}},
			{ID: "a2", ProcessingStatus: schema.StatusCompleted, Metadata: map[string]interface{}{"is_chunk": true}},
		},
	}
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: rel})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/overview")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.EqualValues(t, 10, out["datapoint_count"])
	require.EqualValues(t, 2, out["session_count"])
	require.NotNil(t, out["status_summary"])
	require.NotNil(t, out["type_summary"])
	require.NotNil(t, out["tier_summary"])
}

func TestHandleDataPoints_QueryErrorAndPagination(t *testing.T) {
	eng := &stubEngine{}
	badRel := &stubRelStore{queryErr: io.ErrUnexpectedEOF}
	s, err := NewServer(Dependencies{Engine: eng, RelStore: badRel})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/datapoints")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	chunk1 := &schema.DataPoint{
		ID: "c1", Content: "one", ProcessingStatus: schema.StatusCompleted,
		Metadata: map[string]interface{}{"is_chunk": true},
	}
	chunk2 := &schema.DataPoint{
		ID: "c2", Content: "two", ProcessingStatus: schema.StatusCompleted,
		Metadata: map[string]interface{}{"is_chunk": true},
	}
	goodRel := &stubRelStore{all: []*schema.DataPoint{chunk1, chunk2}}
	s2, err := NewServer(Dependencies{Engine: eng, RelStore: goodRel})
	require.NoError(t, err)
	ts2 := httptest.NewServer(s2.Handler())
	defer ts2.Close()

	resp2, err := http.Get(ts2.URL + "/api/datapoints?kind=processed&limit=1&offset=0")
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&out))
	require.EqualValues(t, 2, out["total"])
	items, ok := out["items"].([]interface{})
	require.True(t, ok)
	require.Len(t, items, 1)
}

func TestHandleProcessedRewritesKind(t *testing.T) {
	ch := &schema.DataPoint{ID: "p1", ProcessingStatus: schema.StatusCompleted, Metadata: map[string]interface{}{"is_chunk": true}}
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{all: []*schema.DataPoint{ch}}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/processed")
	require.NoError(t, err)
	defer resp.Body.Close()
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, "processed", out["kind"])
	require.EqualValues(t, 1, out["total"])
}

func TestHandleInputsRewritesKind(t *testing.T) {
	in := &schema.DataPoint{ID: "i1", Metadata: map[string]interface{}{"is_input": true}}
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{all: []*schema.DataPoint{in}}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/inputs")
	require.NoError(t, err)
	defer resp.Body.Close()
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, "input", out["kind"])
	require.EqualValues(t, 1, out["total"])
}

func TestHandleSearch_BadRequestAndOK(t *testing.T) {
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/search")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp2, err := http.Get(ts.URL + "/api/search?q=hello&limit=3")
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestHandleRelationships_FromDataPoints(t *testing.T) {
	dp := &schema.DataPoint{
		ID: "dp1",
		Nodes: []*schema.Node{
			{ID: "n1", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "a"}},
		},
		Edges: []schema.Edge{
			{ID: "e1", From: "n1", To: "n2", Type: schema.EdgeTypeRelatedTo, Weight: 0.5},
		},
		Relationships: []schema.Relationship{
			{Type: schema.EdgeTypeRelatedTo, Target: "ext", Weight: 1, Metadata: map[string]interface{}{"k": "v"}},
		},
	}
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{all: []*schema.DataPoint{dp}}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/relationships?limit=50")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	nodes, ok := out["nodes"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, nodes)
	edges, ok := out["edges"].([]interface{})
	require.True(t, ok)
	require.GreaterOrEqual(t, len(edges), 2)
}

func TestHandleGraphStats_NoGraph(t *testing.T) {
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/graph/stats")
	require.NoError(t, err)
	defer resp.Body.Close()
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, false, out["available"])
}

func TestHandleVectors_AndVectorPreview(t *testing.T) {
	dp1 := &schema.DataPoint{ID: "d1", Content: "one", Metadata: map[string]interface{}{"is_chunk": true}}
	dp2 := &schema.DataPoint{ID: "d2", Content: "two", Metadata: map[string]interface{}{"is_chunk": true}}
	dp3 := &schema.DataPoint{ID: "d3", Content: "three", Metadata: map[string]interface{}{"is_chunk": true}}

	vs := &stubVecStore{
		embByID: map[string][]float32{
			"d1": {1, 2, 3, 4, 5, 6, 7},
			"d3": {9, 9, 9},
		},
		metaByID: map[string]map[string]interface{}{
			"d1": {"memory_tier": "core"},
		},
		errByID: map[string]error{
			"d2": io.ErrUnexpectedEOF,
		},
	}

	s, err := NewServer(Dependencies{
		Engine:   &stubEngine{},
		RelStore: &stubRelStore{all: []*schema.DataPoint{dp1, dp2, dp3}},
		VecStore: vs,
	})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	// Should include only available embeddings, and paginate after filtering.
	resp, err := http.Get(ts.URL + "/api/vectors?limit=1&offset=0")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.EqualValues(t, 2, out["total"]) // d1 + d3 available, d2 error -> filtered out
	items, ok := out["items"].([]interface{})
	require.True(t, ok)
	require.Len(t, items, 1)
}

func TestHandleRelationships_FallbackGraphSnapshot(t *testing.T) {
	gs := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	n1 := &schema.Node{ID: "n1", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "x"}}
	n2 := &schema.Node{ID: "n2", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "y"}}
	require.NoError(t, gs.StoreNode(ctx, n1))
	require.NoError(t, gs.StoreNode(ctx, n2))
	require.NoError(t, gs.CreateRelationship(ctx, &schema.Edge{ID: "e1", From: "n1", To: "n2", Type: schema.EdgeTypeRelatedTo, Weight: 1}))

	// RelStore returns datapoints without extracted payload -> should fallback to GraphStore snapshot.
	s, err := NewServer(Dependencies{
		Engine:   &stubEngine{},
		RelStore: &stubRelStore{all: []*schema.DataPoint{{ID: "dp"}}},
		Graph:    gs,
	})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/relationships?limit=10")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	nodes, ok := out["nodes"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, nodes)
}

func TestGraphNeighborsAndPath(t *testing.T) {
	gs := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	require.NoError(t, gs.StoreNode(ctx, &schema.Node{ID: "a", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "a"}}))
	require.NoError(t, gs.StoreNode(ctx, &schema.Node{ID: "b", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "b"}}))
	require.NoError(t, gs.StoreNode(ctx, &schema.Node{ID: "c", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "c"}}))
	require.NoError(t, gs.CreateRelationship(ctx, &schema.Edge{ID: "eab", From: "a", To: "b", Type: schema.EdgeTypeRelatedTo, Weight: 1}))
	require.NoError(t, gs.CreateRelationship(ctx, &schema.Edge{ID: "ebc", From: "b", To: "c", Type: schema.EdgeTypeRelatedTo, Weight: 1}))

	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{}, Graph: gs})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	// neighbors missing node_id
	resp, err := http.Get(ts.URL + "/api/graph/neighbors")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// neighbors ok
	resp2, err := http.Get(ts.URL + "/api/graph/neighbors?node_id=a&depth=2")
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	// path missing params
	resp3, err := http.Get(ts.URL + "/api/graph/path?from=a")
	require.NoError(t, err)
	resp3.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp3.StatusCode)

	// path ok
	resp4, err := http.Get(ts.URL + "/api/graph/path?from=a&to=c&depth=4")
	require.NoError(t, err)
	defer resp4.Body.Close()
	require.Equal(t, http.StatusOK, resp4.StatusCode)
}

func TestHandleSessionMemoryDelete_Validation(t *testing.T) {
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/memory/session", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleThink_MethodAndBody(t *testing.T) {
	s, err := NewServer(Dependencies{Engine: &stubEngine{}, RelStore: &stubRelStore{}})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/think")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	body := strings.NewReader(`{"text":"why"}`)
	resp2, err := http.Post(ts.URL+"/api/think", "application/json", body)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var tr schema.ThinkResult
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&tr))
	require.Equal(t, "ok", tr.Answer)
}

func TestHandleMemory_POST(t *testing.T) {
	dp := &schema.DataPoint{ID: "mem-1", SessionID: "s1", Metadata: map[string]interface{}{}}
	s, err := NewServer(Dependencies{
		Engine:   &stubEngine{addDP: dp},
		RelStore: &stubRelStore{},
	})
	require.NoError(t, err)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	payload := `{"content":"hello world","session_id":"s1"}`
	resp, err := http.Post(ts.URL+"/api/memory", "application/json", strings.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, true, out["ok"])
	require.Equal(t, "mem-1", out["id"])
}

var _ storage.RelationalStore = (*stubRelStore)(nil)
var _ engine.MemoryEngine = (*stubEngine)(nil)
var _ vector.VectorStore = (*stubVecStore)(nil)
