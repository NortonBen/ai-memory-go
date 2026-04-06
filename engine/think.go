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

	analysis := query.Analysis
	if query.AnalyzeQuery && analysis == nil {
		fmt.Printf(" [DEBUG Pre-Think] Analyzing query: %s\n", query.Text)
		var err error
		analysis, err = e.extractor.AnalyzeQuery(ctx, query.Text)
		if err != nil {
			fmt.Printf(" [DEBUG Pre-Think] Analysis failed: %v\n", err)
		} else {
			fmt.Printf(" [DEBUG Pre-Think] Type: %s, Subjects: %v, Keywords: %v\n",
				analysis.QueryType, analysis.Subjects, analysis.SearchKeywords)
		}
	}

	// 1. Initial retrieval sequence
	searchQuery := &schema.SearchQuery{
		Text:      query.Text,
		SessionID: query.SessionID,
		Limit:     query.Limit,
		HopDepth:  query.HopDepth,
		Analysis:  analysis,
		FourTier:  query.FourTier,
	}
	results, err := e.retrieveContext(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	// Attach analysis to result
	finalResult, err := e.dispatchThink(ctx, provider, query, results)
	if err != nil {
		return nil, err
	}
	finalResult.Analysis = analysis
	return finalResult, nil
}

func (e *defaultMemoryEngine) dispatchThink(ctx context.Context, provider extractor.LLMProvider, query *schema.ThinkQuery, results *schema.SearchResults) (*schema.ThinkResult, error) {
	if !query.EnableThinking {
		return e.singleShotThink(ctx, provider, query, results)
	}
	return e.iterativeThink(ctx, provider, query, results)
}

