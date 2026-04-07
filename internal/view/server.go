package view

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	memorygraph "github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

//go:embed static/index.html
var staticFS embed.FS

type Dependencies struct {
	Engine    engine.MemoryEngine
	Graph     memorygraph.GraphStore
	RelStore  storage.RelationalStore
	VecStore  vector.VectorStore
	AppName   string
	AppPrefix string
}

type Server struct {
	deps Dependencies
	mux  *http.ServeMux
}

func NewServer(deps Dependencies) (*Server, error) {
	if deps.Engine == nil || deps.RelStore == nil {
		return nil, fmt.Errorf("engine and relational store are required")
	}
	if deps.AppName == "" {
		deps.AppName = "AI Memory Viewer"
	}
	if deps.AppPrefix == "" {
		deps.AppPrefix = "/api"
	}

	s := &Server{
		deps: deps,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	p := strings.TrimRight(s.deps.AppPrefix, "/")
	s.mux.HandleFunc(p+"/health", s.handleHealth)
	s.mux.HandleFunc(p+"/overview", s.handleOverview)
	s.mux.HandleFunc(p+"/datapoints", s.handleDataPoints)
	s.mux.HandleFunc(p+"/inputs", s.handleInputs)
	s.mux.HandleFunc(p+"/processed", s.handleProcessed)
	s.mux.HandleFunc(p+"/relationships", s.handleRelationships)
	s.mux.HandleFunc(p+"/vectors", s.handleVectors)
	s.mux.HandleFunc(p+"/search", s.handleSearch)
	s.mux.HandleFunc(p+"/memory", s.handleMemory)
	s.mux.HandleFunc(p+"/memory/session", s.handleSessionMemoryDelete)
	s.mux.HandleFunc(p+"/think", s.handleThink)
	s.mux.HandleFunc(p+"/graph/stats", s.handleGraphStats)
	s.mux.HandleFunc(p+"/graph/neighbors", s.handleGraphNeighbors)
	s.mux.HandleFunc(p+"/graph/path", s.handleGraphPath)

	staticSub, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("/", http.FileServer(http.FS(staticSub)))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := s.deps.Engine.Health(ctx)
	status := "ok"
	if err != nil {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    status,
		"app":       s.deps.AppName,
		"timestamp": time.Now().UTC(),
		"error":     errString(err),
	})
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dpCount, _ := s.deps.RelStore.GetDataPointCount(ctx)
	sessionCount, _ := s.deps.RelStore.GetSessionCount(ctx)

	var vecCount int64
	if s.deps.VecStore != nil {
		vecCount, _ = s.deps.VecStore.GetEmbeddingCount(ctx)
	}

	latest, _ := s.deps.RelStore.QueryDataPoints(ctx, &storage.DataPointQuery{
		Limit:     5,
		SortBy:    "updated_at",
		SortOrder: "desc",
	})

	// Build status + type summary for the new parent/chunk workflow.
	summaryQuery := storage.DefaultDataPointQuery()
	summaryQuery.Limit = 100000
	all, _ := s.deps.RelStore.QueryDataPoints(ctx, summaryQuery)
	statusSummary := map[string]int{
		string(schema.StatusPending):    0,
		string(schema.StatusProcessing): 0,
		string(schema.StatusCognified):  0,
		string(schema.StatusCompleted):  0,
		string(schema.StatusFailed):     0,
	}
	typeSummary := map[string]int{
		"input": 0,
		"chunk": 0,
	}
	tierSummary := map[string]int{
		schema.MemoryTierCore:    0,
		schema.MemoryTierGeneral: 0,
		schema.MemoryTierData:    0,
		schema.MemoryTierStorage: 0,
	}
	for _, dp := range all {
		statusSummary[string(dp.ProcessingStatus)]++
		if dataPointKind(dp) == "chunk" {
			typeSummary["chunk"]++
		} else {
			typeSummary["input"]++
		}
		t := schema.MemoryTierFromDataPoint(dp)
		tierSummary[t]++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"app":             s.deps.AppName,
		"datapoint_count": dpCount,
		"session_count":   sessionCount,
		"vector_count":    vecCount,
		"status_summary":  statusSummary,
		"type_summary":    typeSummary,
		"tier_summary":    tierSummary,
		"latest":          toDataPointCards(latest),
	})
}

