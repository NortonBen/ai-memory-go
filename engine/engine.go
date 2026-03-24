package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/google/uuid"
)

// EngineConfig holds configuration for MemoryEngine
type EngineConfig struct {
	MaxWorkers int
}

// MemoryEngine defines the core memory operations
type MemoryEngine interface {
	// Core pipeline operations
	Add(ctx context.Context, content string, metadata map[string]interface{}) (*schema.DataPoint, error)
	Cognify(ctx context.Context, dataPoint *schema.DataPoint) (*schema.DataPoint, error)
	Memify(ctx context.Context, dataPoint *schema.DataPoint) error
	Search(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error)
	Think(ctx context.Context, query *schema.ThinkQuery) (*schema.ThinkResult, error)

	// Session management
	CreateSession(ctx context.Context, userID string, context map[string]interface{}) (*schema.MemorySession, error)
	GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error)
	CloseSession(ctx context.Context, sessionID string) error

	// Health and lifecycle
	Health(ctx context.Context) error
	Close() error
}

// defaultMemoryEngine is the implementation for orchestrating Add, Cognify, and Search operations.
type defaultMemoryEngine struct {
	extractor   extractor.LLMExtractor
	embedder    vector.EmbeddingProvider
	store       storage.Storage
	graphStore  graph.GraphStore
	vectorStore vector.VectorStore
	workerPool  *WorkerPool
}

// NewMemoryEngine creates a new instance of MemoryEngine using only the relational store (fallback).
func NewMemoryEngine(ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, cfg EngineConfig) MemoryEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5
	}

	pool := NewWorkerPool(cfg.MaxWorkers, ext, emb, store, nil, nil)
	pool.Start()

	return &defaultMemoryEngine{
		extractor:  ext,
		embedder:   emb,
		store:      store,
		workerPool: pool,
	}
}

// NewMemoryEngineWithStores creates a new instance of MemoryEngine including graph and vector stores for advanced features.
func NewMemoryEngineWithStores(ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore, cfg EngineConfig) MemoryEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 5
	}

	pool := NewWorkerPool(cfg.MaxWorkers, ext, emb, store, graphStore, vectorStore)
	pool.Start()

	return &defaultMemoryEngine{
		extractor:   ext,
		embedder:    emb,
		store:       store,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		workerPool:  pool,
	}
}

// Add persists the initial DataPoint and optionally starts asynchronous processing.
func (e *defaultMemoryEngine) Add(ctx context.Context, content string, metadata map[string]interface{}) (*schema.DataPoint, error) {
	sessionID, _ := metadata["session_id"].(string)
	if sessionID == "" {
		sessionID = "default"
	}

	dp := &schema.DataPoint{
		ID:               uuid.New().String(),
		Content:          content,
		ContentType:      "text",
		SessionID:        sessionID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: schema.StatusPending,
		Metadata:         metadata,
	}

	// Persist the initial pending DataPoint
	err := e.store.StoreDataPoint(ctx, dp)
	if err != nil {
		return nil, fmt.Errorf("failed to store initial DataPoint: %w", err)
	}

	// Queue for processing
	e.workerPool.Submit(&AddTask{
		DataPoint: dp,
	})

	return dp, nil
}

// Cognify process updates relationships and embeddings for an existing DataPoint synchronously.
func (e *defaultMemoryEngine) Cognify(ctx context.Context, dataPoint *schema.DataPoint) (*schema.DataPoint, error) {
	// For now, reuse the async worker pool submission or do it synchronously
	e.workerPool.Submit(&CognifyTask{
		DataPoint: dataPoint,
	})
	return dataPoint, nil
}

// Memify finalizes the memory integration (e.g. promoting concepts).
func (e *defaultMemoryEngine) Memify(ctx context.Context, dataPoint *schema.DataPoint) error {
	// Add logic to finalize memory integration
	return nil
}

// basicSearch is the fallback when graphStore or vectorStore are not provided.
func (e *defaultMemoryEngine) basicSearch(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error) {
	emb := []float32{}
	if query.Text != "" && query.Mode != schema.ModeGraphTraversal {
		var err error
		emb, err = e.embedder.GenerateEmbedding(ctx, query.Text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
		}
	}

	storageQuery := &storage.DataPointQuery{
		SearchText: query.Text,
		Limit:      query.Limit,
	}

	dps, err := e.store.QueryDataPoints(ctx, storageQuery)
	if err != nil {
		return nil, err
	}

	results := &schema.SearchResults{
		Results: make([]*schema.SearchResult, 0, len(dps)),
		Total:   len(dps),
	}

	for _, dp := range dps {
		score := 0.0
		if len(emb) > 0 && len(dp.Embedding) > 0 {
			score = 1.0 // Placeholder
		}
		results.Results = append(results.Results, schema.NewSearchResult(dp, score, query.Mode))
	}
	return results, nil
}

