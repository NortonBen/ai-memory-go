package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

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

	want := effectiveSearchSessionID(query.SessionID)
	dps, err := e.store.QueryDataPoints(ctx, &storage.DataPointQuery{
		SearchText:           query.Text,
		Limit:                query.Limit,
		SessionID:            want,
		IncludeGlobalSession: true,
	})
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
	if e.useFourTierSearch(query) {
		return e.retrieveContextFourTier(ctx, query)
	}
	return e.retrieveContextLegacy(ctx, query)
}

func (e *defaultMemoryEngine) retrieveContextLegacy(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error) {
	// ---------------------------------------------------------
	// Step 1: Input Processing (Vectorize + Extract Entities) in parallel
	// ---------------------------------------------------------
	// Fetch recent history Context for better entity resolution
	historyContext := e.getHistoryBuffer(ctx, query.SessionID, 10)
	var extractionInput = query.Text
	if historyContext != "" {
		extractionInput = fmt.Sprintf("HISTORY:\n%s\n\nCURRENT USER MESSAGE:\n%s", historyContext, query.Text)
	}

	var emb []float32
	var extractedEntities []*schema.Node
	var embeddingErr error

	var step1WG sync.WaitGroup
	step1WG.Add(2)

	go func() {
		defer step1WG.Done()
		emb, embeddingErr = e.embedder.GenerateEmbedding(ctx, query.Text)
	}()

	go func() {
		defer step1WG.Done()
		if query.Analysis != nil && len(query.Analysis.Subjects) > 0 {
			for _, subject := range query.Analysis.Subjects {
				extractedEntities = append(extractedEntities, &schema.Node{
					ID:   subject,
					Type: schema.NodeTypeEntity,
					Properties: map[string]interface{}{
						"name": subject,
					},
				})
			}
			return
		}

		if e.extractor != nil {
			extractedNodes, err := e.extractor.ExtractEntities(ctx, extractionInput)
			if err == nil {
				for i := range extractedNodes {
					extractedEntities = append(extractedEntities, &extractedNodes[i])
				}
			}
		}
	}()

	step1WG.Wait()
	if embeddingErr != nil {
		return nil, fmt.Errorf("failed to generate embedding for query: %w", embeddingErr)
	}

	// ---------------------------------------------------------
	// Step 2: Multi-hop Retrieval (Vector + Graph) in parallel
	// ---------------------------------------------------------
	threshold := query.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.45
	}

	vectorScores := make(map[string]float64)
	vectorAnchorNodeIDs := make(map[string]bool)
	entityAnchorNodeIDs := make(map[string]bool)

	var step2WG sync.WaitGroup
	step2WG.Add(2)

	// Branch A: Vector search
	go func() {
		defer step2WG.Done()

		branchEmbedding := emb
		if query.Analysis != nil && len(query.Analysis.SearchKeywords) > 0 {
			searchText := strings.Join(query.Analysis.SearchKeywords, " ")
			fmt.Printf(" [DEBUG Search] Using refined keywords: %s\n", searchText)
			refinedEmb, err := e.embedder.GenerateEmbedding(ctx, searchText)
			if err == nil {
				branchEmbedding = refinedEmb
			}
		}

		vecResults, err := e.vectorStore.SimilaritySearch(ctx, branchEmbedding, query.Limit, threshold)
		if err != nil {
			return
		}

		for _, vr := range vecResults {
			sourceID := vectorResultSourceID(vr)

			if vr.Score > vectorScores[sourceID] {
				vectorScores[sourceID] = vr.Score
			}

			linkedNodes, err := e.graphStore.FindNodesByProperty(ctx, "source_id", sourceID)
			if err == nil {
				for _, ln := range linkedNodes {
					vectorAnchorNodeIDs[ln.ID] = true
				}
			}
		}
	}()

	// Branch B: Graph anchors from extracted entities
	go func() {
		defer step2WG.Done()

		for _, entity := range extractedEntities {
			name, _ := entity.Properties["name"].(string)
			if name == "" {
				name = entity.ID
			}
			nodes, err := e.graphStore.FindNodesByEntity(ctx, name, entity.Type)
			if err == nil {
				for _, n := range nodes {
					entityAnchorNodeIDs[n.ID] = true
				}
			} else {
				fmt.Printf(" [DEBUG Search] Error searching for '%s': %v\n", name, err)
			}
		}
	}()

	step2WG.Wait()

	return e.hybridRankFromVectorScores(ctx, query, extractedEntities, vectorScores, vectorAnchorNodeIDs, entityAnchorNodeIDs, nil)
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
		deleteVectorsForDataPoint(ctx, e.vectorStore, id)
		return e.store.DeleteDataPoint(ctx, id)
	}

	if sessionID != "" {
		return e.deleteMemoryWholeSession(ctx, sessionID)
	}

	return fmt.Errorf("must provide either id or sessionID")
}