func (s *Server) handleDataPoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := boundInt(parseInt(r, "limit", 30), 1, 200)
	offset := max(parseInt(r, "offset", 0), 0)
	searchText := strings.TrimSpace(r.URL.Query().Get("q"))
	sessionParam := strings.TrimSpace(r.URL.Query().Get("session"))
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))     // all|input|chunk|processed
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))) // pending|processing|...
	if kind == "" {
		kind = "processed"
	}
	if status == "" {
		status = "all"
	}

	query := storage.DefaultDataPointQuery()
	// We paginate after applying kind/status filter in-memory.
	query.Limit = 100000
	query.Offset = 0
	query.SearchText = searchText
	if sid, unscoped := sessionid.ListFilter(sessionParam); unscoped {
		query.UnscopedSessionOnly = true
	} else {
		query.SessionID = sid
	}

	results, err := s.deps.RelStore.QueryDataPoints(ctx, query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	results = filterDataPoints(results, kind, status)
	if tier := strings.TrimSpace(r.URL.Query().Get("tier")); tier != "" {
		want := schema.NormalizeMemoryTier(tier)
		filtered := make([]*schema.DataPoint, 0, len(results))
		for _, dp := range results {
			if schema.MemoryTierFromDataPoint(dp) == want {
				filtered = append(filtered, dp)
			}
		}
		results = filtered
	}
	if tag := strings.TrimSpace(r.URL.Query().Get("label")); tag != "" {
		want := schema.NormalizeLabel(tag)
		filtered := make([]*schema.DataPoint, 0, len(results))
		for _, dp := range results {
			if schema.DataPointHasAnyLabel(dp, []string{want}) {
				filtered = append(filtered, dp)
			}
		}
		results = filtered
	}

	includeVectors := strings.EqualFold(r.URL.Query().Get("include_vectors"), "true")
	total := len(results)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	items := make([]map[string]interface{}, 0, end-start)
	for _, dp := range results[start:end] {
		parentID, _ := metadataString(dp.Metadata, "parent_id")
		chunkIndex, _ := metadataInt(dp.Metadata, "chunk_index")
		totalChunks, _ := metadataInt(dp.Metadata, "total_chunks")
		processedChunks, _ := metadataInt(dp.Metadata, "processed_chunks")

		itemType := dataPointKind(dp)

		primary := ""
		if dp.Metadata != nil {
			if p, ok := dp.Metadata[schema.MetadataKeyPrimaryLabel].(string); ok {
				primary = p
			}
		}
		card := map[string]interface{}{
			"id":               dp.ID,
			"session_id":       dp.SessionID,
			"type":             dp.ContentType,
			"item_type":        itemType,
			"memory_tier":      schema.MemoryTierFromDataPoint(dp),
			"labels":           schema.LabelsFromMetadata(dp.Metadata),
			"primary_label":    primary,
			"status":           dp.ProcessingStatus,
			"content":          dp.Content,
			"created_at":       dp.CreatedAt,
			"updated_at":       dp.UpdatedAt,
			"node_count":       len(dp.Nodes),
			"edge_count":       len(dp.Edges),
			"parent_id":        parentID,
			"chunk_index":      chunkIndex,
			"total_chunks":     totalChunks,
			"processed_chunks": processedChunks,
		}
		if includeVectors {
			card["vector"] = s.vectorPreview(ctx, dp.ID)
		}
		items = append(items, card)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
		"kind":   kind,
		"status": status,
		"total":  total,
		"items":  items,
	})
}

func (s *Server) handleInputs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	q.Set("kind", "input")
	r.URL.RawQuery = q.Encode()
	s.handleDataPoints(w, r)
}

func (s *Server) handleProcessed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	q.Set("kind", "processed")
	r.URL.RawQuery = q.Encode()
	s.handleDataPoints(w, r)
}

