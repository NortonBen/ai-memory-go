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

// AddOptions holds optional parameters for the Add operation
type AddOptions struct {
	SessionID            string
	Metadata             map[string]interface{}
	WaitUntilComplete    bool
	ConsistencyThreshold float32
}

// AddOption configures AddOptions
type AddOption func(*AddOptions)

// WithMetadata adds custom metadata to the DataPoint
func WithMetadata(metadata map[string]interface{}) AddOption {
	return func(o *AddOptions) {
		if o.Metadata == nil {
			o.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			o.Metadata[k] = v
		}
	}
}

// WithWaitAdd configures whether Add should wait for background extraction to complete.
func WithWaitAdd(wait bool) AddOption {
	return func(o *AddOptions) {
		o.WaitUntilComplete = wait
	}
}

// WithConsistencyThreshold sets the vector distance threshold for invoking consistency reasoning (e.g. 0.1)
func WithConsistencyThreshold(threshold float32) AddOption {
	return func(o *AddOptions) {
		o.ConsistencyThreshold = threshold
	}
}

// WithSessionID explicitly maps a session identifier onto the DataPoint
func WithSessionID(sessionID string) AddOption {
	return func(o *AddOptions) {
		o.SessionID = sessionID
	}
}

// CognifyOptions holds optional parameters for the Cognify operation
type CognifyOptions struct {
	WaitUntilComplete bool
}

// CognifyOption configures CognifyOptions
type CognifyOption func(*CognifyOptions)

// WithWaitCognify causes the process to wait for completion before returning.
func WithWaitCognify(wait bool) CognifyOption {
	return func(o *CognifyOptions) {
		o.WaitUntilComplete = wait
	}
}

// MemifyOptions holds optional parameters for the Memify operation
type MemifyOptions struct {
	WaitUntilComplete bool
}

// MemifyOption configures MemifyOptions
type MemifyOption func(*MemifyOptions)

// WithWaitMemify allows Memify to wait for any background completions (though currently synchronous).
func WithWaitMemify(wait bool) MemifyOption {
	return func(o *MemifyOptions) {
		o.WaitUntilComplete = wait
	}
}