// Search implements the 4-step Cognee-style Hybrid Search pipeline
// Step 1: Input Processing (Vectorize + Extract Entities)
// Step 2: Hybrid Search (Vector + Graph Anchors)
// Step 3: Graph Traversal (1-hop/2-hop discovery)
// Step 4: Context Assembly & Reranking
func (e *defaultMemoryEngine) Search(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error) {
	if e.graphStore == nil || e.vectorStore == nil {
		return e.basicSearch(ctx, query)
	}

	// ---------------------------------------------------------
	// Step 1: Input Processing
	// ---------------------------------------------------------
	emb, err := e.embedder.GenerateEmbedding(ctx, query.Text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
	}

	var extractedEntities []*schema.Node
	if e.extractor != nil {
		extractedNodes, err := e.extractor.ExtractEntities(ctx, query.Text)
		if err == nil {
			for i := range extractedNodes {
				extractedEntities = append(extractedEntities, &extractedNodes[i])
			}
		}
	}

	// Track scores per DataPoint ID
	type itemScore struct {
		dp          *schema.DataPoint
		vectorScore float64
		graphScore  float64
	}
	scores := make(map[string]*itemScore)

	// Helper to track datapoints
	trackDataPoint := func(id string) *itemScore {
		if _, exists := scores[id]; !exists {
			// Fetch DP from relational store
			dp, err := e.store.GetDataPoint(ctx, id)
			if err == nil && dp != nil {
				scores[id] = &itemScore{dp: dp}
			} else {
				fmt.Printf(" [DEBUG] trackDataPoint failed for id=%s: err=%v, dp=%v\n", id, err, dp)
			}
		}
		return scores[id]
	}

	// ---------------------------------------------------------
	// Step 2: Hybrid Search (Vector + Graph)
	// ---------------------------------------------------------
	
	// 2a. Vector Search
	threshold := query.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.45
	}
	vecResults, err := e.vectorStore.SimilaritySearch(ctx, emb, query.Limit, threshold)
	if err == nil {
		for _, vr := range vecResults {
			if item := trackDataPoint(vr.ID); item != nil {
				item.vectorScore = vr.Score
			}
		}
	} else {
		fmt.Printf(" [DEBUG] SimilaritySearch failed: %v\n", err)
	}

	// 2b. Graph Anchor Search
	// Map extracted entities to graph nodes
	anchorNodeIDs := make(map[string]bool)
	for _, entity := range extractedEntities {
		name, _ := entity.Properties["name"].(string)
		if name == "" {
			name = entity.ID
		}
		nodes, err := e.graphStore.FindNodesByEntity(ctx, name, entity.Type)
		if err == nil {
			for _, n := range nodes {
				anchorNodeIDs[n.ID] = true
			}
		}
	}

	// ---------------------------------------------------------
	// Step 3: Graph Traversal
	// ---------------------------------------------------------
	hopDepth := query.HopDepth
	if hopDepth <= 0 {
		hopDepth = 2 // default to 2
	}

	graphNodesContext := make(map[string]*schema.Node)

	for nodeID := range anchorNodeIDs {
		// Traverse up to HopDepth
		neighbors, err := e.graphStore.TraverseGraph(ctx, nodeID, hopDepth, nil)
		if err == nil {
			for _, neighbor := range neighbors {
				// Collect the explicitly traversed nodes
				graphNodesContext[neighbor.ID] = neighbor

				// Enhance graph score if related via graph
				if sourceID, ok := neighbor.Properties["source_id"].(string); ok {
					if item := trackDataPoint(sourceID); item != nil {
						item.graphScore += 0.5
					}
				}
			}
		}
	}

	// ---------------------------------------------------------
	// Step 4: Context Assembly & Reranking
	// ---------------------------------------------------------
	var rankedItems []*itemScore
	for _, item := range scores {
		rankedItems = append(rankedItems, item)
	}

	// Apply scoring weights: 0.40 Vector + 0.30 Graph + 0.20 Temporal + 0.10 User (Simplified here)
	for _, item := range rankedItems {
		temporalScore := 0.0
		// Newer data points get slight temporal boost (e.g. within last 7 days)
		if time.Since(item.dp.CreatedAt).Hours() < 24*7 {
			temporalScore = 1.0
		}
		
		finalScore := (item.vectorScore * 0.40) + (item.graphScore * 0.30) + (temporalScore * 0.20)
		item.vectorScore = finalScore // hijack vectorScore to store final score
	}

	// Sort descending
	sort.Slice(rankedItems, func(i, j int) bool {
		return rankedItems[i].vectorScore > rankedItems[j].vectorScore
	})

	// Limit results
	if query.Limit > 0 && len(rankedItems) > query.Limit {
		rankedItems = rankedItems[:query.Limit]
	}

	results := &schema.SearchResults{
		Results: make([]*schema.SearchResult, 0, len(rankedItems)),
		Total:   len(scores),
	}

	for _, item := range rankedItems {
		results.Results = append(results.Results, schema.NewSearchResult(item.dp, item.vectorScore, query.Mode))
	}

	// ---------------------------------------------------------
	// Step 4b: Context Assembly into a single string
	// ---------------------------------------------------------
	var contextBuilder strings.Builder
	contextBuilder.WriteString("--- LIBRARIES FROM VECTOR SEARCH ---\n")
	for i, item := range rankedItems {
		contextBuilder.WriteString(fmt.Sprintf("[%d] (Score: %.2f):\n%s\n", i+1, item.vectorScore, item.dp.Content))
	}

	if len(graphNodesContext) > 0 {
		contextBuilder.WriteString("\n--- KNOWLEDGE GRAPH (ENTITIES & RELATIONSHIPS) ---\n")
		i := 1
		for _, n := range graphNodesContext {
			props, _ := json.Marshal(n.Properties)
			contextBuilder.WriteString(fmt.Sprintf("- [NodeType: %s] %s: %s\n", n.Type, n.ID, string(props)))
			i++
			if i > 20 {
				break // limit context length
			}
		}
	}
	results.ParsedContext = contextBuilder.String()

	// ---------------------------------------------------------
	// Step 4c: LLM Answer Generation
	// ---------------------------------------------------------
	if len(rankedItems) > 0 && e.extractor != nil && e.extractor.GetProvider() != nil {
		prompt := fmt.Sprintf("You are an intelligent memory assistant. Use the following context to answer the user's query.\n\nContext:\n%s\n\nUser Query: %s\n\nAnswer:", results.ParsedContext, query.Text)
		answer, err := e.extractor.GetProvider().GenerateCompletion(ctx, prompt)
		if err == nil {
			results.Answer = answer
		} else {
			results.Answer = fmt.Sprintf("Error generating answer: %v", err)
		}
	} else if len(rankedItems) == 0 {
		results.Answer = "I couldn't find any relevant memory context to answer your question."
	}

	return results, nil
}