func (s *Server) handleRelationships(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := boundInt(parseInt(r, "limit", 100), 1, 500)
	sessionParam := strings.TrimSpace(r.URL.Query().Get("session"))

	query := storage.DefaultDataPointQuery()
	query.Limit = limit
	if sid, unscoped := sessionid.ListFilter(sessionParam); unscoped {
		query.UnscopedSessionOnly = true
	} else {
		query.SessionID = sid
	}

	results, err := s.deps.RelStore.QueryDataPoints(ctx, query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	nodes := map[string]map[string]interface{}{}
	edges := make([]map[string]interface{}, 0)
	for _, dp := range results {
		for _, n := range dp.Nodes {
			if n == nil {
				continue
			}
			nodes[n.ID] = map[string]interface{}{
				"id":         n.ID,
				"type":       n.Type,
				"properties": n.Properties,
			}
		}
		for _, e := range dp.Edges {
			edges = append(edges, map[string]interface{}{
				"id":     e.ID,
				"from":   e.From,
				"to":     e.To,
				"type":   e.Type,
				"weight": e.Weight,
			})
		}
		for _, rel := range dp.Relationships {
			edges = append(edges, map[string]interface{}{
				"id":       fmt.Sprintf("%s:%s:%s", dp.ID, rel.Type, rel.Target),
				"from":     dp.ID,
				"to":       rel.Target,
				"type":     rel.Type,
				"weight":   rel.Weight,
				"metadata": rel.Metadata,
			})
		}
	}

	nodeList := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		nodeList = append(nodeList, n)
	}

	// Fallback: if datapoints do not carry extracted graph payloads, pull from GraphStore directly.
	if len(nodeList) == 0 && len(edges) == 0 && s.deps.Graph != nil {
		graphNodes, graphEdges := s.loadGraphSnapshot(ctx, limit)
		if len(graphNodes) > 0 || len(graphEdges) > 0 {
			nodeList = graphNodes
			edges = graphEdges
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes": nodeList,
		"edges": edges,
		"total": map[string]int{
			"nodes": len(nodeList),
			"edges": len(edges),
		},
	})
}

func (s *Server) loadGraphSnapshot(ctx context.Context, limit int) ([]map[string]interface{}, []map[string]interface{}) {
	nodeTypes := []schema.NodeType{
		schema.NodeTypeConcept,
		schema.NodeTypeWord,
		schema.NodeTypeUserPreference,
		schema.NodeTypeGrammarRule,
		schema.NodeTypeEntity,
		schema.NodeTypeDocument,
		schema.NodeTypeSession,
		schema.NodeTypeUser,
	}

	nodeByID := make(map[string]*schema.Node)
	for _, nt := range nodeTypes {
		results, err := s.deps.Graph.FindNodesByType(ctx, nt)
		if err != nil {
			continue
		}
		for _, n := range results {
			if n == nil {
				continue
			}
			nodeByID[n.ID] = n
			if len(nodeByID) >= limit {
				break
			}
		}
		if len(nodeByID) >= limit {
			break
		}
	}

	nodeList := make([]map[string]interface{}, 0, len(nodeByID))
	for _, n := range nodeByID {
		nodeList = append(nodeList, map[string]interface{}{
			"id":         n.ID,
			"type":       n.Type,
			"properties": n.Properties,
		})
	}

	edgeSet := make(map[string]map[string]interface{})
	for _, n := range nodeByID {
		neighbors, err := s.deps.Graph.FindConnected(ctx, n.ID, nil)
		if err != nil {
			continue
		}
		for _, nb := range neighbors {
			if nb == nil {
				continue
			}
			if _, exists := nodeByID[nb.ID]; !exists && len(nodeByID) < limit {
				nodeByID[nb.ID] = nb
				nodeList = append(nodeList, map[string]interface{}{
					"id":         nb.ID,
					"type":       nb.Type,
					"properties": nb.Properties,
				})
			}
			key := n.ID + "->" + nb.ID
			if _, ok := edgeSet[key]; ok {
				continue
			}
			edgeSet[key] = map[string]interface{}{
				"id":     key,
				"from":   n.ID,
				"to":     nb.ID,
				"type":   "CONNECTED_TO",
				"weight": 1,
			}
		}
	}

	edgeList := make([]map[string]interface{}, 0, len(edgeSet))
	for _, e := range edgeSet {
		edgeList = append(edgeList, e)
	}
	return nodeList, edgeList
}