func (e *defaultMemoryEngine) singleShotThink(ctx context.Context, provider extractor.LLMProvider, query *schema.ThinkQuery, results *schema.SearchResults) (*schema.ThinkResult, error) {
	// If context is too large and segmentation is enabled, process in chunks
	if query.SegmentContext && query.MaxContextLength > 0 && len(results.ParsedContext) > query.MaxContextLength {
		return e.segmentedThinkStep(ctx, provider, query, results, results.ParsedContext)
	}

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

	// Fetch history buffer
	historyBuffer := e.getHistoryBuffer(ctx, query.SessionID, 10)

	prompt := fmt.Sprintf(`You are an AI assistant powered by a Memory Engine.
Use the following retrieved context (Vector Memories and Knowledge Graph relationships) and recent conversation history to answer the user's question accurately.
If the answer is not contained in the context or history, say "Mảnh ký ức này chưa tồn tại trong hệ thống." or answer based on available knowledge, but explicitly state what is missing.

You MUST respond in clean JSON format matching this schema:
%s

Recent Conversation History:
%s

Context:
%s

Question: %s
JSON Response:`, jsonFormatRequirement, historyBuffer, results.ParsedContext, query.Text)

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
		// Fetch history buffer
		historyBuffer := e.getHistoryBuffer(ctx, query.SessionID, 10)

		// Check if current context is too large and needs segmentation
		currentStepContext := currentContext
		if query.SegmentContext && query.MaxContextLength > 0 && len(currentContext) > query.MaxContextLength {
			segmentedResult, err := e.segmentedThinkIteration(ctx, provider, query, currentContext)
			if err == nil {
				// If segmented analysis found an answer, return it
				if segmentedResult.Answer != "" {
					segmentedResult.ContextUsed = initialResults
					return segmentedResult, nil
				}
				// Otherwise, use the cumulative reasoning as the "context" for the next hop request
				currentStepContext = "ANALYSIS SO FAR:\n" + segmentedResult.Reasoning
			}
		}

		prompt := fmt.Sprintf(`You are an AI assistant powered by a Memory Engine.
Use the following retrieved context and recent conversation history to answer the user's question accurately.
If the answer is missing from the context and history, identify the exact entities (names of people, organizations, concepts) that you need more information about to answer the question, and place them in 'missing_entities'.

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

Recent Conversation History:
%s

Context:
%s

Question: %s
JSON Response:`, jsonFormatRequirement, historyBuffer, currentStepContext, query.Text)

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

func (e *defaultMemoryEngine) segmentedThinkStep(ctx context.Context, provider extractor.LLMProvider, query *schema.ThinkQuery, results *schema.SearchResults, fullContext string) (*schema.ThinkResult, error) {
	fmt.Printf(" [Engine] Starting segmented analysis (%d bytes total)...\n", len(fullContext))
	segments := e.splitContextIntoSegmentsV2(fullContext, query.MaxContextLength)

	cumulativeReasoning := ""
	var lastResult schema.ThinkResult

	for i, segment := range segments {
		fmt.Printf(" [Engine] Processing context segment %d/%d (%d bytes)...\n", i+1, len(segments), len(segment))

		prompt := fmt.Sprintf(`You are an AI assistant performing a phased memory analysis.
Below is segment %d/%d of the retrieved context. 
Your task is to integrate this new information into your ongoing reasoning for the user's question.

Previous Analysis & Facts Found:
%s

New Context Segment:
%s

User Question: %s

Please provide your updated step-by-step reasoning and (if confident) the final answer.
JSON format requirement:
{
  "reasoning": "your integrated analysis including this segment",
  "answer": "your answer so far (only provide if you have reached a final conclusion)"
}`, i+1, len(segments), cumulativeReasoning, segment, query.Text)
		
		fmt.Printf(" [DEBUG LLM Prompt Segment %d/%d]:\n%s\n", i+1, len(segments), prompt)

		responseStr, err := provider.GenerateCompletion(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("segmented analysis failed at segment %d: %w", i+1, err)
		}

		responseStr = e.cleanJSONResponse(responseStr)

		var stepResult struct {
			Reasoning string `json:"reasoning"`
			Answer    string `json:"answer"`
		}
		_ = json.Unmarshal([]byte(responseStr), &stepResult)

		if stepResult.Reasoning != "" {
			cumulativeReasoning = stepResult.Reasoning
		}
		if stepResult.Answer != "" {
			lastResult.Answer = stepResult.Answer
			// We can potentially break early if we found a final answer, 
			// but for memory consistency, we might want to scan all segments.
			// For now, let's continue to ensure full context is processed unless it's a perfectly crisp answer.
		}
	}

	lastResult.Reasoning = cumulativeReasoning
	if lastResult.Answer == "" {
		lastResult.Answer = "Mảnh ký ức này không đủ để đưa ra kết luận sau khi phân tích toàn bộ các đoạn."
	}
	lastResult.ContextUsed = results
	return &lastResult, nil
}

func (e *defaultMemoryEngine) segmentedThinkIteration(ctx context.Context, provider extractor.LLMProvider, query *schema.ThinkQuery, fullContext string) (*schema.ThinkResult, error) {
	fmt.Printf(" [Engine] Starting segmented iterative analysis...\n")
	segments := e.splitContextIntoSegmentsV2(fullContext, query.MaxContextLength)

	cumulativeReasoning := ""
	var finalAnswer string
	var aggregatedMissingEntities []string

	for i, segment := range segments {
		fmt.Printf(" [Engine] Iterative segment %d/%d...\n", i+1, len(segments))
		prompt := fmt.Sprintf(`You are an AI agent analyzing context in segments to answer a complex question.
Analyze the information in this segment (%d/%d) and integrate it with your previous findings.

Previous Findings & Accumulated Knowledge:
%s

Current Context Segment:
%s

User Question: %s

Respond in JSON:
{
  "reasoning": "your updated step-by-step reasoning (maintain all key facts found so far)",
  "missing_entities": ["entities mentioned in this segment that you want to explore further in the knowledge graph"],
  "answer": "final answer if conclusion reached"
}`, i+1, len(segments), cumulativeReasoning, segment, query.Text)

		fmt.Printf(" [DEBUG LLM Segment %d/%d]:\n%s\n", i+1, len(segments), prompt)

		responseStr, err := provider.GenerateCompletion(ctx, prompt)
		if err != nil {
			return nil, err
		}

		responseStr = e.cleanJSONResponse(responseStr)
		var stepResult struct {
			Reasoning       string   `json:"reasoning"`
			MissingEntities []string `json:"missing_entities"`
			Answer          string   `json:"answer"`
		}
		_ = json.Unmarshal([]byte(responseStr), &stepResult)

		if stepResult.Reasoning != "" {
			cumulativeReasoning = stepResult.Reasoning
		}
		if stepResult.Answer != "" {
			finalAnswer = stepResult.Answer
		}
		if len(stepResult.MissingEntities) > 0 {
			aggregatedMissingEntities = append(aggregatedMissingEntities, stepResult.MissingEntities...)
		}
	}

	return &schema.ThinkResult{
		Reasoning:       cumulativeReasoning,
		Answer:          finalAnswer,
		MissingEntities: aggregatedMissingEntities,
	}, nil
}

