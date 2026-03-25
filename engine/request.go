package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/google/uuid"
)

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