func (s *Server) handleVectors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := boundInt(parseInt(r, "limit", 25), 1, 200)
	offset := max(parseInt(r, "offset", 0), 0)
	sessionParam := strings.TrimSpace(r.URL.Query().Get("session"))
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))     // all|input|chunk|processed
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))) // pending|processing|...
	if kind == "" {
		kind = "processed"
	}
	if status == "" {
		status = "all"
	}

	query := storage.DefaultDataPointQuery()
	query.Limit = 100000
	if sid, unscoped := sessionid.ListFilter(sessionParam); unscoped {
		query.UnscopedSessionOnly = true
	} else {
		query.SessionID = sid
	}

	results, err := s.deps.RelStore.QueryDataPoints(ctx, query)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	results = filterDataPoints(results, kind, status)
	if tier := strings.TrimSpace(r.URL.Query().Get("tier")); tier != "" {
		want := schema.NormalizeMemoryTier(tier)
		filtered := make([]*schema.DataPoint, 0, len(results))
		for _, dp := range results {
			if schema.MemoryTierFromDataPoint(dp) == want {
				filtered = append(filtered, dp)
			}
		}
		results = filtered
	}
	if tag := strings.TrimSpace(r.URL.Query().Get("label")); tag != "" {
		want := schema.NormalizeLabel(tag)
		filtered := make([]*schema.DataPoint, 0, len(results))
		for _, dp := range results {
			if schema.DataPointHasAnyLabel(dp, []string{want}) {
				filtered = append(filtered, dp)
			}
		}
		results = filtered
	}

	// IMPORTANT: filter by available embeddings first, then paginate.
	// If we paginate before filtering, large batches of pending chunks can make
	// the result look empty even when vectors exist further down the list.
	items := make([]map[string]interface{}, 0, limit)
	totalAvailable := 0
	start := offset
	end := offset + limit
	for _, dp := range results {
		v := s.vectorPreview(ctx, dp.ID)
		available, _ := v["available"].(bool)
		if !available {
			continue
		}
		if totalAvailable >= start && totalAvailable < end {
			vecTier := ""
			if meta, ok := v["metadata"].(map[string]interface{}); ok && meta != nil {
				if s, ok := meta["memory_tier"].(string); ok {
					vecTier = s
				}
			}
			primary := ""
			if dp.Metadata != nil {
				if p, ok := dp.Metadata[schema.MetadataKeyPrimaryLabel].(string); ok {
					primary = p
				}
			}
			items = append(items, map[string]interface{}{
				"id":            dp.ID,
				"session_id":    dp.SessionID,
				"content":       dp.Content,
				"memory_tier":   schema.MemoryTierFromDataPoint(dp),
				"vec_tier":      vecTier,
				"labels":        schema.LabelsFromMetadata(dp.Metadata),
				"primary_label": primary,
				"vector":        v,
			})
		}
		totalAvailable++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":  items,
		"limit":  limit,
		"offset": offset,
		"total":  totalAvailable,
		"kind":   kind,
		"status": status,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	queryText := strings.TrimSpace(r.URL.Query().Get("q"))
	if queryText == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("query param q is required"))
		return
	}

	limit := boundInt(parseInt(r, "limit", 5), 1, 50)
	sessionID := sessionid.ForEngineContext(r.URL.Query().Get("session"))

	sq := &schema.SearchQuery{
		Text:      queryText,
		SessionID: sessionID,
		Limit:     limit,
	}

	if queryBool(r, "four_tier") {
		ft := &schema.FourTierSearchOptions{
			IncludeStorageTier: queryBool(r, "include_storage"),
			AutoStorageIfWeak:  queryBool(r, "auto_storage"),
		}
		if v := strings.TrimSpace(r.URL.Query().Get("weak_threshold")); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				ft.WeakScoreThreshold = f
			}
		}
		if queryBool(r, "no_core") {
			f := false
			ft.SearchCore = &f
		}
		if queryBool(r, "no_general") {
			f := false
			ft.SearchGeneral = &f
		}
		if queryBool(r, "no_data") {
			f := false
			ft.SearchData = &f
		}
		en := true
		ft.Enabled = &en
		sq.FourTier = ft
	}

	resp, err := s.deps.Engine.Search(r.Context(), sq)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleMemory: POST JSON thêm bộ nhớ; DELETE ?id=...&session=... (session tùy chọn, khớp session_id bản ghi khi có).
func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("query param id is required"))
			return
		}
		session := strings.TrimSpace(r.URL.Query().Get("session"))
		if err := s.deps.Engine.DeleteMemory(ctx, id, session); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id})
		return
	case http.MethodPost:
		// fall through to body below
	default:
		w.Header().Set("Allow", "POST, DELETE")
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}

	var body struct {
		Content   string   `json:"content"`
		SessionID string   `json:"session_id"`
		Tier      string   `json:"tier"`
		Cognify   bool     `json:"cognify"`
		Labels    []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("content is required"))
		return
	}
	var opts []engine.AddOption
	if sid, global := sessionid.ForDataPointAdd(body.SessionID); global {
		opts = append(opts, engine.WithGlobalSession())
	} else {
		opts = append(opts, engine.WithSessionID(sid))
	}
	if strings.TrimSpace(body.Tier) != "" {
		opts = append(opts, engine.WithMemoryTier(body.Tier))
	}
	if len(body.Labels) > 0 {
		opts = append(opts, engine.WithLabels(body.Labels...))
	}
	dp, err := s.deps.Engine.Add(ctx, content, opts...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if body.Cognify {
		_, _ = s.deps.Engine.Cognify(ctx, dp, engine.WithWaitCognify(false))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":             true,
		"id":             dp.ID,
		"session_id":     dp.SessionID,
		"memory_tier":    schema.MemoryTierFromDataPoint(dp),
		"labels":        schema.LabelsFromMetadata(dp.Metadata),
		"primary_label": primaryLabelFromDP(dp),
		"cognify_queued": body.Cognify,
	})
}

