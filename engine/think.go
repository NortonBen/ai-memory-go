package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/schema"
)

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

	responseStr = e.cleanJSONResponse(responseStr)

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

		responseStr = e.cleanJSONResponse(responseStr)

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
			lastResult = schema.ThinkResult{
				Reasoning: result.Reasoning,
				Answer:    result.Answer,
			}

			if query.LearnRelationships && e.graphStore != nil {
				edge, err := e.extractor.ExtractBridgingRelationship(ctx, query.Text, result.Answer)
				if err == nil && edge != nil {
					_ = e.graphStore.CreateRelationship(ctx, edge)
				}
			}
			break
		}

		if step < maxSteps && e.graphStore != nil {
			var additionalContext strings.Builder
			additionalContext.WriteString(fmt.Sprintf("\n--- ADDITIONAL CONTEXT FROM HOP %d ---\n", step))

			for _, entityName := range result.MissingEntities {
				nodes, _ := e.graphStore.FindNodesByEntity(ctx, entityName, "")
				for _, n := range nodes {
					props, _ := json.Marshal(n.Properties)
					additionalContext.WriteString(fmt.Sprintf("- Node: %s (Type: %s, Props: %s)\n", n.ID, n.Type, string(props)))
					
					connectedNodes, _ := e.graphStore.FindConnected(ctx, n.ID, nil)
					for _, cn := range connectedNodes {
						cnProps, _ := json.Marshal(cn.Properties)
						additionalContext.WriteString(fmt.Sprintf("  -> Connected: %s (Type: %s, Props: %s)\n", cn.ID, cn.Type, string(cnProps)))
					}
				}

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
