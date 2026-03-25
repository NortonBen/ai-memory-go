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

	// 1. Save user's message to history first (to maintain chronological order)
	userMsg := schema.Message{
		ID:        uuid.New().String(),
		Role:      schema.RoleUser,
		Content:   content,
		Timestamp: time.Now(),
	}
	if e.store != nil && sessionID != "" {
		_ = e.store.AddMessageToSession(ctx, sessionID, userMsg)
	}

	// 2. Fetch recent history Context
	historyContext := e.getHistoryBuffer(ctx, sessionID, 10)

	extractionInput := content
	if historyContext != "" {
		extractionInput = fmt.Sprintf("HISTORY:\n%s\n\nCURRENT USER MESSAGE:\n%s", historyContext, content)
	}

	// 2. Determine Intent & Extract Relationships
	intent, err := e.extractor.ExtractRequestIntent(ctx, extractionInput)
	if err != nil {
		return nil, fmt.Errorf("failed to extract intent: %w", err)
	}

	var finalAnswer string
	var finalReasoning []string

	// 3. Process Relationships (Bot-User, User-User)
	if len(intent.Relationships) > 0 && e.graphStore != nil {
		e.processRelationships(ctx, sessionID, intent.Relationships)
		finalReasoning = append(finalReasoning, fmt.Sprintf("Analyzed %d relationships from context.", len(intent.Relationships)))
	}

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
			finalReasoning = append(finalReasoning, "Updated knowledge graph with facts.")
		}
	}

	if intent.NeedsVectorStorage || (len(intent.Relationships) > 0) {
		addOpts := []AddOption{WithSessionID(sessionID), WithMetadata(options.Metadata)}
		dp, err := e.Add(ctx, content, addOpts...)
		if err == nil {
			_, _ = e.Cognify(ctx, dp, WithWaitCognify(false))
			finalReasoning = append(finalReasoning, "Added raw memory/context to vector store.")
		}
	}

	// 6. Handle Query / Generation
	var result *schema.ThinkResult
	if intent.IsQuery {
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

	// 7. Save Assistant's Response & Default answers
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

	// 8. Save Assistant's Response to history
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

func (e *defaultMemoryEngine) processRelationships(ctx context.Context, sessionID string, relationships []schema.RelationshipInfo) {
	for _, rel := range relationships {
		// Try to find if nodes already exist for these names in this session
		fromID := e.findOrCreateEntityNode(ctx, sessionID, rel.From)
		toID := e.findOrCreateEntityNode(ctx, sessionID, rel.To)
		
		// Create the relationship
		edge := schema.NewEdge(fromID, toID, rel.Type, 1.0)
		edge.SessionID = sessionID
		_ = e.graphStore.CreateRelationship(ctx, edge)
	}
}

func (e *defaultMemoryEngine) findOrCreateEntityNode(ctx context.Context, sessionID, name string) string {
	if name == "" {
		return ""
	}
	
	// SEARCH by name or entity in graph
	// Try 'name' first
	nodes, err := e.graphStore.FindNodesByProperty(ctx, "name", name)
	if err != nil || len(nodes) == 0 {
		// Try 'entity' property (used by some extractors)
		nodes, err = e.graphStore.FindNodesByProperty(ctx, "entity", name)
	}

	if err == nil && len(nodes) > 0 {
		// Check if any node is from this session or a global node
		for _, n := range nodes {
			if n.SessionID == sessionID || n.SessionID == "" {
				return n.ID
			}
		}
	}
	
	// CREATE new node if not found
	node := schema.NewNode(schema.NodeTypeEntity, map[string]interface{}{"name": name, "entity": name})
	node.SessionID = sessionID
	_ = e.graphStore.StoreNode(ctx, node)
	
	// Ensure the new node is immediately searchable via vector
	if e.vectorStore != nil && e.embedder != nil {
		dp := &schema.DataPoint{
			ID:          node.ID,
			Content:     name,
			ContentType: string(schema.NodeTypeEntity),
			SessionID:   sessionID,
		}
		_, _ = e.Cognify(ctx, dp, WithWaitCognify(false))
	}

	return node.ID
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
