package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
)

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

	// Fetch recent history Context for better entity resolution
	historyContext := e.getHistoryBuffer(ctx, query.SessionID, 10)
	var extractionInput = query.Text
	if historyContext != "" {
		extractionInput = fmt.Sprintf("HISTORY:\n%s\n\nCURRENT USER MESSAGE:\n%s", historyContext, query.Text)
	}

	var extractedEntities []*schema.Node
	if e.extractor != nil {
		extractedNodes, err := e.extractor.ExtractEntities(ctx, extractionInput)
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
			sourceID := vr.ID
			if isEntity, ok := vr.Metadata["is_entity"].(bool); ok && isEntity {
				if sid, ok := vr.Metadata["source_id"].(string); ok {
					sourceID = sid
				}
			}

			if item := trackDataPoint(sourceID); item != nil {
				if vr.Score > item.vectorScore {
					item.vectorScore = vr.Score
				}
				
				linkedNodes, err := e.graphStore.FindNodesByProperty(ctx, "source_id", sourceID)
				if err == nil {
					for _, ln := range linkedNodes {
						anchorNodeIDs[ln.ID] = true
					}
				}
			}
		}
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
		} else {
			fmt.Printf(" [DEBUG Search] Error searching for '%s': %v\n", name, err)
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
		neighbors, err := e.graphStore.TraverseGraph(ctx, nodeID, hopDepth, nil)
		if err == nil {
			for _, neighbor := range neighbors {
				graphNodesContext[neighbor.ID] = neighbor

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

	for _, item := range rankedItems {
		temporalScore := 0.0
		if time.Since(item.dp.CreatedAt).Hours() < 24*7 {
			temporalScore = 1.0
		}
		
		finalScore := (item.vectorScore * 0.40) + (item.graphScore * 0.30) + (temporalScore * 0.20)
		item.vectorScore = finalScore // hijack vectorScore to store final score
	}

	sort.Slice(rankedItems, func(i, j int) bool {
		return rankedItems[i].vectorScore > rankedItems[j].vectorScore
	})

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
				break
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
	if e.extractor != nil && e.extractor.GetProvider() != nil {
		// Enhancement: Prepend recent chat history to ensure context awareness for pronouns
		historyContext := e.getHistoryBuffer(ctx, query.SessionID, 10)
		var finalPromptBuilder strings.Builder
		if historyContext != "" {
			finalPromptBuilder.WriteString("Recent Conversation History:\n")
			finalPromptBuilder.WriteString(historyContext)
			finalPromptBuilder.WriteString("\n")
		}

		finalPromptBuilder.WriteString("Use the following context to answer the user's query.\n\nContext:\n")
		finalPromptBuilder.WriteString(results.ParsedContext)
		finalPromptBuilder.WriteString(fmt.Sprintf("\n\nUser Query: %s\n\nAnswer:", query.Text))

		answer, err := e.extractor.GetProvider().GenerateCompletion(ctx, finalPromptBuilder.String())
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
func (e *defaultMemoryEngine) DeleteMemory(ctx context.Context, id string, sessionID string) error {
	if id != "" {
		if sessionID != "" {
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
