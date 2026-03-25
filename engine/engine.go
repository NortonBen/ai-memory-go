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

	engine := &defaultMemoryEngine{
		extractor:   ext,
		embedder:    emb,
		store:       store,
		graphStore:  graphStore,
		vectorStore: vectorStore,
		workerPool:  pool,
	}

	// Start background history analysis if enabled
	if cfg.EnableBackgroundAnalysis {
		interval := cfg.AnalysisInterval
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				// In a real implementation, we would iterate over active sessions.
				// For now, we'll focus on the 'default' session or use a global tracker.
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				_ = engine.AnalyzeHistory(ctx, "default")
				cancel()
			}
		}()
	}

	return engine
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

	return dp, nil
}

// Cognify process updates relationships and embeddings for an existing DataPoint synchronously.
func (e *defaultMemoryEngine) Cognify(ctx context.Context, dataPoint *schema.DataPoint, opts ...CognifyOption) (*schema.DataPoint, error) {
	options := &CognifyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	threshold := float32(0.0)
	if th, ok := dataPoint.Metadata["consistency_threshold"].(float64); ok {
		threshold = float32(th)
	} else if th, ok := dataPoint.Metadata["consistency_threshold"].(float32); ok {
		threshold = th
	}

	task := &CognifyTask{
		DataPoint:            dataPoint,
		ConsistencyThreshold: threshold,
	}

	if options.WaitUntilComplete {
		// Run synchronously and wait for the extraction & embedding to finish
		err := task.Execute(ctx, e.workerPool.extractor, e.workerPool.embedder, e.workerPool.store, e.workerPool.graphStore, e.workerPool.vectorStore, e.workerPool)
		return dataPoint, err
	}

	// Run asynchronously
	e.workerPool.Submit(task)
	return dataPoint, nil
}

// CognifyPending sweeps the relational store for items that have ProcessingStatus == StatusPending and processes them synchronously.
func (e *defaultMemoryEngine) CognifyPending(ctx context.Context, sessionID string) error {
	q := &storage.DataPointQuery{
		SessionID: sessionID,
		Limit:     1000,
	}
	dps, err := e.store.QueryDataPoints(ctx, q)
	if err != nil {
		return fmt.Errorf("failed to query data points: %w", err)
	}

	for _, dp := range dps {
		if dp.ProcessingStatus == schema.StatusPending {
			fmt.Printf("Cognifying pending data point: %s (Content: %.30s...)\n", dp.ID, dp.Content)
			_, err := e.Cognify(ctx, dp, WithWaitCognify(true))
			if err != nil {
				fmt.Printf("Failed to cognify data point %s: %v\n", dp.ID, err)
			}
		}
	}
	return nil
}