// handleSessionMemoryDelete: DELETE ?session=... — xóa toàn bộ dữ liệu session (SQL + vector + graph + lịch sử chat).
func (s *Server) handleSessionMemoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", http.MethodDelete)
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	session := strings.TrimSpace(r.URL.Query().Get("session"))
	if session == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("query param session is required"))
		return
	}
	if err := s.deps.Engine.DeleteMemory(r.Context(), "", session); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":              true,
		"deleted_session": session,
	})
}

func primaryLabelFromDP(dp *schema.DataPoint) string {
	if dp == nil || dp.Metadata == nil {
		return ""
	}
	if p, ok := dp.Metadata[schema.MetadataKeyPrimaryLabel].(string); ok {
		return p
	}
	return ""
}

func queryBool(r *http.Request, key string) bool {
	return strings.EqualFold(strings.TrimSpace(r.URL.Query().Get(key)), "true") ||
		strings.TrimSpace(r.URL.Query().Get(key)) == "1"
}

// thinkRequest JSON cho POST /api/think (tách tag để client không phụ thuộc tên field Go).
type thinkRequest struct {
	Text               string                       `json:"text"`
	SessionID          string                       `json:"session_id"`
	Limit              int                          `json:"limit"`
	HopDepth           int                          `json:"hop_depth"`
	AnalyzeQuery       bool                         `json:"analyze_query"`
	EnableThinking     bool                         `json:"enable_thinking"`
	MaxThinkingSteps   int                          `json:"max_thinking_steps"`
	LearnRelationships bool                         `json:"learn_relationships"`
	IncludeReasoning   bool                         `json:"include_reasoning"`
	MaxContextLength   int                          `json:"max_context_length"`
	SegmentContext     bool                         `json:"segment_context"`
	FourTier *schema.FourTierSearchOptions `json:"four_tier,omitempty"`
}

func (s *Server) handleThink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	var req thinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("text is required"))
		return
	}
	sessionID := sessionid.ForEngineContext(req.SessionID)
	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 50 {
		limit = 50
	}
	tq := &schema.ThinkQuery{
		Text:               text,
		SessionID:          sessionID,
		Limit:              limit,
		HopDepth:           req.HopDepth,
		AnalyzeQuery:       req.AnalyzeQuery,
		EnableThinking:     req.EnableThinking,
		MaxThinkingSteps:   req.MaxThinkingSteps,
		LearnRelationships: req.LearnRelationships,
		IncludeReasoning:   req.IncludeReasoning,
		MaxContextLength:   req.MaxContextLength,
		SegmentContext:     req.SegmentContext,
		FourTier: req.FourTier,
	}
	if tq.MaxThinkingSteps <= 0 && tq.EnableThinking {
		tq.MaxThinkingSteps = 3
	}

	res, err := s.deps.Engine.Think(r.Context(), tq)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGraphStats(w http.ResponseWriter, r *http.Request) {
	if s.deps.Graph == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"available": false,
			"reason":    "graph store is not initialized",
		})
		return
	}

	ctx := r.Context()
	nodeCount, nodeErr := s.deps.Graph.GetNodeCount(ctx)
	edgeCount, edgeErr := s.deps.Graph.GetEdgeCount(ctx)
	components, compErr := s.deps.Graph.GetConnectedComponents(ctx)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"available":          true,
		"node_count":         nodeCount,
		"edge_count":         edgeCount,
		"connected_clusters": len(components),
		"errors": map[string]string{
			"node_count": nodeErrString(nodeErr),
			"edge_count": nodeErrString(edgeErr),
			"components": nodeErrString(compErr),
		},
	})
}

func (s *Server) handleGraphNeighbors(w http.ResponseWriter, r *http.Request) {
	if s.deps.Graph == nil {
		writeErr(w, http.StatusNotImplemented, fmt.Errorf("graph store is not initialized"))
		return
	}

	ctx := r.Context()
	nodeID := strings.TrimSpace(r.URL.Query().Get("node_id"))
	if nodeID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("query param node_id is required"))
		return
	}
	depth := boundInt(parseInt(r, "depth", 2), 1, 6)
	neighbors, err := s.deps.Graph.TraverseGraph(ctx, nodeID, depth, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":   nodeID,
		"depth":     depth,
		"neighbors": neighbors,
		"total":     len(neighbors),
	})
}