// CreateSession creates a new memory session.
func (e *defaultMemoryEngine) CreateSession(ctx context.Context, userID string, contextData map[string]interface{}) (*schema.MemorySession, error) {
	session := &schema.MemorySession{
		ID:         uuid.New().String(),
		UserID:     userID,
		Context:    contextData,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		IsActive:   true,
	}
	return session, nil
}

// GetSession retrieves an active memory session.
func (e *defaultMemoryEngine) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) {
	return nil, fmt.Errorf("not implemented")
}

// CloseSession ends a memory session.
func (e *defaultMemoryEngine) CloseSession(ctx context.Context, sessionID string) error {
	return nil
}

// Health checks the status of the memory engine.
func (e *defaultMemoryEngine) Health(ctx context.Context) error {
	return nil
}

// Close gracefully shuts down the engine and its worker pool.
func (e *defaultMemoryEngine) Close() error {
	e.workerPool.Stop()
	return nil
}

// Think performs a Hybrid Search and explicitly traverses the knowledge graph to generate a reasoned answer.
func (e *defaultMemoryEngine) Think(ctx context.Context, query *schema.ThinkQuery) (*schema.ThinkResult, error) {
	if e.extractor == nil {
		return nil, fmt.Errorf("extractor (LLM) is required for Think")
	}

	provider := e.extractor.GetProvider()
	if provider == nil {
		return nil, fmt.Errorf("LLM provider is required for Think")
	}

	// 1. Perform Hybrid Search to get vector-ranked DataPoints and traversed Graph Nodes
	searchQuery := &schema.SearchQuery{
		Text:      query.Text,
		SessionID: query.SessionID,
		Limit:     query.Limit,
		HopDepth:  query.HopDepth,
	}
	results, err := e.Search(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	// 2. Construct Prompt using the comprehensive ParsedContext from Search()
	var jsonFormatRequirement string
	if query.IncludeReasoning {
		jsonFormatRequirement = `{
  "reasoning": "your step by step thought process based on the context",
  "answer": "your final brief answer to the user"
}`
	} else {
		jsonFormatRequirement = `{
  "answer": "your final brief answer to the user"
}`
	}

	prompt := fmt.Sprintf(`You are an AI assistant powered by a Memory Engine.
Use the following retrieved context (Vector Memories and Knowledge Graph relationships) to answer the user's question accurately.
If the answer is not contained in the context, say "Mảnh ký ức này chưa tồn tại trong hệ thống." or answer based on available knowledge, but explicitly state what is missing.

You MUST respond in clean JSON format matching this schema:
%s

Context:
%s

Question: %s
JSON Response:`, jsonFormatRequirement, results.ParsedContext, query.Text)

	// 3. Generate Answer
	responseStr, err := provider.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm completion failed: %w", err)
	}

	// 4. Parse Response
	responseStr = strings.TrimPrefix(responseStr, "```json")
	responseStr = strings.TrimPrefix(responseStr, "```")
	responseStr = strings.TrimSuffix(responseStr, "```")
	responseStr = strings.TrimSpace(responseStr)

	var result schema.ThinkResult
	err = json.Unmarshal([]byte(responseStr), &result)
	if err != nil {
		// Fallback if parsing fails
		result = schema.ThinkResult{
			Answer: responseStr,
		}
		if query.IncludeReasoning {
			result.Reasoning = "Failed to parse JSON reasoning."
		}
	}
	
	result.ContextUsed = results

	return &result, nil
}