// Memify finalizes the memory integration (e.g. promoting concepts).
func (e *defaultMemoryEngine) Memify(ctx context.Context, dataPoint *schema.DataPoint, opts ...MemifyOption) error {
	options := &MemifyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	threshold := float32(0.0)
	if th, ok := dataPoint.Metadata["consistency_threshold"].(float64); ok {
		threshold = float32(th)
	} else if th, ok := dataPoint.Metadata["consistency_threshold"].(float32); ok {
		threshold = th
	}

	task := &MemifyTask{
		DataPoint:            dataPoint,
		ConsistencyThreshold: threshold,
	}

	if options.WaitUntilComplete {
		return task.Execute(ctx, e.workerPool.extractor, e.workerPool.embedder, e.workerPool.store, e.workerPool.graphStore, e.workerPool.vectorStore, e.workerPool)
	}

	e.workerPool.Submit(task)
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

// DeleteMemory deletes a memory by its specific ID.
// If id is empty but sessionID is provided, it deletes all memories in that session.
func (e *defaultMemoryEngine) DeleteMemory(ctx context.Context, id string, sessionID string) error {
	if id != "" {
		if sessionID != "" {
			// Ensure it belongs to the session
			dp, err := e.store.GetDataPoint(ctx, id)
			if err != nil {
				return err
			}
			if dp.SessionID != sessionID {
				return fmt.Errorf("datapoint %s does not belong to session %s", id, sessionID)
			}
		}
		
		if e.graphStore != nil {
			nodes, err := e.graphStore.FindNodesByProperty(ctx, "source_id", id)
			if err == nil {
				for _, n := range nodes {
					_ = e.graphStore.DeleteNode(ctx, n.ID)
				}
			}
		}
		
		return e.store.DeleteDataPoint(ctx, id)
	}

	if sessionID != "" {
		return e.store.DeleteDataPointsBySession(ctx, sessionID)
	}

	return fmt.Errorf("must provide either id or sessionID")
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

// Request processes a session chat, extracts relationships, answers queries, or deletes memory.
func (e *defaultMemoryEngine) Request(ctx context.Context, sessionID string, content string, opts ...RequestOption) (*schema.ThinkResult, error) {
	options := &RequestOptions{
		Metadata: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(options)
	}

	if e.extractor == nil {
		return nil, fmt.Errorf("extractor is required for Request")
	}

	// 1. Save user's message to history
	userMsg := schema.Message{
		ID:        uuid.New().String(),
		Role:      schema.RoleUser,
		Content:   content,
		Timestamp: time.Now(),
	}
	if e.store != nil && sessionID != "" {
		_ = e.store.AddMessageToSession(ctx, sessionID, userMsg)
	}

	// 2. Load recent history
	var historyContext string
	if e.store != nil && sessionID != "" {
		messages, err := e.store.GetSessionMessages(ctx, sessionID)
		if err == nil && len(messages) > 0 {
			var sb strings.Builder
			sb.WriteString("Chat History:\n")
			start := 0
			if len(messages) > 6 {
				start = len(messages) - 6 // Last 3 interactions (3 user, 3 assistant)
			}
			for _, m := range messages[start:] {
				sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
			}
			historyContext = sb.String()
		}
	}

	extractionInput := content
	if historyContext != "" {
		extractionInput = fmt.Sprintf("HISTORY:\n%s\n\nCURRENT USER MESSAGE:\n%s", historyContext, content)
	}

	// 3. Determine Intent
	intent, err := e.extractor.ExtractRequestIntent(ctx, extractionInput)
	if err != nil {
		return nil, fmt.Errorf("failed to extract intent: %w", err)
	}

	var finalAnswer string
	var finalReasoning []string

	// 4. Handle Deletion Intent
	if intent.IsDelete {
		for _, target := range intent.DeleteTargets {
			if e.graphStore != nil {
				nodes, _ := e.graphStore.FindNodesByEntity(ctx, target, "")
				for _, n := range nodes {
					_ = e.graphStore.DeleteNode(ctx, n.ID)
				}
			}
			if e.vectorStore != nil && e.embedder != nil {
				targetEmb, err := e.embedder.GenerateEmbedding(ctx, target)
				if err == nil {
					vecResults, _ := e.vectorStore.SimilaritySearch(ctx, targetEmb, 5, 0.80)
					for _, vr := range vecResults {
						_ = e.DeleteMemory(ctx, vr.ID, sessionID)
					}
				}
			}
		}
		finalReasoning = append(finalReasoning, "Executed deletion targets.")
	}

	// 5. Handle Statement/Fact Intent (Graph & Vector)
	// We always try to extract entities from the current message + context
	// to learn relationships from user feedback or statements
	if e.graphStore != nil {
		entities, err := e.extractor.ExtractEntities(ctx, extractionInput)
		if err == nil && len(entities) > 0 {
			for i := range entities {
				if err := entities[i].Validate(); err == nil {
					entities[i].SessionID = sessionID
					_ = e.graphStore.StoreNode(ctx, &entities[i])
				}
			}

			edges, err := e.extractor.ExtractRelationships(ctx, extractionInput, entities)
			if err == nil {
				for idx, edge := range edges {
					edge.SessionID = sessionID
					if edge.ID == "" {
						edge.ID = fmt.Sprintf("edge_%s_%d", sessionID, idx)
					}
					_ = e.graphStore.CreateRelationship(ctx, &edge)
				}
			}
			finalReasoning = append(finalReasoning, "Updated knowledge graph.")
		}
	}

	if intent.NeedsVectorStorage {
		addOpts := []AddOption{WithSessionID(sessionID), WithMetadata(options.Metadata)}
		dp, err := e.Add(ctx, content, addOpts...)
		if err == nil {
			_, _ = e.Cognify(ctx, dp, WithWaitCognify(false))
			finalReasoning = append(finalReasoning, "Added raw memory to vector store.")
		}
	}

	// 6. Handle Query / Generation
	var result *schema.ThinkResult
	if intent.IsQuery {
		// Default values for agentic thinking
		hopDepth := options.HopDepth
		if hopDepth <= 0 {
			hopDepth = 2
		}
		maxSteps := options.MaxThinkingSteps
		if maxSteps <= 0 {
			maxSteps = 3
		}

		thinkQuery := &schema.ThinkQuery{
			Text:               extractionInput,
			SessionID:          sessionID,
			Limit:              5,
			EnableThinking:     options.EnableThinking,
			MaxThinkingSteps:   maxSteps,
			HopDepth:           hopDepth,
			LearnRelationships: options.LearnRelationships,
			IncludeReasoning:   options.IncludeReasoning,
		}
		res, err := e.Think(ctx, thinkQuery)
		if err == nil && res != nil {
			result = res
			finalAnswer = res.Answer
			finalReasoning = append(finalReasoning, res.Reasoning)
		}
	}

	// If it wasn't a query but we did operations, provide a default response
	if finalAnswer == "" {
		if intent.IsDelete {
			finalAnswer = "I have forgotten the requested information."
		} else {
			finalAnswer = "I have memorized this information."
		}
	}

	if result == nil {
		result = &schema.ThinkResult{
			Answer:    finalAnswer,
			Reasoning: strings.Join(finalReasoning, "\n"),
		}
	}
	result.Intent = intent

	// 7. Save Assistant's Response to history
	asstMsg := schema.Message{
		ID:        uuid.New().String(),
		Role:      schema.RoleAssistant,
		Content:   finalAnswer,
		Timestamp: time.Now(),
	}
	if e.store != nil && sessionID != "" {
		_ = e.store.AddMessageToSession(ctx, sessionID, asstMsg)
	}

	return result, nil
}
// AnalyzeHistory processes recent chat history to extract deeper relationships or update existing ones
func (e *defaultMemoryEngine) AnalyzeHistory(ctx context.Context, sessionID string) error {
	if e.store == nil {
		return fmt.Errorf("relational store not available")
	}

	// 1. Fetch recent messages
	messages, err := e.store.GetSessionMessages(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to fetch session messages: %w", err)
	}

	if len(messages) < 2 {
		return nil // Not enough history to analyze
	}

	// 2. Format history for analysis
	var historyBuilder strings.Builder
	for i, msg := range messages {
		// Only take last 50 if history is very long
		if len(messages) > 50 && i < len(messages)-50 {
			continue
		}
		role := "User"
		if msg.Role == schema.RoleAssistant {
			role = "Assistant"
		}
		historyBuilder.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}
	history := historyBuilder.String()

	// 3. Use Extractor to find NEW entities and relationships
	nodes, err := e.extractor.ExtractEntities(ctx, history)
	if err != nil {
		return fmt.Errorf("failed to extract entities from history: %w", err)
	}

	edges, err := e.extractor.ExtractRelationships(ctx, history, nodes)
	if err != nil {
		return fmt.Errorf("failed to extract relationships from history: %w", err)
	}

	// 4. Save extracted knowledge to Graph store
	if e.graphStore != nil {
		// Save nodes first
		for _, node := range nodes {
			_ = e.graphStore.StoreNode(ctx, &node)
		}
		// Then save relationships
		for _, edge := range edges {
			err = e.graphStore.CreateRelationship(ctx, &edge)
			if err != nil {
				fmt.Printf("Warning: failed to add historical relation: %v\n", err)
			}
		}
	}

	return nil
}