func (s *Server) handleGraphPath(w http.ResponseWriter, r *http.Request) {
	if s.deps.Graph == nil {
		writeErr(w, http.StatusNotImplemented, fmt.Errorf("graph store is not initialized"))
		return
	}

	ctx := r.Context()
	fromID := strings.TrimSpace(r.URL.Query().Get("from"))
	toID := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromID == "" || toID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("query params from and to are required"))
		return
	}

	maxDepth := boundInt(parseInt(r, "depth", 4), 1, 10)
	path, err := s.deps.Graph.FindPath(ctx, fromID, toID, maxDepth)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"from":      fromID,
		"to":        toID,
		"max_depth": maxDepth,
		"path":      path,
		"total":     len(path),
	})
}

func (s *Server) vectorPreview(ctx context.Context, id string) map[string]interface{} {
	if s.deps.VecStore == nil {
		return map[string]interface{}{"available": false}
	}
	embedding, metadata, err := s.deps.VecStore.GetEmbedding(ctx, id)
	if err != nil {
		return map[string]interface{}{"available": false, "error": err.Error()}
	}

	previewLen := 6
	if len(embedding) < previewLen {
		previewLen = len(embedding)
	}
	preview := make([]float32, previewLen)
	copy(preview, embedding[:previewLen])

	return map[string]interface{}{
		"available": true,
		"dim":       len(embedding),
		"preview":   preview,
		"metadata":  metadata,
	}
}

func toDataPointCards(points []*schema.DataPoint) []map[string]interface{} {
	cards := make([]map[string]interface{}, 0, len(points))
	for _, dp := range points {
		itemType := dataPointKind(dp)
		cards = append(cards, map[string]interface{}{
			"id":          dp.ID,
			"content":     dp.Content,
			"session_id":  dp.SessionID,
			"item_type":   itemType,
			"memory_tier": schema.MemoryTierFromDataPoint(dp),
			"status":      dp.ProcessingStatus,
			"updated_at":  dp.UpdatedAt,
		})
	}
	return cards
}

func isChunkDP(dp *schema.DataPoint) bool {
	if dp == nil || dp.Metadata == nil {
		return false
	}
	if asBool(dp.Metadata["is_chunk"]) {
		return true
	}
	// Fallback for older records that may miss explicit flags.
	_, hasParent := metadataString(dp.Metadata, "parent_id")
	if hasParent {
		return true
	}
	_, hasChunkIndex := metadataInt(dp.Metadata, "chunk_index")
	return hasChunkIndex
}

func metadataString(meta map[string]interface{}, key string) (string, bool) {
	if meta == nil {
		return "", false
	}
	v, ok := meta[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func metadataInt(meta map[string]interface{}, key string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	v, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func isInputDP(dp *schema.DataPoint) bool {
	if dp == nil {
		return false
	}
	if dp.Metadata == nil {
		// Backward compatible default: non-tagged records are treated as input-like.
		return true
	}
	if asBool(dp.Metadata["is_input"]) {
		return true
	}
	if isChunkDP(dp) {
		return false
	}
	// Fallback for old input rows without explicit is_input marker.
	return true
}

func dataPointKind(dp *schema.DataPoint) string {
	if isChunkDP(dp) {
		return "chunk"
	}
	return "input"
}

func asBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	case int:
		return t == 1
	case int64:
		return t == 1
	case float64:
		return int(t) == 1
	default:
		return false
	}
}

func filterDataPoints(points []*schema.DataPoint, kind string, status string) []*schema.DataPoint {
	out := make([]*schema.DataPoint, 0, len(points))
	for _, dp := range points {
		if dp == nil {
			continue
		}

		switch kind {
		case "input":
			if !isInputDP(dp) {
				continue
			}
		case "chunk":
			if !isChunkDP(dp) {
				continue
			}
		case "processed":
			// "processed" must be true chunk datapoints created after chunking.
			if !isChunkDP(dp) {
				continue
			}
		}

		if status != "" && status != "all" {
			if strings.ToLower(string(dp.ProcessingStatus)) != status {
				continue
			}
		}

		out = append(out, dp)
	}
	return out
}

func parseInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func boundInt(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nodeErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": errString(err)})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