// MemoryEngine defines the core memory operations
type MemoryEngine interface {
	// Core pipeline operations
	Add(ctx context.Context, content string, opts ...AddOption) (*schema.DataPoint, error)
	Cognify(ctx context.Context, dataPoint *schema.DataPoint, opts ...CognifyOption) (*schema.DataPoint, error)
	Memify(ctx context.Context, dataPoint *schema.DataPoint, opts ...MemifyOption) error
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
func (e *defaultMemoryEngine) Add(ctx context.Context, content string, opts ...AddOption) (*schema.DataPoint, error) {
	options := &AddOptions{
		Metadata: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(options)
	}

	sessionID := options.SessionID
	if sessionID == "" {
		// Fallback check if someone used sessionID or session_id in metadata instead
		if sid, ok := options.Metadata["session_id"].(string); ok && sid != "" {
			sessionID = sid
			delete(options.Metadata, "session_id")
		} else if sid, ok := options.Metadata["sessionID"].(string); ok && sid != "" {
			sessionID = sid
			delete(options.Metadata, "sessionID")
		} else {
			sessionID = "default"
		}
	}

	// Deduplication: Check if this exact content already exists for this session
	searchQuery := &storage.DataPointQuery{
		SearchText: content,
		SearchMode: "exact",
		SessionID:  sessionID,
		Limit:      1,
	}
	if existingMatches, err := e.store.QueryDataPoints(ctx, searchQuery); err == nil && len(existingMatches) > 0 {
		return existingMatches[0], nil
	}

	dp := &schema.DataPoint{
		ID:               uuid.New().String(),
		Content:          content,
		ContentType:      "text",
		SessionID:        sessionID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: schema.StatusPending,
		Metadata:         options.Metadata,
	}

	// Persist the initial pending DataPoint
	err := e.store.StoreDataPoint(ctx, dp)
	if err != nil {
		return nil, fmt.Errorf("failed to store initial DataPoint: %w", err)
	}

	task := &AddTask{
		DataPoint:            dp,
		ConsistencyThreshold: options.ConsistencyThreshold,
	}

	if options.WaitUntilComplete {
		// Wait for execution to finish synchronously
		err = task.Execute(ctx, e.workerPool.extractor, e.workerPool.embedder, e.workerPool.store, e.workerPool.graphStore, e.workerPool.vectorStore)
		return dp, err
	}

	// Queue for processing
	e.workerPool.Submit(task)

	return dp, nil
}

// Cognify process updates relationships and embeddings for an existing DataPoint synchronously.
func (e *defaultMemoryEngine) Cognify(ctx context.Context, dataPoint *schema.DataPoint, opts ...CognifyOption) (*schema.DataPoint, error) {
	options := &CognifyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	task := &CognifyTask{
		DataPoint: dataPoint,
	}

	if options.WaitUntilComplete {
		// Run synchronously and wait for the extraction & embedding to finish
		err := task.Execute(ctx, e.workerPool.extractor, e.workerPool.embedder, e.workerPool.store, e.workerPool.graphStore, e.workerPool.vectorStore)
		return dataPoint, err
	}

	// Run asynchronously
	e.workerPool.Submit(task)
	return dataPoint, nil
}

// Memify finalizes the memory integration (e.g. promoting concepts).
func (e *defaultMemoryEngine) Memify(ctx context.Context, dataPoint *schema.DataPoint, opts ...MemifyOption) error {
	options := &MemifyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	dataPoint.ProcessingStatus = schema.StatusCompleted
	dataPoint.UpdatedAt = time.Now()

	if e.store != nil {
		if err := e.store.StoreDataPoint(ctx, dataPoint); err != nil {
			return fmt.Errorf("failed to persist Memify status: %w", err)
		}
	}

	// Currently Memify completes instantly. The WaitUntilComplete flag is preserved
	// for future background integrations if needed.

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

func (e *defaultMemoryEngine) retrieveContext(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error) {
	// ---------------------------------------------------------
	// Step 1: Input Processing (Vectorize + Extract Entities)
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
	// Step 2: Hybrid Search (Vector + Graph Anchors)
	// ---------------------------------------------------------
	anchorNodeIDs := make(map[string]bool)
	
	// 2a. Vector Search
	threshold := query.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.45
	}
	vecResults, err := e.vectorStore.SimilaritySearch(ctx, emb, query.Limit, threshold)
	if err == nil {
		for _, vr := range vecResults {
			// vr.ID could be a DataPoint ID, or "entity-xxxx"
			sourceID := vr.ID
			if isEntity, ok := vr.Metadata["is_entity"].(bool); ok && isEntity {
				if sid, ok := vr.Metadata["source_id"].(string); ok {
					sourceID = sid
				}
			}

			if item := trackDataPoint(sourceID); item != nil {
				// If multiple vectors match the same datapoint, track the best score
				if vr.Score > item.vectorScore {
					item.vectorScore = vr.Score
				}
				
				// Treat the Graph Nodes linked to this DataPoint as Graph Anchors!
				linkedNodes, err := e.graphStore.FindNodesByProperty(ctx, "source_id", sourceID)
				if err == nil {
					for _, ln := range linkedNodes {
						anchorNodeIDs[ln.ID] = true
					}
				}
			}
		}
	} else {
		fmt.Printf(" [DEBUG] SimilaritySearch failed: %v\n", err)
	}

	// 2b. Graph Anchor Search (from Extracted Entities)
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

	return results, nil
}

// Search implements the 4-step Cognee-style Hybrid Search pipeline, terminating with a direct string answer.
func (e *defaultMemoryEngine) Search(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error) {
	if e.graphStore == nil || e.vectorStore == nil {
		return e.basicSearch(ctx, query)
	}

	results, err := e.retrieveContext(ctx, query)
	if err != nil {
		return nil, err
	}

	// ---------------------------------------------------------
	// Step 4c: LLM Answer Generation (Plain text)
	// ---------------------------------------------------------
	if len(results.Results) > 0 && e.extractor != nil && e.extractor.GetProvider() != nil {
		prompt := fmt.Sprintf("You are an intelligent memory assistant. Use the following context to answer the user's query.\n\nContext:\n%s\n\nUser Query: %s\n\nAnswer:", results.ParsedContext, query.Text)
		answer, err := e.extractor.GetProvider().GenerateCompletion(ctx, prompt)
		if err == nil {
			results.Answer = answer
		} else {
			results.Answer = fmt.Sprintf("Error generating answer: %v", err)
		}
	} else if len(results.Results) == 0 {
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
	if e.extractor == nil || e.extractor.GetProvider() == nil {
		return nil, fmt.Errorf("extractor (LLM) is required for Think")
	}

	provider := e.extractor.GetProvider()

	// 1. Initial retrieval sequence
	searchQuery := &schema.SearchQuery{
		Text:      query.Text,
		SessionID: query.SessionID,
		Limit:     query.Limit,
		HopDepth:  query.HopDepth,
	}
	results, err := e.retrieveContext(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	// 2. Dispatch to single-shot or iterative logic
	if !query.EnableThinking {
		return e.singleShotThink(ctx, provider, query, results)
	}

	return e.iterativeThink(ctx, provider, query, results)
}

func (e *defaultMemoryEngine) singleShotThink(ctx context.Context, provider extractor.LLMProvider, query *schema.ThinkQuery, results *schema.SearchResults) (*schema.ThinkResult, error) {
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

	responseStr, err := provider.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm completion failed: %w", err)
	}

	responseStr = strings.TrimPrefix(responseStr, "```json")
	responseStr = strings.TrimPrefix(responseStr, "```")
	responseStr = strings.TrimSuffix(responseStr, "```")
	responseStr = strings.TrimSpace(responseStr)

	var result schema.ThinkResult
	err = json.Unmarshal([]byte(responseStr), &result)
	if err != nil {
		result = schema.ThinkResult{Answer: responseStr}
		if query.IncludeReasoning {
			result.Reasoning = "Failed to parse JSON reasoning."
		}
	}
	result.ContextUsed = results

	return &result, nil
}

func (e *defaultMemoryEngine) iterativeThink(ctx context.Context, provider extractor.LLMProvider, query *schema.ThinkQuery, initialResults *schema.SearchResults) (*schema.ThinkResult, error) {
	maxSteps := query.MaxThinkingSteps
	if maxSteps <= 0 {
		maxSteps = 3
	}

	currentContext := initialResults.ParsedContext
	var lastResult schema.ThinkResult

	var jsonFormatRequirement string
	if query.IncludeReasoning {
		jsonFormatRequirement = `{
  "reasoning": "your step by step thought process based on the context",
  "missing_entities": ["entity1", "entity2"],
  "answer": "your final brief answer to the user (leave empty string if you need more information)"
}`
	} else {
		jsonFormatRequirement = `{
  "missing_entities": ["entity1", "entity2"],
  "answer": "your final brief answer to the user (leave empty string if you need more information)"
}`
	}

	for step := 1; step <= maxSteps; step++ {
		prompt := fmt.Sprintf(`You are an AI assistant powered by a Memory Engine.
Use the following retrieved context to answer the user's question accurately.
If the answer is missing from the context, identify the exact entities (names of people, organizations, concepts) that you need more information about to answer the question, and place them in 'missing_entities'.

You MUST ALWAYS respond in clean JSON format matching the schema below. 
You MUST ALWAYS include the 'reasoning' field to explain your thought process.
Note: IF you provide an answer, 'missing_entities' MUST be empty. IF you provide missing_entities, 'answer' MUST be empty.

%s

Example valid response:
{
  "reasoning": "I have found that X is Y, but I still need to know Z to answer.",
  "missing_entities": ["Z"],
  "answer": ""
}

Context:
%s

Question: %s
JSON Response:`, jsonFormatRequirement, currentContext, query.Text)

		responseStr, err := provider.GenerateCompletion(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("llm completion failed at step %d: %w", step, err)
		}

		responseStr = strings.TrimPrefix(responseStr, "```json")
		responseStr = strings.TrimPrefix(responseStr, "```")
		responseStr = strings.TrimSuffix(responseStr, "```")
		responseStr = strings.TrimSpace(responseStr)

		var result struct {
			Reasoning       string   `json:"reasoning"`
			MissingEntities []string `json:"missing_entities"`
			Answer          string   `json:"answer"`
		}

		err = json.Unmarshal([]byte(responseStr), &result)
		if err != nil {
			lastResult = schema.ThinkResult{Reasoning: "Failed to parse JSON: " + err.Error(), Answer: responseStr}
			break
		}

		if len(result.MissingEntities) == 0 && result.Answer != "" {
			// Found the answer!
			lastResult = schema.ThinkResult{
				Reasoning: result.Reasoning,
				Answer:    result.Answer,
			}

			// Learn Relationships
			if query.LearnRelationships && e.graphStore != nil {
				edge, err := e.extractor.ExtractBridgingRelationship(ctx, query.Text, result.Answer)
				if err == nil && edge != nil {
					// We might need to resolve IDs here if possible, but for demonstration:
					// Just storing Name -> Name with 'is_bridging' to help subsequent semantic graph queries.
					_ = e.graphStore.CreateRelationship(ctx, edge)
				}
			}
			break
		}

		// Keep thinking! We have missing entities.
		if step < maxSteps && e.graphStore != nil {
			var additionalContext strings.Builder
			additionalContext.WriteString(fmt.Sprintf("\n--- ADDITIONAL CONTEXT FROM HOP %d ---\n", step))

			for _, entityName := range result.MissingEntities {
				// 1. Graph search for exact/partial entities
				nodes, _ := e.graphStore.FindNodesByEntity(ctx, entityName, "")
				for _, n := range nodes {
					// Format Node
					props, _ := json.Marshal(n.Properties)
					additionalContext.WriteString(fmt.Sprintf("- Node: %s (Type: %s, Props: %s)\n", n.ID, n.Type, string(props)))
					
					// Find Connected Nodes iteratively
					connectedNodes, _ := e.graphStore.FindConnected(ctx, n.ID, nil)
					for _, cn := range connectedNodes {
						cnProps, _ := json.Marshal(cn.Properties)
						additionalContext.WriteString(fmt.Sprintf("  -> Connected: %s (Type: %s, Props: %s)\n", cn.ID, cn.Type, string(cnProps)))
					}
				}

				// 2. Vector search for the missing entity concept
				if e.embedder != nil && e.vectorStore != nil {
					emb, err := e.embedder.GenerateEmbedding(ctx, entityName)
					if err == nil {
						vecResults, _ := e.vectorStore.SimilaritySearch(ctx, emb, 2, 0.45)
						for _, vr := range vecResults {
							if dp, err := e.store.GetDataPoint(ctx, vr.ID); err == nil && dp != nil {
								additionalContext.WriteString(fmt.Sprintf("- Memory: %s\n", dp.Content))
							}
						}
					}
				}
			}

			currentContext += additionalContext.String()

			lastResult = schema.ThinkResult{
				Reasoning: result.Reasoning + "\n[Engine: Needed more context, retrieving...]",
				Answer:    "Mảnh ký ức này chưa tồn tại trong hệ thống (Max hops reached).",
			}
		} else {
			lastResult = schema.ThinkResult{
				Reasoning: result.Reasoning,
				Answer:    "Mảnh ký ức này chưa tồn tại trong hệ thống.",
			}
		}
	}

	lastResult.ContextUsed = initialResults
	return &lastResult, nil
}

